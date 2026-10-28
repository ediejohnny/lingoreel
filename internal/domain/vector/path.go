// Package vector reads the drawings of the app's icons: SVG path data on a
// grid, turned into figures of straight lines and cubic curves that any
// graphics library can draw.
package vector

import (
	"fmt"
	"math"
	"strconv"
)

// Point is a place on the drawing's grid.
type Point struct{ X, Y float64 }

// Segment runs from where the one before it ends (or from its figure's
// start) to To: straight, or a cubic curve pulled by C1 and C2.
type Segment struct {
	Curve  bool
	C1, C2 Point
	To     Point
}

// Figure is one connected stroke; a closed one returns to its start.
type Figure struct {
	Start    Point
	Segments []Segment
	Closed   bool
}

// maxPathLength bounds what Parse reads: an icon's path is a few hundred
// characters.
const maxPathLength = 4096

// Parse reads SVG path data with the commands M, L, H, V, C, S, A and Z, in
// absolute (upper case) and relative (lower case) form. Coordinates after a
// move continue as lines, as SVG says.
func Parse(d string) ([]Figure, error) {
	if len(d) > maxPathLength {
		return nil, fmt.Errorf("path data of %d characters; at most %d", len(d), maxPathLength)
	}
	p := parser{text: d}
	for {
		p.skipSpace()
		if p.done() {
			break
		}
		if err := p.command(); err != nil {
			return nil, err
		}
	}
	p.end()
	return p.figures, nil
}

type parser struct {
	text    string
	at      int
	figures []Figure
	figure  *Figure
	pen     Point
	// control is the last cubic's second control point, which S mirrors.
	control Point
	smooth  bool
}

func (p *parser) done() bool { return p.at >= len(p.text) }

func (p *parser) skipSpace() {
	for !p.done() && (p.text[p.at] == ' ' || p.text[p.at] == ',' || p.text[p.at] == '\t' || p.text[p.at] == '\n' || p.text[p.at] == '\r') {
		p.at++
	}
}

// startsNumber says whether the next thing to read is a number.
func (p *parser) startsNumber() bool {
	p.skipSpace()
	if p.done() {
		return false
	}
	c := p.text[p.at]
	return c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9')
}

// number reads one number: a sign, digits with at most one point, and an
// exponent; "-2.3-5.8" and ".5.5" are two numbers each.
func (p *parser) number() (float64, error) {
	p.skipSpace()
	start := p.at
	if !p.done() && (p.text[p.at] == '-' || p.text[p.at] == '+') {
		p.at++
	}
	digits, point := 0, false
	for !p.done() {
		c := p.text[p.at]
		if c >= '0' && c <= '9' {
			digits++
		} else if c == '.' && !point {
			point = true
		} else {
			break
		}
		p.at++
	}
	if digits > 0 && !p.done() && (p.text[p.at] == 'e' || p.text[p.at] == 'E') {
		p.at++
		if !p.done() && (p.text[p.at] == '-' || p.text[p.at] == '+') {
			p.at++
		}
		for !p.done() && p.text[p.at] >= '0' && p.text[p.at] <= '9' {
			p.at++
		}
	}
	if digits == 0 {
		return 0, fmt.Errorf("a number expected at %d", start)
	}
	v, err := strconv.ParseFloat(p.text[start:p.at], 64)
	if err != nil || math.IsInf(v, 0) {
		return 0, fmt.Errorf("a number expected at %d", start)
	}
	return v, nil
}

// flag reads an arc's flag, a single 0 or 1 that needs no space after it.
func (p *parser) flag() (bool, error) {
	p.skipSpace()
	if p.done() || (p.text[p.at] != '0' && p.text[p.at] != '1') {
		return false, fmt.Errorf("an arc flag expected at %d", p.at)
	}
	p.at++
	return p.text[p.at-1] == '1', nil
}

func (p *parser) numbers(n int) ([]float64, error) {
	values := make([]float64, n)
	for i := range values {
		v, err := p.number()
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
}

// point reads a pair of coordinates, relative to the pen when asked.
func (p *parser) point(relative bool) (Point, error) {
	v, err := p.numbers(2)
	if err != nil {
		return Point{}, err
	}
	if relative {
		return Point{p.pen.X + v[0], p.pen.Y + v[1]}, nil
	}
	return Point{v[0], v[1]}, nil
}

func (p *parser) command() error {
	letter := p.text[p.at]
	p.at++
	relative := letter >= 'a' && letter <= 'z'
	switch letter | 0x20 {
	case 'm':
		to, err := p.point(relative)
		if err != nil {
			return err
		}
		p.move(to)
		return p.repeat(func() error { return p.lineTo(relative) })
	case 'l':
		return p.atLeastOnce(func() error { return p.lineTo(relative) })
	case 'h', 'v':
		return p.atLeastOnce(func() error {
			v, err := p.number()
			if err != nil {
				return err
			}
			to := p.pen
			switch letter {
			case 'h':
				to.X += v
			case 'H':
				to.X = v
			case 'v':
				to.Y += v
			default:
				to.Y = v
			}
			p.add(Segment{To: to})
			return nil
		})
	case 'c':
		return p.atLeastOnce(func() error {
			c1, err := p.point(relative)
			if err != nil {
				return err
			}
			c2, err := p.point(relative)
			if err != nil {
				return err
			}
			to, err := p.point(relative)
			if err != nil {
				return err
			}
			p.curve(c1, c2, to)
			return nil
		})
	case 's':
		return p.atLeastOnce(func() error {
			c1 := p.pen
			if p.smooth {
				c1 = Point{2*p.pen.X - p.control.X, 2*p.pen.Y - p.control.Y}
			}
			c2, err := p.point(relative)
			if err != nil {
				return err
			}
			to, err := p.point(relative)
			if err != nil {
				return err
			}
			p.curve(c1, c2, to)
			return nil
		})
	case 'a':
		return p.atLeastOnce(func() error { return p.arc(relative) })
	case 'z':
		p.close()
		return nil
	}
	return fmt.Errorf("unknown path command %q at %d", letter, p.at-1)
}

// atLeastOnce runs a command's reading once, then again while numbers follow.
func (p *parser) atLeastOnce(read func() error) error {
	if err := p.needFigure(); err != nil {
		return err
	}
	if err := read(); err != nil {
		return err
	}
	return p.repeat(read)
}

func (p *parser) repeat(read func() error) error {
	for p.startsNumber() {
		if err := read(); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) needFigure() error {
	if p.figure == nil {
		return fmt.Errorf("a path must start with a move, at %d", p.at-1)
	}
	return nil
}

func (p *parser) lineTo(relative bool) error {
	to, err := p.point(relative)
	if err != nil {
		return err
	}
	p.add(Segment{To: to})
	return nil
}

func (p *parser) arc(relative bool) error {
	radii, err := p.numbers(3)
	if err != nil {
		return err
	}
	large, err := p.flag()
	if err != nil {
		return err
	}
	sweep, err := p.flag()
	if err != nil {
		return err
	}
	to, err := p.point(relative)
	if err != nil {
		return err
	}
	for _, s := range arcCurves(p.pen, radii[0], radii[1], radii[2], large, sweep, to) {
		p.add(s)
	}
	return nil
}

func (p *parser) move(to Point) {
	p.end()
	p.figure = &Figure{Start: to}
	p.pen, p.smooth = to, false
}

func (p *parser) add(s Segment) {
	p.figure.Segments = append(p.figure.Segments, s)
	p.pen, p.smooth = s.To, false
}

func (p *parser) curve(c1, c2, to Point) {
	p.add(Segment{Curve: true, C1: c1, C2: c2, To: to})
	p.control, p.smooth = c2, true
}

// close ends the figure at its start; what follows without a move starts
// there again, as SVG says.
func (p *parser) close() {
	if p.figure == nil {
		return
	}
	start := p.figure.Start
	p.figure.Closed = true
	p.end()
	p.figure = &Figure{Start: start}
	p.pen, p.smooth = start, false
}

// end keeps the figure being drawn when it has anything in it.
func (p *parser) end() {
	if p.figure != nil && (len(p.figure.Segments) > 0 || p.figure.Closed) {
		p.figures = append(p.figures, *p.figure)
	}
	p.figure = nil
}

// arcCurves turns an SVG elliptical arc into cubic curves of at most a
// quarter turn each (SVG 1.1, appendix F.6.5 and F.6.6): radii too small
// for the two ends grow until they fit; zero radii make a straight line.
func arcCurves(from Point, rx, ry, rotation float64, large, sweep bool, to Point) []Segment {
	if from == to {
		return nil
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		return []Segment{{To: to}}
	}
	phi := rotation * math.Pi / 180
	cos, sin := math.Cos(phi), math.Sin(phi)
	dx, dy := (from.X-to.X)/2, (from.Y-to.Y)/2
	x1 := cos*dx + sin*dy
	y1 := -sin*dx + cos*dy
	if grow := x1*x1/(rx*rx) + y1*y1/(ry*ry); grow > 1 {
		rx, ry = rx*math.Sqrt(grow), ry*math.Sqrt(grow)
	}
	num := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	den := rx*rx*y1*y1 + ry*ry*x1*x1
	k := math.Sqrt(math.Max(0, num/den))
	if large == sweep {
		k = -k
	}
	cx1, cy1 := k*rx*y1/ry, -k*ry*x1/rx
	cx := cos*cx1 - sin*cy1 + (from.X+to.X)/2
	cy := sin*cx1 + cos*cy1 + (from.Y+to.Y)/2
	angle := func(ux, uy, vx, vy float64) float64 { return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy) }
	start := angle(1, 0, (x1-cx1)/rx, (y1-cy1)/ry)
	delta := angle((x1-cx1)/rx, (y1-cy1)/ry, (-x1-cx1)/rx, (-y1-cy1)/ry)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	} else if sweep && delta < 0 {
		delta += 2 * math.Pi
	}
	pieces := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	step := delta / float64(pieces)
	t := 4.0 / 3 * math.Tan(step/4)
	on := func(a float64) (Point, Point) {
		ex, ey := rx*math.Cos(a), ry*math.Sin(a)
		tx, ty := -rx*math.Sin(a), ry*math.Cos(a)
		place := func(x, y float64) Point { return Point{cos*x - sin*y + cx, sin*x + cos*y + cy} }
		return place(ex, ey), Point{cos*tx - sin*ty, sin*tx + cos*ty}
	}
	segments := make([]Segment, 0, pieces)
	a := start
	p0, d0 := on(a)
	for i := range pieces {
		b := a + step
		p1, d1 := on(b)
		if i == pieces-1 {
			p1 = to
		}
		segments = append(segments, Segment{
			Curve: true,
			C1:    Point{p0.X + t*d0.X, p0.Y + t*d0.Y},
			C2:    Point{p1.X - t*d1.X, p1.Y - t*d1.Y},
			To:    p1,
		})
		a, p0, d0 = b, p1, d1
	}
	return segments
}
