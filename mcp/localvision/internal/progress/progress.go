// Package progress carries an optional, context-scoped progress Sink through
// the inference and download paths so the CLI spinner and MCP
// notifications/progress can report byte / token / elapsed progress.
//
// It is deliberately a ctx-carried value rather than a parameter on
// tools.Executor.Run. Executor.Run already takes ctx and is shared by many test
// mocks (recordingExecutor, pathCapturingExecutor); adding a Sink parameter
// would ripple into all of them and the locked tools.Executor contract. The sink
// is the *only* ctx value carried this way; if a second is ever needed, promote
// the sink to an explicit Run parameter then.
//
// Everything here is nil-safe: when no sink is attached to the ctx (the default
// — e.g. today's tests, or an MCP client that sent no _meta.progressToken),
// Report/Heartbeat are no-ops and existing behavior is unchanged.
package progress

import (
	"context"
	"sync"
	"time"
)

// Update is one progress report. Fields are deliberately generic so the same
// type serves downloads (Current/Total = bytes, Unit = "bytes"), inference
// (Current = elapsed seconds, Total = soft budget seconds, Unit = "s"), and
// plain phase transitions (only Phase/Detail set).
type Update struct {
	// Phase is the lifecycle stage: "downloading", "loading", "ready", "inferring".
	Phase string
	// Detail carries context (e.g. the model display name, the file label).
	Detail string
	// Current is progress so far (bytes, seconds, or tokens).
	Current float64
	// Total is the total to reach (bytes or budget seconds); 0 means unknown.
	Total float64
	// Unit is "bytes", "s", "tokens", or "" (for phase-only updates).
	Unit string
	// Message is free-form text for the client (e.g. "inferring…").
	Message string
}

// Sink receives Update values. Implementations must be safe for concurrent use.
type Sink interface {
	Progress(Update)
}

// Func adapts a plain function to Sink.
type Func func(Update)

// Progress implements Sink.
func (f Func) Progress(u Update) { f(u) }

type noopSink struct{}

// Progress implements Sink (discards).
func (noopSink) Progress(Update) {}

// Noop returns a Sink that discards every Update.
func Noop() Sink { return noopSink{} }

type ctxKey struct{}

// WithSink returns a copy of ctx carrying sink. A nil sink returns ctx
// unchanged (so callers can pass SinkFrom(src) without branching).
func WithSink(ctx context.Context, sink Sink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, sink)
}

// SinkFrom returns the Sink attached to ctx by WithSink, or nil if none.
func SinkFrom(ctx context.Context) Sink {
	if ctx == nil {
		return nil
	}
	if s, ok := ctx.Value(ctxKey{}).(Sink); ok {
		return s
	}
	return nil
}

// Report sends u to the sink attached to ctx, if any. No-op when no sink is
// attached. This is the helper most call sites want: it collapses the nil check.
func Report(ctx context.Context, u Update) {
	if s := SinkFrom(ctx); s != nil {
		s.Progress(u)
	}
}

// heartbeatInterval is the cadence at which Heartbeat emits an elapsed update.
// It is a package-level var (not a const) so tests can shrink it.
var heartbeatInterval = 2 * time.Second

// Heartbeat spawns a goroutine that emits an {phase, detail, Current=elapsed,
// Unit:"s", Total=budgetSec} update immediately and then every heartbeatInterval
// until stop is called or ctx is cancelled. It returns the stop function; stop
// is idempotent and safe to defer. A nil sink returns a no-op stop without
// spawning a goroutine (so callers on a sink-less ctx pay nothing).
//
// budgetSec is a soft, UX-only estimate (e.g. expected generation time); it is
// surfaced as Total so clients can show a rough fraction. Accuracy is secondary
// to "something is happening" — see the v0.6 plan, decision 1.
func Heartbeat(ctx context.Context, sink Sink, phase, detail string, budgetSec float64) (stop func()) {
	if sink == nil {
		return func() {}
	}
	start := time.Now()
	stopCh := make(chan struct{}) // stop() closes this to signal shutdown
	exited := make(chan struct{}) // the goroutine closes this on exit
	var once sync.Once
	stop = func() {
		once.Do(func() {
			close(stopCh)
			<-exited // join: never let the ticker goroutine outlive stop()
		})
	}

	emit := func() {
		sink.Progress(Update{
			Phase:   phase,
			Detail:  detail,
			Current: time.Since(start).Seconds(),
			Total:   budgetSec,
			Unit:    "s",
			Message: phase,
		})
	}

	go func() {
		defer close(exited)
		emit()
		t := time.NewTicker(heartbeatInterval)
		defer t.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				emit()
			}
		}
	}()
	return stop
}

// Throttled returns a Sink that forwards at most one Update per minInterval,
// dropping intermediate bursts. It is safe for concurrent use. A nil sink
// returns nil (so the result is still nil-safe at call sites). The downloader
// already throttles its own byte callbacks, so this is only needed for paths
// that can emit faster than the transport wants (e.g. a future token stream).
func Throttled(sink Sink, minInterval time.Duration) Sink {
	if sink == nil {
		return nil
	}
	var mu sync.Mutex
	last := time.Time{}
	return Func(func(u Update) {
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		if now.Sub(last) >= minInterval {
			last = now
			sink.Progress(u)
		}
	})
}
