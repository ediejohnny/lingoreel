//go:build windows && amd64

package gdiplus_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// inkOf counts a bitmap's opaque and fully clear pixels, and finds the
// strongest pixel's color, unpremultiplied.
func inkOf(dib *win32.DIB) (opaque, clear int, strongest appearance.Color) {
	for i := 0; i < len(dib.Pixels); i += 4 {
		a := dib.Pixels[i+3]
		switch {
		case a == 0:
			clear++
		case a == 0xff:
			opaque++
		}
		if a > strongest.A {
			strongest = appearance.Color{R: dib.Pixels[i+2], G: dib.Pixels[i+1], B: dib.Pixels[i], A: a}
		}
	}
	return opaque, clear, strongest
}

// Every icon draws inside its square, leaving room around it, in the color
// asked: the menu's text color, or its gray when the item is unavailable.
func TestEveryMenuIconDrawsInTheColorAsked(t *testing.T) {
	t.Parallel()
	r := newRenderer(t)
	text := appearance.Color{R: 0x10, G: 0x20, B: 0x30, A: 0xff}
	gray := appearance.Color{R: 0x90, G: 0x90, B: 0x90, A: 0xff}
	for _, icon := range contextmenu.Icons() {
		for _, color := range []appearance.Color{text, gray} {
			dib := newDIB(t, 20, 20)
			if err := r.DrawIcon(dib, icon, color); err != nil {
				t.Fatalf("icon %d: %v", icon, err)
			}
			opaque, clear, strongest := inkOf(dib)
			if opaque == 0 || clear < len(dib.Pixels)/4/4 {
				t.Errorf("icon %d: %d opaque and %d clear pixels of 400; want ink and room around it", icon, opaque, clear)
			}
			if strongest.R != color.R || strongest.G != color.G || strongest.B != color.B {
				t.Errorf("icon %d drawn in %+v, want %+v", icon, strongest, color)
			}
		}
	}
	// An icon the renderer does not know draws nothing, and clears what was there.
	dib := newDIB(t, 20, 20)
	if err := r.DrawIcon(dib, contextmenu.IconPlay, text); err != nil {
		t.Fatal(err)
	}
	if err := r.DrawIcon(dib, contextmenu.Icon(999), text); err != nil {
		t.Fatal(err)
	}
	if opaque, _, _ := inkOf(dib); opaque != 0 {
		t.Fatalf("an unknown icon left %d pixels", opaque)
	}
}

// No icon draws past its square, which is all of the menu's bitmap: on a
// bitmap wider or taller than the square, the room past it stays clear but
// for the smoothed edge of a line on the square's border (at 16 pixels the
// corners and the gear leave a third of a pixel there). The frame of "CC"
// reached past both sides at every size.
func TestNoMenuIconDrawsPastItsSquare(t *testing.T) {
	t.Parallel()
	r := newRenderer(t)
	const (
		room  = 8
		faint = 0x80 // half a pixel
	)
	text := appearance.Color{R: 0x10, G: 0x20, B: 0x30, A: 0xff}
	for _, size := range []int{16, 20, 24, 32} {
		for _, icon := range contextmenu.Icons() {
			for _, wide := range []bool{true, false} {
				width, height := size+room, size
				if !wide {
					width, height = size, size+room
				}
				dib := newDIB(t, width, height)
				if err := r.DrawIcon(dib, icon, text); err != nil {
					t.Fatal(err)
				}
				for y := range height {
					for x := range width {
						if (x >= size || y >= size) && dib.Pixels[(y*width+x)*4+3] > faint {
							t.Fatalf("icon %d at %d pixels drew past its square, at (%d, %d)", icon, size, x, y)
						}
					}
				}
			}
		}
	}
}
