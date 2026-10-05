//go:build windows && amd64

package gdiplus_test

import (
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// wideBar draws the bar of a 1280x720 window for a state; pixels are read
// at client coordinates through the band's top.
type wideBar struct {
	dib *win32.DIB
	bar controlbar.Bar
	top int
}

func drawWide(t *testing.T, state controlbar.State, look gdiplus.BarLook) wideBar {
	t.Helper()
	r := newRenderer(t)
	state.Position, state.Duration, state.Speed, state.Volume = 25*time.Second, 100*time.Second, 1, 1
	state.Capabilities = playback.Capabilities{Pause: true, Seek: true, Speed: true, Volume: true, FrameStepForward: true, FrameStepBackward: true}
	bar := controlbar.Layout(layout.Rect{W: 1280, H: 720}, 1, state)
	top := int(math.Floor(bar.Band.Y))
	bar.Band = layout.Rect{X: 0, Y: float64(top), W: 1280, H: math.Ceil(bar.Band.Bottom() - float64(top))}
	dib := newDIB(t, 1280, int(bar.Band.H))
	if look.Label == nil {
		look.Label = func(controlbar.Item) string { return "1,00×" }
	}
	if err := r.DrawBar(dib, bar, look); err != nil {
		t.Fatalf("DrawBar: %v", err)
	}
	return wideBar{dib: dib, bar: bar, top: top}
}

func (w wideBar) pixel(x, clientY int) (b, g, r, a byte) {
	i := ((clientY-w.top)*w.dib.Width + x) * 4
	return w.dib.Pixels[i], w.dib.Pixels[i+1], w.dib.Pixels[i+2], w.dib.Pixels[i+3]
}

// inked counts the pixels in a rectangle brighter than the panel: text and
// icons.
func (w wideBar) inked(rect layout.Rect) int {
	n := 0
	for y := int(rect.Y); y < int(rect.Bottom()); y++ {
		for x := int(rect.X); x < int(rect.Right()); x++ {
			if _, g, _, _ := w.pixel(x, y); g > 0x70 {
				n++
			}
		}
	}
	return n
}

func TestBarWritesTheStudyText(t *testing.T) {
	without := drawWide(t, controlbar.State{}, gdiplus.BarLook{})
	with := drawWide(t, controlbar.State{}, gdiplus.BarLook{Status: "Início do trecho: 10:38"})
	if !with.bar.Status.Valid() {
		t.Fatal("no room for the study text in a wide bar")
	}
	if got := without.inked(without.bar.Status); got != 0 {
		t.Fatalf("%d pixels inked where no study text goes", got)
	}
	if got := with.inked(with.bar.Status); got < 60 {
		t.Fatalf("only %d pixels of the study text", got)
	}
}

func TestBarDrawsAnOpenMenu(t *testing.T) {
	state := controlbar.State{
		OpenMenu: controlbar.MenuKaraoke,
		Study: controlbar.StudyState{
			Karaoke: true, Indicator: study.IndicatorBar, MinGap: 10 * time.Second, Lead: 5 * time.Second,
			Mode: study.ModeOnDemand, HasPrimary: true, HasTranslation: true,
		},
	}
	look := gdiplus.BarLook{Entry: func(entry controlbar.Entry) controlbar.EntryText {
		return controlbar.EntryText{Text: "Opção do menu", Value: "10 s"}
	}}
	w := drawWide(t, state, look)
	panel := w.bar.MenuPanel
	if !panel.Valid() {
		t.Fatal("the menu did not open")
	}
	// The panel's own dark color, opaque enough to read over the video.
	if _, _, _, a := w.pixel(int(panel.Right())-3, int(panel.Y+panel.H/2)); a < 0xe0 {
		t.Fatalf("menu background alpha %02x, want nearly opaque", a)
	}
	// Above the menu, the video shows through.
	if _, _, _, a := w.pixel(int(panel.X+panel.W/2), w.top); a != 0 && w.top < int(panel.Y) {
		t.Fatalf("pixel above the menu painted (alpha %02x)", a)
	}
	for _, it := range w.bar.MenuItems {
		switch {
		case it.Entry.Kind == controlbar.EntryToggle && it.Entry.Field == controlbar.FieldKaraoke:
			// Checked: the accent box at the row's left.
			x, y := int(it.Bounds.X+12+7), int(it.Bounds.Y+it.Bounds.H/2)
			if b, _, r, _ := w.pixel(x-4, y-4); b < 0xc0 || r > 0x80 {
				t.Errorf("checked box pixel r %02x b %02x, want the accent", r, b)
			}
			if got := w.inked(layout.Rect{X: it.Bounds.X + 40, Y: it.Bounds.Y, W: 160, H: it.Bounds.H}); got < 40 {
				t.Errorf("only %d pixels of the entry's text", got)
			}
		case it.Entry.Kind == controlbar.EntryStepper && it.Entry.Delta != 0:
			if got := w.inked(it.Bounds); got == 0 {
				t.Errorf("stepper button %+v drew nothing", it.Entry)
			}
		}
	}
}
