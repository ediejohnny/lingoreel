//go:build windows && amd64

package playerwindow_test

import (
	"math"
	"os"
	"slices"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

var (
	findWindowEx = windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowExW")
	getWindowPtr = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowLongPtrW")
	getObject    = windows.NewLazySystemDLL("gdi32.dll").NewProc("GetObjectW")
	getTextColor = windows.NewLazySystemDLL("gdi32.dll").NewProc("GetTextColor")
)

// logFont is a LOGFONTW.
type logFont struct {
	Height, Width, Escapement, Orientation, Weight int32
	Italic, Underline, StrikeOut, CharSet          byte
	OutPrecision, ClipPrecision, Quality, Pitch    byte
	FaceName                                       [32]uint16
}

// fontOf is the font a control draws its text with.
func fontOf(hwnd windows.HWND) logFont {
	var font logFont
	handle, _, _ := sendMessage.Call(uintptr(hwnd), 0x0031, 0, 0) // WM_GETFONT
	getObject.Call(handle, unsafe.Sizeof(font), uintptr(unsafe.Pointer(&font)))
	return font
}

const (
	gwOwner      = 4   // GetWindow: the owner
	gwlExStyle   = -20 // GetWindowLongPtr: the extended style
	wsExTopmost  = 0x8
	wmChar       = 0x0102
	wmSetText    = 0x000C
	wmKillFocus  = 0x0008
	wmLButtonUp  = 0x0202
	charEnterKey = '\r'
	charEscKey   = 0x1B
)

// editorRig is a paused player window showing one line, "That's sick!", on
// top, and where that line was drawn.
type editorRig struct {
	t      *testing.T
	w      *playerwindow.Window
	host   windows.HWND
	remote *fakeRemote
	box    layout.Rect
}

func newEditorRig(t *testing.T) *editorRig {
	t.Helper()
	return newEditorRigWith(t, nil)
}

// newEditorRigWith opens the rig's window with the controls' look given.
func newEditorRigWith(t *testing.T, chrome *playerwindow.Chrome) *editorRig {
	t.Helper()
	w, err := playerwindow.Open(t.Context(), playerwindow.Options{
		Title: ptbr.WindowTitle("v.mkv"), Placement: offScreen, VideoWidth: 1280, VideoHeight: 720,
		Look:       defaultLook(),
		Labels:     playerwindow.Labels{Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime, EditHint: ptbr.EditHint, EditEmpty: ptbr.EditEmpty},
		NoActivate: true, Chrome: chrome,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	r := &editorRig{t: t, w: w, host: windows.HWND(w.Handle()), remote: newRemote(&recordingPlayer{})}
	r.remote.lineText = "That's sick!"
	r.observe(playback.StatusPaused)
	w.Attach(r.remote)
	if err := w.Show(session.Screen{Primary: "That's sick!", TranslationVisible: true}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the line drawn", func() bool {
		box, ok := w.LineBounds(session.LinePrimary)
		r.box = box
		return ok
	})
	return r
}

func (r *editorRig) observe(status playback.Status) {
	r.remote.clock.Observe(playback.Sample{Position: 10 * time.Second, Duration: 100 * time.Second, Speed: 1, Status: status, ObservedAt: time.Now()})
}

// doubleClick double clicks the player window at a point of its client area.
func (r *editorRig) doubleClick(x, y int) {
	r.t.Helper()
	post(r.t, r.host, win32.WM_LBUTTONDOWN, 1, at(x, y))
	post(r.t, r.host, wmLButtonUp, 0, at(x, y))
	post(r.t, r.host, win32.WM_LBUTTONDBLCLK, 1, at(x, y))
	post(r.t, r.host, wmLButtonUp, 0, at(x, y))
}

// onTheLine double clicks the middle of the line.
func (r *editorRig) onTheLine() {
	r.t.Helper()
	r.doubleClick(int(r.box.X+r.box.W/2), int(r.box.Y+r.box.H/2))
}

// editorBox finds the box that corrects a line: a window of this process.
func editorBox() windows.HWND {
	var found windows.HWND
	callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var pid uint32
		windows.GetWindowThreadProcessId(hwnd, &pid)
		name := make([]uint16, 64)
		if n, err := windows.GetClassName(hwnd, &name[0], int32(len(name))); err == nil && pid == uint32(os.Getpid()) && windows.UTF16ToString(name[:n]) == "LingoReelEditor" {
			found = hwnd
		}
		return 1
	})
	_ = windows.EnumWindows(callback, nil)
	return found
}

// open double clicks the line and waits for the box; it returns the box and
// its text field.
func (r *editorRig) open() (box, field windows.HWND) {
	r.t.Helper()
	r.onTheLine()
	testkit.Eventually(r.t, "the box over the line", func() bool { box = editorBox(); return box != 0 && windows.IsWindowVisible(box) })
	field = child(box, "Edit")
	if field == 0 {
		r.t.Fatal("the box has no text field")
	}
	return box, field
}

func child(parent windows.HWND, class string) windows.HWND {
	name, _ := windows.UTF16PtrFromString(class)
	hwnd, _, _ := findWindowEx.Call(uintptr(parent), 0, uintptr(unsafe.Pointer(name)), 0)
	return windows.HWND(hwnd)
}

func textOf(hwnd windows.HWND) string {
	buffer := make([]uint16, 512)
	n, _, _ := sendMessage.Call(uintptr(hwnd), 0x000D, uintptr(len(buffer)), uintptr(unsafe.Pointer(&buffer[0]))) // WM_GETTEXT
	return windows.UTF16ToString(buffer[:n])
}

func setText(hwnd windows.HWND, text string) {
	utf16, _ := windows.UTF16PtrFromString(text)
	sendMessage.Call(uintptr(hwnd), wmSetText, 0, uintptr(unsafe.Pointer(utf16)))
}

// gone waits for the box to close.
func (r *editorRig) gone(box windows.HWND) {
	r.t.Helper()
	testkit.Eventually(r.t, "the box to close", func() bool { return !windows.IsWindow(box) })
}

// noEditsSince checks no text was saved beyond the first n, a moment later.
func (r *editorRig) noEditsSince(n int, what string) {
	r.t.Helper()
	sendMessage.Call(uintptr(r.host), wmNull, 0, 0)
	time.Sleep(200 * time.Millisecond) // the worker that would save
	if got := r.remote.edited(); len(got) != n {
		r.t.Fatalf("%s saved %q", what, got[n:])
	}
}

// Two clicks on a line of the paused video open the box over it, with the
// line's text, instead of the fullscreen; closing the window closes it.
func TestTwoClicksOnAPausedLineOpenTheBoxOverIt(t *testing.T) {
	testkit.Slow(t)
	r := newEditorRig(t)
	before, err := win32.WindowRect(r.host)
	if err != nil {
		t.Fatal(err)
	}
	box, field := r.open()
	if got := textOf(field); got != "That's sick!" {
		t.Fatalf("the box holds %q, want the line's text", got)
	}
	// The line's font: its family, and a height that fits the line drawn.
	font := fontOf(field)
	if face, want := windows.UTF16ToString(font.FaceName[:]), appearance.DefaultPrimaryStyle().FontFamily; face != want {
		t.Fatalf("the box writes in %q, the line in %q", face, want)
	}
	if height := float64(-font.Height); height < r.box.H/2 || height > r.box.H {
		t.Fatalf("the box's font is %v px tall for a line %v px tall", height, r.box.H)
	}
	if got := textOf(child(box, "Static")); got != ptbr.EditHint {
		t.Fatalf("hint %q", got)
	}
	if !slices.Contains(r.remote.selections(), session.LinePrimary) {
		t.Fatal("the line was not picked for the edit")
	}
	// Over the line: the middle of the line is inside the box.
	client, err := win32.ClientRectOnScreen(r.host)
	if err != nil {
		t.Fatal(err)
	}
	where, err := win32.WindowRect(box)
	if err != nil {
		t.Fatal(err)
	}
	x, y := client.Left+int32(r.box.X+r.box.W/2), client.Top+int32(r.box.Y+r.box.H/2)
	if x < where.Left || x >= where.Right || y < where.Top || y >= where.Bottom {
		t.Fatalf("box at %+v, the line's middle at (%d, %d)", where, x, y)
	}
	// The field writes from its top: its top is the line's, so the text
	// stays where the line was, and more lines go below.
	text, err := win32.WindowRect(field)
	if err != nil {
		t.Fatal(err)
	}
	if top := client.Top + int32(r.box.Y); text.Top < top-2 || text.Top > top+2 {
		t.Fatalf("the text field starts at y %d, the line at %d", text.Top, top)
	}
	// Owned by no window and over every other: while it has the keys the
	// player window is not in the foreground, so the global shortcuts let go
	// of them, and the box shows over the subtitles and the video.
	if owner, _, _ := getWindow.Call(uintptr(box), gwOwner); owner != 0 {
		t.Fatalf("the box is owned by %x", owner)
	}
	index := int64(gwlExStyle)
	if style, _, _ := getWindowPtr.Call(uintptr(box), uintptr(index)); style&wsExTopmost == 0 {
		t.Fatal("the box is not over the other windows")
	}
	if after, _ := win32.WindowRect(r.host); after != before {
		t.Fatalf("the window went from %+v to %+v: the two clicks also went fullscreen", before, after)
	}
	if err := r.w.Close(); err != nil {
		t.Fatal(err)
	}
	if windows.IsWindow(box) {
		t.Fatal("the box outlived the player window")
	}
}

// The box takes its colors and its hint's size from the controls' look.
func TestTheBoxTakesItsLook(t *testing.T) {
	testkit.Slow(t)
	chrome := playerwindow.DefaultChrome()
	orange := appearance.RGB(0xFF, 0x8C, 0x00)
	chrome.Edit = appearance.EditStyle{Background: orange, Text: appearance.RGB(0, 0, 0), Hint: appearance.RGB(0x10, 0x20, 0x30), HintSize: 20}
	r := newEditorRigWith(t, &chrome)
	box, field := r.open()
	hint := child(box, "Static")
	// The box as it paints itself: its corner, outside the field and the
	// hint, and the field's corner are the background asked for.
	picture := printedClient(t, box)
	outer, inner := clientOnScreen(t, box), clientOnScreen(t, field)
	spots := []struct {
		name string
		x, y int32
	}{{"the box", 2, 2}, {"the field", inner.Left - outer.Left + 2, inner.Top - outer.Top + 2}}
	for _, spot := range spots {
		if got := pixelAt(picture, spot.x, spot.y); got != 0xFF8C00 {
			t.Fatalf("%s is painted %06x, want ff8c00", spot.name, got)
		}
	}
	// The colors the box gives its field and its hint to write with, as
	// COLORREFs (0x00BBGGRR).
	if got := textColorFor(box, win32.WM_CTLCOLOREDIT, field); got != 0x000000 {
		t.Fatalf("the field writes in %06x, want black", got)
	}
	if got := textColorFor(box, win32.WM_CTLCOLORSTATIC, hint); got != 0x302010 {
		t.Fatalf("the hint writes in %06x, want 302010", got)
	}
	scale := float64(win32.DpiForWindow(r.host)) / 96
	if height, want := -fontOf(hint).Height, int32(math.Round(20*scale)); height != want {
		t.Fatalf("the hint's font is %d px tall, want %d", height, want)
	}
}

// printedClient is a window's client area as the window paints it, children
// included (PrintWindow through WM_PRINT: off screen, the desktop's copy of a
// window stays blank).
func printedClient(t *testing.T, hwnd windows.HWND) *win32.DIB {
	t.Helper()
	client := clientOnScreen(t, hwnd)
	dib, err := win32.NewDIB(int(client.Right-client.Left), int(client.Bottom-client.Top))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dib.Close)
	const clientOnly = 1
	if ok, _, err := printWindow.Call(uintptr(hwnd), uintptr(dib.DC), clientOnly); ok == 0 {
		t.Fatalf("PrintWindow: %v", err)
	}
	return dib
}

// pixelAt is a picture's color at a point, as 0xRRGGBB.
func pixelAt(dib *win32.DIB, x, y int32) uint32 {
	i := (int(y)*dib.Width + int(x)) * 4
	return uint32(dib.Pixels[i+2])<<16 | uint32(dib.Pixels[i+1])<<8 | uint32(dib.Pixels[i])
}

func clientOnScreen(t *testing.T, hwnd windows.HWND) win32.Rect {
	t.Helper()
	client, err := win32.ClientRectOnScreen(hwnd)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// textColorFor asks a window the colors one of its controls draws with
// (WM_CTLCOLOR...), as Windows does before the control paints, and returns
// the text color it set (a COLORREF).
func textColorFor(parent windows.HWND, message uint32, control windows.HWND) uint32 {
	dib, err := win32.NewDIB(1, 1)
	if err != nil {
		return math.MaxUint32
	}
	defer dib.Close()
	sendMessage.Call(uintptr(parent), uintptr(message), uintptr(dib.DC), uintptr(control))
	color, _, _ := getTextColor.Call(uintptr(dib.DC))
	return uint32(color)
}

// Enter saves the text (a broken line comes as one), Esc and an unchanged
// text save nothing, an empty text keeps the box open on Enter and is dropped
// when the box loses the keys, and losing the keys saves a real change.
func TestTheBoxSavesWithEnterAndLeavesWithEsc(t *testing.T) {
	testkit.Slow(t)
	r := newEditorRig(t)

	box, field := r.open()
	setText(field, "Que loucura!\r\nDe verdade")
	post(t, field, wmChar, charEnterKey, 0)
	r.gone(box)
	testkit.Eventually(t, "the text saved", func() bool { return len(r.remote.edited()) == 1 })
	if got := r.remote.edited()[0]; got != "Que loucura!\nDe verdade" {
		t.Fatalf("saved %q", got)
	}

	box, field = r.open()
	setText(field, "Not this")
	post(t, field, wmChar, charEscKey, 0)
	r.gone(box)
	r.noEditsSince(1, "Esc")

	box, _ = r.open()
	post(t, child(box, "Edit"), wmChar, charEnterKey, 0)
	r.gone(box)
	r.noEditsSince(1, "an unchanged text")

	box, field = r.open()
	setText(field, "   ")
	post(t, field, wmChar, charEnterKey, 0)
	sendMessage.Call(uintptr(r.host), wmNull, 0, 0)
	time.Sleep(200 * time.Millisecond)
	if !windows.IsWindow(box) {
		t.Fatal("Enter on an empty text closed the box")
	}
	post(t, field, wmKillFocus, 0, 0)
	r.gone(box)
	r.noEditsSince(1, "an empty text")

	box, field = r.open()
	setText(field, "Fixed.")
	post(t, field, wmKillFocus, 0, 0)
	r.gone(box)
	testkit.Eventually(t, "the text saved on leaving", func() bool { return len(r.remote.edited()) == 2 })
	if got := r.remote.edited()[1]; got != "Fixed." {
		t.Fatalf("saved %q on leaving", got)
	}
}

// A line of two lines opens as two lines (the field breaks only on CR LF),
// and saved untouched it changes nothing.
func TestALineOfTwoLinesOpensAsTwo(t *testing.T) {
	testkit.Slow(t)
	r := newEditorRig(t)
	r.remote.mu.Lock()
	r.remote.lineText = "Line one\nLine two"
	r.remote.mu.Unlock()
	box, field := r.open()
	if got := textOf(field); got != "Line one\r\nLine two" {
		t.Fatalf("the box holds %q", got)
	}
	post(t, field, wmChar, charEnterKey, 0)
	r.gone(box)
	r.noEditsSince(0, "an untouched line of two lines")
}

// A line is corrected with the video paused: the box closes, saving
// nothing, once it plays.
func TestTheBoxClosesWhenTheVideoPlays(t *testing.T) {
	testkit.Slow(t)
	r := newEditorRig(t)
	box, field := r.open()
	setText(field, "Halfway")
	r.observe(playback.StatusPlaying)
	r.gone(box)
	r.noEditsSince(0, "a box closed by the video playing")
}

// Playing, two clicks on the line go fullscreen as anywhere else.
func TestTwoClicksOnAPlayingLineGoFullscreen(t *testing.T) {
	r := newEditorRig(t)
	r.observe(playback.StatusPlaying)
	// Hidden, so the fullscreen never covers the screen (TestFullscreenAndBack).
	win32.ShowWindow(r.host, win32.SW_HIDE)
	if err := win32.SetWindowPos(r.host, 0, win32.Rect{Left: 100, Top: 100, Right: 756, Bottom: 499}, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE); err != nil {
		t.Fatal(err)
	}
	monitor, err := win32.MonitorBounds(r.host)
	if err != nil {
		t.Fatal(err)
	}
	r.onTheLine()
	testkit.Eventually(t, "the fullscreen", func() bool { rect, err := win32.WindowRect(r.host); return err == nil && rect == monitor })
	if box := editorBox(); box != 0 {
		t.Fatal("two clicks on a playing line opened the box")
	}
}
