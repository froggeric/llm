package llama

import (
	"errors"
	"fmt"
	"strings"
)

// errors.go defines typed errors surfaced by the lifecycle, client, and
// binary-management code. Each is a struct (not a sentinel) so callers can
// type-assert and pull structured context (last 1KB of stderr, port, hashes)
// for the MCP error path and doctor output.

// ErrNotRunning is returned by Client.ChatVision when the underlying
// subprocess is no longer in StateReady (crashed, stopped, or never started).
// Callers should treat this as a transient condition: a follow-up Acquire
// will respawn the subprocess.
type ErrNotRunning struct {
	// State is the lifecycle state observed at call time.
	State State
	// ModelID is the model the client was bound to, if known.
	ModelID string
}

func (e *ErrNotRunning) Error() string {
	if e == nil {
		return "not running"
	}
	return fmt.Sprintf("llama-server not running (state=%s, model=%q)", e.State, e.ModelID)
}

// IsErrNotRunning reports whether err is an *ErrNotRunning.
func IsErrNotRunning(err error) bool {
	var e *ErrNotRunning
	return errors.As(err, &e)
}

// ErrCrashed is returned when the subprocess exited unexpectedly. It carries
// the last 1KB of stderr (or whatever fit in the limited ring buffer) so the
// MCP client / doctor command can surface a useful diagnostic.
//
// F1.4 / F3.7: this is populated by the watcher goroutine that calls
// cmd.Wait() immediately after cmd.Start().
type ErrCrashed struct {
	// ModelID is the model that was loaded when the crash happened.
	ModelID string
	// ExitErr is the error returned by cmd.Wait() (typically an
	// *exec.ExitError, but may be a stdlib error if the process couldn't
	// be reaped).
	ExitErr error
	// StderrTail is the last ~1KB of stderr captured by the watcher
	// goroutine. May be empty if the subprocess was killed without
	// emitting anything.
	StderrTail string
}

func (e *ErrCrashed) Error() string {
	if e == nil {
		return "subprocess crashed"
	}
	tail := e.StderrTail
	if len(tail) > 256 {
		tail = tail[len(tail)-256:]
	}
	exitStr := "unknown"
	if e.ExitErr != nil {
		exitStr = e.ExitErr.Error()
	}
	if strings.TrimSpace(tail) == "" {
		return fmt.Sprintf("llama-server crashed (model=%q, exit=%s); no stderr captured",
			e.ModelID, exitStr)
	}
	return fmt.Sprintf("llama-server crashed (model=%q, exit=%s); last stderr:\n%s",
		e.ModelID, exitStr, tail)
}

func (e *ErrCrashed) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.ExitErr
}

// IsErrCrashed reports whether err is an *ErrCrashed.
func IsErrCrashed(err error) bool {
	var e *ErrCrashed
	return errors.As(err, &e)
}

// ErrTimeout is returned by Shutdown when the subprocess fails to exit
// within the configured graceful-shutdown window. The caller may treat this
// as "best effort" — the subprocess has been sent SIGKILL and reaping is
// the OS's responsibility at this point.
type ErrTimeout struct {
	// Op names the operation that timed out (e.g. "shutdown", "spawn").
	Op string
	// ModelID is the model that was active, if any.
	ModelID string
}

func (e *ErrTimeout) Error() string {
	if e == nil {
		return "timed out"
	}
	if e.ModelID == "" {
		return fmt.Sprintf("llama-server %s timed out", e.Op)
	}
	return fmt.Sprintf("llama-server %s timed out (model=%q)", e.Op, e.ModelID)
}

// ErrIntegrityFailStruct is the structured form of the package-level
// ErrIntegrityFail sentinel. It carries actual vs. expected hashes plus the
// path so the doctor command can tell the user exactly which file failed
// and how. F1.5: integrity is verified on every load, not just download.
//
// Note: the legacy package-level sentinel ErrIntegrityFail (defined in
// lifecycle.go for back-compat with the contract phase) is preserved for
// callers that only need errors.Is(err, ErrIntegrityFail) checks. The
// structured form satisfies errors.Is by Unwrap()ping to the sentinel.
type ErrIntegrityFailStruct struct {
	// Path is the file whose hash did not match.
	Path string
	// Expected is the normalized expected hex hash.
	Expected string
	// Actual is the normalized computed hex hash.
	Actual string
}

func (e *ErrIntegrityFailStruct) Error() string {
	if e == nil {
		return ErrIntegrityFail.Error()
	}
	return fmt.Sprintf("integrity check failed for %s: expected %s, got %s",
		e.Path, e.Expected, e.Actual)
}

func (e *ErrIntegrityFailStruct) Unwrap() error { return ErrIntegrityFail }

// ErrPortInUse is returned by the spawner when llama-server fails to bind
// the chosen port. The lifecycle catches this by sampling a new port and
// retrying up to 3 times before surfacing it. F4.7.
type ErrPortInUse struct {
	// Port is the port we tried to bind.
	Port int
	// Attempts is how many port-sampling retries we did.
	Attempts int
}

func (e *ErrPortInUse) Error() string {
	if e == nil {
		return "port in use"
	}
	return fmt.Sprintf("could not find a free port for llama-server after %d attempts (last tried %d)",
		e.Attempts, e.Port)
}

// IsErrPortInUse reports whether err is an *ErrPortInUse.
func IsErrPortInUse(err error) bool {
	var e *ErrPortInUse
	return errors.As(err, &e)
}

// limitedBuffer is an io.Writer that keeps only the last `max` bytes written
// to it. Used as cmd.Stderr so the crash-watcher can surface a useful tail
// without unbounded memory growth. F1.4 / F3.7.
//
// It is safe for concurrent use: cmd.Wait() reads tail concurrently with
// the spawner's writes.
type limitedBuffer struct {
	max int
	buf []byte
}

// newLimitedBuffer returns a buffer that retains at most the last max bytes.
// max <= 0 is treated as 1024 (the documented default for stderr capture).
func newLimitedBuffer(max int) *limitedBuffer {
	if max <= 0 {
		max = 1024
	}
	return &limitedBuffer{max: max}
}

// Write appends p to the buffer, evicting from the head if the result would
// exceed max. Returns len(p), nil always (we never drop writes from the
// caller's perspective; we just forget old bytes).
func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	// If a single Write exceeds max, keep only the tail.
	if len(p) >= b.max {
		start := len(p) - b.max
		b.buf = append(b.buf[:0], p[start:]...)
		return len(p), nil
	}
	b.buf = append(b.buf, p...)
	// Trim from the head if over capacity.
	if len(b.buf) > b.max {
		b.buf = b.buf[len(b.buf)-b.max:]
	}
	return len(p), nil
}

// Tail returns a copy of the retained bytes.
func (b *limitedBuffer) Tail() []byte {
	out := make([]byte, len(b.buf))
	copy(out, b.buf)
	return out
}

// String returns the retained bytes as a string. Convenience for tests and
// error formatting.
func (b *limitedBuffer) String() string {
	return string(b.Tail())
}

// Reset clears the buffer. Used by tests.
func (b *limitedBuffer) Reset() {
	b.buf = b.buf[:0]
}
