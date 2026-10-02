//go:build windows && amd64

package playerwindow

import (
	"math"
	"time"

	"golang.org/x/sys/windows"

	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/layout"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/infrastructure/windows/gdiplus"
	"lingoreel/internal/infrastructure/windows/keyboard"
	"lingoreel/internal/infrastructure/windows/win32"
)

const (
	wmCreate      = 0x0001
	sizeMinimized = 1
)

// dispatchHost is the window procedure of the player window.
func dispatchHost(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	w := lookup(hwnd)
	if w == nil {
		return win32.DefWindowProc(hwnd, message, wparam, lparam)
	}
	switch message {
	case win32.WM_CLOSE:
		// The session ends the player and then closes the window.
		w.closedOnce.Do(func() { close(w.closed) })
		return 0
	case win32.WM_PARENTNOTIFY:
		// The player's video window must not take the mouse or the keys.
		if wparam&0xFFFF == wmCreate {
			win32.EnableWindow(windows.HWND(lparam), false)
		}
	case win32.WM_SIZE:
		if wparam == sizeMinimized {
			w.visibility.Hide()
		}
		w.refreshBar()
	case win32.WM_MOVE:
		w.refreshBar()
	case win32.WM_DPICHANGED:
		_ = win32.SetWindowPos(hwnd, 0, win32.RectAt(lparam), win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
		w.refreshBar()
		return 0
	case win32.WM_MOUSEMOVE:
		w.trackLeave(0, hwnd)
		w.showCursor()
		point := win32.PointOf(lparam)
		w.pointer(controlbar.InZone(w.clientArea(), float64(point.Y), w.scale()))
		return 0
	case win32.WM_MOUSELEAVE:
		w.tracking[0] = false
		w.pointer(w.cursorInZone())
		return 0
	case win32.WM_LBUTTONDOWN:
		if w.speedMenu {
			w.speedMenu = false
			w.pointer(w.cursorInZone())
		}
		return 0
	case win32.WM_LBUTTONDBLCLK:
		w.setFullscreen(!w.fullscreen)
		return 0
	case win32.WM_KEYDOWN, win32.WM_SYSKEYDOWN:
		// Unknown keys and Alt combinations go on: Alt+F4 must still close.
		if w.keyDown(lparam) {
			return 0
		}
	case win32.WM_KEYUP, win32.WM_SYSKEYUP:
		// Taking the right Alt's release also keeps the window menu away.
		if w.keyUp(lparam) {
			return 0
		}
	case win32.WM_KILLFOCUS:
		w.releaseReveal()
	case win32.WM_SETCURSOR:
		if w.hideCursor && lparam&0xFFFF == win32.HTCLIENT {
			win32.SetCursor(0)
			return 1
		}
	case win32.WM_TIMER:
		w.timer(wparam)
		return 0
	}
	return win32.DefWindowProc(hwnd, message, wparam, lparam)
}

// dispatchBar is the window procedure of the control bar.
func dispatchBar(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	w := lookup(hwnd)
	if w == nil {
		return win32.DefWindowProc(hwnd, message, wparam, lparam)
	}
	switch message {
	case win32.WM_MOUSEACTIVATE:
		return win32.MA_NOACTIVATE
	case win32.WM_MOUSEMOVE:
		w.trackLeave(1, hwnd)
		w.showCursor()
		x, y := w.inClient(win32.PointOf(lparam))
		w.hover = w.layout.HitTest(x, y)
		if w.drag != controlbar.ActionNone {
			w.dragTo(x)
		}
		w.pointer(true)
		return 0
	case win32.WM_MOUSELEAVE:
		w.tracking[1] = false
		w.hover = controlbar.Item{}
		w.pointer(w.cursorInZone())
		return 0
	case win32.WM_LBUTTONDOWN:
		x, y := w.inClient(win32.PointOf(lparam))
		w.press(w.layout.HitTest(x, y), x)
		return 0
	case win32.WM_LBUTTONUP:
		x, _ := w.inClient(win32.PointOf(lparam))
		w.releaseDrag(x)
		return 0
	case win32.WM_CAPTURECHANGED:
		w.drag = controlbar.ActionNone
		w.refreshBar()
		return 0
	}
	return win32.DefWindowProc(hwnd, message, wparam, lparam)
}

// trackLeave asks once per entry for the message telling the mouse left.
func (w *Window) trackLeave(index int, hwnd windows.HWND) {
	if !w.tracking[index] {
		win32.TrackMouseLeave(hwnd)
		w.tracking[index] = true
	}
}

// pointer updates the bar's visibility; active tells the mouse wants it.
func (w *Window) pointer(active bool) {
	shown, recheck := w.visibility.Update(time.Now(), active || w.drag != controlbar.ActionNone || w.speedMenu)
	if recheck > 0 {
		_ = win32.SetTimer(w.host, timerHide, uint32(recheck/time.Millisecond)+1)
	} else {
		win32.KillTimer(w.host, timerHide)
	}
	if !shown {
		w.speedMenu = false
	}
	w.refreshBar()
}

func (w *Window) timer(id uintptr) {
	switch id {
	case timerRefresh:
		// Also checks where the mouse is: a click with no move afterwards
		// sends no leave message, and the bar must still go.
		w.pointer(w.cursorInZone())
	case timerHide:
		w.pointer(w.cursorInZone())
	case timerCursor:
		win32.KillTimer(w.host, timerCursor)
		if w.fullscreen && !w.barShown {
			w.hideCursor = true
			win32.SetCursor(0)
		}
	}
}

// showCursor brings the mouse back and, in fullscreen, hides it again once
// it stays still.
func (w *Window) showCursor() {
	if w.hideCursor {
		w.hideCursor = false
		win32.SetCursor(win32.ArrowCursor())
	}
	if w.fullscreen {
		_ = win32.SetTimer(w.host, timerCursor, uint32(cursorIdle/time.Millisecond))
	}
}

// cursorInZone tells whether the mouse is over the bar's zone or the bar.
func (w *Window) cursorInZone() bool {
	point, err := win32.CursorInClient(w.host)
	if err != nil {
		return false
	}
	area := w.clientArea()
	x, y := float64(point.X), float64(point.Y)
	if !area.Contains(x, y) {
		return false
	}
	return controlbar.InZone(area, y, w.scale()) || (w.barShown && w.layout.Covers(x, y))
}

func (w *Window) clientArea() layout.Rect {
	bounds, err := win32.ClientRectOnScreen(w.host)
	if err != nil {
		return layout.Rect{}
	}
	return layout.Rect{W: float64(bounds.Right - bounds.Left), H: float64(bounds.Bottom - bounds.Top)}
}

func (w *Window) scale() float64 {
	return controlbar.Scale(win32.DpiForWindow(w.host))
}

// inClient converts a point of the bar window into the player window's
// client pixels, where the bar's layout lives.
func (w *Window) inClient(point win32.Point) (float64, float64) {
	return float64(point.X) + w.layout.Band.X, float64(point.Y) + w.layout.Band.Y
}

// press runs the item under a click.
func (w *Window) press(item controlbar.Item, x float64) {
	if !item.Enabled {
		return
	}
	if item.Action != controlbar.ActionSpeed && item.Action != controlbar.ActionSpeedChoice {
		w.speedMenu = false
	}
	switch item.Action {
	case controlbar.ActionPlayPause:
		w.do(func(r watch.Remote) error { return r.Controls().TogglePause() })
	case controlbar.ActionBack:
		w.do(func(r watch.Remote) error { return r.Controls().SeekBy(-controlbar.JumpStep) })
	case controlbar.ActionForward:
		w.do(func(r watch.Remote) error { return r.Controls().SeekBy(controlbar.JumpStep) })
	case controlbar.ActionMute:
		w.do(func(r watch.Remote) error { return r.Controls().ToggleMute() })
	case controlbar.ActionTranslation:
		w.do(func(r watch.Remote) error { r.SetTranslationVisible(!r.TranslationVisible()); return nil })
	case controlbar.ActionSpeed:
		w.speedMenu = !w.speedMenu
	case controlbar.ActionSpeedChoice:
		speed := item.Speed
		w.speedMenu = false
		w.do(func(r watch.Remote) error { return r.Controls().SetSpeed(speed) })
	case controlbar.ActionFullscreen:
		w.setFullscreen(!w.fullscreen)
	case controlbar.ActionTrack, controlbar.ActionVolume:
		w.drag = item.Action
		win32.SetCapture(w.bar)
		w.dragTo(x)
	}
	w.pointer(true)
}

// dragTo follows a drag: the volume changes at once, the track waits for
// the release to seek and meanwhile shows where it would go.
func (w *Window) dragTo(x float64) {
	w.dragX = x
	if w.drag == controlbar.ActionVolume {
		volume := w.layout.VolumeAt(x)
		w.do(func(r watch.Remote) error { return r.Controls().SetVolume(volume) })
	}
}

func (w *Window) releaseDrag(x float64) {
	action := w.drag
	if action == controlbar.ActionNone {
		return
	}
	w.dragTo(x)
	w.drag = controlbar.ActionNone
	win32.ReleaseCapture()
	if action == controlbar.ActionTrack {
		bar := w.layout
		w.do(func(r watch.Remote) error {
			return r.Controls().SeekTo(bar.TrackPosition(x, r.State().Duration))
		})
	}
	w.pointer(w.cursorInZone())
}

// keyDown runs the command of a key going down, by the profile's
// shortcuts, and reports whether the window took the key. The sync
// shortcuts never get here: their system-wide hotkeys take them first.
func (w *Window) keyDown(lparam uintptr) bool {
	key, known := keyboard.KeyOfMessage(lparam)
	if !known {
		return false
	}
	// Esc closes the speed list first, whatever its shortcut is.
	if key == shortcut.KeyEscape && w.speedMenu {
		w.speedMenu = false
		w.pointer(w.cursorInZone())
		return true
	}
	command, found := w.shortcuts().Find(keyboard.ChordOf(key, keyboard.HeldModifiers()))
	if !found {
		return false
	}
	if keyboard.Repeated(lparam) && !command.Repeats() {
		return true // a held key: once is enough
	}
	return w.run(command, key)
}

// keyUp sends the translation key's release; the others need none.
func (w *Window) keyUp(lparam uintptr) bool {
	key, known := keyboard.KeyOfMessage(lparam)
	if !known || key != w.revealKey {
		return false
	}
	w.releaseReveal()
	return true
}

// releaseReveal lets go of the translation key: on its release, or when
// the window loses the keyboard with the key down.
func (w *Window) releaseReveal() {
	if w.revealKey == "" {
		return
	}
	w.revealKey = ""
	w.do(func(r watch.Remote) error { r.Reveal(false); return nil })
}

// run carries out a command of the player window and reports whether it
// applied (leaving the fullscreen when not in it does not).
func (w *Window) run(command shortcut.Command, key shortcut.Key) bool {
	switch command {
	case shortcut.PlayPause:
		w.do(func(r watch.Remote) error { return r.Controls().TogglePause() })
	case shortcut.SeekBack:
		w.do(func(r watch.Remote) error { return r.Controls().SeekBy(-controlbar.JumpStep) })
	case shortcut.SeekForward:
		w.do(func(r watch.Remote) error { return r.Controls().SeekBy(controlbar.JumpStep) })
	case shortcut.VolumeUp:
		w.do(func(r watch.Remote) error { return r.Controls().ChangeVolume(playback.VolumeStep) })
	case shortcut.VolumeDown:
		w.do(func(r watch.Remote) error { return r.Controls().ChangeVolume(-playback.VolumeStep) })
	case shortcut.Mute:
		w.do(func(r watch.Remote) error { return r.Controls().ToggleMute() })
	case shortcut.ReplayLine:
		w.do(func(r watch.Remote) error { return r.ReplayLine() })
	case shortcut.Translation:
		w.revealKey = key
		w.do(func(r watch.Remote) error { r.Reveal(true); return nil })
	case shortcut.Fullscreen:
		w.setFullscreen(!w.fullscreen)
	case shortcut.ExitFullscreen:
		if !w.fullscreen {
			return false
		}
		w.setFullscreen(false)
	default:
		return false
	}
	return true
}

// setFullscreen makes the window cover its monitor without a frame, or
// puts it back where it was.
func (w *Window) setFullscreen(on bool) {
	if on == w.fullscreen || w.host == 0 {
		return
	}
	if on {
		placement, err := win32.Placement(w.host)
		if err != nil {
			w.logger.Warn("player window could not go fullscreen", "error", err)
			return
		}
		monitor, err := win32.MonitorBounds(w.host)
		if err != nil {
			w.logger.Warn("player window could not go fullscreen", "error", err)
			return
		}
		w.restore, w.style = placement, win32.Style(w.host)
		win32.SetStyle(w.host, w.style&^win32.WS_OVERLAPPEDWINDOW)
		_ = win32.SetWindowPos(w.host, 0, monitor, win32.SWP_NOOWNERZORDER|win32.SWP_FRAMECHANGED|win32.SWP_NOACTIVATE)
		w.fullscreen = true
		w.showCursor()
	} else {
		win32.SetStyle(w.host, w.style)
		restore := w.restore
		if restore.ShowCmd == win32.SW_SHOWNORMAL {
			// Same state without activating: the window already is, or the
			// user moved on to another one.
			restore.ShowCmd = win32.SW_SHOWNOACTIVATE
		}
		if err := win32.SetPlacement(w.host, restore); err != nil {
			w.logger.Warn("player window could not leave fullscreen", "error", err)
		}
		_ = win32.SetWindowPos(w.host, 0, win32.Rect{},
			win32.SWP_NOMOVE|win32.SWP_NOSIZE|win32.SWP_NOZORDER|win32.SWP_NOOWNERZORDER|win32.SWP_FRAMECHANGED|win32.SWP_NOACTIVATE)
		w.fullscreen = false
		win32.KillTimer(w.host, timerCursor)
		w.showCursor()
	}
	w.refreshBar()
}

// state is what the bar shows now, from the session it drives.
func (w *Window) state(remote watch.Remote) controlbar.State {
	sample := remote.State()
	controls := remote.Controls()
	volume, muted := controls.Volume()
	state := controlbar.State{
		Position: sample.Position, Duration: sample.Duration, Paused: sample.Status != playback.StatusPlaying,
		Speed: sample.Speed, Volume: volume, Muted: muted,
		HasTranslation: remote.HasTranslation(), TranslationVisible: remote.TranslationVisible(),
		Fullscreen: w.fullscreen, Capabilities: controls.Capabilities(), SpeedMenuOpen: w.speedMenu,
	}
	if w.drag == controlbar.ActionTrack {
		state.Position = w.layout.TrackPosition(w.dragX, sample.Duration)
	}
	return state
}

// refreshBar draws the bar when it shows and hides it otherwise; the
// subtitles make room for it.
func (w *Window) refreshBar() {
	if w.host == 0 || w.bar == 0 {
		return
	}
	remote := w.currentRemote()
	visible := w.visibility.Shown() && remote != nil && !win32.IsIconic(w.host)
	if visible {
		visible = w.drawBar(remote)
	}
	if visible == w.barShown {
		return
	}
	w.barShown = visible
	if visible {
		_ = win32.SetTimer(w.host, timerRefresh, uint32(refreshEvery/time.Millisecond))
		return
	}
	win32.KillTimer(w.host, timerRefresh)
	win32.ShowWindow(w.bar, win32.SW_HIDE)
	w.hover = controlbar.Item{}
	if w.overlay != nil {
		_ = w.overlay.ReserveBottom(0)
	}
	if w.fullscreen {
		w.showCursor()
	}
}

// drawBar lays out and paints the bar; it reports whether the bar shows.
func (w *Window) drawBar(remote watch.Remote) bool {
	origin, err := win32.ClientRectOnScreen(w.host)
	if err != nil {
		return false
	}
	state := w.state(remote)
	bar := controlbar.Layout(w.clientArea(), w.scale(), state)
	if !bar.Band.Valid() {
		return false
	}
	// Whole pixels: the bitmap and the window start at the band's top.
	top := math.Floor(bar.Band.Y)
	bar.Band = layout.Rect{X: 0, Y: top, W: math.Ceil(bar.Band.W), H: math.Ceil(bar.Band.Bottom() - top)}
	w.layout = bar
	width, height := int(bar.Band.W), int(bar.Band.H)
	if w.dib == nil || w.dib.Width != width || w.dib.Height != height {
		w.dib.Close()
		if w.dib, err = win32.NewDIB(width, height); err != nil {
			w.dib = nil
			w.logger.Warn("control bar bitmap failed", "error", err)
			return false
		}
	} else {
		w.dib.Clear()
	}
	look := w.look(bar, state)
	if err := w.renderer.DrawBar(w.dib, bar, look); err != nil {
		w.logger.Warn("control bar drawing failed", "error", err)
		return false
	}
	position := win32.Point{X: origin.Left, Y: origin.Top + int32(top)}
	if err := win32.UpdateLayered(w.bar, position, win32.Size{CX: int32(width), CY: int32(height)}, w.dib.DC); err != nil {
		w.logger.Warn("control bar update failed", "error", err)
		return false
	}
	win32.ShowWindow(w.bar, win32.SW_SHOWNA)
	if w.overlay != nil {
		_ = w.overlay.ReserveBottom(int(math.Ceil(bar.Panel.H)))
	}
	return true
}

func (w *Window) look(bar controlbar.Bar, state controlbar.State) gdiplus.BarLook {
	labels := w.options.Labels
	look := gdiplus.BarLook{Hover: w.hover, Label: func(controlbar.Item) string { return "" }}
	if labels.Item != nil {
		look.Label = labels.Item
	}
	if labels.Time != nil && bar.Time.Valid() {
		look.Time = labels.Time(state.Position, state.Duration)
	}
	if labels.Tip != nil && w.hover.Action != controlbar.ActionNone && w.drag == controlbar.ActionNone {
		look.Tip = labels.Tip(w.hover, state, w.shortcuts())
	}
	look.Thumb = w.drag == controlbar.ActionTrack || w.hover.Action == controlbar.ActionTrack
	// The hovered item's bounds follow the latest layout.
	for _, item := range bar.Items {
		if item.Action == w.hover.Action && item.Speed == w.hover.Speed {
			look.Hover = item
		}
	}
	return look
}
