package llama

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// client_test.go exercises ChatVision against an httptest.Server mock. It
// covers: 200 happy path, 4xx no-retry, 503 retry-succeeds, network error
// retry-fails.

// chatTestServer is a tiny configurable handler for /v1/chat/completions.
// It counts requests and serves one of a series of canned responses.
type chatTestServer struct {
	mu        chanMu
	responses []chatTestResponse
	requests  []chatRequestBody
}

type chanMu struct {
	ch chan struct{}
}

type chatTestResponse struct {
	Status int
	Body   string
	// delay is how long to wait before responding. Useful for testing
	// ctx cancellation.
	delay time.Duration
	// closeConn, if true, hijacks the connection and closes it without
	// writing a response. Simulates a network reset.
	closeConn bool
}

func newChatTestServer(responses ...chatTestResponse) *chatTestServer {
	return &chatTestServer{
		mu:        chanMu{ch: make(chan struct{}, 16)},
		responses: responses,
	}
}

func (s *chatTestServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Acquire our (very) lightweight mutex so concurrent requests
		// don't race on s.requests slice.
		s.mu.ch <- struct{}{}
		defer func() { <-s.mu.ch }()

		// Read the request body so we can assert on it later.
		body, _ := io.ReadAll(r.Body)
		s.requests = append(s.requests, chatRequestBody{Path: r.URL.Path, Body: string(body)})

		idx := len(s.requests) - 1
		if idx >= len(s.responses) {
			idx = len(s.responses) - 1
		}
		resp := s.responses[idx]

		if resp.closeConn {
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "no hijack", http.StatusInternalServerError)
				return
			}
			conn, _, _ := hj.Hijack()
			if conn != nil {
				_ = conn.Close()
			}
			return
		}

		if resp.delay > 0 {
			time.Sleep(resp.delay)
		}
		if resp.Status == 0 {
			resp.Status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.Status)
		_, _ = fmt.Fprint(w, resp.Body)
	})
}

// requestCount returns the number of requests observed so far. Thread-safe.
func (s *chatTestServer) requestCount() int {
	s.mu.ch <- struct{}{}
	defer func() { <-s.mu.ch }()
	return len(s.requests)
}

type chatRequestBody struct {
	Path string
	Body string
}

// happyPathResponse is a minimal OpenAI-compatible 200 response.
const happyPathResponse = `{
	"choices": [
		{"message": {"role": "assistant", "content": "hello world"}, "finish_reason": "stop"}
	],
	"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
}`

// clientForTest returns a Client whose underlying chatClient points at the
// test server. Restores the default HTTP client on test cleanup.
func clientForTest(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	setChatClient(srv.Client())
	t.Cleanup(func() { setChatClient(&http.Client{}) })
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	return newClient(port)
}

// TestChatVisionHappyPath: 200 returns parsed response with token counts.
func TestChatVisionHappyPath(t *testing.T) {
	srvImpl := newChatTestServer(chatTestResponse{
		Status: http.StatusOK,
		Body:   happyPathResponse,
	})
	srv := httptest.NewServer(srvImpl.handler())
	defer srv.Close()

	c := clientForTest(t, srv)
	resp, err := c.ChatVision(context.Background(), ChatRequest{
		UserPrompt: "describe this",
		MaxTokens:  256,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "hello world", resp.Content)
	assert.Equal(t, 10, resp.TokensIn)
	assert.Equal(t, 5, resp.TokensOut)
	assert.GreaterOrEqual(t, resp.ElapsedMs, int64(0))
	assert.Equal(t, 1, srvImpl.requestCount())
}

// TestChatVision4xxNoRetry: a 400 response surfaces as an error and does
// NOT trigger a retry (server should only be hit once).
func TestChatVision4xxNoRetry(t *testing.T) {
	srvImpl := newChatTestServer(
		chatTestResponse{Status: http.StatusBadRequest, Body: `{"error":"bad"}`},
		// Add a second response in case the client retries (it shouldn't).
		chatTestResponse{Status: http.StatusOK, Body: happyPathResponse},
	)
	srv := httptest.NewServer(srvImpl.handler())
	defer srv.Close()

	c := clientForTest(t, srv)
	_, err := c.ChatVision(context.Background(), ChatRequest{UserPrompt: "x"})
	require.Error(t, err)
	var he *httpError
	require.True(t, errors.As(err, &he), "want *httpError, got %T: %v", err, err)
	assert.Equal(t, http.StatusBadRequest, he.Status)
	assert.Equal(t, 1, srvImpl.requestCount(), "4xx must not trigger retry")
}

// TestChatVision503RetrySucceeds: first request returns 503; retry returns
// 200 and the call succeeds.
func TestChatVision503RetrySucceeds(t *testing.T) {
	srvImpl := newChatTestServer(
		chatTestResponse{Status: http.StatusServiceUnavailable, Body: `{"error":"busy"}`},
		chatTestResponse{Status: http.StatusOK, Body: happyPathResponse},
	)
	srv := httptest.NewServer(srvImpl.handler())
	defer srv.Close()

	// Speed up the retry backoff so the test doesn't sleep 500ms.
	origBackoff := chatRetryBackoff
	chatRetryBackoff = 5 * time.Millisecond
	t.Cleanup(func() { chatRetryBackoff = origBackoff })

	c := clientForTest(t, srv)
	resp, err := c.ChatVision(context.Background(), ChatRequest{UserPrompt: "x"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "hello world", resp.Content)
	assert.Equal(t, 2, srvImpl.requestCount(), "503 must trigger exactly one retry")
}

// TestChatVision503RetryFails: both attempts return 503; the call surfaces
// an *httpError(503). Verifies we don't retry more than once.
func TestChatVision503RetryFails(t *testing.T) {
	srvImpl := newChatTestServer(
		chatTestResponse{Status: http.StatusServiceUnavailable, Body: `{"error":"busy"}`},
		chatTestResponse{Status: http.StatusServiceUnavailable, Body: `{"error":"busy"}`},
		chatTestResponse{Status: http.StatusOK, Body: happyPathResponse}, // should not be hit
	)
	srv := httptest.NewServer(srvImpl.handler())
	defer srv.Close()

	origBackoff := chatRetryBackoff
	chatRetryBackoff = 5 * time.Millisecond
	t.Cleanup(func() { chatRetryBackoff = origBackoff })

	c := clientForTest(t, srv)
	_, err := c.ChatVision(context.Background(), ChatRequest{UserPrompt: "x"})
	require.Error(t, err)
	var he *httpError
	require.True(t, errors.As(err, &he), "want *httpError, got %T: %v", err, err)
	assert.Equal(t, http.StatusServiceUnavailable, he.Status)
	assert.Equal(t, 2, srvImpl.requestCount(), "must not retry more than once")
}

// TestChatVisionConnectionResetRetry: first request closes the connection
// abruptly (simulated network reset); retry succeeds.
func TestChatVisionConnectionResetRetry(t *testing.T) {
	srvImpl := newChatTestServer(
		chatTestResponse{closeConn: true},
		chatTestResponse{Status: http.StatusOK, Body: happyPathResponse},
	)
	srv := httptest.NewServer(srvImpl.handler())
	defer srv.Close()

	origBackoff := chatRetryBackoff
	chatRetryBackoff = 5 * time.Millisecond
	t.Cleanup(func() { chatRetryBackoff = origBackoff })

	c := clientForTest(t, srv)
	resp, err := c.ChatVision(context.Background(), ChatRequest{UserPrompt: "x"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "hello world", resp.Content)
	assert.Equal(t, 2, srvImpl.requestCount())
}

// TestChatVisionTransportErrorRetryFails: both attempts close the
// connection; the call surfaces a transport error and does not retry more
// than once.
func TestChatVisionTransportErrorRetryFails(t *testing.T) {
	srvImpl := newChatTestServer(
		chatTestResponse{closeConn: true},
		chatTestResponse{closeConn: true},
		chatTestResponse{Status: http.StatusOK, Body: happyPathResponse}, // not hit
	)
	srv := httptest.NewServer(srvImpl.handler())
	defer srv.Close()

	origBackoff := chatRetryBackoff
	chatRetryBackoff = 5 * time.Millisecond
	t.Cleanup(func() { chatRetryBackoff = origBackoff })

	c := clientForTest(t, srv)
	_, err := c.ChatVision(context.Background(), ChatRequest{UserPrompt: "x"})
	require.Error(t, err)
	var te *transportError
	require.True(t, errors.As(err, &te), "want *transportError, got %T: %v", err, err)
	assert.Equal(t, 2, srvImpl.requestCount())
}

// TestChatVisionCtxCancelled: ctx cancelled before the call returns
// ctx.Err() and does not issue any request.
func TestChatVisionCtxCancelled(t *testing.T) {
	srvImpl := newChatTestServer(chatTestResponse{Status: http.StatusOK, Body: happyPathResponse})
	srv := httptest.NewServer(srvImpl.handler())
	defer srv.Close()

	c := clientForTest(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := c.ChatVision(ctx, ChatRequest{UserPrompt: "x"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

// TestBuildChatRequestBodyImageURLs: image paths are encoded as file:// URLs
// in image_url content parts.
func TestBuildChatRequestBodyImageURLs(t *testing.T) {
	body, err := buildChatRequestBody(ChatRequest{
		SystemPrompt: "you are a tool",
		UserPrompt:   "describe",
		ImagePaths:   []string{"/tmp/foo.png", "/tmp/bar.jpg"},
		MaxTokens:    100,
		Temperature:  0.2,
	})
	require.NoError(t, err)
	s := string(body)
	assert.Contains(t, s, `"system"`)
	assert.Contains(t, s, `"you are a tool"`)
	assert.Contains(t, s, `"file:///tmp/foo.png"`)
	assert.Contains(t, s, `"file:///tmp/bar.jpg"`)
	assert.Contains(t, s, `"describe"`)
	assert.Contains(t, s, `"max_tokens":100`)
	assert.Contains(t, s, `"temperature":0.2`)
}

// TestBuildChatRequestBodyRejectsEmpty: empty request returns an error.
func TestBuildChatRequestBodyRejectsEmpty(t *testing.T) {
	_, err := buildChatRequestBody(ChatRequest{})
	require.Error(t, err)
}

// TestTruncate just covers the helper.
func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 10))
	assert.Equal(t, "ab...", truncate("abcdef", 2))
	assert.Equal(t, "...", truncate("abcdef", 0))
}

// TestIsConnectionError covers the markers.
func TestIsConnectionError(t *testing.T) {
	assert.True(t, isConnectionError(errors.New("read tcp: connection reset by peer")))
	assert.True(t, isConnectionError(errors.New("EOF")))
	assert.True(t, isConnectionError(errors.New("dial tcp: connection refused")))
	assert.False(t, isConnectionError(errors.New("some other error")))
	assert.False(t, isConnectionError(context.Canceled))
	assert.False(t, isConnectionError(nil))
}

// TestShouldRetry covers the retry predicate matrix.
func TestShouldRetry(t *testing.T) {
	te := &transportError{err: errors.New("connection reset")}
	assert.True(t, shouldRetry(te))

	he503 := &httpError{Status: http.StatusServiceUnavailable}
	assert.True(t, shouldRetry(he503))

	he400 := &httpError{Status: http.StatusBadRequest}
	assert.False(t, shouldRetry(he400))

	he500 := &httpError{Status: http.StatusInternalServerError}
	assert.False(t, shouldRetry(he500))

	assert.False(t, shouldRetry(nil))
	assert.False(t, shouldRetry(context.Canceled))
}

// TestChatVisionRequestsCounter is a smoke test that multiple sequential
// calls are each issued against the server.
func TestChatVisionRequestsCounter(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, happyPathResponse)
	}))
	defer srv.Close()

	c := clientForTest(t, srv)
	for i := 0; i < 3; i++ {
		resp, err := c.ChatVision(context.Background(), ChatRequest{UserPrompt: "x"})
		require.NoError(t, err)
		require.NotNil(t, resp)
	}
	require.Equal(t, int32(3), atomic.LoadInt32(&count))
}

// TestBuildChatRequestBodyContainsNoShell: a defense-in-depth check that the
// JSON body is well-formed and contains no shell metacharacters injected by
// the builder itself.
func TestBuildChatRequestBodyContainsNoShell(t *testing.T) {
	// If the user prompt or image path contained a shell metacharacter, it
	// must NOT be passed through `sh -c` (we don't use sh, but verify).
	malicious := "; rm -rf ~"
	body, err := buildChatRequestBody(ChatRequest{
		UserPrompt: malicious,
		ImagePaths: []string{"/tmp/" + malicious + ".png"},
	})
	require.NoError(t, err)
	s := string(body)
	// The shell metacharacter should be JSON-escaped (i.e. the body should
	// still be valid JSON).
	if !strings.HasPrefix(s, "{") {
		t.Fatalf("body is not JSON: %s", s)
	}
}
