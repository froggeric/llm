//go:build !darwin && !linux && !windows

package main

// isTerminal on unsupported platforms (freebsd etc.) conservatively reports
// false so output stays plain.
func isTerminal(fd uintptr) bool { return false }
