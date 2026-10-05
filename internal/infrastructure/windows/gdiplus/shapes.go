//go:build windows && amd64

package gdiplus

import (
	"errors"
	"unsafe"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

// shape collects figures into one GDI+ path; the first error sticks and
// every later call does nothing, so a drawing reads as a list of figures.
type shape struct {
	path uintptr
	err  error
}

func newShape() *shape {
	s := &shape{}
	s.err = call(procCreatePath, fillModeWinding, uintptr(unsafe.Pointer(&s.path)))
	return s
}

func (s *shape) close() {
	if s.path != 0 {
		procDeletePath.Call(s.path)
		s.path = 0
	}
}

func (s *shape) do(proc func() error) *shape {
	if s.err == nil {
		s.err = proc()
	}
	return s
}

// ellipse adds the ellipse inside a rectangle.
func (s *shape) ellipse(r layout.Rect) *shape {
	return s.do(func() error {
		return call(procAddPathEllipse, s.path, f32(r.X), f32(r.Y), f32(r.W), f32(r.H))
	})
}

// polygon adds a closed polygon.
func (s *shape) polygon(points ...pointF) *shape {
	return s.do(func() error {
		if len(points) < 3 {
			return errors.New("a polygon needs three points")
		}
		return call(procAddPathPolygon, s.path, uintptr(unsafe.Pointer(&points[0])), uintptr(len(points)))
	})
}

// line adds an open segment as its own figure.
func (s *shape) line(x1, y1, x2, y2 float64) *shape {
	s.do(func() error { return call(procStartPathFigure, s.path) })
	return s.do(func() error { return call(procAddPathLine, s.path, f32(x1), f32(y1), f32(x2), f32(y2)) })
}

// arc adds an open arc of the ellipse inside a rectangle, angles in degrees
// clockwise from the x axis.
func (s *shape) arc(r layout.Rect, start, sweep float64) *shape {
	s.do(func() error { return call(procStartPathFigure, s.path) })
	return s.do(func() error {
		return call(procAddPathArc, s.path, f32(r.X), f32(r.Y), f32(r.W), f32(r.H), f32(start), f32(sweep))
	})
}

// rounded adds a rectangle with round corners of the given radius.
func (s *shape) rounded(r layout.Rect, radius float64) *shape {
	radius = min(radius, r.W/2, r.H/2)
	if radius <= 0 {
		return s.polygon(pointF{float32(r.X), float32(r.Y)}, pointF{float32(r.Right()), float32(r.Y)},
			pointF{float32(r.Right()), float32(r.Bottom())}, pointF{float32(r.X), float32(r.Bottom())})
	}
	d := 2 * radius
	s.do(func() error { return call(procStartPathFigure, s.path) })
	for _, corner := range []struct{ x, y, start float64 }{
		{r.X, r.Y, 180}, {r.Right() - d, r.Y, 270}, {r.Right() - d, r.Bottom() - d, 0}, {r.X, r.Bottom() - d, 90},
	} {
		s.do(func() error {
			return call(procAddPathArc, s.path, f32(corner.x), f32(corner.y), f32(d), f32(d), f32(corner.start), f32(90))
		})
	}
	return s.do(func() error { return call(procClosePathFigure, s.path) })
}

// fill paints the shape.
func (s *shape) fill(graphics uintptr, color appearance.Color) error {
	defer s.close()
	if s.err != nil {
		return s.err
	}
	return fillPath(graphics, s.path, color)
}

// stroke draws the shape's lines with round ends.
func (s *shape) stroke(graphics uintptr, color appearance.Color, width float64) error {
	defer s.close()
	if s.err != nil {
		return s.err
	}
	var pen uintptr
	if err := call(procCreatePen1, colorOf(color), f32(width), unitPixel, uintptr(unsafe.Pointer(&pen))); err != nil {
		return err
	}
	defer procDeletePen.Call(pen)
	if err := call(procSetPenLineCap, pen, lineCapRound, lineCapRound, dashCapFlat); err != nil {
		return err
	}
	if err := call(procSetPenLineJoin, pen, lineJoinRound); err != nil {
		return err
	}
	return call(procDrawPath, graphics, pen, s.path)
}
