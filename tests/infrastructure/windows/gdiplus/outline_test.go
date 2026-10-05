//go:build windows && amd64

package gdiplus_test

import (
	"image"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
)

// changedBounds is the smallest rectangle where two drawings differ.
func changedBounds(a, b []byte, width int) image.Rectangle {
	var changed image.Rectangle
	for i := 0; i+3 < len(a) && i+3 < len(b); i += 4 {
		if a[i] != b[i] || a[i+1] != b[i+1] || a[i+2] != b[i+2] || a[i+3] != b[i+3] {
			x, y := (i/4)%width, (i/4)/width
			changed = changed.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return changed
}

func TestTheOutlineFramesTheLineAndNothingElse(t *testing.T) {
	r := newRenderer(t)
	plain := draw(t, r, appearance.DefaultPrimaryStyle(), appearance.DefaultTranslationStyle(), true)
	box := plain.frame.Primary.Bounds
	selection := appearance.DefaultSelectionLook()
	outline := &gdiplus.Outline{
		Box:   layout.Rect{X: box.X - 4, Y: box.Y - 4, W: box.W + 8, H: box.H + 8},
		Width: 1.5, Radius: 6, Color: selection.Color, Edge: selection.Edge,
	}
	look := plain.look
	look.Highlight = outline
	framed := newDIB(t, 640, 360)
	if err := r.Draw(framed, plain.frame, look); err != nil {
		t.Fatal(err)
	}

	// Only the frame's edge changed: on all four sides of it, and nowhere
	// else (the translation below stays as it was).
	changed := changedBounds(plain.dib.Pixels, framed.Pixels, framed.Width)
	edge := rectOf(outline.Box)
	if changed.Empty() || !changed.In(edge.Inset(-3)) || !edge.Inset(3).In(changed) {
		t.Fatalf("the outline changed %v, want the frame %v", changed, edge)
	}
	inside := rectOf(box).Inset(2)
	if !changedBounds(cut(plain.dib.Pixels, framed.Width, inside), cut(framed.Pixels, framed.Width, inside), inside.Dx()).Empty() {
		t.Fatal("the outline painted over the line")
	}

	// A light stroke over a dark one: seen on bright and dark pictures.
	light, dark := false, false
	for i := 0; i < len(framed.Pixels); i += 4 {
		if plain.dib.Pixels[i+3] != 0 {
			continue
		}
		b, g, rr, a := int(framed.Pixels[i]), int(framed.Pixels[i+1]), int(framed.Pixels[i+2]), int(framed.Pixels[i+3])
		if a < 100 {
			continue
		}
		light = light || (b*10 >= a*8 && g*10 >= a*8 && rr*10 >= a*8)
		dark = dark || (b*10 <= a*3 && g*10 <= a*3 && rr*10 <= a*3)
	}
	if !light || !dark {
		t.Fatalf("light stroke %v, dark stroke %v: want both", light, dark)
	}

	// The strokes take the colors asked for.
	orange := appearance.Color{R: 0xFF, G: 0x8C, B: 0x00, A: 0xFF}
	look.Highlight = &gdiplus.Outline{Box: outline.Box, Width: 2, Radius: 6, Color: orange, Edge: orange}
	tinted := newDIB(t, 640, 360)
	if err := r.Draw(tinted, plain.frame, look); err != nil {
		t.Fatal(err)
	}
	frame := layout.Rect{X: outline.Box.X - 3, Y: outline.Box.Y - 3, W: outline.Box.W + 6, H: outline.Box.H + 6}
	if colored(tinted, frame, orange) == 0 {
		t.Fatal("the outline is not in the color asked for")
	}

	// No box or no width draws no outline.
	for _, none := range []*gdiplus.Outline{{Width: 1.5, Radius: 6}, {Box: outline.Box, Width: 0, Radius: 6}} {
		look.Highlight = none
		again := newDIB(t, 640, 360)
		if err := r.Draw(again, plain.frame, look); err != nil {
			t.Fatal(err)
		}
		if got := changedBounds(plain.dib.Pixels, again.Pixels, again.Width); !got.Empty() {
			t.Fatalf("outline %+v changed %v", none, got)
		}
	}
}

func TestOutlineAroundFollowsItsLookAndTheScaling(t *testing.T) {
	box := layout.Rect{X: 100, Y: 50, W: 200, H: 40}
	look := appearance.SelectionLook{
		Color: appearance.Color{R: 0xFF, A: 0xFF}, Edge: appearance.Color{B: 0xFF, A: 0x80}, Width: 3, Gap: 10, Radius: 5,
	}
	at100, at200 := gdiplus.OutlineAround(box, 96, look), gdiplus.OutlineAround(box, 192, look)
	if at100 == nil || at200 == nil {
		t.Fatalf("no outline around a box: %v, %v", at100, at200)
	}
	// The look's gap outside the box on every side, twice as far at 200%.
	for i, outline := range []*gdiplus.Outline{at100, at200} {
		gap := look.Gap * float64(i+1)
		want := layout.Rect{X: box.X - gap, Y: box.Y - gap, W: box.W + 2*gap, H: box.H + 2*gap}
		if outline.Box != want {
			t.Fatalf("the outline %+v does not frame %+v %v away", outline.Box, box, gap)
		}
		if outline.Width != look.Width*float64(i+1) || outline.Radius != look.Radius*float64(i+1) {
			t.Fatalf("stroke %v and corners %v at %d00%%", outline.Width, outline.Radius, i+1)
		}
		if outline.Color != look.Color || outline.Edge != look.Edge {
			t.Fatalf("colors %v/%v, want the look's %v/%v", outline.Color, outline.Edge, look.Color, look.Edge)
		}
	}
	// A broken scaling reads as 100%; no box, no outline.
	if broken := gdiplus.OutlineAround(box, 0, look); broken == nil || *broken != *at100 {
		t.Fatalf("at 0 dpi: %+v, want %+v", broken, at100)
	}
	for _, empty := range []layout.Rect{{}, {X: 1, Y: 1, W: 0, H: 10}, {W: 10, H: -1}} {
		if got := gdiplus.OutlineAround(empty, 96, look); got != nil {
			t.Fatalf("an outline %+v around the empty box %+v", got, empty)
		}
	}
}

// cut copies a region of a drawing, row by row.
func cut(pixels []byte, width int, region image.Rectangle) []byte {
	var out []byte
	for y := region.Min.Y; y < region.Max.Y; y++ {
		out = append(out, pixels[(y*width+region.Min.X)*4:(y*width+region.Max.X)*4]...)
	}
	return out
}
