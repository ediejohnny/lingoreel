//go:build windows && amd64

package gdiplus

import (
	"math"

	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/layout"
	"lingoreel/internal/infrastructure/windows/win32"
)

// Colors of the control bar: the dark theme's tokens (DESIGN), since the bar
// always sits over a video.
var (
	barPanel    = appearance.Color{R: 0x0e, G: 0x12, B: 0x17, A: 0xb8}
	barMenu     = appearance.Color{R: 0x1b, G: 0x22, B: 0x2b, A: 0xf0}
	barTip      = appearance.Color{R: 0x1b, G: 0x22, B: 0x2b, A: 0xf0}
	barHover    = appearance.Color{R: 0xff, G: 0xff, B: 0xff, A: 0x2e}
	barRail     = appearance.Color{R: 0xff, G: 0xff, B: 0xff, A: 0x55}
	barAccent   = appearance.Color{R: 0x5b, G: 0x9b, B: 0xff, A: 0xff}
	barIcon     = appearance.Color{R: 0xe7, G: 0xea, B: 0xef, A: 0xff}
	barDisabled = appearance.Color{R: 0xe7, G: 0xea, B: 0xef, A: 0x5c}
)

// Bar geometry at 100% scaling, in pixels.
const (
	barFontSize    = 13.0
	barIcon16      = 16.0
	barRadius      = 6.0
	barThumbRadius = 6.0
	barStroke      = 2.0
	barTipPadding  = 6.0
	barTipGap      = 6.0
	barFont        = "Segoe UI"
)

// BarLook is what the bar shows besides its layout: texts already worded
// for the user and the item under the mouse.
type BarLook struct {
	// Time is the "current / total" text.
	Time string
	// Label words an item: the speed on its button and in the list.
	Label func(controlbar.Item) string
	// Tip is the hovered item's tip, with its key; empty shows none.
	Tip string
	// Hover is the item under the mouse; ActionNone for none.
	Hover controlbar.Item
	// Thumb shows the round handle on the track (mouse over it or dragging).
	Thumb bool
}

// DrawBar paints the bar onto a cleared bitmap that covers bar.Band.
func (r *Renderer) DrawBar(target *win32.DIB, bar controlbar.Bar, look BarLook) error {
	c, err := r.canvasOn(target)
	if err != nil {
		return err
	}
	defer c.close()
	if err := call(procTranslateWorldTransform, c.graphics, f32(-bar.Band.X), f32(-bar.Band.Y), matrixOrderAppend); err != nil {
		return err
	}
	g, s := c.graphics, bar.Scale
	font := layout.Font{Family: barFont, Size: barFontSize * s}

	if err := fillRect(g, barPanel, bar.Panel); err != nil {
		return err
	}
	if err := r.drawTrack(g, bar, look); err != nil {
		return err
	}
	for _, item := range bar.Items {
		if err := r.drawItem(g, bar, item, look, font); err != nil {
			return err
		}
	}
	if bar.Time.Valid() && look.Time != "" {
		if err := r.writeLine(g, look.Time, font, bar.Time, false, barIcon); err != nil {
			return err
		}
	}
	if bar.Menu.Valid() {
		if err := newShape().rounded(bar.Menu, barRadius*s).fill(g, barMenu); err != nil {
			return err
		}
		for _, item := range bar.Items {
			if item.Action != controlbar.ActionSpeedChoice {
				continue
			}
			if err := r.drawChoice(g, bar, item, look, font); err != nil {
				return err
			}
		}
	}
	if look.Tip != "" && look.Hover.Action != controlbar.ActionNone && look.Hover.Action != controlbar.ActionSpeedChoice {
		return r.drawTip(g, bar, look, font)
	}
	return nil
}

func (r *Renderer) drawTrack(g uintptr, bar controlbar.Bar, look BarLook) error {
	track := bar.Track
	if !track.Valid() {
		return nil
	}
	if err := fillRect(g, barRail, track); err != nil {
		return err
	}
	played := track
	played.W = track.W * bar.Played
	if played.W > 0 {
		if err := fillRect(g, barAccent, played); err != nil {
			return err
		}
	}
	if !look.Thumb {
		return nil
	}
	radius := barThumbRadius * bar.Scale
	center := layout.Rect{X: played.Right() - radius, Y: track.Y + track.H/2 - radius, W: 2 * radius, H: 2 * radius}
	return newShape().ellipse(center).fill(g, barAccent)
}

func (r *Renderer) drawItem(g uintptr, bar controlbar.Bar, item controlbar.Item, look BarLook, font layout.Font) error {
	s := bar.Scale
	b := item.Bounds
	color := barIcon
	if !item.Enabled {
		color = barDisabled
	}
	hovered := item.Enabled && look.Hover.Action == item.Action && look.Hover.Action != controlbar.ActionSpeedChoice
	open := item.Action == controlbar.ActionSpeed && item.Active // the speed list is open
	if (hovered || open) && item.Action != controlbar.ActionVolume && item.Action != controlbar.ActionTrack {
		if err := newShape().rounded(b, barRadius*s).fill(g, barHover); err != nil {
			return err
		}
	}
	// The icon sits in a 16-pixel square at the center of the button.
	icon := barIcon16 * s
	box := layout.Rect{X: b.X + (b.W-icon)/2, Y: b.Y + (b.H-icon)/2, W: icon, H: icon}
	stroke := barStroke * s
	switch item.Action {
	case controlbar.ActionPlayPause:
		if item.Active { // playing: show pause
			thick := icon * 0.3
			return newShape().
				rounded(layout.Rect{X: box.X + icon*0.12, Y: box.Y, W: thick, H: icon}, s).
				rounded(layout.Rect{X: box.Right() - icon*0.12 - thick, Y: box.Y, W: thick, H: icon}, s).
				fill(g, color)
		}
		return newShape().polygon(pt(box.X+icon*0.15, box.Y), pt(box.Right(), box.Y+icon/2), pt(box.X+icon*0.15, box.Bottom())).fill(g, color)
	case controlbar.ActionBack, controlbar.ActionForward:
		half := icon / 2
		sh := newShape()
		for i := range 2 {
			x := box.X + float64(i)*half
			if item.Action == controlbar.ActionBack {
				sh.polygon(pt(x+half, box.Y+icon*0.15), pt(x, box.Y+icon/2), pt(x+half, box.Bottom()-icon*0.15))
			} else {
				sh.polygon(pt(x, box.Y+icon*0.15), pt(x+half, box.Y+icon/2), pt(x, box.Bottom()-icon*0.15))
			}
		}
		return sh.fill(g, color)
	case controlbar.ActionMute:
		body := newShape().polygon(
			pt(box.X, box.Y+icon*0.35), pt(box.X+icon*0.25, box.Y+icon*0.35), pt(box.X+icon*0.55, box.Y+icon*0.1),
			pt(box.X+icon*0.55, box.Bottom()-icon*0.1), pt(box.X+icon*0.25, box.Bottom()-icon*0.35), pt(box.X, box.Bottom()-icon*0.35))
		if err := body.fill(g, color); err != nil {
			return err
		}
		if item.Active { // muted: a cross instead of the waves
			x := box.X + icon*0.7
			return newShape().line(x, box.Y+icon*0.35, x+icon*0.3, box.Bottom()-icon*0.35).
				line(x, box.Bottom()-icon*0.35, x+icon*0.3, box.Y+icon*0.35).stroke(g, color, stroke*0.8)
		}
		wave := layout.Rect{X: box.X + icon*0.25, Y: box.Y + icon*0.2, W: icon * 0.6, H: icon * 0.6}
		return newShape().arc(wave, -45, 90).stroke(g, color, stroke*0.8)
	case controlbar.ActionVolume:
		return r.drawVolume(g, bar, item, hovered)
	case controlbar.ActionTranslation:
		frame := layout.Rect{X: b.X + 6*s, Y: b.Y + 10*s, W: b.W - 12*s, H: b.H - 20*s}
		if item.Active {
			if err := newShape().rounded(frame, 3*s).fill(g, barAccent); err != nil {
				return err
			}
		} else if err := newShape().rounded(frame, 3*s).stroke(g, color, stroke*0.75); err != nil {
			return err
		}
		small := font
		small.Size, small.Bold = font.Size*0.85, true
		return r.writeLine(g, "PT", small, frame, true, color)
	case controlbar.ActionSpeed:
		return r.writeLine(g, look.Label(item), font, b, true, color)
	case controlbar.ActionFullscreen:
		return r.drawCorners(g, box, item.Active, color, stroke)
	}
	return nil
}

// drawCorners draws the four corners of the fullscreen icon, pointing out
// to enter fullscreen and in to leave it.
func (r *Renderer) drawCorners(g uintptr, box layout.Rect, inward bool, color appearance.Color, stroke float64) error {
	arm := box.W * 0.35
	sh := newShape()
	for _, corner := range []struct{ x, y, dx, dy float64 }{
		{box.X, box.Y, 1, 1}, {box.Right(), box.Y, -1, 1}, {box.X, box.Bottom(), 1, -1}, {box.Right(), box.Bottom(), -1, -1},
	} {
		x, y := corner.x, corner.y
		if inward {
			x, y = x+corner.dx*arm, y+corner.dy*arm
			sh.line(x, y, x-corner.dx*arm, y).line(x, y, x, y-corner.dy*arm)
			continue
		}
		sh.line(x, y, x+corner.dx*arm, y).line(x, y, x, y+corner.dy*arm)
	}
	return sh.stroke(g, color, stroke)
}

func (r *Renderer) drawVolume(g uintptr, bar controlbar.Bar, item controlbar.Item, hovered bool) error {
	line := bar.VolumeTrack
	if !line.Valid() {
		return nil
	}
	rail, fillColor := barRail, barIcon
	if !item.Enabled {
		rail, fillColor = barDisabled, barDisabled
	}
	if err := fillRect(g, rail, line); err != nil {
		return err
	}
	level := line
	level.W = line.W * math.Min(math.Max(bar.VolumeLevel, 0), 1)
	if level.W > 0 {
		if err := fillRect(g, fillColor, level); err != nil {
			return err
		}
	}
	if !hovered {
		return nil
	}
	radius := barThumbRadius * bar.Scale
	return newShape().ellipse(layout.Rect{X: level.Right() - radius, Y: line.Y + line.H/2 - radius, W: 2 * radius, H: 2 * radius}).fill(g, fillColor)
}

func (r *Renderer) drawChoice(g uintptr, bar controlbar.Bar, item controlbar.Item, look BarLook, font layout.Font) error {
	b := item.Bounds
	inset := layout.Rect{X: b.X + 2*bar.Scale, Y: b.Y + 2*bar.Scale, W: b.W - 4*bar.Scale, H: b.H - 4*bar.Scale}
	switch {
	case item.Active:
		if err := newShape().rounded(inset, 4*bar.Scale).fill(g, barAccent); err != nil {
			return err
		}
	case look.Hover.Action == controlbar.ActionSpeedChoice && look.Hover.Speed == item.Speed:
		if err := newShape().rounded(inset, 4*bar.Scale).fill(g, barHover); err != nil {
			return err
		}
	}
	return r.writeLine(g, look.Label(item), font, b, true, barIcon)
}

// drawTip shows the hovered item's tip above it, kept inside the band.
func (r *Renderer) drawTip(g uintptr, bar controlbar.Bar, look BarLook, font layout.Font) error {
	s := bar.Scale
	width := r.Width(look.Tip, font) + 2*barTipPadding*s
	height := r.LineHeight(font) + 2*barTipPadding*s
	anchor := look.Hover.Bounds
	x := anchor.X + anchor.W/2 - width/2
	x = math.Max(bar.Band.X+barTipGap*s, math.Min(x, bar.Band.Right()-barTipGap*s-width))
	y := bar.Panel.Y - barTipGap*s - height
	if look.Hover.Action == controlbar.ActionTrack {
		y = bar.Track.Y - barTipGap*s - height
	}
	box := layout.Rect{X: x, Y: math.Max(y, bar.Band.Y), W: width, H: height}
	if err := newShape().rounded(box, 4*s).fill(g, barTip); err != nil {
		return err
	}
	return r.writeLine(g, look.Tip, font, box, true, barIcon)
}

// writeLine writes one line inside a box, vertically centered and either
// centered or left aligned.
func (r *Renderer) writeLine(g uintptr, text string, font layout.Font, box layout.Rect, center bool, color appearance.Color) error {
	if text == "" || !usable(font) {
		return nil
	}
	width, height := r.Width(text, font), r.LineHeight(font)
	x := box.X
	if center {
		x = box.X + (box.W-width)/2
	}
	block := &layout.Block{Font: font, Scale: 1, Lines: []layout.Line{{Text: text, X: x, Y: box.Y + (box.H-height)/2, Width: width, Height: height}}}
	path, err := r.path(block)
	if err != nil {
		return err
	}
	defer procDeletePath.Call(path)
	return fillPath(g, path, color)
}

func pt(x, y float64) pointF {
	return pointF{X: float32(x), Y: float32(y)}
}
