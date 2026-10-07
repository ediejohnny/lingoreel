//go:build windows && amd64

package playerwindow_test

import (
	"slices"
	"sort"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/uithread"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The video inside the app's own window. fakeApp stands for that window
// (Wails): framed, on a thread of its own, filled by a child that draws its
// screen (the WebView), which takes the keys whenever the window gets them,
// as the WebView does. Its close is counted and refused, as the app decides
// on its own when to close.

const (
	wsExControlParent = 0x00010000
	wsExAppWindow     = 0x00040000
	scanF4            = 0x3E
	altContext        = 1 << 29 // a key message's "Alt is down" bit
)

var getParent = windows.NewLazySystemDLL("user32.dll").NewProc("GetParent")

type fakeApp struct {
	thread *uithread.Thread
	hwnd   windows.HWND
	screen windows.HWND

	mu           sync.Mutex
	closes       int
	contextMenus int
}

var (
	fakeAppProc  = syscall.NewCallback(dispatchFakeApp)
	fakeAppOnce  sync.Once
	fakeAppClass *uint16
	fakeAppErr   error
	fakeApps     = struct {
		sync.Mutex
		m map[windows.HWND]*fakeApp
	}{m: map[windows.HWND]*fakeApp{}}
)

func dispatchFakeApp(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	fakeApps.Lock()
	app := fakeApps.m[hwnd]
	fakeApps.Unlock()
	if app != nil {
		switch message {
		case win32.WM_CLOSE:
			app.mu.Lock()
			app.closes++
			app.mu.Unlock()
			return 0
		case win32.WM_CONTEXTMENU:
			app.mu.Lock()
			app.contextMenus++
			app.mu.Unlock()
			return 0
		case win32.WM_SETFOCUS:
			if app.screen != 0 {
				win32.SetFocus(app.screen)
			}
			return 0
		case win32.WM_SIZE:
			// As Wails does: the screen takes the client size read now.
			if client, err := win32.ClientRectOnScreen(hwnd); err == nil && app.screen != 0 {
				_ = win32.SetWindowPos(app.screen, 0, win32.Rect{Right: client.Right - client.Left, Bottom: client.Bottom - client.Top},
					win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
			}
		}
	}
	return win32.DefWindowProc(hwnd, message, wparam, lparam)
}

// newFakeApp opens the stand-in app window at bounds, shown without taking
// the focus or hidden.
func newFakeApp(t *testing.T, bounds win32.Rect, show bool) *fakeApp {
	t.Helper()
	thread, err := uithread.Start()
	if err != nil {
		t.Fatal(err)
	}
	app := &fakeApp{thread: thread}
	var createErr error
	if err := thread.Do(func() {
		fakeAppOnce.Do(func() {
			fakeAppClass, _ = windows.UTF16PtrFromString("LingoReelTestApp")
			_, fakeAppErr = win32.RegisterClass(&win32.WndClassEx{
				WndProc: fakeAppProc, Instance: win32.ModuleHandle(), Cursor: win32.ArrowCursor(), ClassName: fakeAppClass,
			})
		})
		if createErr = fakeAppErr; createErr != nil {
			return
		}
		title, _ := windows.UTF16PtrFromString("LingoReel")
		app.hwnd, createErr = win32.CreateTitledWindow(wsExControlParent|wsExAppWindow, win32.WS_OVERLAPPEDWINDOW, fakeAppClass, title, bounds, 0, win32.ModuleHandle())
		if createErr != nil {
			return
		}
		fakeApps.Lock()
		fakeApps.m[app.hwnd] = app
		fakeApps.Unlock()
		client, err := win32.ClientRectOnScreen(app.hwnd)
		if createErr = err; err != nil {
			return
		}
		app.screen, createErr = win32.CreateChild("STATIC", 0, app.hwnd, win32.Rect{Right: client.Right - client.Left, Bottom: client.Bottom - client.Top})
		if show {
			win32.ShowWindow(app.hwnd, win32.SW_SHOWNOACTIVATE)
		}
	}); err != nil || createErr != nil {
		t.Fatalf("opening the stand-in app window: %v %v", err, createErr)
	}
	t.Cleanup(func() {
		_ = thread.Do(func() {
			if windows.IsWindow(app.hwnd) {
				_ = win32.DestroyWindow(app.hwnd)
			}
			fakeApps.Lock()
			delete(fakeApps.m, app.hwnd)
			fakeApps.Unlock()
		})
		_ = thread.Close()
	})
	return app
}

func (a *fakeApp) handle() uintptr { return uintptr(a.hwnd) }

func (a *fakeApp) closeCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.closes
}

// onScreenHidden is where an app window goes for the fullscreen tests: on a
// monitor (leaving fullscreen puts a window back like SetWindowPlacement,
// which pulls one lying wholly off screen onto a monitor), but hidden, so the
// fullscreen never covers the user's screen.
var onScreenHidden = win32.Rect{Left: 100, Top: 100, Right: 1116, Bottom: 739}

// offScreenApp is a shown app window the user never sees.
var offScreenApp = win32.Rect{Left: -3000, Top: 100, Right: -3000 + 1016, Bottom: 100 + 639}

// panelOpen is the app's header on top and its panel on the right.
var panelOpen = layout.Insets{Top: 56, Right: 380}

func openOn(t *testing.T, stage *playerwindow.Stage) *playerwindow.Window {
	t.Helper()
	w, err := playerwindow.Open(t.Context(), playerwindow.Options{
		Title: ptbr.WindowTitle("v.mkv"), VideoWidth: 1280, VideoHeight: 720,
		Look:       defaultLook(),
		Labels:     playerwindow.Labels{Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime, EditHint: ptbr.EditHint, EditEmpty: ptbr.EditEmpty},
		NoActivate: true, Stage: stage,
		// No menu ever shows on screen in the tests.
		Choose: func(uintptr, int32, int32) uint32 { return 0 },
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}

// stageFor opens a stage on an app window with the panel open.
func stageFor(app *fakeApp, onFullscreen func(bool)) *playerwindow.Stage {
	stage := playerwindow.NewStage(playerwindow.StageOptions{Parent: app.handle, OnFullscreen: onFullscreen})
	stage.SetLayout(playerwindow.StageLayout{Visible: true, Insets: panelOpen})
	return stage
}

// videoPlace is where the video belongs on screen: the app window's client
// area less the insets at its scaling (VideoArea's own cases are tested in
// the domain).
func videoPlace(t *testing.T, app *fakeApp, insets layout.Insets, fullscreen bool) win32.Rect {
	t.Helper()
	client, err := win32.ClientRectOnScreen(app.hwnd)
	if err != nil {
		t.Fatal(err)
	}
	area := layout.VideoArea(layout.Rect{W: float64(client.Right - client.Left), H: float64(client.Bottom - client.Top)}, insets, win32.DpiForWindow(app.hwnd), fullscreen)
	return win32.Rect{
		Left: client.Left + int32(area.X), Top: client.Top + int32(area.Y),
		Right: client.Left + int32(area.X+area.W), Bottom: client.Top + int32(area.Y+area.H),
	}
}

// placedAt waits for the video's client area to be at want.
func placedAt(t *testing.T, host windows.HWND, want win32.Rect, what string) {
	t.Helper()
	testkit.Eventually(t, what, func() bool {
		got, err := win32.ClientRectOnScreen(host)
		return err == nil && got == want
	})
}

func TestTheVideoGoesInsideTheAppWindowAroundItsControls(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	w := openOn(t, stageFor(app, nil))
	host := windows.HWND(w.Handle())

	if parent, _, _ := getParent.Call(uintptr(host)); windows.HWND(parent) != app.hwnd {
		t.Fatalf("the video's window is in %x, not in the app's %x", parent, app.hwnd)
	}
	if win32.Style(host)&win32.WS_CHILD == 0 {
		t.Fatal("the video's window is a window of its own, not a part of the app's")
	}
	videoThread, _ := windows.GetWindowThreadProcessId(host, nil)
	appThread, _ := windows.GetWindowThreadProcessId(app.hwnd, nil)
	if videoThread == appThread {
		t.Fatal("the video's window runs on the app window's thread")
	}
	placedAt(t, host, videoPlace(t, app, panelOpen, false), "the video beside the panel, under the header")
	if !windows.IsWindowVisible(host) {
		t.Fatal("the video is hidden")
	}
	if top := win32.TopChild(app.hwnd); top != host {
		t.Fatalf("the app's own screen (%x) is on top of the video (%x)", top, host)
	}
	// The subtitles and the bar belong to the app's window, above it; the
	// video took no focus.
	if ownedWindow(app.hwnd, "LingoReelOverlay") == 0 || controlBar(app.hwnd) == 0 {
		t.Fatal("the subtitles or the bar do not belong to the app's window")
	}
	if foreground := windows.GetForegroundWindow(); foreground == app.hwnd || foreground == host {
		t.Fatal("opening the video took the focus")
	}
}

func TestTheVideoFollowsTheAppWindow(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	stage := stageFor(app, nil)
	w := openOn(t, stage)
	host := windows.HWND(w.Handle())
	w.Attach(newRemote(&recordingPlayer{}))
	placedAt(t, host, videoPlace(t, app, panelOpen, false), "the video in place")

	// The app's window changes size: the video keeps the room the controls
	// leave, at once.
	var latencies []time.Duration
	for i := range 20 {
		bounds := win32.Rect{Left: -3000, Top: 100, Right: -3000 + 900 + 20*int32(i), Bottom: 100 + 600 + 5*int32(i)}
		start := time.Now()
		if err := win32.SetWindowPos(app.hwnd, 0, bounds, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE); err != nil {
			t.Fatal(err)
		}
		placedAt(t, host, videoPlace(t, app, panelOpen, false), "the video to follow the app's window")
		latencies = append(latencies, time.Since(start))
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	t.Logf("the video follows the app's window in %v (p50), %v (p95)", latencies[9], latencies[18])

	// The panel closes and opens: the video takes and gives back its room.
	stage.SetLayout(playerwindow.StageLayout{Visible: true, Insets: layout.Insets{Top: 56}})
	placedAt(t, host, videoPlace(t, app, layout.Insets{Top: 56}, false), "the video to take the panel's room")

	// The bar shows over the video's bottom, and moves with the app's window.
	bar := controlBar(app.hwnd)
	client, _ := win32.ClientRectOnScreen(host)
	post(t, host, win32.WM_MOUSEMOVE, 0, at(int(client.Right-client.Left)/2, int(client.Bottom-client.Top)-20))
	testkit.Eventually(t, "the bar to show", func() bool { return windows.IsWindowVisible(bar) })
	before, _ := win32.WindowRect(bar)
	appRect, _ := win32.WindowRect(app.hwnd)
	moved := win32.Rect{Left: appRect.Left + 50, Top: appRect.Top + 30, Right: appRect.Right + 50, Bottom: appRect.Bottom + 30}
	if err := win32.SetWindowPos(app.hwnd, 0, moved, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the bar to move with the app's window", func() bool {
		after, err := win32.WindowRect(bar)
		return err == nil && after.Left == before.Left+50 && after.Top == before.Top+30
	})

	// Another screen of the app: the video, its subtitles and its bar hide;
	// back to the video, it shows again where it was.
	stage.SetLayout(playerwindow.StageLayout{Visible: false, Insets: layout.Insets{Top: 56}})
	subtitles := ownedWindow(app.hwnd, "LingoReelOverlay")
	testkit.Eventually(t, "the video, the subtitles and the bar to hide", func() bool {
		return !windows.IsWindowVisible(host) && !windows.IsWindowVisible(subtitles) && !windows.IsWindowVisible(bar)
	})
	stage.SetLayout(playerwindow.StageLayout{Visible: true, Insets: layout.Insets{Top: 56}})
	placedAt(t, host, videoPlace(t, app, layout.Insets{Top: 56}, false), "the video back")
	testkit.Eventually(t, "the video to show again", func() bool { return windows.IsWindowVisible(host) })
}

// Closing the app hides its window first, while the video still plays:
// the video, its subtitles and its bar go with it at once, instead of
// staying on the desktop until the player has closed. Shown again, the
// video comes back.
func TestHidingTheAppWindowHidesTheVideoAtOnce(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	stage := stageFor(app, nil)
	w := openOn(t, stage)
	host := windows.HWND(w.Handle())
	w.Attach(newRemote(&recordingPlayer{}))
	placedAt(t, host, videoPlace(t, app, panelOpen, false), "the video in place")
	bar := controlBar(app.hwnd)
	client, _ := win32.ClientRectOnScreen(host)
	post(t, host, win32.WM_MOUSEMOVE, 0, at(int(client.Right-client.Left)/2, int(client.Bottom-client.Top)-20))
	testkit.Eventually(t, "the bar to show", func() bool { return windows.IsWindowVisible(bar) })
	subtitles := ownedWindow(app.hwnd, "LingoReelOverlay")

	start := time.Now()
	win32.ShowWindow(app.hwnd, win32.SW_HIDE)
	testkit.Eventually(t, "the video, the subtitles and the bar to hide with the app's window", func() bool {
		return !windows.IsWindowVisible(subtitles) && !windows.IsWindowVisible(bar) && !windows.IsWindowVisible(host)
	})
	// The window's own event, not a timer (250 ms before): nothing lingers.
	if hidden := time.Since(start); hidden > 150*time.Millisecond {
		t.Errorf("hidden %v after the app's window", hidden)
	}

	win32.ShowWindow(app.hwnd, win32.SW_SHOWNOACTIVATE)
	testkit.Eventually(t, "the video to show again with the app's window", func() bool { return windows.IsWindowVisible(host) })
}

// The controls' look set on the stage reaches the windows on it, the open
// one at once: a bar twice the size is twice as tall.
func TestTheStagesLookReachesItsWindows(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	stage := stageFor(app, nil)
	// Before the window opens: the bar stays long enough for the test.
	chrome := playerwindow.DefaultChrome()
	chrome.HideDelay = 10 * time.Second
	stage.SetChrome(chrome)
	w := openOn(t, stage)
	host := windows.HWND(w.Handle())
	w.Attach(newRemote(&recordingPlayer{}))
	placedAt(t, host, videoPlace(t, app, panelOpen, false), "the video in place")
	bar := controlBar(app.hwnd)
	client, _ := win32.ClientRectOnScreen(host)
	post(t, host, win32.WM_MOUSEMOVE, 0, at(int(client.Right-client.Left)/2, int(client.Bottom-client.Top)-20))
	testkit.Eventually(t, "the bar to show", func() bool { return windows.IsWindowVisible(bar) })
	before, err := win32.WindowRect(bar)
	if err != nil {
		t.Fatal(err)
	}
	chrome.Bar.Size = 2
	stage.SetChrome(chrome)
	// Whole pixels: the bar's band is rounded once.
	testkit.Eventually(t, "the bar twice as tall", func() bool {
		after, err := win32.WindowRect(bar)
		grown := (after.Bottom - after.Top) - 2*(before.Bottom-before.Top)
		return err == nil && windows.IsWindowVisible(bar) && grown >= -1 && grown <= 1
	})
}

// The app's window erases its background without clipping its children, as
// Wails' does: a resize then painted it over the parts of the video VLC
// paints itself (the black bars), white for good. With the video in it, it
// clips its children.
func TestTheAppWindowNeverPaintsOverTheVideo(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	if win32.Style(app.hwnd)&win32.WS_CLIPCHILDREN != 0 {
		t.Fatal("the fake app clips its children from the start; Wails' window does not")
	}
	openOn(t, stageFor(app, nil))
	if win32.Style(app.hwnd)&win32.WS_CLIPCHILDREN == 0 {
		t.Fatal("the app's window still paints over the video")
	}
}

func TestTheVideoStaysAboveTheAppsOwnChildren(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	w := openOn(t, stageFor(app, nil))
	host := windows.HWND(w.Handle())
	testkit.Eventually(t, "the video on top", func() bool { return win32.TopChild(app.hwnd) == host })
	// The app's screen comes to the top of its children, as the WebView may
	// when it changes size: the video goes back above it.
	if err := app.thread.Do(func() {
		_ = win32.SetWindowPos(app.screen, 0, win32.Rect{Right: 400, Bottom: 300}, win32.SWP_NOACTIVATE)
	}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the video back on top of the app's screen", func() bool { return win32.TopChild(app.hwnd) == host })
}

// Leaving the fullscreen of a maximized app window gives the app's screen
// (the WebView, which follows WM_SIZE) the window's size again: Windows sends
// no WM_SIZE when a maximized window gets its frame back, and the screen hung
// 53 px below the window, hiding the settings' Save button (user report). It
// shows a maximized window for about two seconds, so it runs with -Full only.
func TestLeavingFullscreenGivesTheAppsScreenTheWindowsSize(t *testing.T) {
	if testing.Short() {
		t.Skip("shows a maximized window on screen: runs with -Full")
	}
	const swShowMaximized = 3
	app := newFakeApp(t, onScreenHidden, false)
	if err := app.thread.Do(func() { win32.ShowWindow(app.hwnd, swShowMaximized) }); err != nil {
		t.Fatal(err)
	}
	stage := stageFor(app, nil)
	openOn(t, stage)
	screenFits := func() bool {
		client, err := win32.ClientRectOnScreen(app.hwnd)
		screen, err2 := win32.WindowRect(app.screen)
		return err == nil && err2 == nil && screen == client
	}
	testkit.Eventually(t, "the app's screen filling the maximized window", screenFits)
	maximized, err := win32.WindowRect(app.hwnd)
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := win32.MonitorBounds(app.hwnd)
	if err != nil {
		t.Fatal(err)
	}

	stage.SetFullscreen(true)
	testkit.Eventually(t, "the fullscreen", func() bool { r, err := win32.WindowRect(app.hwnd); return err == nil && r == monitor })
	stage.SetFullscreen(false)
	testkit.Eventually(t, "the maximized window back", func() bool { r, err := win32.WindowRect(app.hwnd); return err == nil && r == maximized })
	testkit.Eventually(t, "the app's screen back to the window's size", screenFits)
}

func TestFullscreenCoversTheMonitorWithTheAppWindow(t *testing.T) {
	app := newFakeApp(t, onScreenHidden, false)
	var mu sync.Mutex
	var told []bool
	stage := stageFor(app, func(on bool) { mu.Lock(); told = append(told, on); mu.Unlock() })
	w := openOn(t, stage)
	host := windows.HWND(w.Handle())
	toldSoFar := func() []bool { mu.Lock(); defer mu.Unlock(); return slices.Clone(told) }

	before, err := win32.WindowRect(app.hwnd)
	if err != nil {
		t.Fatal(err)
	}
	style := win32.Style(app.hwnd)
	monitor, err := win32.MonitorBounds(app.hwnd)
	if err != nil {
		t.Fatal(err)
	}
	owned := map[string]windows.HWND{"subtitles": ownedWindow(app.hwnd, "LingoReelOverlay"), "bar": controlBar(app.hwnd)}
	onTop := func(when string) {
		t.Helper()
		for name, window := range owned {
			if window == 0 {
				t.Fatalf("no %s window", name)
			}
			if zOrder(window) > zOrder(app.hwnd) {
				t.Fatalf("%s: the %s (place %d) is behind the app's window (place %d)", when, name, zOrder(window), zOrder(app.hwnd))
			}
		}
	}
	// F on the video: the app's window covers the monitor, without its frame,
	// and the video covers the app's controls.
	post(t, host, win32.WM_KEYDOWN, 0, key(scanF, false, false))
	testkit.Eventually(t, "the app's window to cover its monitor", func() bool { r, err := win32.WindowRect(app.hwnd); return err == nil && r == monitor })
	if win32.Style(app.hwnd)&win32.WS_OVERLAPPEDWINDOW != 0 {
		t.Fatal("the app's frame stayed in fullscreen")
	}
	placedAt(t, host, videoPlace(t, app, panelOpen, true), "the video over the whole app window")
	testkit.Eventually(t, "the app told of the fullscreen", func() bool { return slices.Equal(toldSoFar(), []bool{true}) })
	onTop("fullscreen")

	// Esc: back where it was, with its frame, and the controls' room again.
	post(t, host, win32.WM_KEYDOWN, 0, key(scanEscape, false, false))
	testkit.Eventually(t, "the app's window back", func() bool { r, err := win32.WindowRect(app.hwnd); return err == nil && r == before })
	if got := win32.Style(app.hwnd); got != style {
		t.Fatalf("the app's style is %#x after the fullscreen, was %#x", got, style)
	}
	placedAt(t, host, videoPlace(t, app, panelOpen, false), "the video beside the panel again")
	testkit.Eventually(t, "the app told of the way back", func() bool { return slices.Equal(toldSoFar(), []bool{true, false}) })
	onTop("back from fullscreen")

	// The app asks for it too, as its panel's button does.
	stage.SetFullscreen(true)
	testkit.Eventually(t, "the fullscreen asked by the app", func() bool { r, err := win32.WindowRect(app.hwnd); return err == nil && r == monitor })
	stage.SetFullscreen(false)
	testkit.Eventually(t, "the way back asked by the app", func() bool { r, err := win32.WindowRect(app.hwnd); return err == nil && r == before })
	if windows.GetForegroundWindow() == app.hwnd {
		t.Fatal("the fullscreen took the focus")
	}
}

// Alt+F4 on the video closes the app's window: everything, as the user
// asked of a single window.
func TestAltF4OnTheVideoClosesTheAppWindow(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	w := openOn(t, stageFor(app, nil))
	host := windows.HWND(w.Handle())
	w.Attach(newRemote(&recordingPlayer{}))
	sendMessage.Call(uintptr(host), win32.WM_SYSKEYDOWN, win32.VK_F4, key(scanF4, false, false)|altContext)
	testkit.Eventually(t, "the app's window asked to close", func() bool { return app.closeCount() == 1 })
}

// A right click on the video never reaches the app's window: the menu it
// asks for would climb there by a message that waits for the app's thread.
func TestARightClickOnTheVideoStaysOffTheAppWindow(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	w := openOn(t, stageFor(app, nil))
	host := windows.HWND(w.Handle())
	w.Attach(newRemote(&recordingPlayer{}))
	sendMessage.Call(uintptr(host), win32.WM_RBUTTONUP, 0, at(50, 50))
	// The message would have reached the app before SendMessage returned.
	app.mu.Lock()
	defer app.mu.Unlock()
	if app.contextMenus != 0 {
		t.Fatalf("the app's window got %d context menu messages", app.contextMenus)
	}
}

// The window's keys reach the session on a stage too, the sync ones with
// them (no system-wide hotkeys take them there), held keys said held.
func TestTheSyncKeysReachTheSessionOnAStage(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	w := openOn(t, stageFor(app, nil))
	host := windows.HWND(w.Handle())
	remote := newRemote(&recordingPlayer{})
	w.Attach(remote)
	w.SetShortcuts(shortcut.Map{shortcut.SyncPrimaryEarlier: {Key: "KeyP"}})
	send(host, win32.WM_KEYDOWN, key(scanP, false, false))
	send(host, win32.WM_KEYDOWN, key(scanP, false, true))
	want := []string{"sync-primary-earlier", "sync-primary-earlier (held)"}
	testkit.Eventually(t, "the sync key to reach the session", func() bool { return len(remote.ran()) >= len(want) })
	if got := remote.ran(); !slices.Equal(got, want) {
		t.Fatalf("ran %q, want %q", got, want)
	}
}

func TestClosingTheVideoLeavesTheAppWindowAsItWas(t *testing.T) {
	app := newFakeApp(t, onScreenHidden, false)
	stage := stageFor(app, nil)
	w := openOn(t, stage)
	host := windows.HWND(w.Handle())
	before, _ := win32.WindowRect(app.hwnd)
	style := win32.Style(app.hwnd)
	monitor, _ := win32.MonitorBounds(app.hwnd)
	stage.SetFullscreen(true)
	testkit.Eventually(t, "the fullscreen", func() bool { r, err := win32.WindowRect(app.hwnd); return err == nil && r == monitor })

	// Closed in fullscreen: the app's window comes back with its frame, and
	// nothing of the video stays in it.
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if windows.IsWindow(host) {
		t.Fatal("the video's window stayed")
	}
	if after, _ := win32.WindowRect(app.hwnd); after != before {
		t.Fatalf("the app's window is at %+v, was at %+v", after, before)
	}
	if got := win32.Style(app.hwnd); got != style {
		t.Fatalf("the app's style is %#x, was %#x", got, style)
	}
	if top := win32.TopChild(app.hwnd); top != app.screen {
		t.Fatalf("the app's top child is %x, want its own screen", top)
	}
	if ownedWindow(app.hwnd, "LingoReelOverlay") != 0 || controlBar(app.hwnd) != 0 || ownedWindow(app.hwnd, "LingoReelPreview") != 0 {
		t.Fatal("the subtitles, the bar or the seek preview stayed with the app's window")
	}
	// The stage outlives the window: asking it things now does nothing.
	stage.SetLayout(playerwindow.StageLayout{Visible: true})
	stage.SetFullscreen(true)
	// Anything asked of the app's window has run once its thread answers.
	if err := app.thread.Do(func() {}); err != nil {
		t.Fatal(err)
	}
	if after, _ := win32.WindowRect(app.hwnd); after != before {
		t.Fatal("a stage without a video changed the app's window")
	}
}

// Closing the app's window takes the video with it: the session hears it
// and ends the player.
func TestTheAppWindowGoingEndsTheSession(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	w := openOn(t, stageFor(app, nil))
	if err := app.thread.Do(func() { _ = win32.DestroyWindow(app.hwnd) }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.Closed():
	case <-time.After(2 * time.Second):
		t.Fatal("the app's window went and the session was not told")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close after the app's window went: %v", err)
	}
}

// Closing the video never waits for the app's thread: when the app closes,
// its thread may be busy closing too.
func TestCloseDoesNotWaitForABusyAppWindow(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	w := openOn(t, stageFor(app, nil))
	release := make(chan struct{})
	busy := make(chan struct{})
	if err := app.thread.Post(func() {
		close(busy)
		select {
		case <-release:
		case <-time.After(3 * time.Second):
		}
	}); err != nil {
		t.Fatal(err)
	}
	<-busy
	start := time.Now()
	err := w.Close()
	waited := time.Since(start)
	close(release)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if waited > time.Second {
		t.Fatalf("Close waited %v for the app's busy thread", waited)
	}
}

// The box that corrects a line belongs to the app's window on a stage: above
// it, hidden with it, never over the other programs.
func TestTheEditorBoxBelongsToTheAppWindow(t *testing.T) {
	app := newFakeApp(t, offScreenApp, true)
	w := openOn(t, stageFor(app, nil))
	host := windows.HWND(w.Handle())
	remote := newRemote(&recordingPlayer{})
	remote.lineText = "That's sick!"
	remote.clock.Observe(playback.Sample{Position: 10 * time.Second, Duration: 100 * time.Second, Speed: 1, Status: playback.StatusPaused, ObservedAt: time.Now()})
	w.Attach(remote)
	if err := w.Show(session.Screen{Primary: "That's sick!", TranslationVisible: true}); err != nil {
		t.Fatal(err)
	}
	var line layout.Rect
	testkit.Eventually(t, "the line drawn", func() bool {
		box, ok := w.LineBounds(session.LinePrimary)
		line = box
		return ok
	})
	x, y := int(line.X+line.W/2), int(line.Y+line.H/2)
	post(t, host, win32.WM_LBUTTONDOWN, 1, at(x, y))
	post(t, host, wmLButtonUp, 0, at(x, y))
	post(t, host, win32.WM_LBUTTONDBLCLK, 1, at(x, y))
	post(t, host, wmLButtonUp, 0, at(x, y))
	var box windows.HWND
	testkit.Eventually(t, "the box over the line", func() bool { box = editorBox(); return box != 0 && windows.IsWindowVisible(box) })
	if owner, _, _ := getWindow.Call(uintptr(box), gwOwner); windows.HWND(owner) != app.hwnd {
		t.Fatalf("the box is owned by %x, not by the app's window %x", owner, app.hwnd)
	}
	index := int64(gwlExStyle)
	if style, _, _ := getWindowPtr.Call(uintptr(box), uintptr(index)); style&wsExTopmost != 0 {
		t.Fatal("the box is over every other program")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if windows.IsWindow(box) {
		t.Fatal("the box outlived the video")
	}
}
