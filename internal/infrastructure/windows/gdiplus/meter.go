//go:build windows && amd64

package gdiplus

import (
	"math"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

// MeterLook colors a notice's level: Fill up to the mark and Loud past it,
// as the control bar's volume does. The empty part and the mark take the
// notice's text color.
type MeterLook struct {
	Fill, Loud appearance.Color
}

// meterTrackAlpha keeps the empty part of the bar faint against the box.
const meterTrackAlpha = 0x50

// drawMeter paints the level in its bar, on whole pixels so its edges stay
// sharp, and a tick where the usual range ends.
func drawMeter(c *canvas, meter *layout.PlacedMeter, notice appearance.SubtitleStyle, look MeterLook) error {
	// GDI+ puts pixel edges on halves.
	left, right := math.Round(meter.Bar.X)-0.5, math.Round(meter.Bar.Right())-0.5
	top := math.Round(meter.Bar.Y) - 0.5
	bottom := math.Max(math.Round(meter.Bar.Bottom())-0.5, top+1)
	if right-left < 1 {
		return nil
	}
	at := func(share float64) float64 {
		return left + math.Round((right-left)*math.Min(math.Max(share, 0), 1))
	}
	span := func(from, to float64) layout.Rect {
		return layout.Rect{X: from, Y: top, W: to - from, H: bottom - top}
	}
	text := notice.Color
	if err := fillRect(c.graphics, text.WithAlpha(uint8(uint16(text.A)*meterTrackAlpha/0xFF)), span(left, right)); err != nil {
		return err
	}
	level, mark := at(meter.Level), at(meter.Mark)
	if usual := math.Min(level, mark); usual > left {
		if err := fillRect(c.graphics, look.Fill, span(left, usual)); err != nil {
			return err
		}
	}
	if level > mark {
		if err := fillRect(c.graphics, look.Loud, span(mark, level)); err != nil {
			return err
		}
	}
	if mark >= right {
		return nil
	}
	// The tick reaches half the bar's thickness past each side of it.
	thickness := bottom - top
	width := math.Max(1, math.Round(thickness/3))
	reach := math.Round(thickness / 2)
	return fillRect(c.graphics, text, layout.Rect{X: mark - math.Floor(width/2), Y: top - reach, W: width, H: thickness + 2*reach})
}
