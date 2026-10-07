//go:build windows && amd64

package gdiplus_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// The seek preview drawn on its own: a 100x80 bitmap, the still's box at
// (10, 10) 64x36 (half of a 128x72 still), the time's box at (20, 52) 60x20.
var previewLayout = controlbar.Preview{
	Box:     layout.Rect{W: 100, H: 80},
	Picture: layout.Rect{X: 10, Y: 10, W: 64, H: 36},
	Label:   layout.Rect{X: 20, Y: 52, W: 60, H: 20},
}

// halves is a 128x72 still, red on the left half and blue on the right, with
// its top 8 rows green.
func halves() *gdiplus.Still {
	picture := image.NewRGBA(image.Rect(0, 0, 128, 72))
	for y := range 72 {
		for x := range 128 {
			c := color.RGBA{R: 255, A: 255}
			switch {
			case y < 8:
				c = color.RGBA{G: 255, A: 255}
			case x >= 64:
				c = color.RGBA{B: 255, A: 255}
			}
			picture.SetRGBA(x, y, c)
		}
	}
	return gdiplus.StillOf(picture)
}

func drawPreview(t *testing.T, look gdiplus.PreviewLook) *win32.DIB {
	t.Helper()
	r := newRenderer(t)
	dib := newDIB(t, 100, 80)
	look.Style, look.Scale = appearance.DefaultBarStyle(), 1
	if err := r.DrawPreview(dib, previewLayout, look); err != nil {
		t.Fatalf("DrawPreview: %v", err)
	}
	return dib
}

func at(dib *win32.DIB, x, y int) (b, g, r, a byte) {
	i := (y*dib.Width + x) * 4
	return dib.Pixels[i], dib.Pixels[i+1], dib.Pixels[i+2], dib.Pixels[i+3]
}

// The still fills its box at half its size, rows the right way up and every
// pixel opaque; outside the still and the time nothing is painted.
func TestThePreviewDrawsTheStillScaledIntoItsBox(t *testing.T) {
	dib := drawPreview(t, gdiplus.PreviewLook{Still: halves(), Time: "1:23"})
	for _, c := range []struct {
		name    string
		x, y    int
		r, g, b bool
	}{
		{"left", 25, 30, true, false, false},
		{"right", 60, 30, false, false, true},
		{"top", 40, 12, false, true, false},
	} {
		b, g, r, a := at(dib, c.x, c.y)
		if a != 0xff || (r > 0xc0) != c.r || (g > 0xc0) != c.g || (b > 0xc0) != c.b {
			t.Errorf("%s of the still (%d, %d): %02x%02x%02x alpha %02x", c.name, c.x, c.y, r, g, b, a)
		}
	}
	for _, p := range [][2]int{{2, 2}, {90, 30}, {5, 75}, {95, 75}} {
		if _, _, _, a := at(dib, p[0], p[1]); a != 0 {
			t.Errorf("(%d, %d) painted outside the still and the time (alpha %d)", p[0], p[1], a)
		}
	}
}

// A still standing for a moment it was not taken at is blurred: across the
// middle of the still, where red meets blue, many more pixels mix the two.
func TestAStillOnlyNearTheMomentIsBlurred(t *testing.T) {
	mixed := func(dib *win32.DIB) int {
		count := 0
		for x := 14; x < 70; x++ {
			if b, _, r, _ := at(dib, x, 30); r > 0x30 && b > 0x30 {
				count++
			}
		}
		return count
	}
	sharp, blurred := mixed(drawPreview(t, gdiplus.PreviewLook{Still: halves()})), mixed(drawPreview(t, gdiplus.PreviewLook{Still: halves(), Blurred: true}))
	if sharp > 2 || blurred < 6 {
		t.Fatalf("pixels mixing red and blue: %d sharp, %d blurred; want at most 2 and at least 6", sharp, blurred)
	}
}

// With no still yet the box is the panel's dark, and the time sits in a box
// of the menus' color with its text on it.
func TestThePreviewWaitsInAnEmptyBoxOverTheTime(t *testing.T) {
	dib := drawPreview(t, gdiplus.PreviewLook{Time: "1:23:45"})
	style := appearance.DefaultBarStyle()
	if _, _, _, a := at(dib, 40, 28); a != style.Panel.A {
		t.Fatalf("the empty box has alpha %02x, want the panel's %02x", a, style.Panel.A)
	}
	text, box := 0, 0
	for y := 53; y < 71; y++ { // off the rounded corners
		for x := 21; x < 79; x++ {
			b, g, r, a := at(dib, x, y)
			switch {
			case a == 0:
				t.Fatalf("(%d, %d) inside the time's box is not painted", x, y)
			case r > 0xc0 && g > 0xc0 && b > 0xc0:
				text++
			default:
				box++
			}
		}
	}
	if text < 20 || box < 400 {
		t.Fatalf("%d text pixels, %d box pixels: want the time written on its box", text, box)
	}
}
