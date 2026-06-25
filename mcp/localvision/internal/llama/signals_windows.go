//go:build windows

package llama

import (
	"os/exec"

	"golang.org/x/sys/windows"
)

// stillActive is the Win32 STILL_ACTIVE exit-code sentinel (macro value 259).
// golang.org/x/sys/windows does not export this constant, so define it locally.
const stillActive = 259

// sigterm is a no-op on Windows: there is no graceful signal for a spawned
// subprocess (os.Process.Signal returns EWINDOWS for anything but
// SIGKILL/Interrupt). killSubprocessLocked escalates to sigkill (which calls
// TerminateProcess via os.Process.Kill) after SIGKILLAfter.
func sigterm(cmd *exec.Cmd) error {
	return nil
}

// processAlive reports whether the subprocess is still running on Windows via
// OpenProcess + GetExitCodeProcess.
//
// The handle from a successful OpenProcess MUST be closed (defer CloseHandle):
// killSubprocessLocked's 50ms poll loop plus the post-timeout probe would
// otherwise leak ~100 handles per kill cycle — unbounded over a long-running
// server.
//
// Returning a real liveness answer (not a constant false) is what lets the
// SIGKILL escalation fire: killSubprocessLocked only sends sigkill when
// processAlive is true after the SIGKILLAfter grace window. A constant false
// made it conclude the process was already dead and skip the kill — the root
// cause of the Windows orphan.
func processAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(cmd.Process.Pid))
	if err != nil {
		return false // process gone (ERROR_INVALID_PARAMETER on a dead/recycled PID)
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
