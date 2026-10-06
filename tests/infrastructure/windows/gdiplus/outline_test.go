//go:build windows && amd64

package gdiplus_test

import (
	"image"
	"strings"
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
		Width: selection.Width, Radius: 6, Color: selection.Color, Edge: selection.Edge,
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

	// The line in its color and a dark edge: seen on bright and dark pictures.
	around := layout.Rect{X: outline.Box.X - 3, Y: outline.Box.Y - 3, W: outline.Box.W + 6, H: outline.Box.H + 6}
	dark := false
	for i := 0; i < len(framed.Pixels); i += 4 {
		b, g, rr, a := int(framed.Pixels[i]), int(framed.Pixels[i+1]), int(framed.Pixels[i+2]), int(framed.Pixels[i+3])
		dark = dark || (plain.dib.Pixels[i+3] == 0 && a >= 100 && b*10 <= a*3 && g*10 <= a*3 && rr*10 <= a*3)
	}
	if colored(framed, around, selection.Color) == 0 || !dark {
		t.Fatalf("line in its color %v, dark edge %v: want both", colored(framed, around, selection.Color) > 0, dark)
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

// Wherever the box falls between pixels, the frame lands on whole pixels:
// the line's rows in its color, the edge's row right outside them, and
// nothing smeared inside, so it reads as one clean line.
func TestTheOutlineIsSharp(t *testing.T) {
	r := newRenderer(t)
	plain := draw(t, r, appearance.DefaultPrimaryStyle(), appearance.DefaultTranslationStyle(), true)
	blue, black := appearance.Color{R: 0x5B, G: 0x9B, B: 0xFF, A: 0xFF}, appearance.Color{A: 0xFF}
	for _, c := range []struct {
		name       string
		width      float64
		lineRows   int
		box        layout.Rect
		x, insideY int
	}{
		{"a pixel wide at 100%", 1, 1, layout.Rect{X: 100.3, Y: 50.7, W: 200.4, H: 40.2}, 200, 52},
		{"1.5 px at 150%", 2.25, 2, layout.Rect{X: 100.6, Y: 50.2, W: 200.4, H: 40.2}, 200, 52},
	} {
		t.Run(c.name, func(t *testing.T) {
			look := plain.look
			look.Highlight = &gdiplus.Outline{Box: c.box, Width: c.width, Color: blue, Edge: black}
			framed := newDIB(t, 640, 360)
			if err := r.Draw(framed, plain.frame, look); err != nil {
				t.Fatal(err)
			}
			// Up the column through the top side, from inside the box.
			var rows []string
			for y := c.insideY; y >= 40; y-- {
				p := framed.Pixels[(y*framed.Width+c.x)*4:]
				switch {
				case p[3] == 0:
					rows = append(rows, ".")
				case p[3] == 0xFF && near(p[2], blue.R) && near(p[1], blue.G) && near(p[0], blue.B):
					rows = append(rows, "L")
				case p[3] == 0xFF && p[2] < 8 && p[1] < 8 && p[0] < 8:
					rows = append(rows, "E")
				default:
					rows = append(rows, "~") // smeared
				}
			}
			want := strings.Repeat("L", c.lineRows) + "E"
			got := strings.Trim(strings.Join(rows, ""), ".")
			if got != want {
				t.Fatalf("rows from inside out %q, want %q (L line, E edge, ~ smeared)", strings.Join(rows, ""), want)
			}
		})
	}
}
