//go:build windows && amd64

package preview_test

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"slices"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/preview"
)

// sample is the screen of the settings' preview.
var sample = session.Screen{Primary: "This is a preview.", Translation: "Esta é uma prévia.", TranslationVisible: true}

func start(t *testing.T) *preview.Renderer {
	t.Helper()
	r, err := preview.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return r
}

func decode(t *testing.T, data []byte) image.Image {
	t.Helper()
	picture, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	return picture
}

// count is how many pixels of a band of rows are close to a color.
func count(picture image.Image, top, bottom int, r, g, b uint8) int {
	n := 0
	bounds := picture.Bounds()
	for y := max(top, bounds.Min.Y); y < min(bottom, bounds.Max.Y); y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pr, pg, pb, _ := picture.At(x, y).RGBA()
			near := func(a uint32, want uint8) bool { d := int(a>>8) - int(want); return d > -12 && d < 12 }
			if near(pr, r) && near(pg, g) && near(pb, b) {
				n++
			}
		}
	}
	return n
}

func TestThePreviewShowsBothLinesWithTheirColors(t *testing.T) {
	r := start(t)
	profile := settings.DefaultProfile()
	look := watch.LookOf(profile)
	data, err := r.Preview(look, 1280, 720, 96, sample)
	if err != nil {
		t.Fatal(err)
	}
	picture := decode(t, data)
	if picture.Bounds().Dx() != 1280 || picture.Bounds().Dy() != 720 {
		t.Fatalf("size %v", picture.Bounds())
	}
	// No text at the top: the sample sky.
	if got := count(picture, 0, 2, 0x5B, 0x7C, 0x99); got < 2000 {
		t.Fatalf("only %d sky pixels in the first rows", got)
	}
	if got := count(picture, 0, 400, 0xFF, 0xFF, 0xFF); got != 0 {
		t.Fatalf("%d white pixels in the upper half, where no line goes", got)
	}
	// The primary line's bottom is at 82% (590 px), the translation's at 91%
	// (655 px); at 720 px the fonts are 32 and 28 × 720/1080 = 21 and 19 px:
	// white text in both bands. The counts are low because the default font
	// (Segoe UI, the app's own) has thinner strokes than Arial had.
	if got := count(picture, 560, 592, 0xFF, 0xFF, 0xFF); got < 150 {
		t.Fatalf("only %d white pixels where the primary line goes", got)
	}
	// The translation is 90% opaque: white over the ground (2A 2D 33) is
	// 0.9 × 255 + 0.1 × 42..51 = 234..235, about EA.
	if got := count(picture, 628, 657, 0xEA, 0xEA, 0xEA); got < 100 {
		t.Fatalf("only %d light pixels where the translation goes", got)
	}

	// A yellow primary line: yellow appears, only in its band.
	profile.Primary.Color = appearance.RGB(0xFF, 0xE6, 0x00)
	data, err = r.Preview(watch.LookOf(profile), 1280, 720, 96, sample)
	if err != nil {
		t.Fatal(err)
	}
	yellow := decode(t, data)
	if got := count(yellow, 560, 592, 0xFF, 0xE6, 0x00); got < 150 {
		t.Fatalf("only %d yellow pixels in the primary band", got)
	}
	if got := count(yellow, 600, 720, 0xFF, 0xE6, 0x00); got != 0 {
		t.Fatalf("%d yellow pixels in the translation", got)
	}
}

func TestThePreviewRefusesSizesOutsideTheLimits(t *testing.T) {
	r := start(t)
	look := watch.LookOf(settings.DefaultProfile())
	for _, c := range []struct{ w, h, dpi int }{{0, 360, 96}, {640, 8, 96}, {5000, 360, 96}, {640, 360, 0}, {640, 360, 1000}, {-1, -1, -1}} {
		if _, err := r.Preview(look, c.w, c.h, c.dpi, session.Screen{Primary: "a", Translation: "b"}); !errors.Is(err, preview.ErrInvalidSize) {
			t.Errorf("Preview(%d×%d at %d) = %v", c.w, c.h, c.dpi, err)
		}
	}
	// Hostile text is drawn, not refused, and so is the outline of a line
	// that has no text.
	hostile := session.Screen{Primary: "\x00�" + string(make([]byte, 2000)), Highlighted: true, Highlight: session.LineTranslation}
	if _, err := r.Preview(look, 320, 180, 144, hostile); err != nil {
		t.Fatalf("hostile text: %v", err)
	}
}

// changed is the smallest rectangle where two pictures differ.
func changed(a, b image.Image) image.Rectangle {
	var out image.Rectangle
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				out = out.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return out
}

func TestThePreviewOutlinesTheLineBeingEdited(t *testing.T) {
	r := start(t)
	look := watch.LookOf(settings.DefaultProfile())
	draw := func(screen session.Screen) image.Image {
		t.Helper()
		data, err := r.Preview(look, 1280, 720, 96, screen)
		if err != nil {
			t.Fatal(err)
		}
		return decode(t, data)
	}
	plain := draw(sample)
	top, bottom := sample, sample
	top.Highlighted, top.Highlight = true, session.LinePrimary
	bottom.Highlighted, bottom.Highlight = true, session.LineTranslation

	// The primary's text sits in rows 560-592 and the translation's in
	// 628-657 (see above): each outline frames its own line, a few pixels
	// around it, and stays clear of the other.
	primary := changed(plain, draw(top))
	if primary.Empty() || primary.Min.Y < 540 || primary.Max.Y > 610 || primary.Min.Y > 560 || primary.Max.Y < 592 {
		t.Fatalf("the outline of the line on top changed %v, want a frame around rows 560-592", primary)
	}
	translation := changed(plain, draw(bottom))
	if translation.Empty() || translation.Min.Y < 610 || translation.Max.Y > 680 || translation.Min.Y > 628 || translation.Max.Y < 657 {
		t.Fatalf("the outline of the line below changed %v, want a frame around rows 628-657", translation)
	}
	if primary.Overlaps(translation) {
		t.Fatalf("the two outlines overlap: %v and %v", primary, translation)
	}
	// No highlight draws exactly what an unmarked screen does.
	if again := changed(plain, draw(sample)); !again.Empty() {
		t.Fatalf("drawing the same screen twice changed %v", again)
	}
}

func TestFontsListTheInstalledFamilies(t *testing.T) {
	fonts, err := start(t).Fonts()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(fonts, "Arial") || !slices.IsSorted(fonts) {
		t.Fatalf("%d fonts, Arial missing or not sorted: %v", len(fonts), fonts[:min(len(fonts), 10)])
	}
	for _, font := range fonts {
		if font == "" || font[0] == '@' {
			t.Fatalf("font %q listed", font)
		}
	}
}
