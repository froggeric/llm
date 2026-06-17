// Package mcpserver wires together the official MCP Go SDK
// (github.com/modelcontextprotocol/go-sdk) with the local-vision-mcp
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

	"github.com/froggeric/llm/mcp/local-vision-mcp/internal/llama"
	"github.com/froggeric/llm/mcp/local-vision-mcp/internal/models"
	"github.com/froggeric/llm/mcp/local-vision-mcp/internal/tools"
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
func (e *CatalogExecutor) Run(ctx context.Context, toolID, systemPrompt, userPrompt string, images []tools.ImageRef, maxTokens int) (string, error) {
	if e.catalog == nil {
		return "", fmt.Errorf("executor: catalog is nil; first-run setup required")
	}
	if e.lifecycle == nil {
		return "", fmt.Errorf("executor: lifecycle manager is nil; first-run setup required")
	}

	// Step 1: pick the model. Returns ErrNoFittingModel if hardware can't
	// fit anything; we surface that as a structured MCP error.
	modelID, err := e.catalog.ModelFor(toolID, e.hardware)
	if err != nil {
		return "", fmt.Errorf("selecting model for tool %q: %w", toolID, err)
	}
	e.logger.Debug("selected model for tool", "tool_id", toolID, "model_id", modelID)

	// Step 2: load (or reuse) the model. Acquire blocks on an internal
	// mutex so concurrent tool calls don't race on the subprocess.
	client, release, err := e.lifecycle.Acquire(ctx, modelID)
	if err != nil {
		return "", fmt.Errorf("loading model %q: %w", modelID, err)
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

	req := llama.ChatRequest{
		Model:        modelID,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		ImagePaths:   imagePaths,
		MaxTokens:    maxTokens,
		// Tools favor determinism. The catalog/tool layer can override
		// per-tool if needed in a future revision; MVP uses 0.1.
		Temperature: 0.1,
	}

	resp, err := client.ChatVision(ctx, req)
	if err != nil {
		return "", fmt.Errorf("inference for model %q: %w", modelID, err)
	}

	e.logger.Debug("inference complete",
		"model_id", modelID,
		"tokens_in", resp.TokensIn,
		"tokens_out", resp.TokensOut,
		"elapsed_ms", resp.ElapsedMs,
	)
	return resp.Content, nil
}

// Compile-time check that CatalogExecutor satisfies tools.Executor.
var _ tools.Executor = (*CatalogExecutor)(nil)

// errExecutorUnavailable is returned when no executor is configured (e.g.
// the server is in first-run mode and Track C/D haven't fully landed).
var errExecutorUnavailable = errors.New("no executor configured; first-run setup required")
