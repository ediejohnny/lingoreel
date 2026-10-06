//go:build windows && amd64

package playerwindow_test

import (
	"slices"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

var (
	getWindowText = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowTextW")
	sendMessage   = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageW")
	getTopWindow  = windows.NewLazySystemDLL("user32.dll").NewProc("GetTopWindow")
	getWindow     = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindow")
)

// Every window here opens off screen and never takes the focus: the user
// keeps working while the tests run.
var offScreen = player.Placement{X: -3000, Y: 100, Width: 640, Height: 360}

// defaultLook is how a new profile draws the subtitles and their marks.
func defaultLook() session.Look { return watch.LookOf(settings.DefaultProfile()) }

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

// fakeRemote is the session the window's controls drive; the commands it
// runs are recorded (the session's own tests check what each one does).
type fakeRemote struct {
	controls *session.Controls
	clock    *session.Clock
	mu       sync.Mutex
	shown    bool
	mode     study.Mode
	reveals  []bool
	runs     []string
	selected []session.Line
	cleared  int
	// lineText is the text of the line Select picks; edits are the texts
	// Edit was given, and editErr its answer.
	lineText string
	edits    []string
	editErr  error
	// What the menus show; changes records the menus' other calls.
	markStart time.Duration
	marked    bool
	passage   session.PracticeStatus
	plan      study.PracticePlan
	snap      bool
	saved     []watch.SavedPassage
	karaoke   session.Karaoke
	indicator study.Indicator
	repeats   int
	changes   []string
	// visible are the lines on screen and tracks the tracks, for the
	// context menu.
	visible session.Visible
	tracks  watch.Tracks
}

func newRemote(p *recordingPlayer) *fakeRemote {
	return newRemoteFor(p)
}

// newRemoteFor is a fake session driving any player.
func newRemoteFor(p session.Player) *fakeRemote {
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

func (r *fakeRemote) Mode() study.Mode {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mode
}

func (r *fakeRemote) Run(command shortcut.Command, repeat bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run := string(command)
	if repeat {
		run += " (held)"
	}
	r.runs = append(r.runs, run)
	return nil
}

func (r *fakeRemote) Select(line session.Line) (watch.Selection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.selected = append(r.selected, line)
	return watch.Selection{Line: line, Event: subtitle.Event{ID: "e1", OriginalText: r.lineText}}, nil
}

func (r *fakeRemote) ClearSelection() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleared++
}

func (r *fakeRemote) clearings() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cleared
}

func (r *fakeRemote) Edit(change watch.EditChange) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if change.Text != nil {
		r.edits = append(r.edits, *change.Text)
	}
	return r.editErr
}

func (r *fakeRemote) edited() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.edits)
}

func (r *fakeRemote) selections() []session.Line {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.selected)
}

func (r *fakeRemote) ran() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.runs)
}

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

// What the context menu reads besides the bar's state.
func (r *fakeRemote) Shown() session.Visible {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.visible
}
func (r *fakeRemote) Offset(session.Line) subtitle.Offset { return 0 }
func (r *fakeRemote) CanUndo() bool                       { return false }
func (r *fakeRemote) CanRedo() bool                       { return false }
func (r *fakeRemote) Tracks() watch.Tracks {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tracks
}

var _ watch.Remote = (*fakeRemote)(nil)

func open(t *testing.T) *playerwindow.Window {
	t.Helper()
	w, err := playerwindow.Open(t.Context(), playerwindow.Options{
		Title: ptbr.WindowTitle("v.mkv"), Placement: offScreen, VideoWidth: 1280, VideoHeight: 720,
		Labels:     playerwindow.Labels{Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime},
		NoActivate: true,
		// No menu ever shows on screen in the tests.
		Choose: func(uintptr, int32, int32) uint32 { return 0 },
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	return w
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
	scanDigit3     = 0x04
	scanDigit0     = 0x0B
	scanBackspace  = 0x0E
	scanP          = 0x19
	scanF          = 0x21
	scanM          = 0x32
	scanComma      = 0x33
	scanPeriod     = 0x34
	scanAlt        = 0x38 // the right Alt when extended
	scanSpace      = 0x39
	scanF9         = 0x43
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
	testkit.Slow(t)
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
	remote := newRemote(&recordingPlayer{})
	w.Attach(remote)
	// The default keys of the session's commands, without modifiers (a
	// posted message leaves the keyboard state alone).
	presses := []struct {
		scan     uintptr
		extended bool
	}{
		{scanSpace, false}, {scanArrowRight, true}, {scanM, false}, {scanBackspace, false},
		{scanComma, false}, {scanPeriod, false}, {scanF9, false}, {scanDigit3, false}, {scanDigit0, false},
	}
	for _, press := range presses {
		post(t, host, win32.WM_KEYDOWN, 0, key(press.scan, press.extended, false))
	}
	want := []string{
		"play-pause", "seek-forward", "mute", "replay-line",
		"frame-back", "frame-forward", "repeat-phrase", "repeat-last-3", "repeat-last-10",
	}
	testkit.Eventually(t, "every key to run its command", func() bool { return len(remote.ran()) >= len(want) })
	if got := remote.ran(); !slices.Equal(got, want) {
		t.Fatalf("ran %q, want %q (the key before the session ran nothing)", got, want)
	}
}

func TestTheBarJumpsAndTheLockedTranslation(t *testing.T) {
	w := open(t)
	host := windows.HWND(w.Handle())
	remote := newRemote(&recordingPlayer{})
	remote.mode = study.ModeAlwaysTranslated
	w.Attach(remote)
	bar := controlBar(host)
	post(t, host, win32.WM_MOUSEMOVE, 0, at(320, 340))
	testkit.Eventually(t, "the bar to show", func() bool { return windows.IsWindowVisible(bar) })
	// From the left padding of 12: play 12..52, a gap of 4, back 56..96,
	// forward 100..140 (the bar starts at client x 0). The translation
	// (x 480..520) is off in the mode that always shows it.
	post(t, bar, win32.WM_LBUTTONDOWN, 1, at(76, 60))
	post(t, bar, win32.WM_LBUTTONDOWN, 1, at(120, 60))
	post(t, bar, win32.WM_LBUTTONDOWN, 1, at(490, 60))
	testkit.Eventually(t, "the jumps", func() bool { return len(remote.ran()) >= 2 })
	time.Sleep(100 * time.Millisecond)
	if got := remote.ran(); !slices.Equal(got, []string{"seek-back", "seek-forward"}) {
		t.Fatalf("ran %q", got)
	}
	if !remote.TranslationVisible() {
		t.Fatal("the locked translation button hid the translation")
	}
}

func TestTheBarShowsAtTheBottomAndTakesClicks(t *testing.T) {
	testkit.Slow(t)
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
	testkit.Eventually(t, "the bar to show", func() bool { return windows.IsWindowVisible(bar) })

	// The bar window starts at the band: panel 360-58 = 302, band 302-34 =
	// 268. Play is at client (12, 318), so at (20, 60) in the bar; the
	// translation at client x 480..520 (right edge 628, fullscreen 588,
	// speed 524).
	post(t, bar, win32.WM_LBUTTONDOWN, 1, at(20, 60))
	testkit.Eventually(t, "a click on play to pause", func() bool { return p.saw("pause") })
	post(t, bar, win32.WM_LBUTTONDOWN, 1, at(490, 60))
	testkit.Eventually(t, "a click on the translation to hide it", func() bool { return !remote.TranslationVisible() })

	// The real mouse is elsewhere: the bar leaves after its delay.
	start := time.Now()
	testkit.Eventually(t, "the bar to hide", func() bool { return !windows.IsWindowVisible(bar) })
	if waited := time.Since(start); waited < settings.DefaultBar().HideDelay.Std()/2 {
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
	testkit.Eventually(t, "the window to cover its monitor", func() bool {
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

// zOrder is a window's place among the screen's windows, from the top.
func zOrder(hwnd windows.HWND) int {
	const next = 2 // GW_HWNDNEXT
	top, _, _ := getTopWindow.Call(0)
	place := 0
	for window := top; window != 0; window, _, _ = getWindow.Call(window, next) {
		if windows.HWND(window) == hwnd {
			return place
		}
		place++
	}
	return -1
}

// ownedWindow finds a window of a class that the player window owns.
func ownedWindow(host windows.HWND, class string) windows.HWND {
	var found windows.HWND
	callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		name := make([]uint16, 64)
		if n, err := windows.GetClassName(hwnd, &name[0], int32(len(name))); err == nil && windows.UTF16ToString(name[:n]) == class && hwnd != host && win32.RootOwner(hwnd) == host {
			found = hwnd
		}
		return 1
	})
	_ = windows.EnumWindows(callback, nil)
	return found
}

// TestFullscreenKeepsTheSubtitlesAndTheBarOnTop is the reported bug: a
// double click into fullscreen raised the video window above the windows it
// owns, so the subtitles and the bar stayed behind the video, even after
// leaving. Paused, as at the end of a passage, the first click of the double
// click picks the line under it first.
func TestFullscreenKeepsTheSubtitlesAndTheBarOnTop(t *testing.T) {
	w := open(t)
	host := windows.HWND(w.Handle())
	remote := newRemote(&recordingPlayer{})
	remote.clock.Observe(playback.Sample{Position: 10 * time.Second, Duration: 100 * time.Second, Speed: 1, Status: playback.StatusPaused, ObservedAt: time.Now()})
	w.Attach(remote)
	// Hidden and on screen, as in TestFullscreenAndBack.
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
	owned := map[string]windows.HWND{"subtitles": ownedWindow(host, "LingoReelOverlay"), "bar": controlBar(host)}
	onTop := func(when string) {
		t.Helper()
		for name, window := range owned {
			if window == 0 {
				t.Fatalf("no %s window", name)
			}
			if zOrder(window) > zOrder(host) {
				t.Fatalf("%s: the %s (place %d) is behind the video window (place %d)", when, name, zOrder(window), zOrder(host))
			}
		}
	}
	doubleClick := func() {
		const buttonUp = 0x0202 // WM_LBUTTONUP
		post(t, host, win32.WM_LBUTTONDOWN, 1, at(320, 180))
		post(t, host, buttonUp, 0, at(320, 180))
		post(t, host, win32.WM_LBUTTONDBLCLK, 1, at(320, 180))
		post(t, host, buttonUp, 0, at(320, 180))
	}
	onTop("windowed")
	doubleClick()
	testkit.Eventually(t, "the window to cover its monitor", func() bool { r, err := win32.WindowRect(host); return err == nil && r == monitor })
	onTop("fullscreen")
	doubleClick()
	testkit.Eventually(t, "the window back", func() bool { r, err := win32.WindowRect(host); return err == nil && r == before })
	onTop("back from fullscreen")
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

	// Space is no longer Play/Pause; P is. Held, P switches nothing more:
	// only moves through the video repeat, and say they are held.
	send(host, win32.WM_KEYDOWN, key(scanSpace, false, false))
	send(host, win32.WM_KEYDOWN, key(scanP, false, false))
	for range 3 {
		send(host, win32.WM_KEYDOWN, key(scanP, false, true))
	}
	send(host, win32.WM_KEYDOWN, key(scanArrowRight, true, false))
	send(host, win32.WM_KEYDOWN, key(scanArrowRight, true, true))
	want := []string{"play-pause", "seek-forward", "seek-forward (held)"}
	testkit.Eventually(t, "P and the held arrow", func() bool { return len(remote.ran()) >= len(want) })
	time.Sleep(50 * time.Millisecond)
	if got := remote.ran(); !slices.Equal(got, want) {
		t.Fatalf("ran %q, want %q (Space and the repeats of P did nothing)", got, want)
	}

	// The translation key (the right Alt, a system key): down shows, a
	// repeat is no press, up lets go.
	send(host, win32.WM_SYSKEYDOWN, key(scanAlt, true, false))
	send(host, win32.WM_SYSKEYDOWN, key(scanAlt, true, true))
	send(host, win32.WM_SYSKEYUP, key(scanAlt, true, true))
	testkit.Eventually(t, "the translation key down and up", func() bool { return slices.Equal(remote.revealed(), []bool{true, false}) })
	// The left Alt is not the translation key.
	send(host, win32.WM_SYSKEYDOWN, key(scanAlt, false, false))
	send(host, win32.WM_SYSKEYUP, key(scanAlt, false, true))
	// Losing the keyboard with the key down lets it go.
	send(host, win32.WM_SYSKEYDOWN, key(scanAlt, true, false))
	send(host, win32.WM_KILLFOCUS, 0)
	testkit.Eventually(t, "the key let go on losing the focus", func() bool {
		return slices.Equal(remote.revealed(), []bool{true, false, true, false})
	})
	time.Sleep(100 * time.Millisecond)
	if got := remote.revealed(); len(got) != 4 {
		t.Fatalf("reveals = %v", got)
	}
}
