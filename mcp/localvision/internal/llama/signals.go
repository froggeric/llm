package llama

import "os/exec"

// signals.go holds the cross-platform sigkill. The OS-specific sigterm and
// processAlive live in signals_unix.go and signals_windows.go (build-tagged).
//
// Why the split: the untagged, syscall-based versions that used to live here
// compiled on Windows but silently no-op'd there — os.Process.Signal returns
// EWINDOWS for anything but SIGKILL/Interrupt, so SIGTERM did nothing and the
// signal-0 processAlive probe always reported "dead". killSubprocessLocked then
// skipped its SIGKILL escalation, leaving idle-killed llama-server subprocesses
// orphaned on Windows (leaking VRAM). Per-OS files make the kill path actually
// work on every target.
//
// lifecycle.go is unchanged: killSubprocessLocked / killIfIdle / unloadLocked /
// Shutdown call sigterm / processAlive / sigkill, which now resolve per-OS via
// build tags.

// sigkill forcefully terminates the subprocess. Cross-platform: SIGKILL on POSIX
// (syscall.Kill), TerminateProcess on Windows — both reached through
// os.Process.Kill. Callers ignore the error; the watcher goroutine
// (watchSubprocess) is the single reaper (it owns cmd.Wait) and observes the
// exit regardless.
func sigkill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
