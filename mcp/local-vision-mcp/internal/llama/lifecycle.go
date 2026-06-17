// Package llama wraps a local llama.cpp subprocess: lifecycle management,
// binary discovery/download, OpenAI-compatible HTTP client, and integrity
// verification.
//
// This file defines the public interface of the package. Implementations live
// in lifecycle.go's siblings (subprocess.go, client.go, binary.go, health.go).
package llama

import (
	"context"
	"errors"
)

// State is the lifecycle state of the managed subprocess.
type State int

const (
	StateStopped State = iota
	StateLoading
	StateReady
	StateCrashed
)

// String returns a human-readable state name for logs and the doctor command.
func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateLoading:
		return "loading"
	case StateReady:
		return "ready"
	case StateCrashed:
		return "crashed"
	default:
		return "unknown"
	}
}

// ErrNotImplemented is returned by stub functions during the contract phase.
var ErrNotImplemented = errors.New("not implemented")

// ErrIntegrityFail is returned when a model file fails SHA256 verification.
var ErrIntegrityFail = errors.New("model integrity check failed")

// ErrSpawnTimeout is returned when the subprocess does not become healthy
// within the configured startup window.
var ErrSpawnTimeout = errors.New("subprocess failed to become healthy in time")

// LifecycleManager owns a single llama-server subprocess and serializes
// load/unload transitions. At most one model is loaded at a time.
//
// Implementations must be safe for concurrent use: multiple goroutines may
// call Acquire simultaneously, and the manager must serialize model switches
// without deadlocking. The active-inference refcount prevents the idle timer
// from killing the process mid-request.
type LifecycleManager struct {
	// unexported fields added by Track C
}

// New creates a LifecycleManager. The actual implementation may take config
// (idle timeout, port range, binary path, etc.) via an options struct.
// Track C fills this in.
func New() (*LifecycleManager, error) {
	return nil, ErrNotImplemented
}

// Acquire ensures the given model is loaded and returns a Client that can
// serve requests for it.
//
// Concurrent calls block on an internal mutex until the right model is
// loaded. If a different model is currently loaded, it is unloaded first.
// The returned release function MUST be called when the caller is done; it
// decrements the active-inference refcount. When refcount reaches zero, the
// idle timer becomes eligible to fire.
//
// ctx cancellation is propagated: if the ctx is cancelled while waiting for
// a model to load, Acquire returns ctx.Err().
func (m *LifecycleManager) Acquire(ctx context.Context, modelID string) (c *Client, release func(), err error) {
	return nil, nil, ErrNotImplemented
}

// Shutdown gracefully stops the active-inference watcher, sends SIGTERM to
// the subprocess, waits up to the configured timeout, escalates to SIGKILL
// if still alive, and returns.
//
// Safe to call multiple times; subsequent calls are no-ops.
func (m *LifecycleManager) Shutdown(ctx context.Context) error {
	return ErrNotImplemented
}

// State returns the current lifecycle state for observability/diagnostics.
// Used by the doctor command.
func (m *LifecycleManager) State() State {
	return StateStopped
}

// Client is a thin HTTP client that talks to one running llama-server
// subprocess. A Client is only valid between the Acquire that returned it
// and the corresponding release call.
type Client struct {
	// unexported fields added by Track C
}

// ChatRequest is a single vision-language chat completion request.
type ChatRequest struct {
	Model        string   // model ID as known to the registry (informational)
	SystemPrompt string   // task-tuned system prompt from the tool
	UserPrompt   string   // user-turn prompt from the tool
	ImagePaths   []string // absolute local paths to image files
	MaxTokens    int      // per-tool output budget
	Temperature  float64  // usually 0.1 for deterministic output
}

// ChatResponse holds the model's reply plus accounting fields.
type ChatResponse struct {
	Content   string // raw text from the model
	TokensIn  int    // prompt tokens (incl. image tokens) reported by server
	TokensOut int    // generated tokens reported by server
	ElapsedMs int64  // wall-clock inference time
}

// ChatVision sends one chat completion with one or more images to the
// llama-server backing this Client.
//
// ctx is propagated to the underlying HTTP request for cancellation. On
// transient connection errors (network reset, 503), ChatVision retries
// once after 500ms. On 4xx responses it returns the response body as an
// error without retry.
func (c *Client) ChatVision(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return nil, ErrNotImplemented
}
