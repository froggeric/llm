// Package mcpserver wires together the official MCP Go SDK
// (github.com/modelcontextprotocol/go-sdk) with the localvision
// tool registry, llama.cpp lifecycle, and model catalog.
//
// We do NOT hand-roll JSON-RPC (see F3.1). The SDK handles the protocol;
// this package only adds:
//
//   - tool registration: every tool from internal/tools is registered
//     against the SDK so it shows up in tools/list
//   - request routing: a single ToolHandler dispatches tools/call to the
//     right tools.Tool implementation
//   - image-input normalization: raw JSON args become tools.ToolInput
//     before the tool sees them
//   - the executor that runs the inference (catalog → lifecycle → client)
//   - graceful shutdown on SIGTERM/SIGINT
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/froggeric/llm/mcp/localvision/internal/llama"
	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/froggeric/llm/mcp/localvision/internal/tools"
)

// CatalogExecutor is the production tools.Executor: it picks a model via
// the catalog, loads it via the lifecycle manager, and runs a vision chat
// against llama-server.
//
// One CatalogExecutor is shared by all tool handlers in the server. It is
// safe for concurrent use because the underlying LifecycleManager is.
type CatalogExecutor struct {
	catalog   *models.Catalog
	lifecycle *llama.LifecycleManager
	hardware  models.HardwareInfo
	logger    *slog.Logger
	// overrideModel, when non-empty, forces a specific model ID instead of the
	// catalog's per-tool selection. Used by the CLI (--model / config
	// default_model). The MCP server path leaves it empty so the richer
	// per-tool selection applies.
	overrideModel string
}

// NewCatalogExecutor builds an executor wired to the given catalog and
// lifecycle manager. hardware is the detected HardwareInfo, used to choose
// between catalog entries that fit the machine.
//
// logger may be nil; a default is substituted.
func NewCatalogExecutor(catalog *models.Catalog, lifecycle *llama.LifecycleManager, hardware models.HardwareInfo, logger *slog.Logger) *CatalogExecutor {
	if logger == nil {
		logger = slog.Default()
	}
	return &CatalogExecutor{
		catalog:   catalog,
		lifecycle: lifecycle,
		hardware:  hardware,
		logger:    logger,
	}
}

// SetOverrideModel forces a specific model ID for subsequent Run calls instead
// of the catalog's per-tool selection. The ID must exist in the catalog; Run
// warns (but does not fail) if it may not fit the hardware. Empty clears it.
func (e *CatalogExecutor) SetOverrideModel(id string) { e.overrideModel = id }

// Run implements tools.Executor. It:
//  1. Picks the right model for the tool via catalog.ModelFor(toolID, hw)
//  2. Acquires a loaded-model client via lifecycle.Acquire(ctx, modelID)
//  3. Calls client.ChatVision(ctx, req)
//  4. Releases the client (decrements the active-inference refcount)
//
// ctx cancellation propagates end-to-end. If the client cancels
// mid-inference (notifications/cancelled), the SDK cancels ctx and
// ChatVision aborts the HTTP request to llama-server.
//
// Errors are surfaced as wrapped Go errors; the MCP handler translates
// them to structured MCP errors.
func (e *CatalogExecutor) Run(ctx context.Context, toolID, systemPrompt, userPrompt string, images []tools.ImageRef, maxTokens int) (string, tools.Stats, error) {
	if e.catalog == nil {
		return "", tools.Stats{}, fmt.Errorf("executor: catalog is nil; first-run setup required")
	}
	if e.lifecycle == nil {
		return "", tools.Stats{}, fmt.Errorf("executor: lifecycle manager is nil; first-run setup required")
	}

	// Step 1: pick the model. An explicit override (--model / config
	// default_model) wins; otherwise use the catalog's per-tool selection.
	modelID := e.overrideModel
	if modelID == "" {
		var err error
		modelID, err = e.catalog.ModelFor(toolID, e.hardware)
		if err != nil {
			return "", tools.Stats{}, fmt.Errorf("selecting model for tool %q: %w", toolID, err)
		}
	} else if _, ok := e.catalog.Models[modelID]; !ok {
		return "", tools.Stats{}, fmt.Errorf("override model %q is not in the catalog", modelID)
	} else if !e.catalog.Fits(modelID, e.hardware) {
		e.logger.Warn("override model may not fit the detected hardware; proceeding anyway",
			"model_id", modelID)
	}
	e.logger.Debug("selected model for tool", "tool_id", toolID, "model_id", modelID, "override", e.overrideModel != "")

	// Step 2: load (or reuse) the model. Acquire blocks on an internal
	// mutex so concurrent tool calls don't race on the subprocess.
	client, release, err := e.lifecycle.Acquire(ctx, modelID)
	if err != nil {
		return "", tools.Stats{}, fmt.Errorf("loading model %q: %w", modelID, err)
	}
	// release decrements the active-inference refcount. When refcount
	// reaches zero, the idle timer becomes eligible to fire (F1.9).
	defer func() {
		if release != nil {
			release()
		}
	}()

	// Step 3: build the request and call the model.
	imagePaths := make([]string, 0, len(images))
	for _, img := range images {
		if img.LocalPath != "" {
			imagePaths = append(imagePaths, img.LocalPath)
		}
	}

	// Look up the spec to forward any per-model chat_template_kwargs
	// (e.g. enable_thinking=false for Qwen3.5/3.6 hybrid thinkers — a
	// strict win for vision per our v6 benchmark).
	spec, ok := e.catalog.Models[modelID]
	if !ok {
		return "", tools.Stats{}, fmt.Errorf("executor: model %q not in catalog after selection", modelID)
	}

	req := llama.ChatRequest{
		Model:        modelID,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		ImagePaths:   imagePaths,
		MaxTokens:    maxTokens,
		// Tools favor determinism. The catalog/tool layer can override
		// per-tool if needed in a future revision; MVP uses 0.1.
		Temperature: 0.1,
		// v6 benchmark sampling: low temperature with light top-p/top-k pruning.
		TopP: 0.95,
		TopK: 64,
		// Forward any per-model chat_template_kwargs.
		ChatTemplateKwargs: spec.ChatTemplateKwargs,
	}

	e.lifecycle.Phase("inferring", modelID)

	resp, err := client.ChatVision(ctx, req)
	if err != nil {
		return "", tools.Stats{}, fmt.Errorf("inference for model %q: %w", modelID, err)
	}

	e.logger.Debug("inference complete",
		"model_id", modelID,
		"tokens_in", resp.TokensIn,
		"tokens_out", resp.TokensOut,
		"elapsed_ms", resp.ElapsedMs,
	)
	stats := tools.Stats{
		Model:     modelID,
		TokensIn:  resp.TokensIn,
		TokensOut: resp.TokensOut,
		ElapsedMs: resp.ElapsedMs,
	}
	return resp.Content, stats, nil
}

// Compile-time check that CatalogExecutor satisfies tools.Executor.
var _ tools.Executor = (*CatalogExecutor)(nil)

// errExecutorUnavailable is returned when no executor is configured (e.g.
// the server is in first-run mode and Track C/D haven't fully landed).
var errExecutorUnavailable = errors.New("no executor configured; first-run setup required")
