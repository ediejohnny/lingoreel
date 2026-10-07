//go:build windows

package diskfault

import (
	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/fault"
)

// codes are the system's errors for each kind, Windows' first.
var codes = map[fault.Kind][]error{
	fault.DiskFull: append([]error{windows.ERROR_DISK_FULL, windows.ERROR_HANDLE_DISK_FULL}, portable[fault.DiskFull]...),
	fault.InUse:    {windows.ERROR_SHARING_VIOLATION, windows.ERROR_LOCK_VIOLATION, windows.ERROR_USER_MAPPED_FILE},
	fault.ReadOnly: append([]error{windows.ERROR_WRITE_PROTECT}, portable[fault.ReadOnly]...),
	fault.Unavailable: {
		windows.ERROR_PATH_NOT_FOUND, windows.ERROR_INVALID_DRIVE, windows.ERROR_NOT_READY, windows.ERROR_DEV_NOT_EXIST,
		windows.ERROR_BAD_NETPATH, windows.ERROR_NETNAME_DELETED, windows.ERROR_UNEXP_NET_ERR, windows.ERROR_BAD_NET_NAME,
	},
	fault.TooLong: append([]error{windows.ERROR_FILENAME_EXCED_RANGE}, portable[fault.TooLong]...),
}
