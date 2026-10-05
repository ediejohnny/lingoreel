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
