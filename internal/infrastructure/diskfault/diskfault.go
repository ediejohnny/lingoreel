// Package diskfault tells why writing, renaming or removing a file failed,
// from the error the system gave, as a fault.Kind the screens can word.
package diskfault

import (
	"errors"
	"io/fs"
	"syscall"

	"github.com/ediejohnny/lingoreel/internal/domain/fault"
)

// Of tells why err happened; Other when no kind fits, None for no error.
func Of(err error) fault.Kind {
	if err == nil {
		return fault.None
	}
	for _, kind := range []fault.Kind{fault.DiskFull, fault.InUse, fault.ReadOnly, fault.Unavailable, fault.TooLong} {
		for _, target := range codes[kind] {
			if errors.Is(err, target) {
				return kind
			}
		}
	}
	// Last: on Windows a file held by another program can also come back as
	// access denied, and the codes above tell it first when they can.
	if errors.Is(err, fs.ErrPermission) {
		return fault.Denied
	}
	return fault.Other
}

// portable are the codes every system shares, beside each system's own.
var portable = map[fault.Kind][]error{
	fault.DiskFull: {syscall.ENOSPC},
	fault.ReadOnly: {syscall.EROFS},
	fault.TooLong:  {syscall.ENAMETOOLONG},
}
