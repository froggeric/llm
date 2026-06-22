//go:build windows

package models

import "golang.org/x/sys/windows"

// freeSpace returns the free bytes available to the caller on the volume
// holding path (Windows via GetDiskFreeSpaceEx).
func freeSpace(path string) (int64, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return 0, err
	}
	return int64(freeBytesAvailable), nil
}
