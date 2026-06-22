//go:build darwin

package main

import "golang.org/x/sys/unix"

// isTerminal reports whether fd is a terminal. darwin/bsd use TIOCGETA.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TIOCGETA)
	return err == nil
}
