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
	"strings"

	"github.com/froggeric/llm/mcp/localvision/internal/config"
	"github.com/froggeric/llm/mcp/localvision/internal/llama"
	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/froggeric/llm/mcp/localvision/internal/progress"
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
	// overrideModel, when non-empty, forces a specific model ID for EVERY tool
	// (--model flag only). It bypasses per-tool routing. The MCP server path
	// leaves it empty so the per-tool selection applies.
	overrideModel string
	// defaultModel is the config's default_model, used as a FALLBACK when a tool
	// has no per-tool route and no fitting catalog model (rare). It does NOT
	// force a single model — per-tool routing (catalog preferred_for + [tools.*])
	// takes precedence. v0.7.
	defaultModel string
	// safetyMarginGB is the config's safety_margin_gb, threaded into catalog
	// selection (DefaultModel/ModelFor/Fits) so the user's knob actually
	// reaches selection. <=0 → catalog default (4 GB). v0.7.
	safetyMarginGB float64
	// sampleReps, when > 1, opts union-mode tools into multi-sampling: N warm
	// calls at the tool's sampling temperature, fused by a merge pass (F5).
	// Single-mode tools and reps<=1 do one call at 0.1 (today's behavior).
	// Default 0/1 = off.
	sampleReps int
	// toolConfigs holds optional per-tool model + method (sampling) overrides
	// from the user config ([tools.<id>]). Empty/missing = catalog per-tool
	// routing + sampling off. v0.7.
	toolConfigs map[string]config.ToolConfig
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

// SetSampleReps opts union-mode tools into multi-sampling (F5): when reps > 1,
// Run does N warm calls at the tool's sampling temperature and fuses them via a
// merge pass. Single-mode tools (and reps <= 1) do one call at 0.1. Default 0
// = off. Recipe per tool: see tools.SamplingFor.
func (e *CatalogExecutor) SetSampleReps(reps int) { e.sampleReps = reps }

// SetToolConfig installs per-tool model + method (sampling) overrides from the
// user config ([tools.<id>]). Used by both call sites (CLI + MCP server). A nil
// map clears it. v0.7.
func (e *CatalogExecutor) SetToolConfig(tc map[string]config.ToolConfig) { e.toolConfigs = tc }

// SetDefaultModel sets the config's default_model, used as a fallback when a
// tool has no per-tool route and no fitting catalog model. It does NOT force a
// single model (per-tool routing takes precedence). v0.7.
func (e *CatalogExecutor) SetDefaultModel(id string) { e.defaultModel = id }

// SetSafetyMarginGB threads the config's safety_margin_gb into catalog model
// selection (DefaultModel/ModelFor/Fits). <=0 keeps the catalog default (4 GB).
func (e *CatalogExecutor) SetSafetyMarginGB(margin float64) { e.safetyMarginGB = margin }

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

	// Step 1: pick the model. Precedence (highest wins):
	//   --model flag  >  [tools.<id>].model  >  catalog per-tool selection  >  default_model fallback
	// --model forces one model for every tool (backward compat). default_model is
	// a FALLBACK only (it does not defeat per-tool routing). v0.7.
	modelID := e.overrideModel
	source := "override(--model)"
	if modelID == "" {
		if tc, ok := e.toolConfigs[toolID]; ok && tc.Model != "" {
			modelID = tc.Model
			source = "config([tools.<id>])"
		} else if m, err := e.catalog.ModelFor(toolID, e.hardware, e.safetyMarginGB); err == nil {
			modelID = m
			source = "catalog"
		} else if e.defaultModel != "" {
			modelID = e.defaultModel
			source = "default_model"
		} else {
			return "", tools.Stats{}, fmt.Errorf("selecting model for tool %q: %w", toolID, err)
		}
	}
	if _, ok := e.catalog.Models[modelID]; !ok {
		return "", tools.Stats{}, fmt.Errorf("model %q (resolved for tool %q via %s) is not in the catalog", modelID, toolID, source)
	}
	if source != "catalog" && !e.catalog.Fits(modelID, e.hardware, e.safetyMarginGB) {
		e.logger.Warn("resolved model may not fit the detected hardware; proceeding anyway",
			"model_id", modelID, "tool_id", toolID, "source", source)
	}
	e.logger.Debug("selected model for tool", "tool_id", toolID, "model_id", modelID, "source", source)

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

	// Inference. Single call at 0.1 by default (deterministic). When the caller
	// opted into multi-sampling (e.sampleReps > 1) AND the tool's recipe is
	// union (tools.SamplingFor), run N warm calls at the tool's sampling temp
	// and fuse them via a merge pass — the union@N mechanism (F5; source:
	// benchmark/vlm/CATEGORY-REPORT.md). Single-mode tools ignore sampling
	// (their errors are systematic, so repeats can't help). Temperature is the
	// "gate" the benchmark names: at 0.1 the N runs come out ~identical and
	// correlation adds nothing, so the raised temp only applies when sampling.
	// Sampling resolution (F5). Off by default (single call at 0.1). Opt in via
	// --sample N (CLI) or [tools.<id>].method = "union@N" (config); --sample wins
	// when set. The tool's recipe (tools.SamplingFor) supplies the temperature
	// when sampling (0.7 coverage, 0.4 OCR); a single-mode tool sampled by an
	// explicit opt-in uses 0.7. Temperature is the "gate": at 0.1 the N runs are
	// ~identical and correlation adds nothing, so the raised temp only applies
	// when sampling.
	sampling := tools.SamplingFor(toolID)
	reps := 0
	if e.sampleReps > 1 {
		reps = e.sampleReps
	} else if tc, ok := e.toolConfigs[toolID]; ok {
		reps, _ = tools.ParseMethod(tc.Method)
	}
	temp := 0.1
	if reps > 1 {
		temp = sampling.Temp
		if sampling.Mode != tools.SamplingUnion {
			temp = 0.7 // explicit opt-in on a single-mode tool: a sane sampling temp
		}
	}

	req := llama.ChatRequest{
		Model:        modelID,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		ImagePaths:   imagePaths,
		MaxTokens:    maxTokens,
		Temperature:  f64(temp),
		// v6 benchmark sampling: light top-p/top-k pruning.
		TopP:               f64(0.95),
		TopK:               64,
		ChatTemplateKwargs: spec.ChatTemplateKwargs,
	}

	e.lifecycle.Phase(ctx, "inferring", modelID)

	// Heartbeat spans the whole inference (N samples + merge when sampling).
	budget := inferenceBudgetSec(maxTokens, spec.BenchToks)
	if reps > 1 {
		budget *= float64(reps) // rough: N samples + 1 merge
	}
	stopHeartbeat := progress.Heartbeat(ctx, progress.SinkFrom(ctx), "inferring", spec.DisplayName, budget)

	content, stats, inferErr := e.infer(ctx, client, req, reps, modelID, spec, maxTokens)
	stopHeartbeat()
	if inferErr != nil {
		return "", tools.Stats{}, fmt.Errorf("inference for model %q: %w", modelID, inferErr)
	}
	return content, stats, nil
}

// infer runs one ChatVision call (reps<=1) or N warm calls + a merge pass
// (reps>1) on the already-acquired warm client. It degrades gracefully: a
// failed sample stops the loop and uses what was collected; a failed merge
// returns the first sample. Stats aggregate across all calls made.
func (e *CatalogExecutor) infer(ctx context.Context, client *llama.Client, req llama.ChatRequest, reps int, modelID string, spec models.ModelSpec, maxTokens int) (string, tools.Stats, error) {
	if reps <= 1 {
		resp, err := client.ChatVision(ctx, req)
		if err != nil {
			return "", tools.Stats{}, err
		}
		e.logger.Debug("inference complete",
			"model_id", modelID, "tokens_in", resp.TokensIn, "tokens_out", resp.TokensOut, "elapsed_ms", resp.ElapsedMs)
		return resp.Content, tools.Stats{Model: modelID, TokensIn: resp.TokensIn, TokensOut: resp.TokensOut, ElapsedMs: resp.ElapsedMs}, nil
	}

	var samples []string
	var stats tools.Stats
	for i := 0; i < reps; i++ {
		resp, err := client.ChatVision(ctx, req)
		if err != nil {
			e.logger.Warn("sampling call failed; degrading to samples collected so far",
				"model", modelID, "sample", i+1, "err", err)
			break
		}
		samples = append(samples, resp.Content)
		stats.TokensIn += resp.TokensIn
		stats.TokensOut += resp.TokensOut
		stats.ElapsedMs += resp.ElapsedMs
	}
	stats.Model = modelID
	if len(samples) == 0 {
		return "", stats, errors.New("all sampling calls failed")
	}
	if len(samples) == 1 {
		return samples[0], stats, nil
	}
	merged, err := mergeSamples(ctx, client, modelID, spec, samples, maxTokens)
	if err != nil {
		e.logger.Warn("merge pass failed; returning first sample", "err", err)
		return samples[0], stats, nil
	}
	// Fold the merge call's token/time accounting into the running stats so the
	// --meta sidecar / MCP accounting reflects the real cost of the merge pass.
	stats.TokensIn += merged.TokensIn
	stats.TokensOut += merged.TokensOut
	stats.ElapsedMs += merged.ElapsedMs
	e.logger.Debug("multi-sample+merge complete",
		"model_id", modelID, "reps", len(samples), "tokens_out", stats.TokensOut, "elapsed_ms", stats.ElapsedMs)
	return merged.Content, stats, nil
}

// mergeSamples fuses N independent analyses of the same image into one
// comprehensive result via a text-only chat call on the same warm model (no
// image — the inputs already describe it, so this is cheap). Returns the full
// response so callers can fold the merge call's token/time accounting into the
// running stats. Callers fall back to the first sample on error.
func mergeSamples(ctx context.Context, client *llama.Client, modelID string, spec models.ModelSpec, samples []string, maxTokens int) (*llama.ChatResponse, error) {
	var joined strings.Builder
	for i, s := range samples {
		if i > 0 {
			fmt.Fprintf(&joined, "\n\n--- (independent sample %d) ---\n\n", i+1)
		}
		joined.WriteString(s)
	}
	req := llama.ChatRequest{
		Model:              modelID,
		SystemPrompt:       mergePrompt,
		UserPrompt:         joined.String(),
		MaxTokens:          maxTokens,
		Temperature:        f64(0.1), // deterministic merge
		TopP:               f64(0.95),
		TopK:               64,
		ChatTemplateKwargs: spec.ChatTemplateKwargs,
	}
	resp, err := client.ChatVision(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// mergePrompt instructs the model to fuse N independent analyses of one image
// into a single comprehensive, deduplicated result without inventing content.
const mergePrompt = `You are merging several independent analyses of the SAME image, each produced with slight variation so that together they capture more detail than any single one. Produce ONE comprehensive, coherent result that includes EVERY distinct detail mentioned in ANY of the inputs.
- Deduplicate repeated points.
- Where inputs disagree, keep the most specific accurate statement.
- Preserve the original section/heading structure.
- Do NOT invent details that are not present in the inputs; if something is uncertain, keep the uncertainty.
Output only the merged result — no preamble.`

// f64 returns a pointer to v, for the *float64 ChatRequest fields (Temperature,
// TopP). Lets call sites pass a literal value while keeping nil = "use the
// default" (0.1 / 0.95) distinguishable from an explicit 0.0 (greedy).
func f64(v float64) *float64 { return &v }

// inferenceBudgetSec returns a soft, UX-only estimate of how many seconds an
// inference will take, used as the Total for the inference progress heartbeat.
// It is deliberately rough (accuracy is secondary to "something is happening"):
// maxTokens / benchToks-per-second, clamped to [5, 180] s. A non-positive
// benchToks falls back to 60 s.
func inferenceBudgetSec(maxTokens int, benchToks float64) float64 {
	const (
		floor    = 5.0
		ceil     = 180.0
		fallback = 60.0
	)
	if benchToks <= 0 {
		return fallback
	}
	budget := float64(maxTokens) / benchToks
	if budget < floor {
		return floor
	}
	if budget > ceil {
		return ceil
	}
	return budget
}

// Compile-time check that CatalogExecutor satisfies tools.Executor.
var _ tools.Executor = (*CatalogExecutor)(nil)

// errExecutorUnavailable is returned when no executor is configured (e.g.
// the server is in first-run mode and Track C/D haven't fully landed).
var errExecutorUnavailable = errors.New("no executor configured; first-run setup required")
