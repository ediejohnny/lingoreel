//go:build windows

package stillcache

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// freeRoom is how much room the disk holding dir has for the app.
func freeRoom(dir string) (int64, error) {
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(path, &available, &total, &free); err != nil {
		return 0, err
	}
	return int64(min(available, 1<<62)), nil
}

// diskFull tells an error of a disk with no room left.
func diskFull(err error) bool {
	return errors.Is(err, windows.ERROR_DISK_FULL) || errors.Is(err, windows.ERROR_HANDLE_DISK_FULL) || errors.Is(err, syscall.ENOSPC)
}
