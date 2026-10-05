//go:build windows && amd64

package preview_test

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"slices"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/preview"
	"github.com/ediejohnny/lingoreel/tests/testkit"
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

// decode reads the PNG into plain RGBA bytes, so the checks read pixels
// without a call per pixel.
func decode(t *testing.T, data []byte) *image.RGBA {
	t.Helper()
	picture, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	rgba := image.NewRGBA(picture.Bounds())
	draw.Draw(rgba, rgba.Bounds(), picture, picture.Bounds().Min, draw.Src)
	return rgba
}

// count is how many pixels of a band of rows are close to a color.
func count(picture *image.RGBA, top, bottom int, r, g, b uint8) int {
	near := func(got, want uint8) bool { d := int(got) - int(want); return d > -12 && d < 12 }
	n := 0
	bounds := picture.Bounds()
	for y := max(top, bounds.Min.Y); y < min(bottom, bounds.Max.Y); y++ {
		row := picture.Pix[picture.PixOffset(bounds.Min.X, y):picture.PixOffset(bounds.Max.X, y)]
		for i := 0; i < len(row); i += 4 {
			if near(row[i], r) && near(row[i+1], g) && near(row[i+2], b) {
				n++
			}
		}
	}
	return n
}

func TestThePreviewShowsBothLinesWithTheirColors(t *testing.T) {
	testkit.Slow(t)
	r := start(t)
	profile := settings.DefaultProfile()
	look := watch.LookOf(profile)
	data, err := r.Preview(look, 1280, 720, 96, sample, "")
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
	data, err = r.Preview(watch.LookOf(profile), 1280, 720, 96, sample, "")
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
		if _, err := r.Preview(look, c.w, c.h, c.dpi, session.Screen{Primary: "a", Translation: "b"}, ""); !errors.Is(err, preview.ErrInvalidSize) {
			t.Errorf("Preview(%d×%d at %d) = %v", c.w, c.h, c.dpi, err)
		}
	}
	// Hostile text is drawn, not refused, and so is the outline of a line
	// that has no text.
	hostile := session.Screen{Primary: "\x00�" + string(make([]byte, 2000)), Highlighted: true, Highlight: session.LineTranslation}
	if _, err := r.Preview(look, 320, 180, 144, hostile, ""); err != nil {
		t.Fatalf("hostile text: %v", err)
	}
}

// changed is the smallest rectangle where two pictures differ.
func changed(a, b *image.RGBA) image.Rectangle {
	var out image.Rectangle
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			i := a.PixOffset(x, y)
			if !bytes.Equal(a.Pix[i:i+4], b.Pix[i:i+4]) {
				out = out.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return out
}

func TestThePreviewOutlinesTheLineBeingEdited(t *testing.T) {
	testkit.Slow(t)
	r := start(t)
	look := watch.LookOf(settings.DefaultProfile())
	render := func(screen session.Screen) *image.RGBA {
		t.Helper()
		data, err := r.Preview(look, 1280, 720, 96, screen, "")
		if err != nil {
			t.Fatal(err)
		}
		return decode(t, data)
	}
	plain := render(sample)
	top, bottom := sample, sample
	top.Highlighted, top.Highlight = true, session.LinePrimary
	bottom.Highlighted, bottom.Highlight = true, session.LineTranslation

	// The primary's text sits in rows 560-592 and the translation's in
	// 628-657 (see above): each outline frames its own line, a few pixels
	// around it, and stays clear of the other.
	primary := changed(plain, render(top))
	if primary.Empty() || primary.Min.Y < 540 || primary.Max.Y > 610 || primary.Min.Y > 560 || primary.Max.Y < 592 {
		t.Fatalf("the outline of the line on top changed %v, want a frame around rows 560-592", primary)
	}
	translation := changed(plain, render(bottom))
	if translation.Empty() || translation.Min.Y < 610 || translation.Max.Y > 680 || translation.Min.Y > 628 || translation.Max.Y < 657 {
		t.Fatalf("the outline of the line below changed %v, want a frame around rows 628-657", translation)
	}
	if primary.Overlaps(translation) {
		t.Fatalf("the two outlines overlap: %v and %v", primary, translation)
	}
	// No highlight draws exactly what an unmarked screen does.
	if again := changed(plain, render(sample)); !again.Empty() {
		t.Fatalf("drawing the same screen twice changed %v", again)
	}
}

// columns is the first and last column of a band of rows with a pixel close
// to a color; first > last when there is none.
func columns(picture *image.RGBA, top, bottom int, r, g, b uint8) (first, last int) {
	bounds := picture.Bounds()
	first, last = bounds.Max.X, bounds.Min.X-1
	for y := top; y < bottom; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			p := picture.Pix[picture.PixOffset(x, y):]
			near := func(got, want uint8) bool { d := int(got) - int(want); return d > -12 && d < 12 }
			if near(p[0], r) && near(p[1], g) && near(p[2], b) {
				first, last = min(first, x), max(last, x)
			}
		}
	}
	return first, last
}

func TestThePreviewPaintsTheSpokenPartInTheFillColor(t *testing.T) {
	testkit.Slow(t)
	r := start(t)
	look := watch.LookOf(settings.DefaultProfile())
	look.FillColor = appearance.RGB(0xFF, 0x8C, 0x00)
	render := func(fill float64) *image.RGBA {
		t.Helper()
		screen := sample
		screen.Fill = fill
		data, err := r.Preview(look, 1280, 720, 96, screen, "")
		if err != nil {
			t.Fatal(err)
		}
		return decode(t, data)
	}
	// The primary's text sits in rows 560-592 (see above).
	half := render(0.5)
	orangeFirst, orangeLast := columns(half, 560, 592, 0xFF, 0x8C, 0x00)
	whiteFirst, whiteLast := columns(half, 560, 592, 0xFF, 0xFF, 0xFF)
	if orangeFirst > orangeLast || whiteFirst > whiteLast {
		t.Fatalf("half spoken: orange in columns %d-%d, white in %d-%d; want both", orangeFirst, orangeLast, whiteFirst, whiteLast)
	}
	// The spoken part begins at the line's left edge and ends before the
	// part still to come.
	if orangeLast >= whiteLast || orangeFirst > whiteFirst+4 {
		t.Fatalf("orange in columns %d-%d, white in %d-%d: the fill is not the left part of the line", orangeFirst, orangeLast, whiteFirst, whiteLast)
	}
	if first, last := columns(render(0), 560, 720, 0xFF, 0x8C, 0x00); first <= last {
		t.Fatalf("nothing spoken, yet orange in columns %d-%d", first, last)
	}
}

func TestThePreviewShowsANoticeAsTheVideoWould(t *testing.T) {
	testkit.Slow(t)
	r := start(t)
	look := watch.LookOf(settings.DefaultProfile())
	look.Notice.Color, look.Notice.Side = appearance.RGB(0xFF, 0x8C, 0x00), appearance.AlignRight
	render := func(notice string) *image.RGBA {
		t.Helper()
		data, err := r.Preview(look, 1280, 720, 96, sample, notice)
		if err != nil {
			t.Fatal(err)
		}
		return decode(t, data)
	}
	// The notice is at the top, on the right, in its own color; the
	// subtitles are lower down (see above).
	shown := render("Legenda de cima: −0,150 s")
	first, last := columns(shown, 0, 120, 0xFF, 0x8C, 0x00)
	if first > last || first < 640 {
		t.Fatalf("orange in columns %d-%d of the top rows; want the notice on the right half", first, last)
	}
	if first, last := columns(render(""), 0, 120, 0xFF, 0x8C, 0x00); first <= last {
		t.Fatalf("no notice, yet orange in columns %d-%d", first, last)
	}
}

// The bar's preview: the bar at the bottom of the sample picture, in its
// style's colors, and larger with a larger size.
func TestTheBarsPreviewShowsTheStyleAtItsSize(t *testing.T) {
	testkit.Slow(t)
	r := start(t)
	style := appearance.DefaultBarStyle()
	style.Accent = appearance.RGB(0xFF, 0x8C, 0x00)
	render := func(size float64) *image.RGBA {
		t.Helper()
		style.Size = size
		data, err := r.PreviewBar(style, 640, 360, 96, "15:00 / 1:00:00", func(playback.Speed) string { return "1,00×" })
		if err != nil {
			t.Fatal(err)
		}
		return decode(t, data)
	}
	normal, double := render(1), render(2)
	if n := count(normal, 0, 180, 0xFF, 0x8C, 0x00); n != 0 {
		t.Fatalf("%d orange pixels in the top half; the bar belongs at the bottom", n)
	}
	// The played part of the track crosses column 60 at both sizes (it runs
	// from the bar's left padding to a quarter of the track); a larger bar
	// has a thicker track.
	thickness := func(picture *image.RGBA) int {
		column := image.Rect(60, 180, 61, 360)
		return count(picture.SubImage(column).(*image.RGBA), 180, 360, 0xFF, 0x8C, 0x00)
	}
	if small, large := thickness(normal), thickness(double); small == 0 || large < small*3/2 {
		t.Fatalf("the played track is %d px thick at 1×, %d px at 2×; want it thicker at 2×", small, large)
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
