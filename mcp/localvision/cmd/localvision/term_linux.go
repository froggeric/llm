//go:build linux

package main

import "golang.org/x/sys/unix"

// isTerminal reports whether fd is a terminal. linux uses TCGETS.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}
