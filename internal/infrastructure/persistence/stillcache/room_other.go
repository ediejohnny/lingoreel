//go:build !windows

package stillcache

import (
	"errors"
	"syscall"
)

// freeRoom is unknown until each system tells it: the stills are then
// bounded by their own limits and by the disk filling up.
func freeRoom(string) (int64, error) { return 0, errors.ErrUnsupported }

// diskFull tells an error of a disk with no room left.
func diskFull(err error) bool { return errors.Is(err, syscall.ENOSPC) }
