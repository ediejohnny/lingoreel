//go:build windows && amd64

package appwindow_test

import (
	"errors"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/appwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// hiddenWindow creates a window that is never shown, with a title.
func hiddenWindow(t *testing.T, title string) windows.HWND {
	t.Helper()
	className, _ := windows.UTF16PtrFromString("STATIC")
	name, _ := windows.UTF16PtrFromString(title)
	hwnd, err := win32.CreateTitledWindow(0, win32.WS_OVERLAPPEDWINDOW, className, name,
		win32.Rect{Left: 100, Top: 100, Right: 900, Bottom: 700}, 0, win32.ModuleHandle())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = win32.DestroyWindow(hwnd) })
	return hwnd
}

func TestTheWindowPlaceIsReadAndRestored(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const title = "LingoReel test window 7f3a"
	hwnd := hiddenWindow(t, title)
	found, err := appwindow.Find(title)
	if err != nil || found != hwnd {
		t.Fatalf("Find = %v, %v; want %v", found, err, hwnd)
	}
	if _, err := appwindow.Find(title + " other"); !errors.Is(err, appwindow.ErrNotFound) {
		t.Fatalf("Find of another title = %v", err)
	}

	// A place on the primary monitor comes back exactly, hidden.
	want := settings.WindowBounds{X: 120, Y: 80, Width: 700, Height: 500}
	restored, err := appwindow.Restore(hwnd, want)
	if err != nil || !restored {
		t.Fatalf("Restore = %v, %v", restored, err)
	}
	if windows.IsWindowVisible(hwnd) {
		t.Fatal("Restore showed the window")
	}
	got, err := appwindow.Bounds(hwnd)
	if err != nil || got != want {
		t.Fatalf("Bounds = %+v, %v; want %+v", got, err, want)
	}

	// No saved place, or a place no monitor shows: left alone.
	for _, bounds := range []settings.WindowBounds{{}, {X: 30000, Y: 30000, Width: 800, Height: 600}} {
		if restored, err := appwindow.Restore(hwnd, bounds); err != nil || restored {
			t.Fatalf("Restore(%+v) = %v, %v", bounds, restored, err)
		}
	}
	if got, _ := appwindow.Bounds(hwnd); got != want {
		t.Fatalf("a refused place moved the window: %+v", got)
	}
}
