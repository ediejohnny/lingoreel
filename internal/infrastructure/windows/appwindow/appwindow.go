//go:build windows && amd64

// Package appwindow keeps the app window where the user left it: it finds
// the window by its title among the process's own and reads or restores its
// place with the system's placement, which knows the maximized state and
// the size the window returns to.
package appwindow

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// ErrNotFound reports that no window of this process has the title.
var ErrNotFound = errors.New("app window not found")

const swShowMaximized = 3

var getWindowText = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowTextW")

// search collects the window during one EnumWindows call; callbacks made
// with syscall.NewCallback are never freed, so one serves every search.
var search struct {
	sync.Mutex
	title string
	found windows.HWND
}

var searchProc = syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
	var owner uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err != nil || int(owner) != os.Getpid() {
		return 1
	}
	text := make([]uint16, len(search.title)+2)
	n, _, _ := getWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
	if windows.UTF16ToString(text[:n]) == search.title {
		search.found = hwnd
		return 0 // stop
	}
	return 1
})

// Find is the window of this process with exactly this title.
func Find(title string) (windows.HWND, error) {
	search.Lock()
	defer search.Unlock()
	search.title, search.found = title, 0
	// Stopping early makes EnumWindows report a failure: the result counts.
	_ = windows.EnumWindows(searchProc, nil)
	if search.found == 0 {
		return 0, ErrNotFound
	}
	return search.found, nil
}

// Bounds is where the window is: the size it has when not maximized, and
// whether it is maximized.
func Bounds(hwnd windows.HWND) (settings.WindowBounds, error) {
	placement, err := win32.Placement(hwnd)
	if err != nil {
		return settings.WindowBounds{}, err
	}
	r := placement.NormalPosition
	return settings.WindowBounds{
		X: int(r.Left), Y: int(r.Top), Width: int(r.Right - r.Left), Height: int(r.Bottom - r.Top),
		Maximized: placement.ShowCmd == swShowMaximized,
	}, nil
}

// Restore puts the window back where bounds say, unless no monitor shows
// that place any more (a monitor unplugged): the window then stays where
// the app opened it.
func Restore(hwnd windows.HWND, bounds settings.WindowBounds) (bool, error) {
	if !bounds.Saved() {
		return false, nil
	}
	r := win32.Rect{Left: int32(bounds.X), Top: int32(bounds.Y), Right: int32(bounds.X + bounds.Width), Bottom: int32(bounds.Y + bounds.Height)}
	if !win32.OnAnyMonitor(r) {
		return false, nil
	}
	placement, err := win32.Placement(hwnd)
	if err != nil {
		return false, err
	}
	placement.NormalPosition = r
	// Hidden windows stay hidden: the caller shows the window afterwards.
	placement.ShowCmd = win32.SW_HIDE
	if err := win32.SetPlacement(hwnd, placement); err != nil {
		return false, err
	}
	return true, nil
}
