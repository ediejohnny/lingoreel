//go:build windows && amd64

package gdiplus

import (
	"math"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

// The share of the stroke that frames and rings take (thinStroke), and the
// speaker's waves and cross (waveStroke).
const (
	thinStroke = 0.75
	waveStroke = 0.8
)

// Each icon's drawing, in fractions of the icon's side measured from the
// edge the comment names.
const (
	// The triangle of play starts this far in from the left.
	playLeft = 0.15
	// The two bars of pause: their width, in from each side.
	pauseBar   = 0.3
	pauseInset = 0.12
	// The square of stop: in from the top left, and its side.
	stopInset = 0.12
	stopSide  = 0.76
	// The two triangles of back and forward, in from the top and the bottom.
	seekInset = 0.15
	// A frame step, |◀ or ▶|: the bar's width, in from the sides; the top and
	// the height of both; how far the triangle's tip reaches from the bar's
	// side.
	frameBar    = 0.16
	frameInset  = 0.1
	frameTop    = 0.12
	frameHeight = 0.76
	frameTip    = 0.32
	// The flag of a passage: its pole from the left; the cloth from the
	// left, its tip from the top and its foot from the top.
	flagPole   = 0.22
	flagLeft   = 0.26
	flagTip    = 0.27
	flagBottom = 0.55
	// The note of the karaoke mode: its head from the top left and its size,
	// the stem's top, and the flag's end in from the right and down from the
	// top.
	noteHeadX   = 0.1
	noteHeadY   = 0.62
	noteHeadW   = 0.36
	noteHeadH   = 0.3
	noteStemTop = 0.05
	noteFlagEnd = 0.08
	noteFlagY   = 0.22
	// The frame of "CC": out past each side (into the bar button's room), its
	// width, its top and height, its corners; the letters' size, of the
	// bar's text.
	ccOverhang = 0.1
	ccWidth    = 1.2
	ccTop      = 0.12
	ccHeight   = 0.76
	ccRadius   = 0.12
	ccFont     = 0.75
	// The folder: its tab from the top, where the tab ends and its slope
	// ends from the left, the body's top, and its foot up from the bottom.
	folderTabTop   = 0.15
	folderTabEnd   = 0.38
	folderSlopeEnd = 0.5
	folderBodyTop  = 0.28
	folderFoot     = 0.1
	// The gear: its ring's radius, its teeth from the center (inner and
	// outer ends) and their number, and their strokes.
	gearRing        = 0.24
	gearToothIn     = 0.3
	gearToothOut    = 0.48
	gearTeeth       = 8
	gearToothStroke = 1.3
	gearRingStroke  = 1.1
	// The cross of close, in from each corner.
	closeInset = 0.15
	// The speaker: its mouth's top, its neck and its back from the left, the
	// cone's top; the mute cross from the left and its width; the waves'
	// box and its arc, in degrees.
	speakerMouth = 0.35
	speakerNeck  = 0.25
	speakerBack  = 0.55
	speakerCone  = 0.1
	muteX        = 0.7
	muteW        = 0.3
	waveX        = 0.25
	waveY        = 0.2
	waveSize     = 0.6
	waveStart    = -45
	waveSweep    = 90
	// The arms of the fullscreen corners.
	cornerArm = 0.35
)

// drawIcon draws one of the bar's icons in a square box, with the stroke of
// its lines.
func (r *Renderer) drawIcon(g uintptr, icon contextmenu.Icon, box layout.Rect, color appearance.Color, stroke float64, font layout.Font) error {
	side := box.W
	switch icon {
	case contextmenu.IconPlay:
		return newShape().polygon(pt(box.X+side*playLeft, box.Y), pt(box.Right(), box.Y+side/2), pt(box.X+side*playLeft, box.Bottom())).fill(g, color)
	case contextmenu.IconPause:
		thick := side * pauseBar
		return newShape().
			rounded(layout.Rect{X: box.X + side*pauseInset, Y: box.Y, W: thick, H: side}, stroke/2).
			rounded(layout.Rect{X: box.Right() - side*pauseInset - thick, Y: box.Y, W: thick, H: side}, stroke/2).
			fill(g, color)
	case contextmenu.IconStop:
		return newShape().rounded(layout.Rect{X: box.X + side*stopInset, Y: box.Y + side*stopInset, W: side * stopSide, H: side * stopSide}, stroke/2).fill(g, color)
	case contextmenu.IconBack, contextmenu.IconForward:
		half := side / 2
		sh := newShape()
		for i := range 2 {
			x := box.X + float64(i)*half
			if icon == contextmenu.IconBack {
				sh.polygon(pt(x+half, box.Y+side*seekInset), pt(x, box.Y+side/2), pt(x+half, box.Bottom()-side*seekInset))
			} else {
				sh.polygon(pt(x, box.Y+side*seekInset), pt(x+half, box.Y+side/2), pt(x, box.Bottom()-side*seekInset))
			}
		}
		return sh.fill(g, color)
	case contextmenu.IconFrameBack, contextmenu.IconFrameForward:
		// A bar and a triangle: |◀ and ▶|.
		thick := side * frameBar
		if icon == contextmenu.IconFrameBack {
			return newShape().
				rounded(layout.Rect{X: box.X + side*frameInset, Y: box.Y + side*frameTop, W: thick, H: side * frameHeight}, stroke/4).
				polygon(pt(box.Right()-side*frameInset, box.Y+side*frameTop), pt(box.X+side*frameTip, box.Y+side/2), pt(box.Right()-side*frameInset, box.Bottom()-side*frameTop)).
				fill(g, color)
		}
		return newShape().
			polygon(pt(box.X+side*frameInset, box.Y+side*frameTop), pt(box.Right()-side*frameTip, box.Y+side/2), pt(box.X+side*frameInset, box.Bottom()-side*frameTop)).
			rounded(layout.Rect{X: box.Right() - side*frameInset - thick, Y: box.Y + side*frameTop, W: thick, H: side * frameHeight}, stroke/4).
			fill(g, color)
	case contextmenu.IconSpeaker:
		return drawSpeaker(g, box, color, stroke, false)
	case contextmenu.IconFullscreen, contextmenu.IconExitFullscreen:
		return r.drawCorners(g, box, icon == contextmenu.IconExitFullscreen, color, stroke)
	case contextmenu.IconPassage:
		pole := newShape().line(box.X+side*flagPole, box.Y, box.X+side*flagPole, box.Bottom())
		if err := pole.stroke(g, color, stroke); err != nil {
			return err
		}
		return newShape().polygon(pt(box.X+side*flagLeft, box.Y), pt(box.Right(), box.Y+side*flagTip), pt(box.X+side*flagLeft, box.Y+side*flagBottom)).fill(g, color)
	case contextmenu.IconKaraoke:
		head := layout.Rect{X: box.X + side*noteHeadX, Y: box.Y + side*noteHeadY, W: side * noteHeadW, H: side * noteHeadH}
		if err := newShape().ellipse(head).fill(g, color); err != nil {
			return err
		}
		return newShape().
			line(head.Right()-stroke/2, head.Y+head.H/2, head.Right()-stroke/2, box.Y+side*noteStemTop).
			line(head.Right()-stroke/2, box.Y+side*noteStemTop, box.Right()-side*noteFlagEnd, box.Y+side*noteFlagY).
			stroke(g, color, stroke)
	case contextmenu.IconSubtitles:
		frame := layout.Rect{X: box.X - side*ccOverhang, Y: box.Y + side*ccTop, W: side * ccWidth, H: side * ccHeight}
		if err := newShape().rounded(frame, side*ccRadius).stroke(g, color, stroke*thinStroke); err != nil {
			return err
		}
		small := font
		small.Size, small.Bold = font.Size*ccFont, true
		return r.writeLine(g, "CC", small, frame, true, color)
	case contextmenu.IconFolder:
		// A tab on the back, the folder's body below it.
		return newShape().polygon(
			pt(box.X, box.Y+side*folderTabTop), pt(box.X+side*folderTabEnd, box.Y+side*folderTabTop), pt(box.X+side*folderSlopeEnd, box.Y+side*folderBodyTop),
			pt(box.Right(), box.Y+side*folderBodyTop), pt(box.Right(), box.Bottom()-side*folderFoot), pt(box.X, box.Bottom()-side*folderFoot),
		).fill(g, color)
	case contextmenu.IconSettings:
		// A ring with teeth around it.
		center := pt(box.X+side/2, box.Y+side/2)
		ring := side * gearRing
		teeth := newShape()
		for i := range gearTeeth {
			angle := float64(i) * 2 * math.Pi / gearTeeth
			cos, sin := math.Cos(angle), math.Sin(angle)
			teeth.line(float64(center.X)+cos*side*gearToothIn, float64(center.Y)+sin*side*gearToothIn, float64(center.X)+cos*side*gearToothOut, float64(center.Y)+sin*side*gearToothOut)
		}
		if err := teeth.stroke(g, color, stroke*gearToothStroke); err != nil {
			return err
		}
		return newShape().ellipse(layout.Rect{X: float64(center.X) - ring, Y: float64(center.Y) - ring, W: 2 * ring, H: 2 * ring}).stroke(g, color, stroke*gearRingStroke)
	case contextmenu.IconClose:
		inset := side * closeInset
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
		pt(box.X, box.Y+side*speakerMouth), pt(box.X+side*speakerNeck, box.Y+side*speakerMouth), pt(box.X+side*speakerBack, box.Y+side*speakerCone),
		pt(box.X+side*speakerBack, box.Bottom()-side*speakerCone), pt(box.X+side*speakerNeck, box.Bottom()-side*speakerMouth), pt(box.X, box.Bottom()-side*speakerMouth))
	if err := body.fill(g, color); err != nil {
		return err
	}
	if muted {
		x := box.X + side*muteX
		return newShape().line(x, box.Y+side*speakerMouth, x+side*muteW, box.Bottom()-side*speakerMouth).
			line(x, box.Bottom()-side*speakerMouth, x+side*muteW, box.Y+side*speakerMouth).stroke(g, color, stroke*waveStroke)
	}
	wave := layout.Rect{X: box.X + side*waveX, Y: box.Y + side*waveY, W: side * waveSize, H: side * waveSize}
	return newShape().arc(wave, waveStart, waveSweep).stroke(g, color, stroke*waveStroke)
}
