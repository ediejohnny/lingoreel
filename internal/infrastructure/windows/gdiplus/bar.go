//go:build windows && amd64

package gdiplus

import (
	"math"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// Colors of the control bar: the dark theme's tokens, since the bar
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
	// barAccentText is the study text: light enough on the dark panel.
	barAccentText = appearance.Color{R: 0x9c, G: 0xc3, B: 0xff, A: 0xff}
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
	// Entry words the open menu's entries; nil writes none.
	Entry func(controlbar.Entry) controlbar.EntryText
	// Status is the study text ("Início: 10:38"), empty for none.
	Status string
}

// Menu geometry at 100% scaling, in pixels.
const (
	menuTextPad  = 12.0
	menuCheck    = 14.0
	menuCheckGap = 8.0
)

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
	if bar.Status.Valid() && look.Status != "" {
		if err := r.writeFitted(g, look.Status, font, bar.Status, barAccentText); err != nil {
			return err
		}
	}
	if bar.MenuPanel.Valid() {
		if err := r.drawMenuPanel(g, bar, look, font); err != nil {
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
	inMenu := look.Hover.Action == controlbar.ActionEntry || look.Hover.Action == controlbar.ActionEntryText
	if look.Tip != "" && look.Hover.Action != controlbar.ActionNone && look.Hover.Action != controlbar.ActionSpeedChoice && !inMenu {
		return r.drawTip(g, bar, look, font)
	}
	return nil
}

// drawMenuPanel draws the open menu: its background, then each entry.
func (r *Renderer) drawMenuPanel(g uintptr, bar controlbar.Bar, look BarLook, font layout.Font) error {
	s := bar.Scale
	if err := newShape().rounded(bar.MenuPanel, barRadius*s).fill(g, barMenu); err != nil {
		return err
	}
	for _, item := range bar.MenuItems {
		if err := r.drawEntry(g, bar, item, look, font); err != nil {
			return err
		}
	}
	return nil
}

// drawEntry draws one entry of a menu: a line, a title, or a row with its
// mark (a check, a choice dot), its text, and its value, key or buttons.
func (r *Renderer) drawEntry(g uintptr, bar controlbar.Bar, item controlbar.Item, look BarLook, font layout.Font) error {
	s := bar.Scale
	b := item.Bounds
	entry := item.Entry
	color := barIcon
	if !item.Enabled {
		color = barDisabled
	}
	text := controlbar.EntryText{}
	if look.Entry != nil {
		text = look.Entry(entry)
	}
	switch {
	case entry.Kind == controlbar.EntrySeparator:
		y := b.Y + b.H/2
		return fillRect(g, barRail, layout.Rect{X: b.X + menuTextPad*s, Y: y, W: b.W - 2*menuTextPad*s, H: math.Max(s, 1)})
	case entry.Kind == controlbar.EntryLabel:
		small := font
		small.Size, small.Bold = font.Size*0.9, true
		return r.writeLine(g, text.Text, small, layout.Rect{X: b.X + menuTextPad*s, Y: b.Y, W: b.W - 2*menuTextPad*s, H: b.H}, false, barDisabled)
	case entry.Kind == controlbar.EntryStepper && entry.Delta != 0:
		return r.drawStepButton(g, bar, item, look, color)
	}
	hovered := item.Action == controlbar.ActionEntry && item.Enabled && look.Hover.Action == controlbar.ActionEntry && look.Hover.Entry == entry
	if hovered {
		inset := layout.Rect{X: b.X + 4*s, Y: b.Y + s, W: b.W - 8*s, H: b.H - 2*s}
		if err := newShape().rounded(inset, 4*s).fill(g, barHover); err != nil {
			return err
		}
	}
	left := b.X + menuTextPad*s
	mark := layout.Rect{X: left, Y: b.Y + (b.H-menuCheck*s)/2, W: menuCheck * s, H: menuCheck * s}
	switch entry.Kind {
	case controlbar.EntryToggle:
		if err := r.drawCheck(g, mark, entry.Checked, color, s); err != nil {
			return err
		}
		left = mark.Right() + menuCheckGap*s
	case controlbar.EntryChoice:
		if err := r.drawDot(g, mark, entry.Checked, color, s); err != nil {
			return err
		}
		left = mark.Right() + menuCheckGap*s
	}
	right := b.Right() - menuTextPad*s
	if entry.Kind == controlbar.EntryStepper {
		// The two buttons sit on the right; the value goes before them.
		right -= 2*stepButtonSize*s + 4*s
	}
	trailing := text.Value
	if trailing == "" {
		trailing = text.Key
	}
	if trailing != "" {
		width := r.Width(trailing, font)
		if err := r.writeLine(g, trailing, font, layout.Rect{X: right - width, Y: b.Y, W: width, H: b.H}, false, barDisabled); err != nil {
			return err
		}
		right -= width + menuCheckGap*s
	}
	return r.writeFitted(g, text.Text, font, layout.Rect{X: left, Y: b.Y, W: math.Max(right-left, 0), H: b.H}, color)
}

// stepButtonSize matches the domain's stepper buttons.
const stepButtonSize = 24.0

// drawStepButton draws a stepper's [−] or [+].
func (r *Renderer) drawStepButton(g uintptr, bar controlbar.Bar, item controlbar.Item, look BarLook, color appearance.Color) error {
	s := bar.Scale
	b := item.Bounds
	background := barRail
	if item.Enabled && look.Hover.Action == controlbar.ActionEntry && look.Hover.Entry == item.Entry {
		background = barAccent
	}
	if err := newShape().rounded(b, 4*s).fill(g, background.WithAlpha(0x60)); err != nil {
		return err
	}
	cx, cy, arm := b.X+b.W/2, b.Y+b.H/2, b.W*0.25
	sh := newShape().line(cx-arm, cy, cx+arm, cy)
	if item.Entry.Delta > 0 {
		sh.line(cx, cy-arm, cx, cy+arm)
	}
	return sh.stroke(g, color, barStroke*s)
}

// drawCheck draws a toggle's box, with a check when on.
func (r *Renderer) drawCheck(g uintptr, box layout.Rect, on bool, color appearance.Color, s float64) error {
	if on {
		if err := newShape().rounded(box, 3*s).fill(g, barAccent); err != nil {
			return err
		}
		return newShape().
			line(box.X+box.W*0.22, box.Y+box.H*0.52, box.X+box.W*0.43, box.Y+box.H*0.72).
			line(box.X+box.W*0.43, box.Y+box.H*0.72, box.X+box.W*0.78, box.Y+box.H*0.3).
			stroke(g, barIcon, barStroke*s)
	}
	return newShape().rounded(box, 3*s).stroke(g, color, barStroke*s*0.75)
}

// drawDot draws a choice's ring, filled when chosen.
func (r *Renderer) drawDot(g uintptr, box layout.Rect, on bool, color appearance.Color, s float64) error {
	if err := newShape().ellipse(box).stroke(g, color, barStroke*s*0.75); err != nil {
		return err
	}
	if !on {
		return nil
	}
	inner := layout.Rect{X: box.X + box.W*0.28, Y: box.Y + box.H*0.28, W: box.W * 0.44, H: box.H * 0.44}
	return newShape().ellipse(inner).fill(g, barAccent)
}

// writeFitted writes one line, left aligned, cut with "…" when it does not
// fit the box.
func (r *Renderer) writeFitted(g uintptr, text string, font layout.Font, box layout.Rect, color appearance.Color) error {
	if text == "" || box.W <= 0 {
		return nil
	}
	if r.Width(text, font) > box.W {
		runes := []rune(text)
		for len(runes) > 0 && r.Width(string(runes)+"…", font) > box.W {
			runes = runes[:len(runes)-1]
		}
		if len(runes) == 0 {
			return nil
		}
		text = string(runes) + "…"
	}
	return r.writeLine(g, text, font, box, false, color)
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
	hovered := item.Enabled && look.Hover.Action == item.Action && look.Hover.Menu == item.Menu && look.Hover.Action != controlbar.ActionSpeedChoice
	// The speed list or the item's menu is open.
	open := (item.Action == controlbar.ActionSpeed || item.Action == controlbar.ActionMenu) && item.Active
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
	case controlbar.ActionFrameBack, controlbar.ActionFrameForward:
		// A bar and a triangle: |◀ and ▶|.
		thick := icon * 0.16
		if item.Action == controlbar.ActionFrameBack {
			return newShape().
				rounded(layout.Rect{X: box.X + icon*0.1, Y: box.Y + icon*0.12, W: thick, H: icon * 0.76}, s*0.5).
				polygon(pt(box.Right()-icon*0.1, box.Y+icon*0.12), pt(box.X+icon*0.32, box.Y+icon/2), pt(box.Right()-icon*0.1, box.Bottom()-icon*0.12)).
				fill(g, color)
		}
		return newShape().
			polygon(pt(box.X+icon*0.1, box.Y+icon*0.12), pt(box.Right()-icon*0.32, box.Y+icon/2), pt(box.X+icon*0.1, box.Bottom()-icon*0.12)).
			rounded(layout.Rect{X: box.Right() - icon*0.1 - thick, Y: box.Y + icon*0.12, W: thick, H: icon * 0.76}, s*0.5).
			fill(g, color)
	case controlbar.ActionMenu:
		return r.drawMenuIcon(g, item.Menu, box, color, stroke, font)
	}
	return nil
}

// drawMenuIcon draws a menu button's icon: a flag for the passage, a note
// for the Modo Karaokê, "CC" in a frame for the subtitles.
func (r *Renderer) drawMenuIcon(g uintptr, menu controlbar.Menu, box layout.Rect, color appearance.Color, stroke float64, font layout.Font) error {
	icon := box.W
	switch menu {
	case controlbar.MenuPassage:
		pole := newShape().line(box.X+icon*0.22, box.Y, box.X+icon*0.22, box.Bottom())
		if err := pole.stroke(g, color, stroke); err != nil {
			return err
		}
		return newShape().polygon(pt(box.X+icon*0.26, box.Y), pt(box.Right(), box.Y+icon*0.27), pt(box.X+icon*0.26, box.Y+icon*0.55)).fill(g, color)
	case controlbar.MenuKaraoke:
		head := layout.Rect{X: box.X + icon*0.1, Y: box.Y + icon*0.62, W: icon * 0.36, H: icon * 0.3}
		if err := newShape().ellipse(head).fill(g, color); err != nil {
			return err
		}
		return newShape().
			line(head.Right()-stroke/2, head.Y+head.H/2, head.Right()-stroke/2, box.Y+icon*0.05).
			line(head.Right()-stroke/2, box.Y+icon*0.05, box.Right()-icon*0.08, box.Y+icon*0.22).
			stroke(g, color, stroke)
	case controlbar.MenuSubtitles:
		frame := layout.Rect{X: box.X - icon*0.1, Y: box.Y + icon*0.12, W: icon * 1.2, H: icon * 0.76}
		if err := newShape().rounded(frame, icon*0.12).stroke(g, color, stroke*0.75); err != nil {
			return err
		}
		small := font
		small.Size, small.Bold = font.Size*0.75, true
		return r.writeLine(g, "CC", small, frame, true, color)
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
