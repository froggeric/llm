package llama

import (
	"os/exec"
	"syscall"
)

// signals.go provides platform-safe wrappers around subprocess signaling.
// On POSIX (the only platform this MVP supports per F2.2), SIGTERM and
// SIGKILL are well-known. We centralize them here so the lifecycle stays
// readable and so tests can replace them via build tags if needed.

// sigterm sends SIGTERM to the subprocess. Errors are ignored — the caller
// will detect a still-alive process and escalate to SIGKILL.
func sigterm(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(syscall.SIGTERM)
}

// sigkill sends SIGKILL to the subprocess. Errors are ignored.
func sigkill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(syscall.SIGKILL)
}

// processAlive reports whether the subprocess is still running. We probe by
// sending signal 0 (POSIX "signal exists but no signal sent"). If the
// process has exited AND been reaped, the call returns an error.
//
// Caveat: a zombie process (exited but not yet waited on) still answers
// signal 0. Callers that need a definitive "is the process gone" answer
// must call cmd.Wait() (which the lifecycle's watcher goroutine does).
// processAlive is used in the lifecycle to poll for SIGTERM effectiveness
// before escalating to SIGKILL — a zombie will be reaped by the watcher
// soon enough that this approximation is safe.
func processAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	// signal 0 is the POSIX idiom for "is this PID alive?".
	if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
		return true
	}
	return false
}
