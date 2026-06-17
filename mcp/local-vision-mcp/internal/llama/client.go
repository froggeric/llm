package llama

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// loadImageAsDataURI reads the image file at path and returns it as a
// `data:<mime>;base64,<b64>` URI suitable for OpenAI's image_url.url field.
//
// MIME type is sniffed from the file extension; on unknown extensions we
// fall back to image/png (llama-server will fail to decode if it's wrong,
// which surfaces a clear error to the user).
func loadImageAsDataURI(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mime := mimeForExt(filepath.Ext(path))
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// mimeForExt returns a MIME type for the given file extension. Used for
// inline data: URIs sent to llama-server.
func mimeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/png"
	}
}

// client.go implements the OpenAI-compatible HTTP client that talks to
// llama-server. It sends a chat completion with image content and parses
// the response into ChatResponse.
//
// Retry policy (F4.6): on connection errors (network reset, EOF) OR HTTP
// 503, retry ONCE after 500ms. On any 4xx, return immediately without
// retry (the request is bad; retrying won't help). On 200, return the
// parsed response.
//
// Context propagation (F3.6): ctx is plumbed to http.NewRequestWithContext
// so MCP cancellation propagates end-to-end.

// chatClient is the HTTP client used by ChatVision. Indirected at the
// package level so tests can swap in one wired to httptest.Server.
var chatClient = &http.Client{
	Timeout: 0, // ctx controls cancellation; no overall client timeout
}

// setChatClient replaces the package HTTP client used by ChatVision.
// Used by tests.
func setChatClient(c *http.Client) { chatClient = c }

// chatRetryBackoff is the wait before the single retry. F4.6.
//
// Declared as a var (not a const) so tests can override it to keep test
// runtime low; the production value is chatRetryBackoffDefault.
var chatRetryBackoff = 500 * time.Millisecond

// chatRetryBackoffDefault is the production retry backoff (500ms).
const chatRetryBackoffDefault = 500 * time.Millisecond

// chatContentKind enumerates the content kinds we serialize. Only "text"
// and "image_url" are needed for vision chat.
const (
	contentKindText     = "text"
	contentKindImageURL = "image_url"
)

// chatContentPart is one entry in a multimodal message's content array.
// For text-only turns we use a plain string content; for vision turns we
// use an array of these.
type chatContentPart struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	ImageURL *chatImageURLRef `json:"image_url,omitempty"`
}

// chatImageURLRef wraps the URL field of an image_url content part.
// llama-server accepts file:// URLs in addition to data: URIs; we use
// file:// because the image bytes are already on disk (no base64 bloat).
type chatImageURLRef struct {
	URL string `json:"url"`
}

// chatMessage is one message in the OpenAI chat schema. Content is either
// a string (text-only) or an array of chatContentPart (multimodal). We
// marshal carefully to support both.
type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string OR []chatContentPart
}

// chatRequestJSON is the on-the-wire request body sent to /v1/chat/completions.
type chatRequestJSON struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// chatResponseJSON is the subset of the OpenAI response we care about.
// Token counts come from the `usage` field; the assistant message comes
// from `choices[0].message.content`.
type chatResponseJSON struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// chatEndpointPath is the path appended to the base URL.
const chatEndpointPath = "/v1/chat/completions"

// newClient constructs a Client bound to a specific port. The Client is
// only valid between Acquire and release; callers must not retain it past
// release.
func newClient(port int) *Client {
	return &Client{
		port: port,
		base: fmt.Sprintf("http://127.0.0.1:%d", port),
	}
}

// ChatVision implements the contract documented in lifecycle.go.
//
// On a 4xx response, the body is returned as part of an error without retry.
// On 5xx or connection error, retry once after chatRetryBackoff. On 200,
// returns the parsed response.
func (c *Client) ChatVision(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if c == nil {
		return nil, errors.New("nil client")
	}
	body, err := buildChatRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	start := time.Now()
	url := c.base + chatEndpointPath

	resp, err := c.doWithRetry(ctx, url, body)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start).Milliseconds()

	out := &ChatResponse{
		Content:   resp.Choices[0].Message.Content,
		TokensIn:  resp.Usage.PromptTokens,
		TokensOut: resp.Usage.CompletionTokens,
		ElapsedMs: elapsed,
	}
	return out, nil
}

// doWithRetry issues the HTTP POST, retrying once on connection errors or
// 503. Returns the parsed response or an error.
func (c *Client) doWithRetry(ctx context.Context, url string, body []byte) (*chatResponseJSON, error) {
	resp, err := c.postChat(ctx, url, body)
	if err != nil {
		if shouldRetry(err) {
			if err := sleepCtx(ctx, chatRetryBackoff); err != nil {
				return nil, err
			}
			return c.postChat(ctx, url, body)
		}
		return nil, err
	}
	return resp, nil
}

// postChat sends one POST and parses the response. Returns either a parsed
// response (200) or a structured error (4xx/5xx/transport). The caller
// inspects shouldRetry(err) to decide whether to retry.
func (c *Client) postChat(ctx context.Context, url string, body []byte) (*chatResponseJSON, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := chatClient.Do(httpReq)
	if err != nil {
		// Wrap so shouldRetry can detect transport-level failures.
		return nil, &transportError{err: err}
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &transportError{err: fmt.Errorf("read response body: %w", err)}
	}

	switch {
	case httpResp.StatusCode == http.StatusOK:
		var parsed chatResponseJSON
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return nil, fmt.Errorf("decode 200 response: %w (body=%q)", err, truncate(string(respBody), 512))
		}
		if len(parsed.Choices) == 0 {
			return nil, fmt.Errorf("response had no choices (body=%q)", truncate(string(respBody), 512))
		}
		return &parsed, nil

	case httpResp.StatusCode >= 400 && httpResp.StatusCode < 500:
		// 4xx: caller error. No retry.
		return nil, &httpError{
			Status: httpResp.StatusCode,
			Body:   string(respBody),
		}

	case httpResp.StatusCode == http.StatusServiceUnavailable:
		// 503: transient; retry once.
		return nil, &httpError{
			Status: httpResp.StatusCode,
			Body:   string(respBody),
		}

	default:
		// 5xx other than 503: don't retry (would surface as 5xx in error).
		return nil, &httpError{
			Status: httpResp.StatusCode,
			Body:   string(respBody),
		}
	}
}

// shouldRetry returns true for connection errors and HTTP 503. False for
// 4xx, 5xx-other-than-503, context cancellation, and successful responses
// (which never reach shouldRetry as errors).
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	// Context errors are never retried (the caller is cancelling).
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var te *transportError
	if errors.As(err, &te) {
		return isConnectionError(te.err)
	}
	var he *httpError
	if errors.As(err, &he) {
		return he.Status == http.StatusServiceUnavailable
	}
	return false
}

// isConnectionError reports whether err indicates a transport-level failure
// that a retry might fix (network reset, EOF, connection refused). Returns
// false for context cancellation surfaced through the transport.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	// Common connection-reset errors. We match by string for portability
	// across platforms because Go's net package doesn't always export typed
	// errors for these conditions.
	s := err.Error()
	connectionErrorMarkers := []string{
		"connection reset",
		"connection refused",
		"EOF",
		"broken pipe",
		"connection closed",
		"i/o timeout",
		"no such host",
		"network is unreachable",
	}
	for _, m := range connectionErrorMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	// Also retry on net.OpError / syscall.ECONN* types.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return false
}

// transportError wraps a network-level error so shouldRetry can identify it
// by type without string matching. It is not exported; tests use the
// package-level shouldRetry predicate.
type transportError struct{ err error }

func (e *transportError) Error() string { return e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

// httpError is a non-2xx HTTP response. Status is the status code; Body is
// the response body for diagnostic context.
type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("llama-server returned HTTP %d: %s", e.Status, truncate(e.Body, 256))
}

// sleepCtx sleeps for d, returning ctx.Err() if cancelled. Lets the retry
// backoff honor cancellation.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// truncate returns at most n bytes of s, with a "..." marker if truncated.
// Used to bound error message sizes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// buildChatRequestBody constructs the OpenAI-compatible JSON body for a
// vision chat completion. Returns the marshalled bytes.
//
// The user message contains one text part (the user prompt) followed by one
// image_url part per image. llama-server accepts file:// URLs in image_url.url;
// we convert all paths to absolute and prefix "file://".
func buildChatRequestBody(req ChatRequest) ([]byte, error) {
	if req.UserPrompt == "" && len(req.ImagePaths) == 0 {
		return nil, errors.New("ChatRequest has no user prompt and no images")
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	temperature := req.Temperature
	if temperature == 0 {
		// 0.0 is a valid value but tools default to 0.1 for determinism.
		// Caller must explicitly pass 0.0 to disable — we use the small
		// default if they left it unset (which we can't distinguish from
		// a literal 0, but tools always set it).
		temperature = 0.1
	}
	model := req.Model
	if model == "" {
		model = "local"
	}

	msgs := make([]chatMessage, 0, 2)
	if req.SystemPrompt != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: req.SystemPrompt})
	}

	// Build user content: text + N images.
	parts := make([]chatContentPart, 0, 1+len(req.ImagePaths))
	if req.UserPrompt != "" {
		parts = append(parts, chatContentPart{
			Type: contentKindText,
			Text: req.UserPrompt,
		})
	}
	for _, p := range req.ImagePaths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("resolve image path %q: %w", p, err)
		}
		// Inline the image as a data: URI rather than file://. Recent
		// llama-server versions reject file:// URLs unless --media-path
		// is configured, and we don't want to require users to set that.
		// The base64 bloat is acceptable: a 1 MB image becomes ~1.4 MB
		// of base64 in the JSON request, which is small relative to the
		// inference cost.
		dataURI, err := loadImageAsDataURI(abs)
		if err != nil {
			return nil, fmt.Errorf("encode image %q: %w", abs, err)
		}
		parts = append(parts, chatContentPart{
			Type:     contentKindImageURL,
			ImageURL: &chatImageURLRef{URL: dataURI},
		})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: parts})

	body := chatRequestJSON{
		Model:       model,
		Messages:    msgs,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      false,
	}
	out, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}
	return out, nil
}
