//go:build windows && amd64

package playerwindow

import (
	"bytes"
	"image/jpeg"
	"math"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// The seek preview: over the track, above the pointer (or the drag), a still
// of that moment and its time. It is a window of its own, the clicks going
// through it, raised above the subtitles each time it shows; the session
// takes the stills, and the window asks again at every move and refresh.

const (
	previewClass = "LingoReelPreview"
	// keptStills is how many decoded stills the window keeps, for a pointer
	// going back and forth.
	keptStills = 8
	// maxWindowsAbove bounds the walk up the screen's windows, which may
	// change while it runs.
	maxWindowsAbove = 4096
)

var (
	previewProc         = syscall.NewCallback(dispatchPreview)
	previewClassName, _ = windows.UTF16PtrFromString(previewClass)
)

// dispatchPreview is the window procedure of the preview: it takes nothing.
func dispatchPreview(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	if message == win32.WM_MOUSEACTIVATE {
		return win32.MA_NOACTIVATE
	}
	return win32.DefWindowProc(hwnd, message, wparam, lparam)
}

func registerPreview() error {
	_, err := win32.RegisterClass(&win32.WndClassEx{WndProc: previewProc, Instance: win32.ModuleHandle(), ClassName: previewClassName})
	return err
}

// createPreview creates the preview, owned like the bar: above the video,
// hidden with it. Runs on the UI thread.
func (w *Window) createPreview() error {
	const style = win32.WS_EX_LAYERED | win32.WS_EX_TRANSPARENT | win32.WS_EX_NOACTIVATE | win32.WS_EX_TOOLWINDOW
	preview, err := win32.CreateWindow(style, win32.WS_POPUP, previewClassName, w.frame(), win32.ModuleHandle())
	if err != nil {
		return err
	}
	w.preview = preview
	return nil
}

// previewX is where the preview stands: the drag on the track, else the
// pointer over the track, when it can seek.
func (w *Window) previewX() (float64, bool) {
	switch {
	case w.drag == controlbar.ActionTrack:
		return w.dragX, true
	case w.hover.Action == controlbar.ActionTrack && w.hover.Enabled:
		return w.hoverX, true
	}
	return 0, false
}

// drawPreview shows the preview over the bar just drawn, or hides it.
func (w *Window) drawPreview(remote watch.Remote, state controlbar.State, bar controlbar.Bar) {
	x, over := w.previewX()
	moment := w.options.Labels.Moment
	if !over || moment == nil || w.preview == 0 || w.menuOpen {
		w.hidePreview()
		return
	}
	at := bar.TrackPosition(x, state.Duration)
	// Asked even with nothing to show yet: the pointer on the bar is what
	// makes the session start its stills at once.
	remote.WantThumbnail(at)
	view := remote.Thumbnail(at)
	text := moment(at)
	labelW, labelH := w.renderer.PreviewLabel(text, w.chrome.Bar, bar.Scale)
	aspect := 0.0
	var still *gdiplus.Still
	if view.Status != thumbnail.Off {
		aspect = w.stillAspect()
	}
	if view.Status == thumbnail.Ready {
		if still = w.stillOf(view.Picture); still != nil {
			aspect = float64(still.Width) / float64(still.Height)
			w.aspect = aspect
		}
	}
	area := w.clientArea()
	preview, ok := bar.PreviewAt(area, x, aspect, labelW, labelH)
	origin, err := win32.ClientRectOnScreen(w.host)
	if !ok || err != nil {
		w.hidePreview()
		return
	}
	// Whole pixels: the bitmap and the window start at the box's corner.
	left, top := math.Floor(preview.Box.X), math.Floor(preview.Box.Y)
	preview.Box = layout.Rect{X: left, Y: top, W: math.Ceil(preview.Box.Right()) - left, H: math.Ceil(preview.Box.Bottom()) - top}
	width, height := int(preview.Box.W), int(preview.Box.H)
	if w.previewDIB == nil || w.previewDIB.Width != width || w.previewDIB.Height != height {
		w.previewDIB.Close()
		if w.previewDIB, err = win32.NewDIB(width, height); err != nil {
			w.previewDIB = nil
			w.logger.Warn("seek preview bitmap failed", "error", err)
			w.hidePreview()
			return
		}
	} else {
		w.previewDIB.Clear()
	}
	look := gdiplus.PreviewLook{Style: w.chrome.Bar, Scale: bar.Scale, Still: still, Blurred: !view.Sharp, Time: text}
	if err := w.renderer.DrawPreview(w.previewDIB, preview, look); err != nil {
		w.logger.Warn("seek preview drawing failed", "error", err)
		w.hidePreview()
		return
	}
	position := win32.Point{X: origin.Left + int32(left), Y: origin.Top + int32(top)}
	if err := win32.UpdateLayered(w.preview, position, win32.Size{CX: int32(width), CY: int32(height)}, w.previewDIB.DC); err != nil {
		w.logger.Warn("seek preview update failed", "error", err)
		w.hidePreview()
		return
	}
	if !w.previewShown {
		win32.ShowWindow(w.preview, win32.SW_SHOWNA)
		w.previewShown = true
	}
	w.keepPreviewOnTop()
}

// keepPreviewOnTop puts the preview back on top when one of the app's own
// windows is over it, wherever above (other windows, hidden ones too, may
// lie between): the subtitles' window, made on another thread, shows when
// it likes and would cover it. With none of the app's above, it stays where
// it is. The preview owns nothing, so leaving its owner where it is
// (SWP_NOOWNERZORDER) keeps the app's window from jumping in front of other
// programs as the mouse passes.
func (w *Window) keepPreviewOnTop() {
	root := win32.RootOwner(w.preview)
	above := win32.WindowAbove(w.preview)
	for steps := 0; above != 0 && steps < maxWindowsAbove; steps++ {
		if win32.RootOwner(above) == root {
			_ = win32.SetWindowPos(w.preview, 0, win32.Rect{}, win32.SWP_NOMOVE|win32.SWP_NOSIZE|win32.SWP_NOACTIVATE|win32.SWP_NOOWNERZORDER)
			return
		}
		above = win32.WindowAbove(above)
	}
}

// hidePreview takes the preview off the screen; the next one shows on top
// again.
func (w *Window) hidePreview() {
	if w.previewShown {
		win32.ShowWindow(w.preview, win32.SW_HIDE)
		w.previewShown = false
	}
}

// stillAspect is the shape of the box while no still is there: the last
// still's, else the video's, else 16:9.
func (w *Window) stillAspect() float64 {
	switch {
	case w.aspect > 0:
		return w.aspect
	case w.options.VideoWidth > 0 && w.options.VideoHeight > 0:
		return float64(w.options.VideoWidth) / float64(w.options.VideoHeight)
	}
	return 16.0 / 9
}

// stillOf decodes a still once, keeping the latest few; nil when it cannot
// be decoded (an empty box shows instead).
func (w *Window) stillOf(picture thumbnail.Picture) *gdiplus.Still {
	if still, kept := w.stills[picture.At]; kept {
		return still
	}
	decoded, err := jpeg.Decode(bytes.NewReader(picture.JPEG))
	if err != nil {
		return nil
	}
	if w.stills == nil {
		w.stills = map[time.Duration]*gdiplus.Still{}
	}
	if len(w.stillOrder) >= keptStills {
		delete(w.stills, w.stillOrder[0])
		w.stillOrder = w.stillOrder[1:]
	}
	still := gdiplus.StillOf(decoded)
	w.stills[picture.At] = still
	w.stillOrder = append(w.stillOrder, picture.At)
	return still
}
