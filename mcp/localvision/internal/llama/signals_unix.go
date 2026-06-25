//go:build !windows

package llama

import (
	"os/exec"

	"golang.org/x/sys/unix"
)

// sigterm sends SIGTERM for a graceful shutdown attempt. killSubprocessLocked
// then waits up to SIGKILLAfter and escalates to sigkill if the process is
// still alive.
func sigterm(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(unix.SIGTERM)
}

// processAlive reports whether the subprocess is still running, probed via the
// POSIX signal-0 idiom. A zombie (exited but not yet reaped) still answers
// signal 0; the watcher goroutine reaps it via cmd.Wait(), so this approximation
// is safe for killSubprocessLocked's poll-then-escalate pattern.
func processAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	return cmd.Process.Signal(unix.Signal(0)) == nil
}
