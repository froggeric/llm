package progress

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingSink captures every Update it receives. Safe for concurrent use.
type recordingSink struct {
	mu      sync.Mutex
	updates []Update
}

func (r *recordingSink) Progress(u Update) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, u)
}

func (r *recordingSink) snapshot() []Update {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Update, len(r.updates))
	copy(out, r.updates)
	return out
}

func TestSinkFromNilCtx(t *testing.T) {
	assert.Nil(t, SinkFrom(nil), "SinkFrom(nil ctx) must be nil, not panic")
	assert.Nil(t, SinkFrom(context.Background()), "absent sink must be nil")
}

func TestWithSinkRoundTrip(t *testing.T) {
	s := &recordingSink{}
	ctx := WithSink(context.Background(), s)
	got := SinkFrom(ctx)
	require.NotNil(t, got)
	got.Progress(Update{Phase: "downloading", Current: 100})
	assert.Equal(t, []Update{{Phase: "downloading", Current: 100}}, s.snapshot())
}

func TestWithSinkNilIsNoop(t *testing.T) {
	ctx := WithSink(context.Background(), nil)
	assert.Nil(t, SinkFrom(ctx), "WithSink(nil) must not attach anything")
}

func TestReportIsNilSafe(t *testing.T) {
	// No sink attached: Report must not panic.
	assert.NotPanics(t, func() {
		Report(context.Background(), Update{Phase: "inferring"})
	})
	// With a sink: Report forwards.
	s := &recordingSink{}
	Report(WithSink(context.Background(), s), Update{Phase: "inferring", Current: 5})
	require.Len(t, s.snapshot(), 1)
}

func TestNoopSink(t *testing.T) {
	assert.NotPanics(t, func() { Noop().Progress(Update{}) })
}

func TestThrottledCoalesces(t *testing.T) {
	s := &recordingSink{}
	throttled := Throttled(s, 50*time.Millisecond)
	// Fire faster than the window: only the first should land.
	for i := 0; i < 10; i++ {
		throttled.Progress(Update{Current: float64(i)})
	}
	assert.Len(t, s.snapshot(), 1, "burst within the window must coalesce to one")
	// After the window, another should land.
	time.Sleep(60 * time.Millisecond)
	throttled.Progress(Update{Current: 99})
	assert.Len(t, s.snapshot(), 2)
}

func TestThrottledNilSink(t *testing.T) {
	assert.Nil(t, Throttled(nil, time.Second), "Throttled(nil) must stay nil-safe")
}

func TestHeartbeatNilSinkNoGoroutine(t *testing.T) {
	// A nil sink must return a no-op stop without spawning a ticker goroutine.
	stop := Heartbeat(context.Background(), nil, "inferring", "m", 10)
	stop() // must be safe
	// Nothing to assert but non-panic + prompt return; this mainly guards that
	// the nil path short-circuits before launching a goroutine.
}

func TestHeartbeatTicksAndStops(t *testing.T) {
	// Shrink the interval so the test is fast.
	prev := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = prev })

	s := &recordingSink{}
	stop := Heartbeat(context.Background(), s, "inferring", "qwen3-vl-8b", 60)
	defer stop()

	// Initial emit is immediate; let ~3 ticks land.
	time.Sleep(45 * time.Millisecond)
	stop()

	got := s.snapshot()
	require.GreaterOrEqual(t, len(got), 3, "should have emitted an initial + several ticks")
	// Every update carries the phase/detail and the elapsed unit.
	for _, u := range got {
		assert.Equal(t, "inferring", u.Phase)
		assert.Equal(t, "qwen3-vl-8b", u.Detail)
		assert.Equal(t, "s", u.Unit)
		assert.Equal(t, float64(60), u.Total)
	}
	// Elapsed should be non-decreasing and modest.
	assert.GreaterOrEqual(t, got[len(got)-1].Current, got[0].Current)
	assert.Less(t, got[len(got)-1].Current, 5.0, "elapsed should still be small")

	// stop() is idempotent.
	stop()
}

func TestHeartbeatStopsOnCtxCancel(t *testing.T) {
	prev := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = prev })

	var count atomic.Int64
	s := Func(func(Update) { count.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	stop := Heartbeat(ctx, s, "inferring", "m", 10)
	defer stop()

	cancel()
	time.Sleep(60 * time.Millisecond)
	afterCancel := count.Load()
	// No further ticks after cancel.
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, afterCancel, count.Load(), "no ticks after ctx cancel")
}
