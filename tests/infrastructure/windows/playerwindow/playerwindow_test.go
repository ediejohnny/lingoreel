//go:build windows && amd64

package playerwindow_test

import (
	"slices"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/infrastructure/windows/playerwindow"
	"lingoreel/internal/infrastructure/windows/win32"
	"lingoreel/internal/presentation/ptbr"
)

var (
	getWindowText = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowTextW")
	sendMessage   = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageW")
)

// Every window here opens off screen and never takes the focus: the user
// keeps working while the tests run.
var offScreen = player.Placement{X: -3000, Y: 100, Width: 640, Height: 360}

// recordingPlayer is a player whose commands are recorded; the controls
// call it from the window's worker goroutine.
type recordingPlayer struct {
	mu     sync.Mutex
	calls  []string
	seek   time.Duration
	volume []playback.Volume
}

func (p *recordingPlayer) record(call string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, call)
	return nil
}
func (p *recordingPlayer) Capabilities() playback.Capabilities {
	return playback.Capabilities{Pause: true, Seek: true, Speed: true, Volume: true, Precision: playback.PrecisionExact}
}
func (p *recordingPlayer) Play() error  { return p.record("play") }
func (p *recordingPlayer) Pause() error { return p.record("pause") }
func (p *recordingPlayer) SeekTo(position time.Duration) error {
	p.mu.Lock()
	p.seek = position
	p.mu.Unlock()
	return p.record("seek")
}
func (p *recordingPlayer) SetSpeed(playback.Speed) error { return p.record("speed") }
func (p *recordingPlayer) StepFrame(bool) error          { return p.record("frame") }
func (p *recordingPlayer) SelectAudioTrack(int) error    { return p.record("audio") }
func (p *recordingPlayer) SetVolume(volume playback.Volume) error {
	p.mu.Lock()
	p.volume = append(p.volume, volume)
	p.mu.Unlock()
	return p.record("volume")
}
func (p *recordingPlayer) saw(call string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Contains(p.calls, call)
}

// fakeRemote is the session the window's controls drive.
type fakeRemote struct {
	controls *session.Controls
	clock    *session.Clock
	mu       sync.Mutex
	shown    bool
	reveals  []bool
}

func newRemote(p *recordingPlayer) *fakeRemote {
	clock := session.NewClock(playback.PrecisionExact, time.Now)
	clock.Observe(playback.Sample{Position: 10 * time.Second, Duration: 100 * time.Second, Speed: 1, Status: playback.StatusPlaying, ObservedAt: time.Now()})
	return &fakeRemote{controls: session.NewControls(p, clock, 25), clock: clock, shown: true}
}

func (r *fakeRemote) State() playback.Sample      { return r.clock.State() }
func (r *fakeRemote) Controls() *session.Controls { return r.controls }
func (r *fakeRemote) HasTranslation() bool        { return true }
func (r *fakeRemote) TranslationVisible() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.shown
}
func (r *fakeRemote) SetTranslationVisible(visible bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shown = visible
}

func (r *fakeRemote) ReplayLine() error { return r.controls.SeekTo(0) }

func (r *fakeRemote) Reveal(pressed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reveals = append(r.reveals, pressed)
}

func (r *fakeRemote) revealed() []bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.reveals)
}

var _ watch.Remote = (*fakeRemote)(nil)

func open(t *testing.T) *playerwindow.Window {
	t.Helper()
	w, err := playerwindow.Open(t.Context(), playerwindow.Options{
		Title: ptbr.WindowTitle("v.mkv"), Placement: offScreen, VideoWidth: 1280, VideoHeight: 720,
		Labels:     playerwindow.Labels{Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime},
		NoActivate: true,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}

func eventually(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func post(t *testing.T, hwnd windows.HWND, message uint32, wparam, lparam uintptr) {
	t.Helper()
	if err := win32.PostMessage(hwnd, message, wparam, lparam); err != nil {
		t.Fatalf("PostMessage(%#x): %v", message, err)
	}
}

func at(x, y int) uintptr { return uintptr(uint16(x)) | uintptr(uint16(y))<<16 }

// Scan codes (set 1) of the keys the tests press.
const (
	scanEscape     = 0x01
	scanBackspace  = 0x0E
	scanP          = 0x19
	scanF          = 0x21
	scanM          = 0x32
	scanAlt        = 0x38 // the right Alt when extended
	scanSpace      = 0x39
	scanArrowRight = 0x4D // extended
)

// key is a key message's lParam: the scan code, the extended flag and
// whether the key was already down (a repeat).
func key(scan uintptr, extended, repeat bool) uintptr {
	lparam := 1 | scan<<16
	if extended {
		lparam |= 1 << 24
	}
	if repeat {
		lparam |= 1 << 30
	}
	return lparam
}

// controlBar finds the bar window the player window owns.
func controlBar(host windows.HWND) windows.HWND {
	var found windows.HWND
	callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		name := make([]uint16, 64)
		if n, err := windows.GetClassName(hwnd, &name[0], int32(len(name))); err == nil && windows.UTF16ToString(name[:n]) == "LingoReelControls" && win32.RootOwner(hwnd) == host {
			found = hwnd
		}
		return 1
	})
	_ = windows.EnumWindows(callback, nil)
	return found
}

func TestTheClientAreaIsTheOneAsked(t *testing.T) {
	w := open(t)
	host := windows.HWND(w.Handle())
	client, err := win32.ClientRectOnScreen(host)
	if err != nil {
		t.Fatal(err)
	}
	want := win32.Rect{Left: -3000, Top: 100, Right: -3000 + 640, Bottom: 100 + 360}
	if client != want {
		t.Fatalf("client area %+v, want %+v", client, want)
	}
	title := make([]uint16, 128)
	n, _, _ := getWindowText.Call(uintptr(host), uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
	if got := windows.UTF16ToString(title[:n]); got != "v.mkv — LingoReel" {
		t.Fatalf("title %q", got)
	}
	if windows.GetForegroundWindow() == host {
		t.Fatal("NoActivate took the focus")
	}
}

// TestCloseButtonEndsTheWindow is the reported bug: the close button and
// Alt+F4 (both WM_CLOSE) must reach the app, which then ends everything.
func TestCloseButtonEndsTheWindow(t *testing.T) {
	w := open(t)
	host := windows.HWND(w.Handle())
	post(t, host, win32.WM_CLOSE, 0, 0)
	select {
	case <-w.Closed():
	case <-time.After(2 * time.Second):
		t.Fatal("closing the window was not reported")
	}
	// The window waits for the session to end the player before going; the
	// default handling would destroy it right after the message.
	time.Sleep(300 * time.Millisecond)
	if !windows.IsWindow(host) {
		t.Fatal("the window went before the player was ended")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if windows.IsWindow(host) {
		t.Fatal("the window is still there after Close")
	}
	select {
	case <-w.Done():
	default:
		t.Fatal("Done not closed")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestKeysDriveTheSession(t *testing.T) {
	w := open(t)
	host := windows.HWND(w.Handle())
	// Before the session is attached a key does nothing; sent, not posted,
	// so it is handled before the session comes.
	sendMessage.Call(uintptr(host), win32.WM_KEYDOWN, 0, key(scanSpace, false, false))
	p := &recordingPlayer{}
	w.Attach(newRemote(p))
	post(t, host, win32.WM_KEYDOWN, 0, key(scanSpace, false, false))
	eventually(t, "Space to pause", func() bool { return p.saw("pause") })
	post(t, host, win32.WM_KEYDOWN, 0, key(scanArrowRight, true, false))
	eventually(t, "Right to seek", func() bool { return p.saw("seek") })
	p.mu.Lock()
	seek := p.seek
	p.mu.Unlock()
	// Paused at about 10 s (the clock ran a moment before the pause): +5 s.
	if seek < 15*time.Second || seek > 16*time.Second {
		t.Fatalf("seek to %v, want 5 s after about 10 s", seek)
	}
	post(t, host, win32.WM_KEYDOWN, 0, key(scanM, false, false))
	eventually(t, "M to mute", func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.volume) > 0 && p.volume[len(p.volume)-1] == 0
	})
	// Backspace is "Treinar de novo" (the fake goes back to 0).
	post(t, host, win32.WM_KEYDOWN, 0, key(scanBackspace, false, false))
	eventually(t, "Backspace to train again", func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.seek == 0
	})
	if p.saw("play") {
		t.Fatal("the key pressed before the session reached the player")
	}
}

func TestTheBarShowsAtTheBottomAndTakesClicks(t *testing.T) {
	w := open(t)
	host := windows.HWND(w.Handle())
	p := &recordingPlayer{}
	remote := newRemote(p)
	w.Attach(remote)
	bar := controlBar(host)
	if bar == 0 {
		t.Fatal("no control bar window")
	}
	if windows.IsWindowVisible(bar) {
		t.Fatal("the bar shows before the mouse comes")
	}
	// The mouse at the top of the video does not bring the bar.
	post(t, host, win32.WM_MOUSEMOVE, 0, at(320, 40))
	time.Sleep(200 * time.Millisecond)
	if windows.IsWindowVisible(bar) {
		t.Fatal("the bar showed for the top of the video")
	}
	// 640x360 at 100%: the zone starts max(72, 116) pixels from the bottom.
	post(t, host, win32.WM_MOUSEMOVE, 0, at(320, 340))
	eventually(t, "the bar to show", func() bool { return windows.IsWindowVisible(bar) })

	// The bar window starts at the band: panel 360-58 = 302, band 302-34 =
	// 268. Play is at client (12, 318), so at (20, 60) in the bar; the
	// translation at client x 480..520 (right edge 628, fullscreen 588,
	// speed 524).
	post(t, bar, win32.WM_LBUTTONDOWN, 1, at(20, 60))
	eventually(t, "a click on play to pause", func() bool { return p.saw("pause") })
	post(t, bar, win32.WM_LBUTTONDOWN, 1, at(490, 60))
	eventually(t, "a click on the translation to hide it", func() bool { return !remote.TranslationVisible() })

	// The real mouse is elsewhere: the bar leaves after its delay.
	start := time.Now()
	eventually(t, "the bar to hide", func() bool { return !windows.IsWindowVisible(bar) })
	if waited := time.Since(start); waited < controlbar.HideDelay/2 {
		t.Fatalf("the bar left after %v, before its delay", waited)
	}
}

func TestFullscreenAndBack(t *testing.T) {
	w := open(t)
	host := windows.HWND(w.Handle())
	// Hidden, so the fullscreen never covers the user's screen. Leaving
	// fullscreen restores the window like SetWindowPlacement, which pulls a
	// window lying wholly off screen back onto a monitor: the hidden window
	// moves onto the screen first.
	win32.ShowWindow(host, win32.SW_HIDE)
	if err := win32.SetWindowPos(host, 0, win32.Rect{Left: 100, Top: 100, Right: 756, Bottom: 499}, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE); err != nil {
		t.Fatal(err)
	}
	before, err := win32.WindowRect(host)
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := win32.MonitorBounds(host)
	if err != nil {
		t.Fatal(err)
	}
	post(t, host, win32.WM_KEYDOWN, 0, key(scanF, false, false))
	eventually(t, "the window to cover its monitor", func() bool {
		r, err := win32.WindowRect(host)
		return err == nil && r == monitor
	})
	if win32.Style(host)&win32.WS_OVERLAPPEDWINDOW != 0 {
		t.Fatal("the frame stayed in fullscreen")
	}
	post(t, host, win32.WM_KEYDOWN, 0, key(scanEscape, false, false))
	var after win32.Rect
	deadline := time.Now().Add(4 * time.Second)
	for after != before && time.Now().Before(deadline) {
		after, _ = win32.WindowRect(host)
		time.Sleep(20 * time.Millisecond)
	}
	if after != before {
		t.Fatalf("back from fullscreen at %+v, want %+v (monitor %+v)", after, before, monitor)
	}
	if win32.Style(host)&win32.WS_OVERLAPPEDWINDOW != win32.WS_OVERLAPPEDWINDOW {
		t.Fatal("the frame did not come back")
	}
	if windows.GetForegroundWindow() == host {
		t.Fatal("leaving fullscreen took the focus")
	}
}

func (p *recordingPlayer) count(call string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, c := range p.calls {
		if c == call {
			n++
		}
	}
	return n
}

func send(hwnd windows.HWND, message uint32, lparam uintptr) {
	sendMessage.Call(uintptr(hwnd), uintptr(message), 0, lparam)
}

func TestTheProfileKeysDriveTheWindow(t *testing.T) {
	w := open(t)
	host := windows.HWND(w.Handle())
	p := &recordingPlayer{}
	remote := newRemote(p)
	w.Attach(remote)
	w.SetShortcuts(shortcut.Map{shortcut.PlayPause: {Key: "KeyP"}})

	// Space is no longer Play/Pause; P is.
	send(host, win32.WM_KEYDOWN, key(scanSpace, false, false))
	send(host, win32.WM_KEYDOWN, key(scanP, false, false))
	eventually(t, "P to pause", func() bool { return p.saw("pause") })
	// Held, P switches nothing more: only moves through the video repeat.
	for range 3 {
		send(host, win32.WM_KEYDOWN, key(scanP, false, true))
	}
	send(host, win32.WM_KEYDOWN, key(scanArrowRight, true, false))
	send(host, win32.WM_KEYDOWN, key(scanArrowRight, true, true))
	eventually(t, "the held arrow to seek twice", func() bool { return p.count("seek") == 2 })
	if n := p.count("pause") + p.count("play"); n != 1 {
		t.Fatalf("play/pause ran %d times, want once (Space and the repeats did nothing)", n)
	}

	// The translation key (the right Alt, a system key): down shows, a
	// repeat is no press, up lets go.
	send(host, win32.WM_SYSKEYDOWN, key(scanAlt, true, false))
	send(host, win32.WM_SYSKEYDOWN, key(scanAlt, true, true))
	send(host, win32.WM_SYSKEYUP, key(scanAlt, true, true))
	eventually(t, "the translation key down and up", func() bool { return slices.Equal(remote.revealed(), []bool{true, false}) })
	// The left Alt is not the translation key.
	send(host, win32.WM_SYSKEYDOWN, key(scanAlt, false, false))
	send(host, win32.WM_SYSKEYUP, key(scanAlt, false, true))
	// Losing the keyboard with the key down lets it go.
	send(host, win32.WM_SYSKEYDOWN, key(scanAlt, true, false))
	send(host, win32.WM_KILLFOCUS, 0)
	eventually(t, "the key let go on losing the focus", func() bool {
		return slices.Equal(remote.revealed(), []bool{true, false, true, false})
	})
	time.Sleep(100 * time.Millisecond)
	if got := remote.revealed(); len(got) != 4 {
		t.Fatalf("reveals = %v", got)
	}
}
