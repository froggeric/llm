//go:build darwin || linux

package models

import "golang.org/x/sys/unix"

// freeSpace returns the free bytes available to unprivileged users on the
// filesystem holding path (darwin/linux via statfs). Both expose Bavail/Bsize
// on Statfs_t.
func freeSpace(path string) (int64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
