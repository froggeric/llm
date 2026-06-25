package mcpserver

import (
	"testing"

	"github.com/froggeric/llm/mcp/localvision/internal/progress"
	"github.com/stretchr/testify/require"
)

// progress_sink_test.go pins the close-safety of mcpProgressSink.Progress
// (Tier-2 B1): a send on a closed channel panics regardless of a non-blocking
// select, so Progress must be safe to call after close().
//
// Note on the concurrent case: a producer calling Progress() concurrently with
// close() is exactly the scenario the recover guards, but it is NOT testable
// under `go test -race` — the race detector flags the deliberate send/close
// concurrency as a data race independently of the recover (the recover catches
// the panic; the detector still reports the underlying concurrent channel
// access). So this regression guard exercises the post-close case sequentially:
// without the recover, the send panics and the test fails; with it, it doesn't.

func newSinkForTest(buffer int) *mcpProgressSink {
	return &mcpProgressSink{
		ch:     make(chan progress.Update, buffer),
		done:   make(chan struct{}),
		stopCh: make(chan struct{}),
	}
}

func TestProgress_AfterCloseDoesNotPanic(t *testing.T) {
	s := newSinkForTest(2)
	close(s.ch) // the race window: a producer calls Progress after close(s.ch)

	require.NotPanics(t, func() {
		for i := 0; i < 100; i++ {
			s.Progress(progress.Update{Phase: "inferring"})
		}
	})
}
