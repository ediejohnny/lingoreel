//go:build windows && amd64

package playerwindow

import (
	"math"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// The box that corrects a line where the video shows it. Two clicks on a
// line of the paused video open it over the line, in the line's font. Enter
// keeps the text, Shift+Enter breaks the line, Esc leaves the line as it
// was, and leaving the box keeps the text too, as renaming a file in
// Explorer does. The times stay with the editor on the app's panel.
//
// The box is a window of its own, owned by no other: while it has the keys,
// the player window is not in the foreground, so the global shortcuts let
// go of them (typing "<" must not move the sync).

const (
	editorClass = "LingoReelEditor"
	// Sizes at 100% scaling: the narrowest box, the space around the text
	// and the hint below it, and the hint's font.
	editorMinWidth = 360.0
	editorPadding  = 8.0
	editorHint     = 22.0
	editorHintSize = 13.0
	// editorLines is how many lines of text the box has room for at least,
	// each lineSpacing times the font size tall.
	editorLines = 2
	lineSpacing = 1.3
	// editorWatch is how often the box checks the video still is paused.
	editorWatch = 250 * time.Millisecond
	// The characters Enter, Esc and Ctrl+A type.
	charEnter     = '\r'
	charEscape    = 0x1B
	charSelectAll = 0x01

	wmNCDestroy = 0x0082
	vkReturn    = 0x0D
)

// editor is the open box. Owned by the UI thread.
type editor struct {
	popup, edit, hint windows.HWND
	line              session.Line
	original          string
	font, hintFont    windows.Handle
}

var (
	editorProc      = syscall.NewCallback(dispatchEditor)
	editProc        = syscall.NewCallback(dispatchEdit)
	editorClassName *uint16
	// editorBrush paints the box; made once, with the class.
	editorBrush windows.Handle
	// editorColors: the text, the hint and the background.
	editorText       = win32.RGB(0xFF, 0xFF, 0xFF)
	editorHintText   = win32.RGB(0xA8, 0xA8, 0xB0)
	editorBackground = win32.RGB(0x1C, 0x1C, 0x1E)
	// subclassed keeps each edit control's own procedure until it is gone:
	// its last messages come after the box forgot it.
	subclassed = struct {
		sync.Mutex
		m map[windows.HWND]uintptr
	}{m: map[windows.HWND]uintptr{}}
)

func registerEditor() error {
	editorClassName, _ = windows.UTF16PtrFromString(editorClass)
	editorBrush = win32.SolidBrush(editorBackground)
	_, err := win32.RegisterClass(&win32.WndClassEx{
		WndProc: editorProc, Instance: win32.ModuleHandle(), Cursor: win32.ArrowCursor(),
		Background: editorBrush, ClassName: editorClassName,
	})
	return err
}

// editAt opens the box over the line under a double click, when the video
// is paused and a line is drawn there; false leaves the double click to the
// fullscreen.
func (w *Window) editAt(point win32.Point) bool {
	remote := w.currentRemote()
	if remote == nil || remote.State().Status == playback.StatusPlaying || w.overlay == nil {
		return false
	}
	line, hit := w.overlay.LineAt(float64(point.X), float64(point.Y))
	if !hit {
		return false
	}
	spot, ok := w.overlay.SpotOf(line)
	if !ok {
		return false
	}
	w.beginEdit(line, spot, false)
	return true
}

// beginEdit has the session pick a line for an edit, pausing a playing video
// first when asked, and opens the box over the line, at spot, with its text.
func (w *Window) beginEdit(line session.Line, spot overlay.Spot, pause bool) {
	w.do(func(r watch.Remote) error {
		if pause && r.State().Status == playback.StatusPlaying {
			if err := r.Controls().Pause(); err != nil {
				return err
			}
		}
		selection, err := r.Select(line)
		if err != nil {
			return err
		}
		text := selection.Event.Text()
		return w.thread.Post(func() { w.openEditor(line, text, spot) })
	})
}

// openEditor shows the box over the line, with its text.
func (w *Window) openEditor(line session.Line, text string, spot overlay.Spot) {
	if w.host == 0 {
		return
	}
	w.finishEdit(false, false)
	client, err := win32.ClientRectOnScreen(w.host)
	if err != nil {
		w.logger.Warn("could not open the box to correct a line", "error", err)
		return
	}
	scale := w.scale()
	bounds := editorBounds(client, spot.Box, spot.Font.Size, scale)
	// On a stage there are no global shortcuts to let go of: the box is the
	// app window's, above it and hidden with it, like a dialog.
	exStyle, owner := uint32(win32.WS_EX_TOPMOST|win32.WS_EX_TOOLWINDOW), windows.HWND(0)
	if w.options.Stage != nil {
		exStyle, owner = win32.WS_EX_TOOLWINDOW, w.parent
	}
	popup, err := win32.CreateTitledWindow(exStyle, win32.WS_POPUP|win32.WS_BORDER, editorClassName, nil, bounds, owner, win32.ModuleHandle())
	if err != nil {
		w.logger.Warn("could not open the box to correct a line", "error", err)
		return
	}
	e := &editor{popup: popup, line: line, original: text}
	width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
	pad, hint := int32(math.Round(editorPadding*scale)), int32(math.Round(editorHint*scale))
	e.edit, err = win32.CreateChild("EDIT", win32.ES_MULTILINE|win32.ES_CENTER|win32.ES_AUTOVSCROLL, popup,
		win32.Rect{Left: pad, Top: pad, Right: width - pad, Bottom: height - hint})
	if err == nil {
		e.hint, err = win32.CreateChild("STATIC", win32.SS_CENTER, popup,
			win32.Rect{Left: pad, Top: height - hint, Right: width - pad, Bottom: height - pad/2})
	}
	if err == nil {
		e.font, err = win32.CreateFont(spot.Font.Family, int32(math.Round(spot.Font.Size)), spot.Font.Bold, spot.Font.Italic)
	}
	if err == nil {
		e.hintFont, err = win32.CreateFont(appearance.AppFont, int32(math.Round(editorHintSize*scale)), false, false)
	}
	if err != nil {
		w.logger.Warn("could not open the box to correct a line", "error", err)
		e.destroy()
		return
	}
	win32.SendMessage(e.edit, win32.WM_SETFONT, uintptr(e.font), 0)
	win32.SendMessage(e.hint, win32.WM_SETFONT, uintptr(e.hintFont), 0)
	_ = win32.SetText(e.edit, strings.ReplaceAll(text, "\n", "\r\n"))
	_ = win32.SetText(e.hint, w.options.Labels.EditHint)
	// The caret after the text: a fix is usually a word, not a new line.
	end := win32.SendMessage(e.edit, win32.WM_GETTEXTLENGTH, 0, 0)
	win32.SendMessage(e.edit, win32.EM_SETSEL, end, end)
	subclassed.Lock()
	subclassed.m[e.edit] = win32.Subclass(e.edit, editProc)
	subclassed.Unlock()
	remember(popup, w)
	remember(e.edit, w)
	w.editing = e
	_ = win32.SetTimer(w.host, timerEdit, uint32(editorWatch/time.Millisecond))
	if w.options.NoActivate {
		win32.ShowWindow(popup, win32.SW_SHOWNA)
		return
	}
	win32.ShowWindow(popup, win32.SW_SHOW)
	win32.SetForegroundWindow(popup)
	win32.SetFocus(e.edit)
}

// finishEdit closes the box. keep saves a changed text through the session;
// an empty text keeps the box open when stayIfEmpty (Enter: the user is
// told), or is dropped (the box lost the keys: the line stays as it was).
func (w *Window) finishEdit(keep, stayIfEmpty bool) {
	e := w.editing
	if e == nil {
		return
	}
	if keep {
		text := strings.ReplaceAll(win32.Text(e.edit), "\r\n", "\n")
		switch {
		case text == e.original:
		case strings.TrimSpace(text) == "":
			if stayIfEmpty {
				_ = w.overlay.Notify(w.options.Labels.EditEmpty)
				return
			}
		default:
			w.do(func(r watch.Remote) error { return r.Edit(watch.EditChange{Text: &text}) })
		}
	}
	w.editing = nil
	win32.KillTimer(w.host, timerEdit)
	e.destroy()
	// On a stage the keys come back to the video, not to the app's panel.
	if w.options.Stage != nil && !w.options.NoActivate && w.host != 0 {
		win32.SetFocus(w.host)
	}
}

// watchEdit closes the box without saving once the video plays: a line is
// only corrected with the video paused.
func (w *Window) watchEdit() {
	if remote := w.currentRemote(); remote == nil || remote.State().Status == playback.StatusPlaying {
		w.finishEdit(false, false)
	}
}

// destroy removes the box's windows and fonts.
func (e *editor) destroy() {
	forget(e.popup)
	forget(e.edit)
	if e.popup != 0 && windows.IsWindow(e.popup) {
		_ = win32.DestroyWindow(e.popup) // and its controls with it
	}
	win32.DeleteObject(e.font)
	win32.DeleteObject(e.hintFont)
}

// editorBounds places the box over a line's block, on screen: its first line
// of text where the line shows (the field writes from its top), centered
// across it, wide enough to type in and tall enough for two lines plus the
// hint, which go below; never outside the player window's client area.
func editorBounds(client win32.Rect, box layout.Rect, fontSize, scale float64) win32.Rect {
	pad, hint := editorPadding*scale, editorHint*scale
	clientW, clientH := float64(client.Right-client.Left), float64(client.Bottom-client.Top)
	textHeight := max(box.H, editorLines*fontSize*lineSpacing)
	width := min(max(box.W+2*pad, editorMinWidth*scale), clientW)
	height := min(textHeight+2*pad+hint, clientH)
	x := min(max(box.X+box.W/2-width/2, 0), clientW-width)
	y := min(max(box.Y-pad, 0), clientH-height)
	left, top := client.Left+int32(math.Round(x)), client.Top+int32(math.Round(y))
	return win32.Rect{Left: left, Top: top, Right: left + int32(math.Round(width)), Bottom: top + int32(math.Round(height))}
}

// dispatchEditor is the window procedure of the box.
func dispatchEditor(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case win32.WM_CTLCOLOREDIT:
		win32.SetColors(wparam, editorText, editorBackground)
		return uintptr(editorBrush)
	case win32.WM_CTLCOLORSTATIC:
		win32.SetColors(wparam, editorHintText, editorBackground)
		return uintptr(editorBrush)
	case win32.WM_CLOSE:
		// Alt+F4 on the box, or an edit control asking its parent to close.
		if w := lookup(hwnd); w != nil {
			w.finishEdit(false, false)
		}
		return 0
	}
	return win32.DefWindowProc(hwnd, message, wparam, lparam)
}

// dispatchEdit sees the edit control's messages first: Enter, Esc, Ctrl+A
// and the loss of the keys.
func dispatchEdit(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	subclassed.Lock()
	previous := subclassed.m[hwnd]
	if message == wmNCDestroy {
		delete(subclassed.m, hwnd)
	}
	subclassed.Unlock()
	w := lookup(hwnd)
	if w == nil || w.editing == nil || w.editing.edit != hwnd {
		return win32.CallWindowProc(previous, hwnd, message, wparam, lparam)
	}
	switch message {
	case win32.WM_KEYDOWN:
		// The characters do the work; this keeps the control from acting on
		// the keys themselves.
		if wparam == win32.VK_ESCAPE || wparam == vkReturn && !win32.KeyHeld(win32.VK_SHIFT) {
			return 0
		}
	case win32.WM_CHAR:
		switch wparam {
		case charEnter:
			if !win32.KeyHeld(win32.VK_SHIFT) {
				w.finishEdit(true, true)
				return 0
			}
		case charEscape:
			w.finishEdit(false, false)
			return 0
		case charSelectAll:
			win32.SendMessage(hwnd, win32.EM_SETSEL, 0, ^uintptr(0))
			return 0
		}
	case win32.WM_KILLFOCUS:
		result := win32.CallWindowProc(previous, hwnd, message, wparam, lparam)
		w.finishEdit(true, false)
		return result
	}
	return win32.CallWindowProc(previous, hwnd, message, wparam, lparam)
}
