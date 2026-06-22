//go:build !darwin && !linux && !windows

package models

import "errors"

// freeSpace on unsupported platforms returns an error so callers (DiskFreeHuman)
// fall back to "(unknown)" rather than failing the whole operation.
func freeSpace(path string) (int64, error) {
	return 0, errors.New("disk-space detection not supported on this platform")
}
