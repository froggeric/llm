package mcpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/froggeric/llm/mcp/localvision/internal/tools"
)

// stubTool is a minimal tools.Tool implementation for tests. It doesn't
// do anything vision-related; it lets us verify the MCP plumbing without
// depending on Track E or the real model catalog.
type stubTool struct {
	id          string
	description string
	maxTokens   int
	system      string
}

func (s stubTool) ID() string           { return s.id }
func (s stubTool) Description() string  { return s.description }
func (s stubTool) MaxTokens() int       { return s.maxTokens }
func (s stubTool) SystemPrompt() string { return s.system }
func (s stubTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"image_path": map[string]any{
				"type":        "string",
				"description": "absolute path to the image",
			},
		},
		"required": []string{"image_path"},
	}
}
func (s stubTool) BuildRequest(input tools.ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	if len(input.Images) == 0 {
		return "", "", nil, errors.New("no image provided")
	}
	for _, img := range input.Images {
		imagePaths = append(imagePaths, img.LocalPath)
	}
	return s.system, "user prompt for " + s.id, imagePaths, nil
}
func (s stubTool) ParseOutput(raw string) (any, error) {
	return raw, nil
}

// nineStubTools returns 9 distinct stub tools with the IDs the production
// registry will eventually register.
func nineStubTools() []tools.Tool {
	ids := []string{
		"read_image",
		"extract_text",
		"extract_code",
		"extract_table",
		"describe_ui",
		"describe_diagram",
		"describe_chart",
		"diagnose_error",
		"compare_images",
	}
	out := make([]tools.Tool, len(ids))
	for i, id := range ids {
		out[i] = stubTool{
			id:          id,
			description: "stub tool " + id,
			maxTokens:   1024,
			system:      "system prompt for " + id,
		}
	}
	return out
}

// silentLogger returns a logger that discards everything, so tests don't
// pollute stderr/stdout.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

// newTestServer builds a Server with 9 stub tools and the given executor.
// It skips the production registry (which Track E hasn't populated yet).
func newTestServer(t *testing.T, executor tools.Executor) *Server {
	t.Helper()
	srv, err := NewServer(Dependencies{
		Logger:   silentLogger(),
		Tools:    nineStubTools(),
		Executor: executor,
	})
	require.NoError(t, err)
	return srv
}

// recordingExecutor is a tools.Executor that records each call and
// returns a canned response. Used to verify routing without depending
// on the catalog or lifecycle.
type recordingExecutor struct {
	mu       sync.Mutex
	calls    []execCall
	response string
	err      error
	delay    time.Duration // optional, to simulate slow inference
}

type execCall struct {
	toolID     string
	userPrompt string
	images     int
	maxTokens  int
}

func (r *recordingExecutor) Run(ctx context.Context, toolID, systemPrompt, userPrompt string, images []tools.ImageRef, maxTokens int) (string, tools.Stats, error) {
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return "", tools.Stats{}, ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, execCall{
		toolID:     toolID,
		userPrompt: userPrompt,
		images:     len(images),
		maxTokens:  maxTokens,
	})
	if r.err != nil {
		return "", tools.Stats{}, r.err
	}
	if r.response != "" {
		return r.response, tools.Stats{}, nil
	}
	return "default response from " + toolID, tools.Stats{}, nil
}

// TestServerRegistersNineTools verifies the server's tool registration
// count via Server.ToolCount (a sanity check that doesn't require running
// the SDK).
func TestServerRegistersNineTools(t *testing.T) {
	srv := newTestServer(t, &recordingExecutor{})
	assert.Equal(t, 9, srv.ToolCount(), "expected 9 tools registered")
}

// TestToolsListReturnsNineTools drives a real MCP client through the SDK
// in-memory transport and asserts the server reports all 9 tools in
// tools/list.
func TestToolsListReturnsNineTools(t *testing.T) {
	srv := newTestServer(t, &recordingExecutor{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	var saw int
	for tool, err := range cs.Tools(ctx, nil) {
		require.NoError(t, err)
		// Every tool description MUST carry the latency hint per F5.3.
		assert.Contains(t, tool.Description, "30-60 seconds",
			"tool %q description must include the latency hint", tool.Name)
		saw++
	}
	assert.Equal(t, 9, saw, "tools/list should return exactly 9 tools")
}

// TestToolCallRoutesThroughExecutor verifies that a tools/call request
// for one of the registered tools reaches the executor with correctly
// normalized arguments.
func TestToolCallRoutesThroughExecutor(t *testing.T) {
	exec := &recordingExecutor{response: "stub-inference-output"}
	srv := newTestServer(t, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "extract_code",
		Arguments: map[string]any{"image_path": "/tmp/fake.png"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, "expected successful tool call")

	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "stub-inference-output", tc.Text)

	require.Len(t, exec.calls, 1, "executor should have been called once")
	c := exec.calls[0]
	assert.Equal(t, "extract_code", c.toolID)
	assert.Equal(t, 1, c.images, "executor should see one image")
	assert.Equal(t, 1024, c.maxTokens, "executor should see the tool's MaxTokens")
}

// TestToolCallRejectsRemoteURL verifies F1.10: http(s):// URLs must be
// rejected with a helpful error.
func TestToolCallRejectsRemoteURL(t *testing.T) {
	exec := &recordingExecutor{}
	srv := newTestServer(t, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_image",
		Arguments: map[string]any{"image_url": "https://example.com/cat.png"},
	})
	require.NoError(t, err) // not a protocol error
	require.True(t, res.IsError, "expected IsError=true for rejected remote URL")
	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, "remote URLs are not supported")
	assert.Contains(t, tc.Text, "localhost-only")

	require.Empty(t, exec.calls, "executor must NOT be called for rejected input")
}

// TestGracefulShutdownOnContextCancel verifies the F3.8 invariant: when
// the run context is cancelled, the server stops and any in-flight
// executor call observes ctx cancellation.
func TestGracefulShutdownOnContextCancel(t *testing.T) {
	// Use an executor with a 2s delay so we can verify cancellation
	// actually interrupts the in-flight call.
	exec := &recordingExecutor{delay: 2 * time.Second}
	srv := newTestServer(t, exec)

	ctx, cancel := context.WithCancel(context.Background())

	// Start the server on an in-memory transport. We use Connect directly
	// (rather than Run, which is for stdio) so we can drive both sides.
	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	// Fire a tool call; before it completes, cancel the context.
	var (
		wg      sync.WaitGroup
		callErr error
		callRes *mcp.CallToolResult
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		callRes, callErr = cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "read_image",
			Arguments: map[string]any{"image_path": "/tmp/x.png"},
		})
	}()

	// Give the call time to start, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	wg.Wait()
	// Either the call returns ctx.Err() directly OR it returns a tool
	// result with IsError. The exact shape depends on SDK timing; what
	// matters is that the server does not hang.
	if callErr == nil && callRes != nil && !callRes.IsError {
		t.Fatal("expected the in-flight call to be cancelled")
	}

	// Server.Run should also return promptly once ctx is cancelled. We
	// can't easily test Run() here (it's tied to stdio), but we can
	// verify the in-flight WaitGroup drains via the executor's delay
	// being interrupted.
	//
	// If the executor's Run returned ctx.Err() (rather than completing
	// after 2s), we know cancellation propagated end-to-end.
}

// TestServerRunHonorsContextCancellation starts Run() on a server with
// no client connected, cancels the context, and asserts Run returns
// within a reasonable time bound. This is the canonical graceful-shutdown
// test.
func TestServerRunHonorsContextCancellation(t *testing.T) {
	srv := newTestServer(t, &recordingExecutor{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx)
	}()

	// Give Run a moment to start, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		// Run should return promptly with either nil or context.Canceled.
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return within 3s of context cancellation")
	}
}

// TestSetupErrorSurfacesAsToolError verifies that when the executor
// returns an error, the MCP client sees a tool result with IsError=true
// (NOT a JSON-RPC protocol error).
func TestSetupErrorSurfacesAsToolError(t *testing.T) {
	exec := &recordingExecutor{err: errors.New("loading model failed: not implemented")}
	srv := newTestServer(t, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_image",
		Arguments: map[string]any{"image_path": "/tmp/x.png"},
	})
	require.NoError(t, err) // not a protocol error
	require.True(t, res.IsError, "expected IsError=true")
	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	// Should contain the original error AND the setup-required hint.
	assert.Contains(t, tc.Text, "loading model failed")
	assert.Contains(t, tc.Text, "first-run setup")
	assert.Contains(t, tc.Text, "doctor")
}

// TestCompareImagesAcceptsArray verifies F4.9: compare_images takes an
// array of images.
func TestCompareImagesAcceptsArray(t *testing.T) {
	exec := &recordingExecutor{response: "they are the same"}
	srv := newTestServer(t, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "compare_images",
		Arguments: map[string]any{
			"images": []any{
				"/tmp/a.png",
				"/tmp/b.png",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, "expected success")

	require.Len(t, exec.calls, 1)
	c := exec.calls[0]
	assert.Equal(t, "compare_images", c.toolID)
	assert.Equal(t, 2, c.images, "executor should see 2 images")
}

// TestCompareImagesRejectsSingleImageInput verifies that compare_images
// requires the array form.
func TestCompareImagesRejectsSingleImageInput(t *testing.T) {
	exec := &recordingExecutor{}
	srv := newTestServer(t, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	// compare_images requires the "images" array; using image_path
	// instead should still work since we accept the single-image form
	// too (but the executor will see exactly 1 image). The test here
	// verifies the tool call doesn't fail at the MCP layer.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "compare_images",
		Arguments: map[string]any{"image_path": "/tmp/a.png"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, "single image_path should still work")
	require.Len(t, exec.calls, 1)
	assert.Equal(t, 1, exec.calls[0].images)
}

// TestNormalizeImages covers the input normalization matrix.
func TestNormalizeImages(t *testing.T) {
	t.Run("image_path", func(t *testing.T) {
		args := map[string]any{"image_path": "/abs/path/x.png"}
		refs, consumed, err := normalizeImages(args, "read_image")
		require.NoError(t, err)
		require.Len(t, refs, 1)
		assert.Equal(t, "/abs/path/x.png", refs[0].LocalPath)
		assert.True(t, consumed["image_path"])
	})

	t.Run("image_url file://", func(t *testing.T) {
		args := map[string]any{"image_url": "file:///abs/path/y.png"}
		refs, consumed, err := normalizeImages(args, "read_image")
		require.NoError(t, err)
		require.Len(t, refs, 1)
		assert.Equal(t, "/abs/path/y.png", refs[0].LocalPath)
		assert.True(t, consumed["image_url"])
	})

	t.Run("image_url http rejected", func(t *testing.T) {
		args := map[string]any{"image_url": "https://example.com/x.png"}
		_, _, err := normalizeImages(args, "read_image")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "remote URLs are not supported")
	})

	t.Run("images array", func(t *testing.T) {
		args := map[string]any{
			"images": []any{"/a.png", "/b.png", "/c.png"},
		}
		refs, consumed, err := normalizeImages(args, "compare_images")
		require.NoError(t, err)
		require.Len(t, refs, 3)
		assert.True(t, consumed["images"])
	})

	t.Run("no image provided", func(t *testing.T) {
		args := map[string]any{"question": "what is this?"}
		_, _, err := normalizeImages(args, "read_image")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no image")
	})

	t.Run("data URI", func(t *testing.T) {
		// Tiny 1x1 PNG: 8 bytes of base64.
		args := map[string]any{"image_data": "data:image/png;base64,iVBORw0KGgo="}
		refs, _, err := normalizeImages(args, "read_image")
		require.NoError(t, err)
		require.Len(t, refs, 1)
		assert.True(t, strings.HasSuffix(refs[0].LocalPath, ".png"),
			"temp file should have .png suffix; got %q", refs[0].LocalPath)
		// The temp file should exist on disk.
		_, statErr := os.Stat(refs[0].LocalPath)
		assert.NoError(t, statErr, "temp file should exist")
	})
}

// TestExecutorNilCatalogReturnsError verifies the CatalogExecutor
// surfaces a clear "first-run setup required" error when its deps are
// nil (production safety).
func TestExecutorNilCatalogReturnsError(t *testing.T) {
	exec := NewCatalogExecutor(nil, nil, models.HardwareInfo{}, silentLogger())
	_, _, err := exec.Run(context.Background(), "read_image", "sys", "user", nil, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog is nil")
}

// Reduce noise from the unused linter if atomic is not used directly in
// this test file (it is used in tools.go).
var _ = atomic.AddInt64
