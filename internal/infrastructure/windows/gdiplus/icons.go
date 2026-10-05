//go:build windows && amd64

package gdiplus

import (
	"math"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// Icon geometry, as fractions of the icon's side: the stroke of the line
// icons at 100% is 2 pixels on a 16-pixel icon, as on the bar.
const (
	iconStrokeRatio = barStroke / barIcon16
	iconPadRatio    = 0.08
)

// DrawIcon paints one of the context menu's icons over a whole bitmap,
// cleared first, in one color: the system's menu text color, so it shows on
// a light menu and in high contrast alike. An unknown icon draws nothing.
func (r *Renderer) DrawIcon(target *win32.DIB, icon contextmenu.Icon, color appearance.Color) error {
	if target == nil {
		return nil
	}
	target.Clear()
	c, err := r.canvasOn(target)
	if err != nil {
		return err
	}
	defer c.close()
	side := float64(min(target.Width, target.Height))
	pad := math.Round(side * iconPadRatio)
	box := layout.Rect{X: pad, Y: pad, W: side - 2*pad, H: side - 2*pad}
	font := layout.Font{Family: barFont, Size: barFontSize * box.W / barIcon16}
	return r.drawIcon(c.graphics, icon, box, color, box.W*iconStrokeRatio, font)
}

// drawIcon draws an icon in a square box, with the stroke of its lines.
// The bar draws its buttons with it too.
func (r *Renderer) drawIcon(g uintptr, icon contextmenu.Icon, box layout.Rect, color appearance.Color, stroke float64, font layout.Font) error {
	side := box.W
	switch icon {
	case contextmenu.IconPlay:
		return newShape().polygon(pt(box.X+side*0.15, box.Y), pt(box.Right(), box.Y+side/2), pt(box.X+side*0.15, box.Bottom())).fill(g, color)
	case contextmenu.IconPause:
		thick := side * 0.3
		return newShape().
			rounded(layout.Rect{X: box.X + side*0.12, Y: box.Y, W: thick, H: side}, stroke/2).
			rounded(layout.Rect{X: box.Right() - side*0.12 - thick, Y: box.Y, W: thick, H: side}, stroke/2).
			fill(g, color)
	case contextmenu.IconStop:
		return newShape().rounded(layout.Rect{X: box.X + side*0.12, Y: box.Y + side*0.12, W: side * 0.76, H: side * 0.76}, stroke/2).fill(g, color)
	case contextmenu.IconBack, contextmenu.IconForward:
		half := side / 2
		sh := newShape()
		for i := range 2 {
			x := box.X + float64(i)*half
			if icon == contextmenu.IconBack {
				sh.polygon(pt(x+half, box.Y+side*0.15), pt(x, box.Y+side/2), pt(x+half, box.Bottom()-side*0.15))
			} else {
				sh.polygon(pt(x, box.Y+side*0.15), pt(x+half, box.Y+side/2), pt(x, box.Bottom()-side*0.15))
			}
		}
		return sh.fill(g, color)
	case contextmenu.IconFrameBack, contextmenu.IconFrameForward:
		// A bar and a triangle: |◀ and ▶|.
		thick := side * 0.16
		if icon == contextmenu.IconFrameBack {
			return newShape().
				rounded(layout.Rect{X: box.X + side*0.1, Y: box.Y + side*0.12, W: thick, H: side * 0.76}, stroke/4).
				polygon(pt(box.Right()-side*0.1, box.Y+side*0.12), pt(box.X+side*0.32, box.Y+side/2), pt(box.Right()-side*0.1, box.Bottom()-side*0.12)).
				fill(g, color)
		}
		return newShape().
			polygon(pt(box.X+side*0.1, box.Y+side*0.12), pt(box.Right()-side*0.32, box.Y+side/2), pt(box.X+side*0.1, box.Bottom()-side*0.12)).
			rounded(layout.Rect{X: box.Right() - side*0.1 - thick, Y: box.Y + side*0.12, W: thick, H: side * 0.76}, stroke/4).
			fill(g, color)
	case contextmenu.IconSpeaker:
		return drawSpeaker(g, box, color, stroke, false)
	case contextmenu.IconFullscreen, contextmenu.IconExitFullscreen:
		return r.drawCorners(g, box, icon == contextmenu.IconExitFullscreen, color, stroke)
	case contextmenu.IconPassage:
		pole := newShape().line(box.X+side*0.22, box.Y, box.X+side*0.22, box.Bottom())
		if err := pole.stroke(g, color, stroke); err != nil {
			return err
		}
		return newShape().polygon(pt(box.X+side*0.26, box.Y), pt(box.Right(), box.Y+side*0.27), pt(box.X+side*0.26, box.Y+side*0.55)).fill(g, color)
	case contextmenu.IconKaraoke:
		head := layout.Rect{X: box.X + side*0.1, Y: box.Y + side*0.62, W: side * 0.36, H: side * 0.3}
		if err := newShape().ellipse(head).fill(g, color); err != nil {
			return err
		}
		return newShape().
			line(head.Right()-stroke/2, head.Y+head.H/2, head.Right()-stroke/2, box.Y+side*0.05).
			line(head.Right()-stroke/2, box.Y+side*0.05, box.Right()-side*0.08, box.Y+side*0.22).
			stroke(g, color, stroke)
	case contextmenu.IconSubtitles:
		frame := layout.Rect{X: box.X - side*0.1, Y: box.Y + side*0.12, W: side * 1.2, H: side * 0.76}
		if err := newShape().rounded(frame, side*0.12).stroke(g, color, stroke*0.75); err != nil {
			return err
		}
		small := font
		small.Size, small.Bold = font.Size*0.75, true
		return r.writeLine(g, "CC", small, frame, true, color)
	case contextmenu.IconFolder:
		// A tab on the back, the folder's body below it.
		return newShape().polygon(
			pt(box.X, box.Y+side*0.15), pt(box.X+side*0.38, box.Y+side*0.15), pt(box.X+side*0.5, box.Y+side*0.28),
			pt(box.Right(), box.Y+side*0.28), pt(box.Right(), box.Bottom()-side*0.1), pt(box.X, box.Bottom()-side*0.1),
		).fill(g, color)
	case contextmenu.IconSettings:
		// A ring with eight teeth around it.
		center := pt(box.X+side/2, box.Y+side/2)
		ring := side * 0.24
		teeth := newShape()
		for i := range 8 {
			angle := float64(i) * math.Pi / 4
			cos, sin := math.Cos(angle), math.Sin(angle)
			teeth.line(float64(center.X)+cos*side*0.3, float64(center.Y)+sin*side*0.3, float64(center.X)+cos*side*0.48, float64(center.Y)+sin*side*0.48)
		}
		if err := teeth.stroke(g, color, stroke*1.3); err != nil {
			return err
		}
		return newShape().ellipse(layout.Rect{X: float64(center.X) - ring, Y: float64(center.Y) - ring, W: 2 * ring, H: 2 * ring}).stroke(g, color, stroke*1.1)
	case contextmenu.IconClose:
		inset := side * 0.15
		return newShape().
			line(box.X+inset, box.Y+inset, box.Right()-inset, box.Bottom()-inset).
			line(box.X+inset, box.Bottom()-inset, box.Right()-inset, box.Y+inset).
			stroke(g, color, stroke)
	}
	return nil
}

// drawSpeaker draws the loudspeaker: with its waves, or with a cross when
// muted.
func drawSpeaker(g uintptr, box layout.Rect, color appearance.Color, stroke float64, muted bool) error {
	side := box.W
	body := newShape().polygon(
		pt(box.X, box.Y+side*0.35), pt(box.X+side*0.25, box.Y+side*0.35), pt(box.X+side*0.55, box.Y+side*0.1),
		pt(box.X+side*0.55, box.Bottom()-side*0.1), pt(box.X+side*0.25, box.Bottom()-side*0.35), pt(box.X, box.Bottom()-side*0.35))
	if err := body.fill(g, color); err != nil {
		return err
	}
	if muted {
		x := box.X + side*0.7
		return newShape().line(x, box.Y+side*0.35, x+side*0.3, box.Bottom()-side*0.35).
			line(x, box.Bottom()-side*0.35, x+side*0.3, box.Y+side*0.35).stroke(g, color, stroke*0.8)
	}
	wave := layout.Rect{X: box.X + side*0.25, Y: box.Y + side*0.2, W: side * 0.6, H: side * 0.6}
	return newShape().arc(wave, -45, 90).stroke(g, color, stroke*0.8)
}
