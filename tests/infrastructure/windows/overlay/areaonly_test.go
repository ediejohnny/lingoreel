//go:build windows && amd64

package overlay_test

import (
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/uithread"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

var (
	hostWindowOnce  sync.Once
	hostWindowClass *uint16
	hostWindowErr   error
	hostWindowProc  = syscall.NewCallback(func(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
		return win32.DefWindowProc(hwnd, message, wparam, lparam)
	})
)

// The app's own window, with the video in a part of it: the subtitles
// cover that part only and hide with it, never the app's controls around it.
func TestAnAreaOnlyOverlayHidesWithItsArea(t *testing.T) {
	thread, err := uithread.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { thread.Close() })
	var app, video windows.HWND
	var createErr error
	if err := thread.Do(func() {
		hostWindowOnce.Do(func() {
			hostWindowClass, _ = windows.UTF16PtrFromString("LingoReelTestAppWindow")
			_, hostWindowErr = win32.RegisterClass(&win32.WndClassEx{WndProc: hostWindowProc, Instance: win32.ModuleHandle(), ClassName: hostWindowClass})
		})
		if createErr = hostWindowErr; createErr != nil {
			return
		}
		app, createErr = win32.CreateTitledWindow(0, wsOverlapped, hostWindowClass, nil, win32.Rect{Left: offscreenX, Top: 100, Right: offscreenX + 1016, Bottom: 739}, 0, win32.ModuleHandle())
		if createErr != nil {
			return
		}
		video, createErr = win32.CreateChild("STATIC", 0, app, win32.Rect{Top: 56, Right: 628, Bottom: 600})
		win32.ShowWindow(app, win32.SW_SHOWNOACTIVATE)
	}); err != nil || createErr != nil {
		t.Fatalf("opening the app's window: %v %v", err, createErr)
	}
	t.Cleanup(func() { _ = thread.Do(func() { _ = win32.DestroyWindow(app) }) })

	screen, err := overlay.Open(t.Context(), overlay.Options{
		Target: uintptr(app), Area: func() uintptr { return uintptr(video) }, AreaOnly: true,
		VideoWidth: 1280, VideoHeight: 720, Layout: appearance.DefaultLayout(),
		Primary: appearance.DefaultPrimaryStyle(), Translation: appearance.DefaultTranslationStyle(),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { screen.Close() })
	if err := screen.Show(session.Screen{Primary: "That's sick!"}); err != nil {
		t.Fatal(err)
	}
	found := overlayWindows()
	if len(found) != 1 {
		t.Fatalf("found %d overlay windows, want 1", len(found))
	}
	covers := func(target windows.HWND) func() bool {
		return func() bool {
			a, errA := win32.ClientRectOnScreen(found[0])
			b, errB := win32.ClientRectOnScreen(target)
			return errA == nil && errB == nil && a == b && windows.IsWindowVisible(found[0])
		}
	}
	s := &scene{t: t, screen: screen, window: found[0], target: app}
	s.waitUntil("the subtitles over the video", covers(video))

	// The video hidden (another screen of the app): the subtitles hide, and
	// never spread over the whole app window; a new line keeps them hidden.
	if err := thread.Do(func() { win32.ShowWindow(video, win32.SW_HIDE) }); err != nil {
		t.Fatal(err)
	}
	s.waitUntil("the subtitles to hide with the video", s.hidden)
	if err := screen.Show(session.Screen{Primary: "Another line"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if s.visible() {
		t.Fatalf("a new line showed the subtitles while the video was hidden: %s", s.state())
	}

	// Back, and the video's part grows (the app's panel closed): the subtitles
	// follow it.
	if err := thread.Do(func() {
		win32.ShowWindow(video, win32.SW_SHOWNA)
		_ = win32.SetWindowPos(video, 0, win32.Rect{Top: 56, Right: 1000, Bottom: 600}, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
	}); err != nil {
		t.Fatal(err)
	}
	s.waitUntil("the subtitles back over the grown video", covers(video))
	if covers(app)() {
		t.Fatal("the subtitles cover the whole app window")
	}
}
