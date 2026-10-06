package layout_test

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

// monoMeasurer is an independent oracle: every character is half the font
// size wide and lines are 1.25 font sizes apart.
type monoMeasurer struct{}

func (monoMeasurer) Width(text string, font layout.Font) float64 {
	return float64(utf8.RuneCountInString(text)) * font.Size / 2
}

func (monoMeasurer) LineHeight(font layout.Font) float64 { return font.Size * 1.25 }

const eps = 1e-6

func video(w, h float64) layout.Rect { return layout.Rect{W: w, H: h} }

func defaults(v layout.Rect, primary, translation string) layout.Input {
	return layout.Input{
		Video: v, DPI: 96, Layout: appearance.DefaultLayout(),
		Primary:     layout.Text{Style: appearance.DefaultPrimaryStyle(), Content: primary},
		Translation: layout.Text{Style: appearance.DefaultTranslationStyle(), Content: translation},
	}
}

// safeArea mirrors the documented rule: side, top and bottom margins and a
// centered maximum width, all as fractions of the video.
func safeArea(in layout.Input) layout.Rect {
	l := in.Layout
	left := in.Video.X + l.MarginLeft*in.Video.W
	width := in.Video.W * (1 - l.MarginLeft - l.MarginRight)
	if limit := l.MaxWidth * in.Video.W; limit < width {
		left += (width - limit) / 2
		width = limit
	}
	top := in.Video.Y + l.MarginTop*in.Video.H
	return layout.Rect{X: left, Y: top, W: width, H: in.Video.H * (1 - l.MarginTop - l.MarginBottom)}
}

// minFontAt96 is the readable minimum at 100% scaling (documented rule).
const minFontAt96 = 10

// checkInvariants holds for every layout: lines inside the safe area (a pair
// with no room may take the bottom margin, down to the picture's edge), no
// overlap between the blocks, the order the layout names, and no word lost
// or invented.
func checkInvariants(t *testing.T, in layout.Input, frame layout.Frame) {
	t.Helper()
	area := safeArea(in)
	bottom := area.Bottom()
	if frame.Primary != nil && frame.Translation != nil {
		bottom = in.Video.Bottom()
	}
	blocks := map[string]*layout.Block{"primary": frame.Primary, "translation": frame.Translation}
	texts := map[string]string{"primary": in.Primary.Content, "translation": in.Translation.Content}
	for name, block := range blocks {
		if block == nil {
			continue
		}
		var words []string
		for _, line := range block.Lines {
			if line.X < area.X-eps || line.X+line.Width > area.Right()+eps {
				t.Errorf("%s line %q at x %v..%v leaves the safe area %v..%v", name, line.Text, line.X, line.X+line.Width, area.X, area.Right())
			}
			if line.Y < area.Y-eps || line.Y+line.Height > bottom+eps {
				t.Errorf("%s line %q at y %v..%v leaves the safe area %v..%v", name, line.Text, line.Y, line.Y+line.Height, area.Y, bottom)
			}
			words = append(words, strings.Fields(line.Text)...)
		}
		if got, want := strings.Join(words, ""), strings.Join(strings.Fields(texts[name]), ""); got != want {
			t.Errorf("%s text changed:\n got  %q\n want %q", name, got, want)
		}
	}
	if p, tr := frame.Primary, frame.Translation; p != nil && tr != nil {
		if p.Bounds.Y < tr.Bounds.Bottom()-eps && tr.Bounds.Y < p.Bounds.Bottom()-eps {
			t.Errorf("blocks overlap: primary %+v, translation %+v", p.Bounds, tr.Bounds)
		}
		checkOrder(t, in, frame)
	}
}

// checkOrder holds for every pair: the line the layout puts on top never
// sits below the other one.
func checkOrder(t *testing.T, in layout.Input, frame layout.Frame) {
	t.Helper()
	upper, lower := frame.Primary, frame.Translation
	if in.Layout.TranslationOnTop {
		upper, lower = lower, upper
	}
	if upper.Bounds.Bottom() > lower.Bounds.Bottom()+eps {
		t.Errorf("the order flipped (translation on top: %v): upper %+v, lower %+v", in.Layout.TranslationOnTop, upper.Bounds, lower.Bounds)
	}
}

func TestDefaultsScaleWithTheVideo(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]float64{{1280, 720}, {1920, 1080}, {2560, 1440}, {3840, 2160}} {
		w, h := size[0], size[1]
		in := defaults(video(w, h), "Hello there.", "Olá.")
		frame := layout.Compute(in, monoMeasurer{})
		checkInvariants(t, in, frame)
		p, tr := frame.Primary, frame.Translation
		if p == nil || tr == nil || len(p.Lines) != 1 || len(tr.Lines) != 1 {
			t.Fatalf("%vx%v: frame = %+v", w, h, frame)
		}
		if got := p.Bounds.Bottom(); math.Abs(got-0.82*h) > eps {
			t.Errorf("%vx%v: primary bottom = %v, want %v (82%%)", w, h, got, 0.82*h)
		}
		if got := tr.Bounds.Bottom(); math.Abs(got-0.91*h) > eps {
			t.Errorf("%vx%v: translation bottom = %v, want %v (91%%)", w, h, got, 0.91*h)
		}
		if got, want := p.Font.Size, 32*h/1080; math.Abs(got-want) > eps {
			t.Errorf("%vx%v: primary font = %v px, want %v", w, h, got, want)
		}
		if got, want := tr.Font.Size, 28*h/1080; math.Abs(got-want) > eps {
			t.Errorf("%vx%v: translation font = %v px, want %v", w, h, got, want)
		}
		if center := p.Lines[0].X + p.Lines[0].Width/2; math.Abs(center-w/2) > eps {
			t.Errorf("%vx%v: primary centered at %v, want %v", w, h, center, w/2)
		}
	}
}

func TestLetterboxedVideoPositionsInsideThePicture(t *testing.T) {
	t.Parallel()
	picture := layout.VideoRect(layout.Rect{W: 1920, H: 1080}, 2400, 1000)
	in := defaults(picture, "Hi.", "Oi.")
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if got, want := frame.Primary.Bounds.Bottom(), picture.Y+0.82*picture.H; math.Abs(got-want) > eps {
		t.Fatalf("primary bottom = %v, want %v (82%% of the picture, not of the window)", got, want)
	}
	if got, want := frame.Primary.Font.Size, 32*picture.H/1080; math.Abs(got-want) > eps {
		t.Fatalf("font = %v, want %v (follows the picture height)", got, want)
	}
}

// The primary line must stay put when the translation is shown or hidden; a
// short translation must not move it either.
func TestPrimaryDoesNotDependOnAShortTranslation(t *testing.T) {
	t.Parallel()
	alone := layout.Compute(defaults(video(1920, 1080), "I'm not going there.", ""), monoMeasurer{})
	paired := layout.Compute(defaults(video(1920, 1080), "I'm not going there.", "Eu não vou para lá.\nNão mesmo."), monoMeasurer{})
	if alone.Translation != nil {
		t.Fatalf("empty translation produced a block: %+v", alone.Translation)
	}
	if !sameRect(alone.Primary.Bounds, paired.Primary.Bounds) {
		t.Fatalf("primary moved: alone %+v, with translation %+v", alone.Primary.Bounds, paired.Primary.Bounds)
	}
}

func TestLongTranslationNeverCoversThePrimary(t *testing.T) {
	t.Parallel()
	in := defaults(video(1280, 720), "Short.", strings.Repeat("palavra comprida ", 60))
	in.Translation.Style.MaxLines = appearance.MaxLines
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if len(frame.Translation.Lines) < 3 {
		t.Fatalf("translation has %d lines; the test needs one that cannot fit below the primary", len(frame.Translation.Lines))
	}
}

// The order never changes on its own (user report: with a large translation
// in a box, every two-line translation jumped above the English and the
// one-line ones stayed below). A translation with no room below the primary
// stays below it at its size, and the primary rises to make room. At
// 1280x720 the primary (82%) has about 97 px below it, and this translation
// needs more even at 80%.
func TestATranslationThatDoesNotFitBelowLiftsThePrimary(t *testing.T) {
	t.Parallel()
	primary := "I'm not going there."
	translation := strings.TrimSpace(strings.Repeat("uma frase bem longa ", 45))
	in := defaults(video(1280, 720), primary, translation)
	in.Translation.Style.MaxLines = appearance.MaxLines
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)

	alone := layout.Compute(defaults(video(1280, 720), primary, ""), monoMeasurer{})
	if frame.Primary.Bounds.Bottom() >= alone.Primary.Bounds.Bottom()-eps {
		t.Fatalf("the primary stayed at %+v, over the translation's place", frame.Primary.Bounds)
	}
	if got, want := frame.Translation.Bounds.Bottom(), 0.91*720; math.Abs(got-want) > eps {
		t.Fatalf("translation bottom = %v, want its own height %v", got, want)
	}
	if got, want := frame.Translation.Font.Size, 28*720.0/1080*0.8; got < want-eps {
		t.Fatalf("translation font = %v; with room above it must not shrink below 80%% (%v)", got, want)
	}
}

// The user's own profile: big lines in boxes (69 px bold study line at 82%,
// 63 px translation at 91%, padding 8, at most two lines each). One line or
// two, the translation stays below the study line, at its size.
func TestTheUsersBigBoxedLinesKeepTheirOrder(t *testing.T) {
	t.Parallel()
	style := func(base appearance.SubtitleStyle, size, position float64) appearance.SubtitleStyle {
		base.FontSize, base.VerticalPosition, base.MaxLines = size, position, 2
		base.Background.Enabled, base.Background.Padding = true, 8
		return base
	}
	for _, c := range []struct{ primary, translation string }{
		{"I dream about you.", "Eu sonho com você."},
		{"Approaching jump in 3.. 2.. 1.", "Ponto de salto\nem três..."},
		{"and then move on\nwith enormous guilt...", "e depois seguir em frente\ncom uma tremenda culpa..."},
	} {
		in := defaults(video(1920, 1080), c.primary, c.translation)
		in.Primary.Style = style(in.Primary.Style, 69, 0.82)
		in.Primary.Style.Bold = true
		in.Translation.Style = style(in.Translation.Style, 63, 0.91)
		frame := layout.Compute(in, monoMeasurer{})
		checkInvariants(t, in, frame)
		if frame.Translation.Bounds.Y < frame.Primary.Bounds.Bottom()-eps {
			t.Errorf("%q: the translation %+v is not below the study line %+v", c.translation, frame.Translation.Bounds, frame.Primary.Bounds)
		}
		if got := frame.Translation.Font.Size; got < 63*0.8-eps {
			t.Errorf("%q: translation font = %v, shrunk below 80%%", c.translation, got)
		}
	}
}

// With the translation on top, it stays on top: with the default heights it
// sits right above the study line, which takes the lower height; far apart,
// each keeps its own height.
func TestTheTranslationOnTopStaysOnTop(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), "I'm not going there.\nNot at all.", "Eu não vou para lá.\nDe jeito nenhum.")
	in.Layout.TranslationOnTop = true
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if got, want := frame.Primary.Bounds.Bottom(), 0.91*1080; math.Abs(got-want) > eps {
		t.Fatalf("study line bottom = %v, want the lower height %v", got, want)
	}
	line := 1.25 * frame.Translation.Font.Size
	if got, want := frame.Primary.Bounds.Y-frame.Translation.Bounds.Bottom(), in.Layout.LineGap*line; math.Abs(got-want) > eps {
		t.Fatalf("%v px between them, want the gap %v", got, want)
	}

	far := defaults(video(1920, 1080), "Primary.", "Tradução em cima.")
	far.Layout.TranslationOnTop = true
	far.Translation.Style.VerticalPosition = 0.3
	frame = layout.Compute(far, monoMeasurer{})
	checkInvariants(t, far, frame)
	if got := frame.Translation.Bounds.Bottom(); math.Abs(got-0.3*1080) > eps {
		t.Fatalf("translation bottom = %v, want %v", got, 0.3*1080)
	}
	if got := frame.Primary.Bounds.Bottom(); math.Abs(got-0.82*1080) > eps {
		t.Fatalf("primary bottom = %v, want %v", got, 0.82*1080)
	}
}

// The heights never beat the order: a translation placed high on the picture
// with the study line on top still goes below it.
func TestTheOrderBeatsTheHeights(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), "Primary.", "Tradução.")
	in.Translation.Style.VerticalPosition = 0.3
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if got, want := frame.Translation.Bounds.Bottom(), 0.82*1080; math.Abs(got-want) > eps {
		t.Fatalf("translation bottom = %v, want the lower height %v", got, want)
	}
}

// When nothing fits even at the minimum size, the order still holds: the
// lower line goes down to the picture's edge, both shrink to the minimum,
// and the upper one keeps to the top.
func TestTheOrderHoldsEvenWhenNothingFits(t *testing.T) {
	t.Parallel()
	// 3200 characters at the 10 px minimum (5 px each, 576 px per line) need
	// about 28 lines of 12.5 px: more than the whole 360 px picture.
	in := defaults(video(640, 360), "Center.", strings.Repeat("palavra ", 400))
	in.Primary.Style.VerticalPosition = 0.5
	frame := layout.Compute(in, monoMeasurer{})
	checkOrder(t, in, frame)
	if got := frame.Translation.Font.Size; math.Abs(got-minFontAt96) > eps {
		t.Fatalf("translation font = %v, want the minimum %v before giving up", got, minFontAt96)
	}
	if got := frame.Primary.Font.Size; math.Abs(got-minFontAt96) > eps {
		t.Fatalf("primary font = %v, want the minimum %v before giving up", got, minFontAt96)
	}
	if got := frame.Translation.Bounds.Bottom(); math.Abs(got-360) > eps {
		t.Fatalf("translation bottom = %v, want the picture's edge 360", got)
	}
}

// A pair with no room takes the bottom margin before the upper line shrinks:
// at 1280x720 with 25% margins the area is 360 px tall; the study line takes
// its 35% share (about 250 px) and the translation needs about 125 px even at
// the minimum size, so they only fit once the translation goes down into the
// margin.
func TestALowerLineWithNoRoomTakesTheBottomMargin(t *testing.T) {
	t.Parallel()
	in := defaults(video(1280, 720), strings.TrimSpace(strings.Repeat("words ", 340)), strings.TrimSpace(strings.Repeat("palavras ", 255)))
	in.Primary.Style.MaxLines = appearance.MaxLines
	in.Translation.Style.MaxLines = appearance.MaxLines
	in.Layout.MarginTop, in.Layout.MarginBottom = 0.25, 0.25
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if frame.Translation.Bounds.Bottom() <= safeArea(in).Bottom()+eps {
		t.Fatalf("the translation %+v stayed inside the margin; the test needs a pair that only fits past it", frame.Translation.Bounds)
	}
	if got := frame.Translation.Font.Size; math.Abs(got-minFontAt96) > eps {
		t.Fatalf("translation font = %v: it goes into the margin only at the minimum %v", got, minFontAt96)
	}
	alone := layout.Compute(defaults(video(1280, 720), in.Primary.Content, ""), monoMeasurer{})
	if got, want := frame.Primary.Font.Size, alone.Primary.Font.Size; math.Abs(got-want) > eps {
		t.Fatalf("study line font = %v, want its own %v: it shrinks only after the margin", got, want)
	}
}

// A block may not take more than 35% of the picture height; past
// the line limit it keeps shrinking (never cut) until it fits or reaches the
// minimum. 400 words at 1920x1080: at 80% (25.6 px) they need 15 lines of
// 32 px = 480 px, more than 378 px (35%).
func TestLongTextShrinksToItsShareOfThePicture(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), strings.TrimSpace(strings.Repeat("abcd ", 400)), "")
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if got, limit := frame.Primary.Bounds.H, 0.35*1080; got > limit+eps {
		t.Fatalf("block height = %v, want at most %v", got, limit)
	}
	if got := frame.Primary.Font.Size; got >= 25.6-eps {
		t.Fatalf("font = %v, want below the 80%% floor (25.6) to respect the height share", got)
	}
}

// countingMeasurer counts measurements: in the overlay each one is a GDI+
// call on the UI thread.
type countingMeasurer struct {
	monoMeasurer
	widths int
}

func (c *countingMeasurer) Width(text string, font layout.Font) float64 {
	c.widths++
	return c.monoMeasurer.Width(text, font)
}

// Hostile text (the parsers allow 1000 characters per line) must not stall
// the overlay's UI thread. Unbroken 1000-letter words on a wide video make
// long pieces; measuring every prefix of each piece would cost about 1000
// measurements per word and per shrink step (hundreds of thousands in all),
// a binary search about 10 per piece.
func TestHostileTextNeedsFewMeasurements(t *testing.T) {
	t.Parallel()
	word := strings.Repeat("w", 1000)
	hostile := strings.TrimSuffix(strings.Repeat(word+"\n", 8), "\n")
	in := defaults(video(1920, 1080), hostile, hostile)
	in.Primary.Style.VerticalPosition = 0.5
	measurer := &countingMeasurer{}
	frame := layout.Compute(in, measurer)
	if measurer.widths > 50_000 {
		t.Fatalf("layout measured text %d times, want at most 50000", measurer.widths)
	}
	checkLinesOnly(t, in, layout.Frame{Primary: frame.Primary})
}

// The top margin keeps a subtitle placed at the very top off the edge.
func TestTopMarginKeepsTopSubtitlesOffTheEdge(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), "At the top.", "")
	in.Primary.Style.VerticalPosition = 0.01
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if got, want := frame.Primary.Bounds.Y, 0.05*1080; math.Abs(got-want) > eps {
		t.Fatalf("top = %v, want the top margin %v", got, want)
	}
}

func TestPositionsStayInsideTheBottomMargin(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), "Bottom.", "")
	in.Primary.Style.VerticalPosition = 1
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if got, want := frame.Primary.Bounds.Bottom(), 1080*(1-in.Layout.MarginBottom); math.Abs(got-want) > eps {
		t.Fatalf("bottom = %v, want %v", got, want)
	}
}

func TestFixedSizeFollowsTheDisplayDPIAndResponsiveDoesNot(t *testing.T) {
	t.Parallel()
	for _, dpi := range []int{0, 96, 120, 144, 168, 192} {
		in := defaults(video(1920, 1080), "Text.", "")
		in.DPI = dpi
		responsive := layout.Compute(in, monoMeasurer{})
		in.Primary.Style.SizeMode = appearance.SizeFixed
		fixed := layout.Compute(in, monoMeasurer{})

		scale := float64(max(dpi, 96)) / 96
		if dpi == 0 {
			scale = 1
		}
		if got, want := fixed.Primary.Font.Size, 32*scale; math.Abs(got-want) > eps {
			t.Errorf("DPI %d fixed: font = %v, want %v", dpi, got, want)
		}
		if got := responsive.Primary.Font.Size; math.Abs(got-32) > eps {
			t.Errorf("DPI %d responsive: font = %v, want 32 (video height only)", dpi, got)
		}
		if got := fixed.Primary.Scale; math.Abs(got-scale) > eps {
			t.Errorf("DPI %d fixed: scale = %v, want %v", dpi, got, scale)
		}
	}
}

func TestTinyVideoKeepsAReadableFont(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		dpi  int
		want float64
	}{{96, 10}, {144, 15}, {192, 20}} {
		in := defaults(video(320, 180), "Tiny.", "")
		in.DPI = c.dpi
		frame := layout.Compute(in, monoMeasurer{})
		if got := frame.Primary.Font.Size; math.Abs(got-c.want) > eps {
			t.Errorf("DPI %d: font = %v, want the %v px minimum", c.dpi, got, c.want)
		}
	}
}

func TestWrapFitsTheWidthAndKeepsEveryWord(t *testing.T) {
	t.Parallel()
	in := defaults(video(1280, 720), "I'm gonna have to think about it, because this is the kind of decision that changes everything we planned for the summer.", "")
	in.Primary.Style.MaxLines = appearance.MaxLines
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if len(frame.Primary.Lines) < 2 {
		t.Fatalf("expected wrapping, got %d line(s)", len(frame.Primary.Lines))
	}
	for i := 1; i < len(frame.Primary.Lines); i++ {
		if got, want := frame.Primary.Lines[i].Y-frame.Primary.Lines[i-1].Y, frame.Primary.Lines[i].Height; math.Abs(got-want) > eps {
			t.Errorf("line %d is %v below the previous, want the line height %v", i, got, want)
		}
	}
}

func TestWordWiderThanTheLineIsBrokenBetweenCharacters(t *testing.T) {
	t.Parallel()
	word := strings.Repeat("é", 400)
	in := defaults(video(1280, 720), word, "")
	in.Primary.Style.MaxLines = appearance.MaxLines
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	var joined strings.Builder
	for _, line := range frame.Primary.Lines {
		joined.WriteString(line.Text)
	}
	if joined.String() != word {
		t.Fatal("broken word does not join back to the original")
	}
}

func TestExplicitLineBreaksAreKept(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"Eu não vou para lá.\nNão mesmo.": {"Eu não vou para lá.", "Não mesmo."},
		"Eu não vou.\r\nNão mesmo.":       {"Eu não vou.", "Não mesmo."},
		"\n\nUma linha.\n  \n":            {"Uma linha."},
		"  espaços   demais  ":            {"espaços demais"},
		"tab\tno meio":                    {"tab no meio"},
	}
	for content, want := range cases {
		frame := layout.Compute(defaults(video(1920, 1080), content, ""), monoMeasurer{})
		var got []string
		for _, line := range frame.Primary.Lines {
			got = append(got, line.Text)
		}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%q: lines = %q, want %q", content, got, want)
		}
	}
}

// At 1920x1080 the safe area is 1728 px wide. At 32 px a character is 16 px,
// so 108 fit per line; 46 words of "abcd" need 3 lines at 100% and 95%, and
// 2 lines at 90% (14.4 px per character, 120 per line).
func TestMaxLinesShrinksTheFontBeforeAddingLines(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), strings.TrimSpace(strings.Repeat("abcd ", 46)), "")
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if got := len(frame.Primary.Lines); got != 2 {
		t.Fatalf("lines = %d, want 2", got)
	}
	if got := frame.Primary.Font.Size; math.Abs(got-28.8) > eps {
		t.Fatalf("font = %v, want 28.8 (90%%)", got)
	}
}

// 200 words do not fit in 2 lines even at 80% (25.6 px, 27 words per line):
// the text keeps every word on extra lines instead of being cut.
func TestTextThatCannotFitTakesExtraLinesInsteadOfBeingCut(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), strings.TrimSpace(strings.Repeat("abcd ", 200)), "")
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	if got := frame.Primary.Font.Size; math.Abs(got-25.6) > eps {
		t.Fatalf("font = %v, want 25.6 (the 80%% floor)", got)
	}
	if got := len(frame.Primary.Lines); got != 8 {
		t.Fatalf("lines = %d, want 8 (200 words, 27 per line)", got)
	}
}

// With margins of 5% the default maximum width (90%) changes nothing, so this
// uses 50%: at 1920 px the text must stay inside the centered 960 px.
func TestMaxWidthNarrowsTheLinesAroundTheCenter(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), strings.Repeat("palavra ", 40), "")
	in.Layout.MaxWidth = 0.5
	in.Primary.Style.MaxLines = appearance.MaxLines
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	for _, line := range frame.Primary.Lines {
		if line.X < 480-eps || line.X+line.Width > 1440+eps {
			t.Fatalf("line %q at %v..%v leaves the centered 480..1440", line.Text, line.X, line.X+line.Width)
		}
	}
	if len(frame.Primary.Lines) < 2 {
		t.Fatalf("got %d line(s): the narrower width must wrap", len(frame.Primary.Lines))
	}
}

func TestAlignmentUsesTheSafeAreaEdges(t *testing.T) {
	t.Parallel()
	for _, alignment := range []appearance.Alignment{appearance.AlignLeft, appearance.AlignRight, appearance.AlignCenter} {
		in := defaults(video(1920, 1080), "Aligned text.", "")
		in.Primary.Style.Alignment = alignment
		frame := layout.Compute(in, monoMeasurer{})
		checkInvariants(t, in, frame)
		area := safeArea(in)
		line := frame.Primary.Lines[0]
		var got, want float64
		switch alignment {
		case appearance.AlignLeft:
			got, want = line.X, area.X
		case appearance.AlignRight:
			got, want = line.X+line.Width, area.Right()
		default:
			got, want = line.X+line.Width/2, area.X+area.W/2
		}
		if math.Abs(got-want) > eps {
			t.Errorf("%s: edge at %v, want %v", alignment, got, want)
		}
	}
}

func TestBackgroundPaddingSurroundsTheText(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), "Boxed.", "")
	in.Primary.Style.Background = appearance.Background{Enabled: true, Color: appearance.RGB(0, 0, 0), Padding: 10}
	in.Primary.Style.Alignment = appearance.AlignLeft
	frame := layout.Compute(in, monoMeasurer{})
	checkInvariants(t, in, frame)
	block, line := frame.Primary, frame.Primary.Lines[0]
	if math.Abs(line.X-block.Bounds.X-10) > eps || math.Abs(line.Y-block.Bounds.Y-10) > eps {
		t.Fatalf("line at (%v, %v), bounds at (%v, %v): want 10 px of padding", line.X, line.Y, block.Bounds.X, block.Bounds.Y)
	}
	if got := block.Bounds.Bottom(); math.Abs(got-0.82*1080) > eps {
		t.Fatalf("bounds bottom = %v, want the anchor %v", got, 0.82*1080)
	}
	if got, want := block.Bounds.Bottom()-(line.Y+line.Height), 10.0; math.Abs(got-want) > eps {
		t.Fatalf("padding below = %v, want %v", got, want)
	}
}

func TestHostileInputsProduceNoBlocks(t *testing.T) {
	t.Parallel()
	for name, v := range map[string]layout.Rect{
		"zero": video(0, 0), "negative": video(-1920, 1080), "NaN": video(math.NaN(), 1080), "Inf": video(1920, math.Inf(1)),
	} {
		frame := layout.Compute(defaults(v, "Text.", "Texto."), monoMeasurer{})
		if frame.Primary != nil || frame.Translation != nil {
			t.Errorf("%s video: frame = %+v, want empty", name, frame)
		}
	}
	for _, content := range []string{"", " ", "\n\n", "\t \r\n"} {
		frame := layout.Compute(defaults(video(1920, 1080), content, content), monoMeasurer{})
		if frame.Primary != nil || frame.Translation != nil {
			t.Errorf("content %q: frame = %+v, want empty", content, frame)
		}
	}
	in := defaults(video(1920, 1080), "Text.", "")
	in.Primary.Style.FontSize = math.NaN()
	if frame := layout.Compute(in, monoMeasurer{}); frame.Primary != nil {
		t.Errorf("NaN font size produced %+v", frame.Primary)
	}
}

func FuzzComputeKeepsInvariants(f *testing.F) {
	f.Add("I'm gonna have to think about it.", "Vou ter que pensar sobre isso.", 1920.0, 1080.0, 0.82, 0.91, false)
	f.Add(strings.Repeat("x", 500), "a\nb\nc\nd", 320.0, 180.0, 0.1, 0.05, true)
	f.Fuzz(func(t *testing.T, primary, translation string, w, h, pPos, tPos float64, onTop bool) {
		if !(w >= 16 && w <= 8000 && h >= 16 && h <= 8000 && pPos > 0 && pPos <= 1 && tPos > 0 && tPos <= 1) {
			t.Skip()
		}
		if len(primary) > 2000 || len(translation) > 2000 {
			t.Skip()
		}
		in := defaults(video(w, h), primary, translation)
		in.Primary.Style.VerticalPosition = pPos
		in.Translation.Style.VerticalPosition = tPos
		in.Primary.Style.MaxLines = appearance.MaxLines
		in.Translation.Style.MaxLines = appearance.MaxLines
		in.Layout.TranslationOnTop = onTop
		frame := layout.Compute(in, monoMeasurer{})
		if frame.Primary == nil || frame.Translation == nil {
			checkLinesOnly(t, in, frame)
			return
		}
		// The order never flips; they may overlap only once both shrank to
		// the minimum and still found no room.
		checkOrder(t, in, frame)
		if frame.Primary.Font.Size <= minFontAt96+eps && frame.Translation.Font.Size <= minFontAt96+eps {
			frame.Translation = nil
		}
		checkLinesOnly(t, in, frame)
	})
}

// checkLinesOnly is checkInvariants without the vertical bounds, which a
// block taller than the video cannot respect.
func checkLinesOnly(t *testing.T, in layout.Input, frame layout.Frame) {
	t.Helper()
	area := safeArea(in)
	for _, block := range []*layout.Block{frame.Primary, frame.Translation} {
		if block == nil {
			continue
		}
		for _, line := range block.Lines {
			if utf8.RuneCountInString(line.Text) > 1 && (line.X < area.X-eps || line.X+line.Width > area.Right()+eps) {
				t.Fatalf("line %q at x %v..%v leaves %v..%v", line.Text, line.X, line.X+line.Width, area.X, area.Right())
			}
		}
	}
	if p, tr := frame.Primary, frame.Translation; p != nil && tr != nil {
		if p.Bounds.Y < tr.Bounds.Bottom()-eps && tr.Bounds.Y < p.Bounds.Bottom()-eps {
			t.Fatalf("blocks overlap: %+v and %+v", p.Bounds, tr.Bounds)
		}
	}
}
