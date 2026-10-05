//go:build windows && amd64

package gdiplus_test

import (
	"image"
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

func newRenderer(t testing.TB) *gdiplus.Renderer {
	t.Helper()
	r, err := gdiplus.NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	t.Cleanup(r.Close)
	return r
}

func newDIB(t testing.TB, w, h int) *win32.DIB {
	t.Helper()
	dib, err := win32.NewDIB(w, h)
	if err != nil {
		t.Fatalf("NewDIB: %v", err)
	}
	t.Cleanup(dib.Close)
	return dib
}

var arial32 = layout.Font{Family: "Arial", Size: 32}

func TestWidthGrowsWithTextAndSize(t *testing.T) {
	r := newRenderer(t)
	if got := r.Width("", arial32); got != 0 {
		t.Errorf("Width(\"\") = %v, want 0", got)
	}
	one, two := r.Width("a", arial32), r.Width("aa", arial32)
	if !(one > 0 && two > one) {
		t.Fatalf("Width(a) = %v, Width(aa) = %v: want 0 < a < aa", one, two)
	}
	if math.Abs(two-2*one) > 1 {
		t.Errorf("Width(aa) = %v, want about 2 × %v", two, one)
	}
	sentence := "I'm gonna have to think about it."
	small, big := r.Width(sentence, arial32), r.Width(sentence, layout.Font{Family: "Arial", Size: 64})
	if math.Abs(big/small-2) > 0.05 {
		t.Errorf("doubling the size scaled the width by %v, want about 2", big/small)
	}
	bold := r.Width(sentence, layout.Font{Family: "Arial", Size: 32, Bold: true})
	if bold <= small {
		t.Errorf("bold width %v <= regular %v", bold, small)
	}
	// Trailing spaces count: wrapping measures "word " prefixes.
	if spaced := r.Width("a ", arial32); spaced <= one {
		t.Errorf("Width(\"a \") = %v, want more than Width(\"a\") = %v", spaced, one)
	}
}

func TestWidthSurvivesHostileInput(t *testing.T) {
	r := newRenderer(t)
	for _, size := range []float64{0, -12, math.NaN(), math.Inf(1)} {
		if got := r.Width("text", layout.Font{Family: "Arial", Size: size}); got != 0 {
			t.Errorf("size %v: Width = %v, want 0", size, got)
		}
		if got := r.LineHeight(layout.Font{Family: "Arial", Size: size}); got != 0 {
			t.Errorf("size %v: LineHeight = %v, want 0", size, got)
		}
	}
	if with, without := r.Width("a\x00b", arial32), r.Width("ab", arial32); with != without {
		t.Errorf("NUL changed the width: %v vs %v", with, without)
	}
	if got := r.Width("text", layout.Font{Family: "Fonte Que Não Existe", Size: 32}); got <= 0 {
		t.Errorf("unknown family: Width = %v, want the fallback font's width", got)
	}
	if got := r.Width("😀 ação", arial32); got <= 0 {
		t.Errorf("emoji and accents: Width = %v", got)
	}
}

// Arial's line spacing is 2355 font units over an em of 2048.
func TestLineHeightFollowsTheFontMetrics(t *testing.T) {
	r := newRenderer(t)
	if got, want := r.LineHeight(arial32), 32*2355.0/2048; math.Abs(got-want) > 0.01 {
		t.Fatalf("LineHeight = %v, want %v", got, want)
	}
}

type scene struct {
	dib   *win32.DIB
	frame layout.Frame
	look  gdiplus.Look
}

func draw(t *testing.T, r *gdiplus.Renderer, primary, translation appearance.SubtitleStyle, visible bool) scene {
	t.Helper()
	return drawAt(t, r, primary, translation, visible, 640, 360)
}

func drawAt(t *testing.T, r *gdiplus.Renderer, primary, translation appearance.SubtitleStyle, visible bool, width, height int) scene {
	t.Helper()
	dib := newDIB(t, width, height)
	in := layout.Input{
		Video: layout.Rect{W: float64(width), H: float64(height)}, DPI: 96, Layout: appearance.DefaultLayout(),
		Primary:     layout.Text{Style: primary, Content: "That's sick!"},
		Translation: layout.Text{Style: translation, Content: "Que loucura!"},
	}
	frame := layout.Compute(in, r)
	look := gdiplus.Look{Primary: primary, Translation: translation, TranslationVisible: visible}
	if err := r.Draw(dib, frame, look); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	return scene{dib: dib, frame: frame, look: look}
}

// inkBounds is the smallest rectangle with non-transparent pixels in a region.
func inkBounds(dib *win32.DIB, region image.Rectangle) image.Rectangle {
	var ink image.Rectangle
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			if dib.Pixels[(y*dib.Width+x)*4+3] != 0 {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return ink
}

func rectOf(r layout.Rect) image.Rectangle {
	return image.Rect(int(math.Floor(r.X)), int(math.Floor(r.Y)), int(math.Ceil(r.Right())), int(math.Ceil(r.Bottom())))
}

// Drawn at a real video size: at 360 px the default font's stems are about
// one pixel wide, all of it antialiased, and no pixel is solid white.
func TestDrawPaintsOnlyAroundTheText(t *testing.T) {
	r := newRenderer(t)
	s := drawAt(t, r, appearance.DefaultPrimaryStyle(), appearance.DefaultTranslationStyle(), true, 1280, 720)
	whole := image.Rect(0, 0, s.dib.Width, s.dib.Height)
	ink := inkBounds(s.dib, whole)
	if ink.Empty() {
		t.Fatal("nothing was drawn")
	}
	// Outline (3 px) and shadow (2 px) scale with the video: a few pixels
	// each, so 6 px of slack covers them.
	allowed := rectOf(s.frame.Primary.Bounds).Union(rectOf(s.frame.Translation.Bounds)).Inset(-6)
	if !ink.In(allowed) {
		t.Fatalf("ink %v leaves the text area %v", ink, allowed)
	}
	white := false
	for i := 0; i < len(s.dib.Pixels); i += 4 {
		b, g, rr, a := s.dib.Pixels[i], s.dib.Pixels[i+1], s.dib.Pixels[i+2], s.dib.Pixels[i+3]
		if b > a || g > a || rr > a {
			t.Fatalf("pixel %d is not premultiplied: %v %v %v alpha %v", i/4, b, g, rr, a)
		}
		white = white || (b == 255 && g == 255 && rr == 255 && a == 255)
	}
	if !white {
		t.Fatal("no solid white pixel: the fill was not drawn")
	}
}

func TestHiddenTranslationLeavesItsAreaEmpty(t *testing.T) {
	r := newRenderer(t)
	hidden := draw(t, r, appearance.DefaultPrimaryStyle(), appearance.DefaultTranslationStyle(), false)
	area := rectOf(hidden.frame.Translation.Bounds)
	if ink := inkBounds(hidden.dib, area); !ink.Empty() {
		t.Fatalf("hidden translation left ink at %v", ink)
	}
	shown := draw(t, r, appearance.DefaultPrimaryStyle(), appearance.DefaultTranslationStyle(), true)
	if inkBounds(shown.dib, area).Empty() {
		t.Fatal("visible translation drew nothing")
	}
	if !sameRect(hidden.frame.Primary.Bounds, shown.frame.Primary.Bounds) {
		t.Fatal("primary moved when the translation was shown")
	}
}

func TestOpacityCapsTheAlpha(t *testing.T) {
	r := newRenderer(t)
	style := appearance.DefaultPrimaryStyle()
	style.Opacity = 0.5
	style.Shadow.Enabled = false
	s := draw(t, r, style, style, true)
	maxAlpha := byte(0)
	for i := 3; i < len(s.dib.Pixels); i += 4 {
		maxAlpha = max(maxAlpha, s.dib.Pixels[i])
	}
	// Opacity applies to the text as a group: the opaque fill over the
	// outline reaches exactly half, never more (layering would give 192).
	if maxAlpha < 126 || maxAlpha > 128 {
		t.Fatalf("max alpha = %d, want about 128 at 50%% opacity", maxAlpha)
	}
}

func TestBlurSpreadsTheShadow(t *testing.T) {
	r := newRenderer(t)
	sharp := appearance.DefaultPrimaryStyle()
	sharp.Outline.Enabled = false
	sharp.Shadow = appearance.Shadow{Enabled: true, Color: appearance.RGB(0, 0, 0), OffsetX: 4, OffsetY: 4}
	blurred := sharp
	blurred.Shadow.Blur = 12

	whole := image.Rect(0, 0, 640, 360)
	a := inkBounds(draw(t, r, sharp, sharp, false).dib, whole)
	b := inkBounds(draw(t, r, blurred, blurred, false).dib, whole)
	if !(b.Dx() > a.Dx() && b.Dy() > a.Dy()) {
		t.Fatalf("blurred ink %v is not wider and taller than sharp ink %v", b, a)
	}
}

func TestBackgroundFillsTheBlockBounds(t *testing.T) {
	r := newRenderer(t)
	style, err := appearance.PresetHighContrast.Apply(appearance.DefaultPrimaryStyle())
	if err != nil {
		t.Fatal(err)
	}
	s := draw(t, r, style, style, false)
	b := rectOf(s.frame.Primary.Bounds)
	// The box edges fall between pixels, so the first row and column are only
	// partly covered (antialiasing); the next ones are inside.
	x, y := int(math.Ceil(s.frame.Primary.Bounds.X))+1, int(math.Ceil(s.frame.Primary.Bounds.Y))+1
	if got := s.dib.Pixels[(y*s.dib.Width+x)*4+3]; got < 0xC0 || got > 0xD0 {
		t.Fatalf("background alpha inside the corner = %#x, want about 0xC8", got)
	}
	if got := s.dib.Pixels[((b.Min.Y-3)*s.dib.Width+b.Min.X)*4+3]; got != 0 {
		t.Fatalf("alpha just above the box = %#x, want 0", got)
	}
}

func TestNoticeIsDrawnInItsCornerWithItsBox(t *testing.T) {
	r := newRenderer(t)
	dib := newDIB(t, 640, 360)
	look := appearance.DefaultNoticeLook()
	notice := look.Style()
	frame := layout.Compute(layout.Input{
		Video: layout.Rect{W: 640, H: 360}, DPI: 96, Layout: appearance.DefaultLayout(),
		Notice: layout.Notice{
			Text:   layout.Text{Style: notice, Content: "Legenda de cima (inglês): −0,150 s"},
			Margin: look.Margin, MaxWidth: look.MaxWidth,
		},
	}, r)
	if frame.Notice == nil {
		t.Fatal("no notice laid out")
	}
	if err := r.Draw(dib, frame, gdiplus.Look{Notice: notice}); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	box := rectOf(frame.Notice.Bounds)
	ink := inkBounds(dib, image.Rect(0, 0, 640, 360))
	if ink.Empty() || !ink.In(box.Inset(-2)) {
		t.Fatalf("notice ink %v, want it inside its box %v", ink, box)
	}
	// Inside the box's corner only the see-through background: alpha 0xA0.
	x, y := box.Min.X+2, box.Min.Y+2
	if got := dib.Pixels[(y*dib.Width+x)*4+3]; got < 0x98 || got > 0xA8 {
		t.Fatalf("background alpha in the box corner = %#x, want about 0xA0", got)
	}
	// The frame decides whether there is a notice; the look only styles it.
	empty := newDIB(t, 640, 360)
	if err := r.Draw(empty, layout.Frame{}, gdiplus.Look{Notice: notice}); err != nil {
		t.Fatal(err)
	}
	if !inkBounds(empty, image.Rect(0, 0, 640, 360)).Empty() {
		t.Fatal("a frame without a notice drew one")
	}
}

func TestDrawRejectsAMissingBitmap(t *testing.T) {
	r := newRenderer(t)
	if err := r.Draw(nil, layout.Frame{}, gdiplus.Look{}); err == nil {
		t.Fatal("Draw(nil) succeeded")
	}
	if err := r.Draw(&win32.DIB{}, layout.Frame{}, gdiplus.Look{}); err == nil {
		t.Fatal("Draw(empty DIB) succeeded")
	}
}

func TestNewDIBRejectsImpossibleSizes(t *testing.T) {
	for _, size := range [][2]int{{0, 10}, {10, 0}, {-1, 10}, {20000, 10}} {
		if dib, err := win32.NewDIB(size[0], size[1]); err == nil {
			dib.Close()
			t.Errorf("NewDIB(%v) succeeded", size)
		}
	}
}

func sameRect(a, b layout.Rect) bool {
	return a == b
}
