//go:build windows

package main

import "golang.org/x/sys/windows"

// isTerminal reports whether fd is a console. Windows has no termios ioctl;
// GetConsoleMode succeeds only on console handles.
func isTerminal(fd uintptr) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(fd), &mode) == nil
}
