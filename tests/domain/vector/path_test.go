package vector_test

import (
	"math"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/vector"
)

func near(a, b vector.Point) bool { return nearBy(a, b, 1e-9) }

func nearBy(a, b vector.Point, d float64) bool { return math.Abs(a.X-b.X) < d && math.Abs(a.Y-b.Y) < d }

// ends lists where each segment of a figure ends.
func ends(f vector.Figure) []vector.Point {
	var points []vector.Point
	for _, s := range f.Segments {
		points = append(points, s.To)
	}
	return points
}

func parse(t *testing.T, d string) []vector.Figure {
	t.Helper()
	figures, err := vector.Parse(d)
	if err != nil {
		t.Fatalf("Parse(%q): %v", d, err)
	}
	return figures
}

// Lines in absolute and relative form land where SVG puts them: pairs after
// a move are lines, H and V keep the other coordinate, numbers may follow
// one another with no space ("-2.5-1", ".5.5").
func TestLinesLandWhereSVGPutsThem(t *testing.T) {
	t.Parallel()
	figures := parse(t, "M2 3 4 3l1-2.5-1-1h3V1.5v.5.5H2Z")
	if len(figures) != 1 || !figures[0].Closed || figures[0].Start != (vector.Point{X: 2, Y: 3}) {
		t.Fatalf("figures %+v, want one closed figure from (2, 3)", figures)
	}
	want := []vector.Point{{X: 4, Y: 3}, {X: 5, Y: .5}, {X: 4, Y: -.5}, {X: 7, Y: -.5}, {X: 7, Y: 1.5}, {X: 7, Y: 2}, {X: 7, Y: 2.5}, {X: 2, Y: 2.5}}
	got := ends(figures[0])
	if len(got) != len(want) {
		t.Fatalf("ends %v, want %v", got, want)
	}
	for i := range want {
		if !near(got[i], want[i]) || figures[0].Segments[i].Curve {
			t.Fatalf("segment %d ends at %v (curve %v), want a line to %v", i, got[i], figures[0].Segments[i].Curve, want[i])
		}
	}
}

// A move starts a new figure; after Z, drawing goes on from the closed
// figure's start; a move with nothing drawn after it leaves no figure.
func TestFiguresStartAtMovesAndAfterAClose(t *testing.T) {
	t.Parallel()
	figures := parse(t, "M1 1h2Zh5M10 10l1 1M20 20")
	if len(figures) != 3 {
		t.Fatalf("%d figures, want 3: %+v", len(figures), figures)
	}
	if !figures[0].Closed || figures[1].Start != (vector.Point{X: 1, Y: 1}) || !near(ends(figures[1])[0], vector.Point{X: 6, Y: 1}) {
		t.Fatalf("after Z, the figure %+v, want one from (1, 1) to (6, 1)", figures[1])
	}
	if figures[2].Start != (vector.Point{X: 10, Y: 10}) || figures[2].Closed {
		t.Fatalf("third figure %+v, want an open one from (10, 10)", figures[2])
	}
}

// S mirrors the last curve's second control point; after a line it starts
// at the pen, as SVG says.
func TestASmoothCurveMirrorsTheLastControl(t *testing.T) {
	t.Parallel()
	f := parse(t, "M0 0C0 1 2 1 2 0s4-1 4 0L8 0s1 1 2 0")[0]
	if s := f.Segments[1]; !s.Curve || !near(s.C1, vector.Point{X: 2, Y: -1}) || !near(s.C2, vector.Point{X: 6, Y: -1}) || !near(s.To, vector.Point{X: 6, Y: 0}) {
		t.Fatalf("smooth curve %+v, want controls (2, -1) and (6, -1) to (6, 0)", s)
	}
	if s := f.Segments[3]; !near(s.C1, vector.Point{X: 8, Y: 0}) {
		t.Fatalf("smooth curve after a line starts its pull at %v, want the pen (8, 0)", s.C1)
	}
}

// An arc ends exactly where it is asked to, in curves of at most a quarter
// turn that stay on its circle (a cubic's middle sits on it within a
// thousandth), passing on the side its flags choose.
func TestAnArcFollowsItsCircle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		d            string
		center       vector.Point
		r            float64
		pieces       int
		through, end vector.Point // through: where the first curve ends
	}{
		{"half, clockwise over the top", "M2 12A10 10 0 0 1 22 12", vector.Point{X: 12, Y: 12}, 10, 2, vector.Point{X: 12, Y: 2}, vector.Point{X: 22, Y: 12}},
		{"half, the other way under", "M2 12A10 10 0 0 0 22 12", vector.Point{X: 12, Y: 12}, 10, 2, vector.Point{X: 12, Y: 22}, vector.Point{X: 22, Y: 12}},
		{"three quarters, the long way", "M12 2a10 10 0 1 1-10 10", vector.Point{X: 12, Y: 12}, 10, 3, vector.Point{X: 22, Y: 12}, vector.Point{X: 2, Y: 12}},
		{"radii too small grow to fit", "M0 0a1 1 0 0 1 10 0", vector.Point{X: 5, Y: 0}, 5, 2, vector.Point{X: 5, Y: -5}, vector.Point{X: 10, Y: 0}},
	}
	for _, c := range cases {
		f := parse(t, c.d)[0]
		if len(f.Segments) != c.pieces {
			t.Fatalf("%s: %d curves, want %d", c.name, len(f.Segments), c.pieces)
		}
		if got := f.Segments[0].To; !nearBy(got, c.through, 1e-6) {
			t.Fatalf("%s: the first curve ends at %v, want %v", c.name, got, c.through)
		}
		if got := f.Segments[c.pieces-1].To; got != c.end {
			t.Fatalf("%s: ends at %v, want exactly %v", c.name, got, c.end)
		}
		from := f.Start
		for _, s := range f.Segments {
			mid := vector.Point{X: (from.X + 3*s.C1.X + 3*s.C2.X + s.To.X) / 8, Y: (from.Y + 3*s.C1.Y + 3*s.C2.Y + s.To.Y) / 8}
			if d := math.Hypot(mid.X-c.center.X, mid.Y-c.center.Y); math.Abs(d-c.r) > c.r*1e-3 {
				t.Fatalf("%s: a curve's middle %v is %.4f from the center, want %v", c.name, mid, d, c.r)
			}
			from = s.To
		}
	}
}

// An arc with a zero radius is a straight line; one to where it starts
// draws nothing.
func TestDegenerateArcs(t *testing.T) {
	t.Parallel()
	f := parse(t, "M0 0A0 5 0 0 1 4 0A5 5 0 0 1 4 0")[0]
	if len(f.Segments) != 1 || f.Segments[0].Curve || !near(f.Segments[0].To, vector.Point{X: 4, Y: 0}) {
		t.Fatalf("segments %+v, want one line to (4, 0)", f.Segments)
	}
}

// What is not a path is refused, never guessed: an unknown command, a
// drawing before any move, a broken number, an arc flag that is not 0 or 1,
// and data far longer than an icon.
func TestWhatIsNotAPathIsRefused(t *testing.T) {
	t.Parallel()
	for _, d := range []string{
		"M0 0Q1 1 2 2",
		"L1 1",
		"M0 0L1",
		"M0 0L1 -",
		"M0 0a5 5 0 2 1 4 0",
		"M0 0L1 1e999",
		"M" + strings.Repeat("1 1 ", 2000),
	} {
		if _, err := vector.Parse(d); err == nil {
			t.Errorf("Parse(%.20q...) took it", d)
		}
	}
	if figures, err := vector.Parse(""); err != nil || len(figures) != 0 {
		t.Fatalf("an empty path: %v, %v; want nothing and no error", figures, err)
	}
}
