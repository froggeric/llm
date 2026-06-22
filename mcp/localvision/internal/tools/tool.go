// Package tools defines the 9 vision tools the MCP exposes to clients.
//
// Each tool wraps a task-tuned system prompt + an output budget and is
// paired with a model selection in the catalog's PreferredFor field.
//
// This file defines the public interface and shared types. Per-tool
// implementations live in read_image.go, extract_text.go, etc.
package tools

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by stub functions during the contract phase.
var ErrNotImplemented = errors.New("not implemented")

// Tool is the interface every MCP tool implements.
//
// Tools are stateless: the same input always produces the same prompt and
// the same output parsing. State (loaded model, subprocess lifecycle) lives
// in the llama.LifecycleManager and is passed in via Executor.
type Tool interface {
	// ID is the tool identifier surfaced to MCP clients (e.g. "extract_code").
	// Must be lowercase, snake_case, unique within this MCP.
	ID() string

	// Description is the human-readable description sent to MCP clients in
	// tools/list. Should include the use case and an expected-latency hint
	// (e.g. "Takes 30-60 seconds per call").
	Description() string

	// InputSchema returns the JSON Schema for the tool's input arguments.
	// Sent verbatim to MCP clients.
	InputSchema() map[string]any

	// MaxTokens is the per-call output budget for this tool. Tune per tool:
	// extract_code allows 4096, describe_chart allows 1024, etc.
	MaxTokens() int

	// SystemPrompt is the task-tuned system prompt sent to the model.
	// Different per tool — this is the entire reason to have 9 tools instead
	// of one generic "describe image".
	SystemPrompt() string

	// BuildRequest turns a parsed input into a llama.ChatRequest ready to
	// send to the lifecycle manager.
	BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error)

	// ParseOutput post-processes the model's raw text into the final
	// response. Most tools just pass through; some (extract_table) convert
	// to Markdown tables, extract_code strips preamble, etc.
	ParseOutput(raw string) (any, error)
}

// ToolInput is the parsed arguments from an MCP tools/call request.
//
// ImageRef normalization (path / data: URI / file:// URI) happens in
// imageref.go before the tool sees it. Tools always receive at least one
// LocalPath; remote URLs are rejected upstream with a helpful error.
type ToolInput struct {
	Images []ImageRef
	Extra  map[string]any // tool-specific arguments
}

// ImageRef is a normalized reference to a local image file.
//
// All three input formats accepted by the MCP collapse into ImageRef:
//   - file path: LocalPath = the path as given
//   - data:image/...;base64,...: bytes decoded, written to a temp file,
//     LocalPath = temp file path. Temp file is deleted by the executor after
//     the request completes.
//   - file:// URI: parsed, LocalPath = the path component
//
// Remote http(s):// URLs are rejected in imageref.go because llama-server
// is bound to 127.0.0.1 and cannot fetch remote images.
type ImageRef struct {
	LocalPath string // always populated; absolute
	Source    string // original input (for diagnostics)
}

// Executor is the capability a tool needs to do its work. The MCP server
// passes a concrete Executor into each tool call; tools do not own
// lifecycle state.
//
// The interface mirrors llama.LifecycleManager.Acquire + Client.ChatVision
// so tools don't import the llama package directly (avoids an import cycle
// risk if Tool is used in selection logic).
type Executor interface {
	// Run loads the right model for this tool (via the catalog + lifecycle
	// manager) and returns the model's response plus per-inference telemetry
	// (token counts, wall-clock). ctx propagates to the underlying HTTP
	// request for cancellation.
	Run(ctx context.Context, toolID, systemPrompt, userPrompt string, images []ImageRef, maxTokens int) (raw string, stats Stats, err error)
}

// Stats carries per-inference telemetry surfaced by the executor. The MCP
// server ignores it; the CLI uses it for the --meta sidecar in batch mode.
type Stats struct {
	Model     string // the catalog ID that actually ran (override or selection)
	TokensIn  int    // prompt + image tokens consumed
	TokensOut int    // completion tokens generated
	ElapsedMs int64  // wall-clock inference time (model + transport)
}

// Registry holds the 9 tool implementations indexed by ID.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry returns a Registry populated with all 9 tools. Track E
// implements this in registry.go.
//
// The tools are registered in deterministic alphabetical order by ID so
// All() returns them in a stable sequence — Track B's server preserves
// whatever order we return, and stable tools/list output is friendlier to
// clients that diff snapshots.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	for _, t := range allTools() {
		r.Register(t)
	}
	return r
}

// Get returns the tool with the given ID, or false if not found.
func (r *Registry) Get(id string) (Tool, bool) {
	t, ok := r.tools[id]
	return t, ok
}

// All returns every registered tool, sorted by ID for deterministic ordering
// in tools/list responses.
func (r *Registry) All() []Tool {
	if r == nil {
		return nil
	}
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	// Deterministic order: sort by ID. Simple insertion sort avoids importing
	// sort just for this small slice; the catalog has 9 entries.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].ID() > out[j].ID(); j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// Register adds a tool to the registry. Panics on duplicate IDs (programming
// error, not user-facing).
func (r *Registry) Register(t Tool) {
	if t == nil {
		panic("tools: Register called with nil Tool")
	}
	id := t.ID()
	if id == "" {
		panic("tools: Register called with Tool whose ID() is empty")
	}
	if _, dup := r.tools[id]; dup {
		panic("tools: duplicate tool ID " + id)
	}
	r.tools[id] = t
}
