//go:build windows && amd64

package gdiplus_test

import (
	"testing"
	"time"

	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/layout"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/infrastructure/windows/gdiplus"
	"lingoreel/internal/infrastructure/windows/win32"
)

// A 640x360 window at 100%: panel from y 302 (58 high), band from 302-34 =
// 268, so the bitmap is 640x92 and client y = bitmap y + 268. Play is at
// client (12, 318) 40x40, its 16-pixel icon at (24, 330); the track runs
// from x 12 to 628 at y 308..312, a quarter played: to x 12+616/4 = 166.
func drawBar(t *testing.T, look gdiplus.BarLook) (*win32.DIB, controlbar.Bar) {
	t.Helper()
	r := newRenderer(t)
	state := controlbar.State{
		Position: 25 * time.Second, Duration: 100 * time.Second, Speed: 1, Volume: 1,
		Capabilities: playback.Capabilities{Pause: true, Seek: true, Speed: true, Volume: true},
	}
	bar := controlbar.Layout(layout.Rect{W: 640, H: 360}, 1, state)
	if bar.Band != (layout.Rect{X: 0, Y: 268, W: 640, H: 92}) {
		t.Fatalf("band = %+v; the pixel positions below assume 268..360", bar.Band)
	}
	dib := newDIB(t, 640, 92)
	if look.Label == nil {
		look.Label = func(controlbar.Item) string { return "1,00×" }
	}
	if err := r.DrawBar(dib, bar, look); err != nil {
		t.Fatalf("DrawBar: %v", err)
	}
	return dib, bar
}

// pixel returns the premultiplied B, G, R, A at client coordinates.
func pixel(dib *win32.DIB, x, clientY int) (b, g, r, a byte) {
	i := ((clientY-268)*dib.Width + x) * 4
	return dib.Pixels[i], dib.Pixels[i+1], dib.Pixels[i+2], dib.Pixels[i+3]
}

func TestBarPaintsThePanelOnly(t *testing.T) {
	dib, _ := drawBar(t, gdiplus.BarLook{Time: "0:25 / 1:40"})
	// Above the panel, without a tip, the video shows through.
	for y := 268; y < 302; y++ {
		for x := 0; x < 640; x += 7 {
			if _, _, _, a := pixel(dib, x, y); a != 0 {
				t.Fatalf("pixel (%d, %d) painted above the panel (alpha %d)", x, y, a)
			}
		}
	}
	// An empty part of the panel has the panel's translucent dark color.
	if b, g, r, a := pixel(dib, 450, 340); a != 0xb8 || r > 0x20 || g > 0x20 || b > 0x20 {
		t.Fatalf("panel pixel = %02x%02x%02x alpha %02x, want dark at alpha b8", r, g, b, a)
	}
}

func TestBarDrawsTheTrackAndTheIcons(t *testing.T) {
	dib, _ := drawBar(t, gdiplus.BarLook{})
	// Played part of the track: the accent blue.
	if b, _, r, _ := pixel(dib, 80, 310); b < 0xc0 || r > 0x70 {
		t.Fatalf("played track pixel r %02x b %02x, want the accent blue", r, b)
	}
	// Rest of the track: a neutral light rail.
	if b, g, r, _ := pixel(dib, 400, 310); r < 0x50 || absDiff(r, b) > 0x10 || absDiff(g, b) > 0x10 {
		t.Fatalf("rail pixel %02x%02x%02x, want light grey", r, g, b)
	}
	// Playing: the pause icon, two bars at 24+16*0.12 .. +4.8; its first bar
	// covers x 26..30 and the gap between the bars x 31..33 is the panel.
	if b, g, r, _ := pixel(dib, 28, 338); r < 0xc0 || g < 0xc0 || b < 0xc0 {
		t.Fatalf("pause icon pixel %02x%02x%02x, want light", r, g, b)
	}
	if _, _, r, _ := pixel(dib, 32, 338); r > 0x40 {
		t.Fatalf("pixel between the pause bars r %02x, want the panel", r)
	}
}

func TestBarTipShowsAboveItsItem(t *testing.T) {
	plain, _ := drawBar(t, gdiplus.BarLook{})
	_, bar := drawBar(t, gdiplus.BarLook{})
	var play controlbar.Item
	for _, it := range bar.Items {
		if it.Action == controlbar.ActionPlayPause {
			play = it
		}
	}
	tipped, _ := drawBar(t, gdiplus.BarLook{Hover: play, Tip: "Pausar (Espaço)"})
	painted := 0
	for y := 268; y < 302; y++ {
		for x := 0; x < 200; x++ {
			if _, _, _, a := pixel(tipped, x, y); a > 0 {
				painted++
			}
			if _, _, _, a := pixel(plain, x, y); a > 0 {
				t.Fatal("the plain bar painted above the panel")
			}
		}
	}
	if painted < 200 {
		t.Fatalf("the tip painted %d pixels above the panel, want a box with text", painted)
	}
}

func TestDrawBarRejectsAMissingBitmap(t *testing.T) {
	r := newRenderer(t)
	bar := controlbar.Layout(layout.Rect{W: 640, H: 360}, 1, controlbar.State{})
	if err := r.DrawBar(nil, bar, gdiplus.BarLook{}); err == nil {
		t.Fatal("DrawBar on no bitmap succeeded")
	}
}

func absDiff(a, b byte) byte {
	if a > b {
		return a - b
	}
	return b - a
}
