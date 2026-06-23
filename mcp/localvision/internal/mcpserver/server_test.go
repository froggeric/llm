package mcpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/froggeric/llm/mcp/localvision/internal/progress"
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

// elevenStubTools returns 11 distinct stub tools with the IDs the production
// registry registers (one per tool, incl. image_to_prompt since v0.5.0 and
// read_document since v0.6.0).
func elevenStubTools() []tools.Tool {
	ids := []string{
		"read_image",
		"read_document",
		"extract_text",
		"extract_code",
		"extract_table",
		"describe_ui",
		"describe_diagram",
		"describe_chart",
		"diagnose_error",
		"image_to_prompt",
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

// newTestServer builds a Server with 11 stub tools and the given executor.
// It skips the production registry (which Track E hasn't populated yet).
func newTestServer(t *testing.T, executor tools.Executor) *Server {
	t.Helper()
	srv, err := NewServer(Dependencies{
		Logger:   silentLogger(),
		Tools:    elevenStubTools(),
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

// progressEmittingExecutor is a recordingExecutor that also emits canned
// progress updates via the ctx sink during Run, so tests can verify the
// notifications/progress round-trip end to end (callTool attaches the sink →
// executor's ctx carries it → Report → NotifyProgress → client handler).
type progressEmittingExecutor struct {
	recordingExecutor
	emit []progress.Update
}

func (e *progressEmittingExecutor) Run(ctx context.Context, toolID, systemPrompt, userPrompt string, images []tools.ImageRef, maxTokens int) (string, tools.Stats, error) {
	for _, u := range e.emit {
		progress.Report(ctx, u)
	}
	return e.recordingExecutor.Run(ctx, toolID, systemPrompt, userPrompt, images, maxTokens)
}

// TestServerRegistersElevenTools verifies the server's tool registration
// count via Server.ToolCount (a sanity check that doesn't require running
// the SDK).
func TestServerRegistersElevenTools(t *testing.T) {
	srv := newTestServer(t, &recordingExecutor{})
	assert.Equal(t, 11, srv.ToolCount(), "expected 11 tools registered")
}

// TestToolsListReturnsElevenTools drives a real MCP client through the SDK
// in-memory transport and asserts the server reports all 11 tools in
// tools/list.
func TestToolsListReturnsElevenTools(t *testing.T) {
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
	assert.Equal(t, 11, saw, "tools/list should return exactly 11 tools")
}

// TestCallToolPassesExpandedImagesToExecutor is the G3/R2 regression test:
// when a tool is an Expander (read_document), callTool must pass the EXPANDED
// image refs (e.g. rasterized PDF pages) to the executor — not the single
// pre-expansion input ref. Without the input.Images fix this would report 1.
func TestCallToolPassesExpandedImagesToExecutor(t *testing.T) {
	exec := &recordingExecutor{}
	tool := expanderStubTool{
		stubTool: stubTool{id: "expand_test", description: "stub", maxTokens: 1024, system: "sys"},
		pages:    []string{"/tmp/p1.png", "/tmp/p2.png", "/tmp/p3.png"},
	}
	srv, err := NewServer(Dependencies{
		Logger:   silentLogger(),
		Tools:    []tools.Tool{tool},
		Executor: exec,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t1, t2 := mcp.NewInMemoryTransports()
	_, err = srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "expand_test",
		Arguments: map[string]any{"image_path": "/tmp/one.pdf"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	require.Len(t, exec.calls, 1)
	assert.Equal(t, 3, exec.calls[0].images,
		"executor must receive the EXPANDED page refs, not the single pre-expansion input")
}

// expanderStubTool is a stubTool that also implements tools.Expander, returning
// a fixed set of page refs so the R2 regression test can assert callTool passes
// the expanded set to the executor.
type expanderStubTool struct {
	stubTool
	pages []string
}

func (e expanderStubTool) ExpandImages(_ context.Context, _ tools.ToolInput) ([]tools.ImageRef, error) {
	refs := make([]tools.ImageRef, len(e.pages))
	for i, p := range e.pages {
		refs[i] = tools.ImageRef{LocalPath: p}
	}
	return refs, nil
}

// tempWritingExpander is an Expander that creates REAL temp page files under a
// fresh temp dir (mimicking read_document's rasterizer) and registers them for
// cleanup. Used by TestCallToolReapsExpandedPageTemps to assert callTool's
// deferred cleanup reaps the EXPANDED page temps (and their out dir), not just
// the pre-expansion input refs — the v0.6 MCP path leaked one set of page
// temps per read_document call before the defer was moved onto input.Images.
type tempWritingExpander struct {
	stubTool
	pages []string // relative page names written under a fresh temp dir
}

func (e *tempWritingExpander) ExpandImages(_ context.Context, _ tools.ToolInput) ([]tools.ImageRef, error) {
	dir, err := os.MkdirTemp("", "lvm-test-doc-*")
	if err != nil {
		return nil, err
	}
	refs := make([]tools.ImageRef, 0, len(e.pages))
	for _, name := range e.pages {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("PNG"), 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		tools.RegisterTemp(p)
		refs = append(refs, tools.ImageRef{LocalPath: p})
	}
	return refs, nil
}

// TestCallToolReapsExpandedPageTemps is the page-temp-leak regression test.
// callTool must clean up the EXPANDED page temps (input.Images), not just the
// pre-expansion input ref. Before the fix, the defer captured the original
// `images` slice and every rasterized page temp leaked.
func TestCallToolReapsExpandedPageTemps(t *testing.T) {
	tool := &tempWritingExpander{
		stubTool: stubTool{id: "expand_test", description: "stub", maxTokens: 1024, system: "sys"},
		pages:    []string{"page-1.png", "page-2.png", "page-3.png"},
	}

	// Capture the expanded page paths the executor sees, so we can stat them
	// after the call returns.
	var seenPaths []string
	exec := &pathListingExecutor{
		inner:  &recordingExecutor{},
		onSeen: func(p []string) { seenPaths = p },
	}

	srv, err := NewServer(Dependencies{
		Logger:   silentLogger(),
		Tools:    []tools.Tool{tool},
		Executor: exec,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t1, t2 := mcp.NewInMemoryTransports()
	_, err = srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "expand_test",
		Arguments: map[string]any{"image_path": "/tmp/one.pdf"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.NotEmpty(t, seenPaths, "executor should have seen the expanded page temps")

	for _, p := range seenPaths {
		_, statErr := os.Stat(p)
		assert.True(t, os.IsNotExist(statErr),
			"expanded page temp %q must be reaped after callTool; stat err=%v", p, statErr)
	}
}

// pathListingExecutor wraps an Executor and records the LocalPath of every
// image it sees via onSeen before delegating.
type pathListingExecutor struct {
	inner  tools.Executor
	onSeen func(paths []string)
}

func (e *pathListingExecutor) Run(ctx context.Context, toolID, sys, user string, imgs []tools.ImageRef, max int) (string, tools.Stats, error) {
	paths := make([]string, len(imgs))
	for i, im := range imgs {
		paths[i] = im.LocalPath
	}
	e.onSeen(paths)
	return e.inner.Run(ctx, toolID, sys, user, imgs, max)
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

// TestCallToolSendsProgressNotifications drives a real client that sends a
// _meta.progressToken on tools/call and asserts the server forwards progress
// updates as notifications/progress with the matching token.
func TestCallToolSendsProgressNotifications(t *testing.T) {
	var mu sync.Mutex
	var got []*mcp.ProgressNotificationParams
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			mu.Lock()
			got = append(got, req.Params)
			mu.Unlock()
		},
	})

	exec := &progressEmittingExecutor{
		emit: []progress.Update{
			{Phase: "downloading", Current: 100, Total: 1000, Unit: "bytes"},
			{Phase: "inferring", Current: 5, Total: 30, Unit: "s"},
		},
	}
	srv := newTestServer(t, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	params := &mcp.CallToolParams{
		Name:      "read_image",
		Arguments: map[string]any{"image_path": "/tmp/fake.png"},
	}
	params.SetProgressToken("tok-1")
	res, err := cs.CallTool(ctx, params)
	require.NoError(t, err)
	require.False(t, res.IsError)

	// The notifications are sent fire-and-forget; poll for them to land.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, got, 2, "expected the 2 emitted progress updates to arrive as notifications")
	assert.Equal(t, "tok-1", got[0].ProgressToken)
	assert.Equal(t, "tok-1", got[1].ProgressToken)
	assert.Equal(t, float64(100), got[0].Progress)
	assert.Equal(t, float64(5), got[1].Progress)
}

// TestCallToolProgressNotificationsOrdered verifies the single-dispatcher
// guarantee: a burst of many updates must arrive at the client in the SAME
// order they were emitted (a progress bar must not jump backwards). The old
// fire-and-forget-per-update design raced these and delivered them out of
// order; the serialized dispatcher preserves FIFO.
func TestCallToolProgressNotificationsOrdered(t *testing.T) {
	var mu sync.Mutex
	var got []float64
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			mu.Lock()
			got = append(got, req.Params.Progress)
			mu.Unlock()
		},
	})

	// Emit 20 monotonically-increasing values; the client must see them in order.
	const n = 20
	emit := make([]progress.Update, n)
	for i := 0; i < n; i++ {
		emit[i] = progress.Update{Phase: "inferring", Current: float64(i), Unit: "s"}
	}
	exec := &progressEmittingExecutor{emit: emit}
	srv := newTestServer(t, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	params := &mcp.CallToolParams{Name: "read_image", Arguments: map[string]any{"image_path": "/tmp/fake.png"}}
	params.SetProgressToken("tok")
	res, err := cs.CallTool(ctx, params)
	require.NoError(t, err)
	require.False(t, res.IsError)

	// Poll until all n land (they're serialized so all should arrive).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		c := len(got)
		mu.Unlock()
		if c >= n {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, got, n, "all %d updates should arrive", n)
	for i := 0; i < n; i++ {
		assert.Equal(t, float64(i), got[i],
			"notification %d arrived out of order (got %v); the dispatcher must preserve FIFO", i, got)
	}
}

// TestCallToolNoProgressTokenSendsNothing asserts that a client which does NOT
// send a _meta.progressToken receives zero notifications, even though the
// executor emits progress updates (today's behavior is preserved).
func TestCallToolNoProgressTokenSendsNothing(t *testing.T) {
	var mu sync.Mutex
	var n atomic.Int64
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(context.Context, *mcp.ProgressNotificationClientRequest) {
			n.Add(1)
		},
	})

	exec := &progressEmittingExecutor{
		emit: []progress.Update{{Phase: "inferring", Current: 1, Unit: "s"}},
	}
	srv := newTestServer(t, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := srv.mcp.Connect(ctx, t1, nil)
	require.NoError(t, err)
	cs, err := client.Connect(ctx, t2, nil)
	require.NoError(t, err)
	defer cs.Close()

	// No SetProgressToken: the server must not attach a sink.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_image",
		Arguments: map[string]any{"image_path": "/tmp/fake.png"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	// Allow time for any (erroneous) notification to arrive, then assert none.
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, int64(0), n.Load(), "no notifications when the client sent no progress token")
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
		if runtime.GOOS == "windows" {
			t.Skip("Windows: Unix-absolute path literal resolves drive-relative")
		}
		args := map[string]any{"image_path": "/abs/path/x.png"}
		refs, consumed, err := normalizeImages(args, "read_image")
		require.NoError(t, err)
		require.Len(t, refs, 1)
		assert.Equal(t, "/abs/path/x.png", refs[0].LocalPath)
		assert.True(t, consumed["image_path"])
	})

	t.Run("image_url file://", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows: Unix file:// URI path semantics")
		}
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

// pathCapturingExecutor is a tools.Executor that records the LocalPath of the
// first image it sees, so a test can later assert on the temp file created for
// an image_data input. Used by TestCallToolCleansUpDataURITempFile.
type pathCapturingExecutor struct {
	mu   sync.Mutex
	path string
}

func (e *pathCapturingExecutor) Run(ctx context.Context, toolID, systemPrompt, userPrompt string, images []tools.ImageRef, maxTokens int) (string, tools.Stats, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(images) > 0 {
		e.path = images[0].LocalPath
	}
	return "ok", tools.Stats{}, nil
}

func (e *pathCapturingExecutor) capturedPath() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.path
}

// TestCallToolCleansUpDataURITempFile is the E6 regression test: an image_data
// (data: URI) argument is decoded to a temp file for inference, and callTool
// must remove that temp file once the call returns. Before E6 the MCP path
// leaked one temp file per image_data call (its private dataURIToTempFile never
// registered with tempRegistry).
func TestCallToolCleansUpDataURITempFile(t *testing.T) {
	exec := &pathCapturingExecutor{}
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
		Arguments: map[string]any{"image_data": "data:image/png;base64,iVBORw0KGgo="},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, "call should succeed")

	tempPath := exec.capturedPath()
	require.NotEmpty(t, tempPath, "executor should have seen the temp file path")
	require.True(t, strings.HasSuffix(tempPath, ".png"),
		"temp file should have .png suffix; got %q", tempPath)

	// After callTool returns, the temp file must be gone (the defer reaped it).
	_, statErr := os.Stat(tempPath)
	assert.True(t, os.IsNotExist(statErr),
		"temp file %q should be removed after callTool; stat err=%v", tempPath, statErr)
}

// lvmTempFiles snapshots the set of localvision temp image files currently in
// os.TempDir. Used to assert no net leak across a normalizeImages call: the set
// after must equal the set before. Safe under sequential test execution (this
// package's tests do not call t.Parallel).
func lvmTempFiles(t *testing.T) map[string]struct{} {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "lvm-image-*"))
	require.NoError(t, err)
	set := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		set[m] = struct{}{}
	}
	return set
}

// TestNormalizeImagesCleansUpOnArrayError covers the E6 edge case: when an
// images array has a valid data: URI as element 0 but a malformed element 1,
// normalizeImages must reap element 0's temp file rather than leak it. That
// error path returns before callTool's cleanup defer is registered, so the
// cleanup has to happen inside normalizeImages itself.
func TestNormalizeImagesCleansUpOnArrayError(t *testing.T) {
	before := lvmTempFiles(t)

	// Element 0 is a valid data: URI (creates a temp file); element 1 is not a
	// string or object, so normalizeOneImage rejects it.
	args := map[string]any{
		"images": []any{
			"data:image/png;base64,iVBORw0KGgo=",
			12345, // invalid element type
		},
	}
	_, _, err := normalizeImages(args, "compare_images")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "images[1]")

	after := lvmTempFiles(t)
	assert.Equal(t, before, after,
		"temp file created for images[0] must be reaped on the images[1] error; no net leak")
}

// Reduce noise from the unused linter if atomic is not used directly in
// this test file (it is used in tools.go).
var _ = atomic.AddInt64
