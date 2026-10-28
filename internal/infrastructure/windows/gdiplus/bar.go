//go:build windows && amd64

package gdiplus

import (
	"math"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// The bar's see-through white: under a hovered item, and the empty part of
// its lines. They read on any panel the user picks.
var (
	barHover = appearance.Color{R: 0xff, G: 0xff, B: 0xff, A: 0x2e}
	barRail  = appearance.Color{R: 0xff, G: 0xff, B: 0xff, A: 0x55}
)

// disabledAlpha dims the icons' color for what the player cannot do.
const disabledAlpha = 0x5c

// barPalette is every color the bar draws with, from its style: the tips sit
// on the menus' color, a disabled item is the icons' color dimmed, and the
// study text is the accent made lighter, to read on a dark panel.
type barPalette struct {
	panel, menu, tip, hover, rail, accent, icon, disabled, loud, accentText appearance.Color
}

func paletteOf(style appearance.BarStyle) barPalette {
	return barPalette{
		panel: style.Panel, menu: style.Menu, tip: style.Menu, hover: barHover, rail: barRail,
		accent: style.Accent, icon: style.Icon, loud: style.Loud,
		disabled:   style.Icon.WithAlpha(uint8(int(style.Icon.A) * disabledAlpha / 0xff)),
		accentText: lighter(style.Accent),
	}
}

// lighter moves a color two fifths of the way to white.
func lighter(c appearance.Color) appearance.Color {
	up := func(v uint8) uint8 { return v + uint8((0xff-int(v))*2/5) }
	return appearance.Color{R: up(c.R), G: up(c.G), B: up(c.B), A: c.A}
}

// Bar geometry at 100% scaling, in pixels.
const (
	barFontSize = 13.0
	barIcon16   = 16.0
	// barRadius rounds the corners of the bar's boxes (menus, hover,
	// tips) barely: 0.1rem at 100%.
	barRadius      = 1.6
	barThumbRadius = 6.0
	barStroke      = 2.0
	barTipPadding  = 6.0
	barTipGap      = 6.0
)

// BarLook is what the bar shows besides its layout: texts already worded
// for the user, the item under the mouse, and its colors and font.
type BarLook struct {
	// Style is the bar's colors and font; its size is in the layout's scale.
	Style appearance.BarStyle
	// palette is Style's colors, worked out once per drawing.
	palette barPalette
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
	// Status is the study text ("Start: 10:38"), empty for none.
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
	look.palette = paletteOf(look.Style)
	p := look.palette
	font := layout.Font{Family: look.Style.FontFamily, Size: barFontSize * s}

	if err := fillRect(g, p.panel, bar.Panel); err != nil {
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
		if err := r.writeLine(g, look.Time, font, bar.Time, false, p.icon); err != nil {
			return err
		}
	}
	if bar.Status.Valid() && look.Status != "" {
		if err := r.writeFitted(g, look.Status, font, bar.Status, p.accentText); err != nil {
			return err
		}
	}
	if bar.MenuPanel.Valid() {
		if err := r.drawMenuPanel(g, bar, look, font); err != nil {
			return err
		}
	}
	if bar.Menu.Valid() {
		if err := newShape().rounded(bar.Menu, barRadius*s).fill(g, p.menu); err != nil {
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
	p := look.palette
	s := bar.Scale
	if err := newShape().rounded(bar.MenuPanel, barRadius*s).fill(g, p.menu); err != nil {
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
	p := look.palette
	s := bar.Scale
	b := item.Bounds
	entry := item.Entry
	color := p.icon
	if !item.Enabled {
		color = p.disabled
	}
	text := controlbar.EntryText{}
	if look.Entry != nil {
		text = look.Entry(entry)
	}
	switch {
	case entry.Kind == controlbar.EntrySeparator:
		y := b.Y + b.H/2
		return fillRect(g, p.rail, layout.Rect{X: b.X + menuTextPad*s, Y: y, W: b.W - 2*menuTextPad*s, H: math.Max(s, 1)})
	case entry.Kind == controlbar.EntryLabel:
		small := font
		small.Size, small.Bold = font.Size*menuLabelFont, true
		return r.writeLine(g, text.Text, small, layout.Rect{X: b.X + menuTextPad*s, Y: b.Y, W: b.W - 2*menuTextPad*s, H: b.H}, false, p.disabled)
	case entry.Kind == controlbar.EntryStepper && entry.Delta != 0:
		return r.drawStepButton(g, bar, item, look, color)
	}
	hovered := item.Action == controlbar.ActionEntry && item.Enabled && look.Hover.Action == controlbar.ActionEntry && look.Hover.Entry == entry
	if hovered {
		inset := layout.Rect{X: b.X + menuHoverInset*s, Y: b.Y + s, W: b.W - 2*menuHoverInset*s, H: b.H - 2*s}
		if err := newShape().rounded(inset, barRadius*s).fill(g, p.hover); err != nil {
			return err
		}
	}
	left := b.X + menuTextPad*s
	mark := layout.Rect{X: left, Y: b.Y + (b.H-menuCheck*s)/2, W: menuCheck * s, H: menuCheck * s}
	switch entry.Kind {
	case controlbar.EntryToggle:
		if err := r.drawCheck(g, p, mark, entry.Checked, color, s); err != nil {
			return err
		}
		left = mark.Right() + menuCheckGap*s
	case controlbar.EntryChoice:
		if err := r.drawDot(g, p, mark, entry.Checked, color, s); err != nil {
			return err
		}
		left = mark.Right() + menuCheckGap*s
	}
	right := b.Right() - menuTextPad*s
	if entry.Kind == controlbar.EntryStepper {
		// The two buttons sit on the right; the value goes before them.
		right -= 2*stepButtonSize*s + stepButtonGap*s
	}
	trailing := text.Value
	if trailing == "" {
		trailing = text.Key
	}
	if trailing != "" {
		width := r.Width(trailing, font)
		if err := r.writeLine(g, trailing, font, layout.Rect{X: right - width, Y: b.Y, W: width, H: b.H}, false, p.disabled); err != nil {
			return err
		}
		right -= width + menuCheckGap*s
	}
	return r.writeFitted(g, text.Text, font, layout.Rect{X: left, Y: b.Y, W: math.Max(right-left, 0), H: b.H}, color)
}

// A stepper's buttons: their size (the domain's), the gap before them, the
// alpha of their background and the arms of their sign, of the size.
const (
	stepButtonSize  = 24.0
	stepButtonGap   = 4.0
	stepButtonAlpha = 0x60
	stepArm         = 0.25
)

// The marks of the menus' entries, in fractions of their box: the check's
// three points, and the dot inside a chosen ring.
const (
	checkLeftX  = 0.22
	checkLeftY  = 0.52
	checkLowX   = 0.43
	checkLowY   = 0.72
	checkRightX = 0.78
	checkRightY = 0.3
	dotInset    = 0.28
	dotSize     = 0.44
)

// Texts and boxes of the bar, at 100%: a menu's label size, of the menu's
// text; a hovered entry's inset; the translation button's frame inset and
// its letters' size.
const (
	menuLabelFont     = 0.9
	menuHoverInset    = 4.0
	translationInsetX = 6.0
	translationInsetY = 10.0
	translationFont   = 0.85
)

// drawStepButton draws a stepper's [−] or [+].
func (r *Renderer) drawStepButton(g uintptr, bar controlbar.Bar, item controlbar.Item, look BarLook, color appearance.Color) error {
	p := look.palette
	s := bar.Scale
	b := item.Bounds
	background := p.rail
	if item.Enabled && look.Hover.Action == controlbar.ActionEntry && look.Hover.Entry == item.Entry {
		background = p.accent
	}
	if err := newShape().rounded(b, barRadius*s).fill(g, background.WithAlpha(stepButtonAlpha)); err != nil {
		return err
	}
	cx, cy, arm := b.X+b.W/2, b.Y+b.H/2, b.W*stepArm
	sh := newShape().line(cx-arm, cy, cx+arm, cy)
	if item.Entry.Delta > 0 {
		sh.line(cx, cy-arm, cx, cy+arm)
	}
	return sh.stroke(g, color, barStroke*s)
}

// drawCheck draws a toggle's box, with a check when on.
func (r *Renderer) drawCheck(g uintptr, p barPalette, box layout.Rect, on bool, color appearance.Color, s float64) error {
	if on {
		if err := newShape().rounded(box, barRadius*s).fill(g, p.accent); err != nil {
			return err
		}
		return newShape().
			line(box.X+box.W*checkLeftX, box.Y+box.H*checkLeftY, box.X+box.W*checkLowX, box.Y+box.H*checkLowY).
			line(box.X+box.W*checkLowX, box.Y+box.H*checkLowY, box.X+box.W*checkRightX, box.Y+box.H*checkRightY).
			stroke(g, p.icon, barStroke*s)
	}
	return newShape().rounded(box, barRadius*s).stroke(g, color, barStroke*s*thinStroke)
}

// drawDot draws a choice's ring, filled when chosen.
func (r *Renderer) drawDot(g uintptr, p barPalette, box layout.Rect, on bool, color appearance.Color, s float64) error {
	if err := newShape().ellipse(box).stroke(g, color, barStroke*s*thinStroke); err != nil {
		return err
	}
	if !on {
		return nil
	}
	inner := layout.Rect{X: box.X + box.W*dotInset, Y: box.Y + box.H*dotInset, W: box.W * dotSize, H: box.H * dotSize}
	return newShape().ellipse(inner).fill(g, p.accent)
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
	p := look.palette
	track := bar.Track
	if !track.Valid() {
		return nil
	}
	if err := fillRect(g, p.rail, track); err != nil {
		return err
	}
	played := track
	played.W = track.W * bar.Played
	if played.W > 0 {
		if err := fillRect(g, p.accent, played); err != nil {
			return err
		}
	}
	if !look.Thumb {
		return nil
	}
	radius := barThumbRadius * bar.Scale
	center := layout.Rect{X: played.Right() - radius, Y: track.Y + track.H/2 - radius, W: 2 * radius, H: 2 * radius}
	return newShape().ellipse(center).fill(g, p.accent)
}

func (r *Renderer) drawItem(g uintptr, bar controlbar.Bar, item controlbar.Item, look BarLook, font layout.Font) error {
	p := look.palette
	s := bar.Scale
	b := item.Bounds
	color := p.icon
	if !item.Enabled {
		color = p.disabled
	}
	hovered := item.Enabled && look.Hover.Action == item.Action && look.Hover.Menu == item.Menu && look.Hover.Action != controlbar.ActionSpeedChoice
	// The speed list or the item's menu is open.
	open := (item.Action == controlbar.ActionSpeed || item.Action == controlbar.ActionMenu) && item.Active
	if (hovered || open) && item.Action != controlbar.ActionVolume && item.Action != controlbar.ActionTrack {
		if err := newShape().rounded(b, barRadius*s).fill(g, p.hover); err != nil {
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
			return r.drawIcon(g, contextmenu.IconPause, box, color, stroke, font)
		}
		return r.drawIcon(g, contextmenu.IconPlay, box, color, stroke, font)
	case controlbar.ActionBack:
		return r.drawIcon(g, contextmenu.IconBack, box, color, stroke, font)
	case controlbar.ActionForward:
		return r.drawIcon(g, contextmenu.IconForward, box, color, stroke, font)
	case controlbar.ActionMute:
		// Muted: a cross instead of the waves.
		return drawSpeaker(g, box, color, stroke, item.Active)
	case controlbar.ActionVolume:
		return r.drawVolume(g, p, bar, item, hovered)
	case controlbar.ActionTranslation:
		frame := layout.Rect{X: b.X + translationInsetX*s, Y: b.Y + translationInsetY*s, W: b.W - 2*translationInsetX*s, H: b.H - 2*translationInsetY*s}
		if item.Active {
			if err := newShape().rounded(frame, barRadius*s).fill(g, p.accent); err != nil {
				return err
			}
		} else if err := newShape().rounded(frame, barRadius*s).stroke(g, color, stroke*thinStroke); err != nil {
			return err
		}
		small := font
		small.Size, small.Bold = font.Size*translationFont, true
		return r.writeLine(g, "PT", small, frame, true, color)
	case controlbar.ActionSpeed:
		return r.writeLine(g, look.Label(item), font, b, true, color)
	case controlbar.ActionFullscreen:
		if item.Active {
			return r.drawIcon(g, contextmenu.IconExitFullscreen, box, color, stroke, font)
		}
		return r.drawIcon(g, contextmenu.IconFullscreen, box, color, stroke, font)
	case controlbar.ActionFrameBack:
		return r.drawIcon(g, contextmenu.IconFrameBack, box, color, stroke, font)
	case controlbar.ActionFrameForward:
		return r.drawIcon(g, contextmenu.IconFrameForward, box, color, stroke, font)
	case controlbar.ActionMenu:
		return r.drawIcon(g, menuIcon(item.Menu), box, color, stroke, font)
	}
	return nil
}

// menuIcon is a menu button's icon: a flag for the passage, a note for the
// karaoke mode, "CC" in a frame for the subtitles.
func menuIcon(menu controlbar.Menu) contextmenu.Icon {
	switch menu {
	case controlbar.MenuPassage:
		return contextmenu.IconPassage
	case controlbar.MenuKaraoke:
		return contextmenu.IconKaraoke
	case controlbar.MenuSubtitles:
		return contextmenu.IconSubtitles
	}
	return contextmenu.IconNone
}

// drawCorners draws the four corners of the fullscreen icon, pointing out
// to enter fullscreen and in to leave it.
func (r *Renderer) drawCorners(g uintptr, box layout.Rect, inward bool, color appearance.Color, stroke float64) error {
	arm := box.W * cornerArm
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

func (r *Renderer) drawVolume(g uintptr, p barPalette, bar controlbar.Bar, item controlbar.Item, hovered bool) error {
	line := bar.VolumeTrack
	if !line.Valid() {
		return nil
	}
	rail, fillColor := p.rail, p.icon
	if !item.Enabled {
		rail, fillColor = p.disabled, p.disabled
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
	// On a player that amplifies, the part past 100% is in a warning color,
	// and a mark shows where 100% is.
	if full := bar.VolumeFull; full > 0 && full < 1 && item.Enabled {
		at := line.X + line.W*full
		if loud := level.Right() - at; loud > 0 {
			if err := fillRect(g, p.loud, layout.Rect{X: at, Y: line.Y, W: loud, H: line.H}); err != nil {
				return err
			}
		}
		mark := math.Max(1, bar.Scale)
		if err := fillRect(g, p.icon, layout.Rect{X: at - mark/2, Y: line.Y - 2*mark, W: mark, H: line.H + 4*mark}); err != nil {
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
	p := look.palette
	b := item.Bounds
	inset := layout.Rect{X: b.X + 2*bar.Scale, Y: b.Y + 2*bar.Scale, W: b.W - 4*bar.Scale, H: b.H - 4*bar.Scale}
	switch {
	case item.Active:
		if err := newShape().rounded(inset, barRadius*bar.Scale).fill(g, p.accent); err != nil {
			return err
		}
	case look.Hover.Action == controlbar.ActionSpeedChoice && look.Hover.Speed == item.Speed:
		if err := newShape().rounded(inset, barRadius*bar.Scale).fill(g, p.hover); err != nil {
			return err
		}
	}
	return r.writeLine(g, look.Label(item), font, b, true, p.icon)
}

// drawTip shows the hovered item's tip above it, kept inside the band.
func (r *Renderer) drawTip(g uintptr, bar controlbar.Bar, look BarLook, font layout.Font) error {
	p := look.palette
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
	if err := newShape().rounded(box, barRadius*s).fill(g, p.tip); err != nil {
		return err
	}
	return r.writeLine(g, look.Tip, font, box, true, p.icon)
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
