package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/froggeric/llm/mcp/localvision/internal/progress"
	"github.com/froggeric/llm/mcp/localvision/internal/tools"
)

// latencyHint is the latency warning we append to every tool description.
// Per F5.3: the user has a smart-approval-pipeline with timeouts; we need
// the LLM to know up-front that each call takes 30-60 seconds so it can
// configure its timeout accordingly. Per F1.11: we don't stream in MVP,
// so the description carries the latency info instead.
const latencyHint = "\n\nLatency: this tool loads a local vision model and runs one inference. Each call takes 30-60 seconds."

// registerTool installs a single tool against the underlying SDK server.
// We use the low-level Server.AddTool (not the generic AddTool[In, Out])
// because each vision tool has a different input shape and we route every
// call through one dispatcher (callTool) rather than letting the SDK
// generate per-tool handlers via reflection.
//
// The SDK's InputSchema field accepts any value that JSON-marshals to a
// valid JSON-Schema object. tools.Tool.InputSchema() returns
// map[string]any, which is exactly that.
//
// F5.4 (tool name collisions): tool IDs are unprefixed in MVP (e.g.
// "read_image" rather than "vision_read_image"). This is intentional — the
// Claude Code skill layer is expected to namespace us — but it does mean
// that if the user has another vision-capable MCP installed, names may
// collide. We document this risk here; v0.2 may add a configurable prefix.
func (s *Server) registerTool(t tools.Tool) {
	// Augment the description with the latency hint. This is purely
	// additive: tools may already include latency info; the hint ensures
	// every tool carries it even if the per-tool text doesn't.
	description := strings.TrimSpace(t.Description()) + latencyHint

	schema := t.InputSchema()
	if schema == nil {
		// The SDK panics if InputSchema is nil. Fall back to an empty
		// object schema (no params) rather than crashing; the per-tool
		// layer should always provide one, but be defensive.
		schema = map[string]any{"type": "object", "properties": map[string]any{}}
	}

	// Capture the tool by value in a local variable so each closure gets
	// its own tool reference. (Iterating and using the loop variable
	// directly in a closure is a classic Go bug; the call to registerTool
	// per-iteration avoids it, but we still bind to a local here for
	// clarity.)
	toolRef := t
	mcpTool := &mcp.Tool{
		Name:        toolRef.ID(),
		Description: description,
		InputSchema: schema,
	}
	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return s.callTool(ctx, req, toolRef)
	}
	s.mcp.AddTool(mcpTool, handler)
}

// callTool is the dispatcher for tools/call. It:
//
//  1. Parses the raw JSON arguments into a generic map.
//  2. Normalizes the image input (image_path / image_data / image_url /
//     images array) into a []tools.ImageRef.
//  3. Builds a tools.ToolInput and calls tool.BuildRequest to get the
//     system+user prompts and image paths the model needs.
//  4. Invokes the executor to run inference.
//  5. Calls tool.ParseOutput to post-process the model's text.
//  6. Wraps the result as mcp.TextContent.
//
// In-flight tracking: the inFlightWG is incremented at entry and
// decremented at exit, so graceful shutdown (Server.Run) can wait for
// all in-progress calls to drain.
//
// Error handling follows the MCP spec:
//   - Tool execution errors (model load failed, inference failed, image
//     not found) become CallToolResult.IsError=true with a textual
//     description. The LLM sees these and can self-correct.
//   - Protocol errors (bad args, unknown tool — handled by the SDK)
//     become JSON-RPC error responses.
func (s *Server) callTool(ctx context.Context, req *mcp.CallToolRequest, t tools.Tool) (*mcp.CallToolResult, error) {
	// In-flight counter (reserved for future observability). Currently
	// informational only; the inFlightWG in server.go drives graceful
	// shutdown.
	atomic.AddInt64(&inFlightCount, 1)
	defer atomic.AddInt64(&inFlightCount, -1)
	s.inFlightWG.Add(1)
	defer s.inFlightWG.Done()

	logger := s.logger.With(slog.String("tool", t.ID()))
	logger.Debug("tools/call received")

	// Parse the raw arguments. The SDK hands us json.RawMessage; we want
	// a map[string]any so we can pull image fields generically and pass
	// everything else through to the tool via ToolInput.Extra.
	args := map[string]any{}
	if len(req.Params.Arguments) > 0 {
		// Use json.Decoder with UseNumber() so big ints don't lose
		// precision (some image dimension args might be int64).
		dec := json.NewDecoder(strings.NewReader(string(req.Params.Arguments)))
		dec.UseNumber()
		if err := dec.Decode(&args); err != nil {
			logger.Warn("failed to parse arguments", "error", err, "raw", string(req.Params.Arguments))
			res := &mcp.CallToolResult{}
			res.SetError(fmt.Errorf("invalid arguments JSON: %w", err))
			return res, nil
		}
	}

	// Normalize image input. Returns ImageRefs and a "consumed" set of
	// keys we should strip from Extra so the tool doesn't see them twice.
	images, consumed, err := normalizeImages(args, t.ID())
	if err != nil {
		logger.Warn("image input rejected", "error", err)
		res := &mcp.CallToolResult{}
		res.SetError(err)
		return res, nil
	}
	// Temp-file cleanup. Two sets to reap, both safe on real user paths
	// (CleanupImageRefs only removes paths ParseImageRef/RegisterTemp registered):
	//   - the ORIGINAL `images` (registered here): reaps data: URI temps that an
	//     Expander discards — e.g. a data:-URI PDF decoded to a .bin temp that
	//     read_document rasterizes and replaces. No-op on real image_path files.
	//   - the EXPANDED `input.Images` (registered after ExpandInput below): reaps
	//     rasterized page temps and their out dir.
	// Registering ONLY on `images` would leak page temps; ONLY on input.Images
	// would leak a data:-URI PDF's .bin. Both run at return (idempotent).
	defer tools.CleanupImageRefs(images)

	// Build the ToolInput: pass through anything we didn't consume.
	extra := make(map[string]any, len(args)-len(consumed))
	for k, v := range args {
		if !consumed[k] {
			extra[k] = v
		}
	}
	input := tools.ToolInput{
		Images: images,
		Extra:  extra,
	}

	// Attach a progress sink so the model/binary downloads and inference can
	// report progress to the client as notifications/progress — but only if the
	// client opted in by sending a _meta.progressToken. No token ⇒ no
	// notifications (today's behavior, byte-for-byte). The sink forwards Updates
	// to the client in order via a single dispatcher goroutine, each send bounded
	// by a 2 s timeout so a slow or stalled client pipe can never stall the tool
	// call (NotifyProgress is a synchronous transport write in the SDK). Built
	// before expansion so the sink is available to a document Expander too. The
	// deferred close drains the buffer and reaps the dispatcher goroutine so it
	// never outlives the call.
	runCtx := ctx
	if token := req.Params.GetProgressToken(); token != nil {
		sink := newMcpProgressSink(req.Session, token)
		defer sink.close()
		runCtx = progress.WithSink(ctx, sink)
	}

	// Expand document inputs (e.g. read_document rasterizes a PDF into page
	// images). Non-Expanders are a no-op. The expanded refs REPLACE input.Images
	// and flow into both BuildRequest and the executor. The deferred
	// CleanupImageRefs reaps any temp page files (and their out dir) — it MUST
	// run on input.Images (the expanded set), so it is registered here after
	// expansion. On expansion failure we reap the original refs directly before
	// returning (the Expander is responsible for cleaning any partial temps it
	// created before erroring, e.g. read_document's RemoveAll(outDir)).
	input, err = tools.ExpandInput(runCtx, t, input)
	if err != nil {
		tools.CleanupImageRefs(images)
		logger.Warn("tool.ExpandImages failed", "error", err)
		res := &mcp.CallToolResult{}
		res.SetError(fmt.Errorf("expanding input for tool %q: %w", t.ID(), err))
		return res, nil
	}
	defer tools.CleanupImageRefs(input.Images)

	// Ask the tool to build the model request. This is where the tool's
	// task-specific prompt construction happens.
	systemPrompt, userPrompt, imagePaths, err := t.BuildRequest(input)
	if err != nil {
		logger.Warn("tool.BuildRequest failed", "error", err)
		res := &mcp.CallToolResult{}
		res.SetError(fmt.Errorf("building request for tool %q: %w", t.ID(), err))
		return res, nil
	}
	// imagePaths is advisory (a tool may re-order/filter); the executor uses the
	// expanded input.Images below.
	_ = imagePaths

	// Run inference via the executor. runCtx propagates to the lifecycle
	// manager and the HTTP request (so notifications/cancelled + graceful
	// shutdown both work) and carries the progress sink. Pass input.Images —
	// the EXPANDED refs (a PDF becomes its page images); using the pre-expansion
	// `images` here would send the un-rasterized PDF to the model.
	if s.executor == nil {
		res := &mcp.CallToolResult{}
		res.SetError(errExecutorUnavailable)
		return res, nil
	}

	raw, _, err := s.executor.Run(runCtx, t.ID(), systemPrompt, userPrompt, input.Images, t.MaxTokens())
	if err != nil {
		logger.Warn("executor returned error", "error", err)
		res := &mcp.CallToolResult{}

		// Distinguish "first-run setup needed" (catalog/lifecycle not
		// ready) from generic errors so the LLM gets a clear remediation
		// hint.
		msg := err.Error()
		if isSetupError(err) {
			msg = "localvision first-run setup required: " + msg +
				". Run `localvision doctor` to install the llama-server binary and download a model."
		}
		res.SetError(errors.New(msg))
		return res, nil
	}

	// Ask the tool to post-process the raw text. Most tools pass through;
	// some (extract_code, extract_table) do real work here.
	parsed, err := t.ParseOutput(input, raw)
	if err != nil {
		logger.Warn("tool.ParseOutput failed", "error", err)
		res := &mcp.CallToolResult{}
		res.SetError(fmt.Errorf("parsing output of tool %q: %w", t.ID(), err))
		return res, nil
	}

	// Format the response. If the tool returned a string, embed it
	// directly as TextContent. If it returned a structured value, embed
	// both the JSON serialization (for machine consumers) and a
	// pretty-printed fallback (for the LLM).
	return buildResult(parsed, raw), nil
}

// inFlightCount is reserved for future metrics/observability hooks. We
// don't currently expose it but the field is here so we can add a gauge
// without restructuring Server.
// nolint:unused
var inFlightCount int64

// buildResult formats the tool's parsed output as an MCP CallToolResult.
//
//   - string output: single TextContent with the string
//   - any other type: StructuredContent set, plus a TextContent with the
//     JSON-marshaled form (pretty-printed) so a text-only LLM consumer
//     can still see the content
func buildResult(parsed any, raw string) *mcp.CallToolResult {
	res := &mcp.CallToolResult{}

	if str, ok := parsed.(string); ok {
		res.Content = []mcp.Content{&mcp.TextContent{Text: str}}
		return res
	}

	// Structured output: send both the structured value and a text form.
	if parsed != nil {
		// StructuredContent must marshal to a JSON object. Wrap non-objects
		// (slices, numbers) in an object envelope so the SDK doesn't reject.
		if _, err := json.Marshal(parsed); err == nil {
			res.StructuredContent = map[string]any{"result": parsed}
		}
	}

	text := raw
	if pretty, err := json.MarshalIndent(parsed, "", "  "); err == nil && len(pretty) > 2 {
		text = string(pretty)
	}
	res.Content = []mcp.Content{&mcp.TextContent{Text: text}}
	return res
}

// normalizeImages extracts image references from the raw args according
// to F1.10 and F4.9.
//
// Accepted shapes:
//
//   - image_path: "/abs/path/to/img.png"   (primary; F1.10)
//   - image_data: "data:image/png;base64,..."   (fallback; bytes decoded
//     and written to a temp file, LocalPath = temp path)
//   - image_url: "file:///abs/path/to/img.png"   (file:// URI accepted)
//   - image_url: "https://example.com/img.png"   (REJECTED; llama-server
//     is localhost-only and can't fetch remote URLs)
//   - images: ["path1", {"path": "..."}, ...]   (array form, used by
//     compare_images per F4.9; elements may be strings or objects with
//     any of the single-image keys above)
//
// Returns the normalized ImageRef slice, the set of arg keys consumed
// (so the caller can strip them before passing the rest through as Extra),
// and an error if no usable image was supplied or a remote URL was found.
func normalizeImages(args map[string]any, toolID string) ([]tools.ImageRef, map[string]bool, error) {
	consumed := map[string]bool{}
	var refs []tools.ImageRef

	// Array form (compare_images).
	if raw, ok := args["images"]; ok {
		consumed["images"] = true
		arr, ok := raw.([]any)
		if !ok {
			return nil, consumed, fmt.Errorf("%w: \"images\" must be an array", errInvalidImageInput)
		}
		for i, el := range arr {
			ref, elConsumed, err := normalizeOneImage(el)
			if err != nil {
				// A later element is malformed, but earlier elements may
				// already have created temp files (data: URIs). Reap them so
				// a partially-valid images array doesn't leak (E6). This error
				// returns before callTool's cleanup defer is registered.
				tools.CleanupImageRefs(refs)
				return nil, consumed, fmt.Errorf("images[%d]: %w", i, err)
			}
			refs = append(refs, ref)
			// normalizeOneImage may have populated temp files; we don't
			// need elConsumed at the top level because the array was the
			// single arg key consumed.
			_ = elConsumed
		}
	}

	// Single-image form (read_image, extract_text, etc.).
	if len(refs) == 0 {
		ref, singleConsumed, err := normalizeOneImage(args)
		if err != nil {
			// If there were no image_* keys at all, fall through with no
			// refs; the tool's BuildRequest will surface the "missing
			// image" error if it cares.
			if !errors.Is(err, errNoImageProvided) {
				return nil, consumed, err
			}
		} else {
			refs = append(refs, ref)
			for k := range singleConsumed {
				consumed[k] = true
			}
		}
	}

	if len(refs) == 0 {
		return nil, consumed, fmt.Errorf("%w: no image supplied (expected one of image_path, image_data, image_url, or images)", errNoImageProvided)
	}

	return refs, consumed, nil
}

// normalizeOneImage handles a single image input, which may be a string
// (treated as image_path) or an object with one of the standard keys.
//
// Returns the normalized ImageRef plus the set of input keys that were
// consumed (so the top-level caller can strip them from Extra).
func normalizeOneImage(v any) (tools.ImageRef, map[string]bool, error) {
	consumed := map[string]bool{}

	// String form: assume it's a path or a URI. We distinguish by prefix.
	if s, ok := v.(string); ok {
		ref, err := refFromString(s, "image_path")
		if err != nil {
			return tools.ImageRef{}, consumed, err
		}
		return ref, consumed, nil
	}

	// Object form: look for image_path / image_data / image_url.
	m, ok := v.(map[string]any)
	if !ok {
		return tools.ImageRef{}, consumed, fmt.Errorf("%w: expected string or object, got %T", errInvalidImageInput, v)
	}

	// Try each key in priority order: path > data > url.
	if raw, ok := m["image_path"]; ok {
		consumed["image_path"] = true
		s, _ := raw.(string)
		ref, err := refFromString(s, "image_path")
		if err != nil {
			return tools.ImageRef{}, consumed, err
		}
		return ref, consumed, nil
	}
	if raw, ok := m["image_data"]; ok {
		consumed["image_data"] = true
		s, _ := raw.(string)
		ref, err := refFromString(s, "image_data")
		if err != nil {
			return tools.ImageRef{}, consumed, err
		}
		return ref, consumed, nil
	}
	if raw, ok := m["image_url"]; ok {
		consumed["image_url"] = true
		s, _ := raw.(string)
		ref, err := refFromString(s, "image_url")
		if err != nil {
			return tools.ImageRef{}, consumed, err
		}
		return ref, consumed, nil
	}

	// Maybe it's a path key with a different convention? Accept "path"
	// as an alias for ergonomics.
	if raw, ok := m["path"]; ok {
		consumed["path"] = true
		s, _ := raw.(string)
		ref, err := refFromString(s, "image_path")
		if err != nil {
			return tools.ImageRef{}, consumed, err
		}
		return ref, consumed, nil
	}

	return tools.ImageRef{}, consumed, fmt.Errorf("%w: no image_path/image_data/image_url key", errNoImageProvided)
}

// refFromString builds an ImageRef from a string input. sourceField tells
// us which field the user supplied (used in error messages).
func refFromString(s, sourceField string) (tools.ImageRef, error) {
	if s == "" {
		return tools.ImageRef{}, fmt.Errorf("%w: empty %s", errInvalidImageInput, sourceField)
	}

	// data: URI → delegate to tools.ParseImageRef, which decodes the bytes,
	// writes a temp file, registers it with tempRegistry (so CleanupImageRefs
	// reaps it after inference), and redacts Source for privacy. This keeps the
	// MCP path on the same normalization the CLI uses, instead of a private
	// duplicate that never registered its temp files (the v0.4 leak).
	if strings.HasPrefix(s, "data:") {
		ref, err := tools.ParseImageRef(s)
		if err != nil {
			return tools.ImageRef{}, fmt.Errorf("decoding data URI: %w", err)
		}
		return ref, nil
	}

	// file:// URI → extract the path component.
	if strings.HasPrefix(s, "file://") {
		u, err := url.Parse(s)
		if err != nil {
			return tools.ImageRef{}, fmt.Errorf("%w: malformed file:// URI: %v", errInvalidImageInput, err)
		}
		p := u.Path
		if p == "" {
			return tools.ImageRef{}, fmt.Errorf("%w: file:// URI has no path", errInvalidImageInput)
		}
		return tools.ImageRef{LocalPath: p, Source: s}, nil
	}

	// http(s):// URL → REJECTED. Per F1.10: llama-server is bound to
	// 127.0.0.1 and cannot fetch remote URLs.
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return tools.ImageRef{}, fmt.Errorf(
			"%w: remote URLs are not supported (llama-server is localhost-only and cannot fetch %q). "+
				"Download the image locally and pass it via image_path, or embed the bytes via image_data as a data: URI",
			errUnsupportedImageSource, s,
		)
	}

	// Otherwise: treat as a file path. Make it absolute so subprocess
	// argv is unambiguous (llama-server may have a different cwd).
	abs, err := filepath.Abs(s)
	if err != nil {
		return tools.ImageRef{}, fmt.Errorf("resolving path %q: %w", s, err)
	}
	return tools.ImageRef{LocalPath: abs, Source: s}, nil
}

// isSetupError returns true if err is one of the "first-run setup needed"
// errors — i.e. the lifecycle or catalog isn't ready yet. We use this to
// produce a more helpful MCP error message guiding the user to run doctor.
func isSetupError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "first-run setup") ||
		strings.Contains(msg, "not implemented") ||
		strings.Contains(msg, "no model") ||
		strings.Contains(msg, "loading model")
}

// Image-input errors. Wrapped rather than sentinel because the wrap
// context (which key, which index) is part of the message.
var (
	errInvalidImageInput      = errors.New("invalid image input")
	errNoImageProvided        = errors.New("no image provided")
	errUnsupportedImageSource = errors.New("unsupported image source")
)

// progressNotifyTimeout bounds how long a single NotifyProgress send may take
// before the sink gives up on it. NotifyProgress is a SYNCHRONOUS transport
// write in the SDK (its handler calls the connection's Notify directly), so
// without a timeout a slow or stalled client stdin pipe could block the
// dispatcher and back up subsequent updates. Progress is best-effort: dropping
// an update on timeout is acceptable.
const progressNotifyTimeout = 2 * time.Second

// progressSinkBufferSize caps how many updates the dispatcher buffers. Updates
// beyond this are dropped (non-blocking send) — progress is best-effort and a
// burst faster than the transport can emit should not grow an unbounded queue.
const progressSinkBufferSize = 32

// progressCloseDeadline bounds the TOTAL time close() may spend draining the
// buffer on callTool return. A stuck client pipe could otherwise stall each
// send for progressNotifyTimeout; with a full buffer that is unacceptably
// long. Once the deadline fires, remaining buffered updates are abandoned
// (progress is best-effort) and the dispatcher exits so callTool returns.
const progressCloseDeadline = 5 * time.Second

// mcpProgressSink adapts a progress.Sink to the MCP notifications/progress
// transport. All Updates are forwarded to the client via a SINGLE dispatcher
// goroutine that sends them in order through ServerSession.NotifyProgress,
// each bounded by progressNotifyTimeout. This preserves delivery order (a
// client progress bar must not jump backwards) while never blocking the tool
// call: NotifyProgress is a synchronous transport write in the SDK, so a slow
// or stalled client stdin pipe could otherwise stall the producer. Progress is
// best-effort: updates dropped on a full buffer, a send timeout, or the close
// deadline are acceptable. close() drains the buffer (best-effort, bounded by
// progressCloseDeadline) and must be called when the owning callTool returns so
// the dispatcher goroutine does not outlive the call.
type mcpProgressSink struct {
	session *mcp.ServerSession
	token   any
	ch      chan progress.Update
	done    chan struct{}
	stopCh  chan struct{} // closed by close() to signal the dispatcher to abandon remaining sends
	once    sync.Once
}

// newMcpProgressSink builds a sink that forwards Updates to session as
// notifications/progress tagged with token. The dispatcher goroutine is
// started here and runs until close() is called.
func newMcpProgressSink(session *mcp.ServerSession, token any) *mcpProgressSink {
	s := &mcpProgressSink{
		session: session,
		token:   token,
		ch:      make(chan progress.Update, progressSinkBufferSize),
		done:    make(chan struct{}),
		stopCh:  make(chan struct{}),
	}
	go s.dispatch()
	return s
}

// Progress enqueues u for ordered delivery. It is non-blocking: if the buffer
// is full (the client is slower than the producer), u is dropped — progress is
// best-effort and a flood must not back-pressure the tool call.
//
// It is safe to call concurrently with close(): close() closes s.ch, and a send
// on a closed channel panics regardless of a non-blocking select, so the send is
// guarded by a recover. Producers (lifecycle phase callbacks, the heartbeat
// goroutine) join before close() runs in the current call graph, but this keeps
// the sink's "concurrency-safe" contract honest if that ordering ever changes.
func (s *mcpProgressSink) Progress(u progress.Update) {
	defer func() { _ = recover() }() // close() may close s.ch concurrently
	select {
	case s.ch <- u:
	default:
		// Buffer full: drop. A later update carries newer progress anyway.
	}
}

// close drains any buffered updates (bounded by progressCloseDeadline total)
// and stops the dispatcher goroutine. Idempotent. If the deadline fires first
// (a stuck client pipe), remaining updates are abandoned and the dispatcher
// exits so callTool returns promptly.
func (s *mcpProgressSink) close() {
	s.once.Do(func() {
		close(s.ch)
		// Bound the drain: if the dispatcher is stuck on a slow send, give up
		// after progressCloseDeadline and let the goroutine wind down on its
		// own (it checks stopCh between sends).
		timer := time.NewTimer(progressCloseDeadline)
		defer timer.Stop()
		select {
		case <-s.done:
		case <-timer.C:
			close(s.stopCh)
			<-s.done
		}
	})
}

// dispatch is the single writer goroutine. It sends updates in FIFO order, each
// with its own timeout, until the channel is closed (by close()) — then it
// drains any remaining buffered updates before exiting. It watches stopCh so a
// close() that hits progressCloseDeadline can abort a stuck drain.
func (s *mcpProgressSink) dispatch() {
	defer close(s.done)
	for {
		select {
		case <-s.stopCh:
			return
		case u, ok := <-s.ch:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), progressNotifyTimeout)
			_ = s.session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
				ProgressToken: s.token,
				Progress:      u.Current,
				Total:         u.Total,
				Message:       mcpProgressMessage(u),
			})
			cancel()
		}
	}
}

// mcpProgressMessage renders a short human label for a progress Update (the
// notifications/progress Message field).
func mcpProgressMessage(u progress.Update) string {
	if u.Message != "" {
		return u.Message
	}
	if u.Detail != "" {
		return u.Phase + " " + u.Detail
	}
	return u.Phase
}
