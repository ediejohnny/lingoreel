package diskfault_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/fault"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/diskfault"
)

// The system's codes come back as the kinds the screens word, however deep
// they are wrapped.
func TestTheSystemsCodesTellTheKind(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows' own codes")
	}
	wrap := func(code error) error {
		return fmt.Errorf("saving: %w", &fs.PathError{Op: "write", Path: `C:\x`, Err: code})
	}
	cases := []struct {
		err  error
		want fault.Kind
	}{
		{nil, fault.None},
		{errors.New("something else"), fault.Other},
		{wrap(windows.ERROR_DISK_FULL), fault.DiskFull},
		{wrap(windows.ERROR_HANDLE_DISK_FULL), fault.DiskFull},
		{wrap(syscall.ENOSPC), fault.DiskFull},
		{wrap(windows.ERROR_ACCESS_DENIED), fault.Denied},
		{wrap(windows.ERROR_SHARING_VIOLATION), fault.InUse},
		{wrap(windows.ERROR_LOCK_VIOLATION), fault.InUse},
		{wrap(windows.ERROR_WRITE_PROTECT), fault.ReadOnly},
		{wrap(windows.ERROR_NOT_READY), fault.Unavailable},
		{wrap(windows.ERROR_PATH_NOT_FOUND), fault.Unavailable},
		{wrap(windows.ERROR_NETNAME_DELETED), fault.Unavailable},
		{wrap(windows.ERROR_FILENAME_EXCED_RANGE), fault.TooLong},
		{errors.Join(errors.New("first"), wrap(windows.ERROR_DISK_FULL)), fault.DiskFull},
	}
	for _, c := range cases {
		if got := diskfault.Of(c.err); got != c.want {
			t.Errorf("Of(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

// Real failures on this computer: a file another program holds without
// sharing it is in use, and a file marked read-only is denied.
func TestRealFailuresTellTheKind(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows' own sharing")
	}
	dir := t.TempDir()
	held := filepath.Join(dir, "held.json")
	if err := os.WriteFile(held, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(held)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windows.CloseHandle(handle) })
	if got := diskfault.Of(os.WriteFile(held, []byte("[]"), 0o600)); got != fault.InUse {
		t.Errorf("writing a file held without sharing = %v, want InUse", got)
	}

	locked := filepath.Join(dir, "locked.json")
	if err := os.WriteFile(locked, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })
	if got := diskfault.Of(os.WriteFile(locked, []byte("[]"), 0o600)); got != fault.Denied {
		t.Errorf("writing a read-only file = %v, want Denied", got)
	}
	if got := diskfault.Of(os.WriteFile(filepath.Join(dir, "gone", "x.json"), nil, 0o600)); got != fault.Unavailable {
		t.Errorf("writing into a folder that is not there = %v, want Unavailable", got)
	}
}
