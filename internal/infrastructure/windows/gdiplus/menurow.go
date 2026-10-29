//go:build windows && amd64

package gdiplus

import (
	"math"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// MenuColors are the colors of the context menu the app draws: its
// background, a highlighted row's, the text, a disabled row's text, a key's
// background and text, and the separators.
type MenuColors struct {
	Back, Hover, Text, Disabled, KeyBack, KeyText, Line appearance.Color
}

// MenuRowLook is one row of the context menu: a separator, or an item with
// its text, its key, and its icon (a square bitmap of IconSide, zero for
// none) or its check or choice mark, highlighted or disabled, opening a
// submenu or not.
type MenuRowLook struct {
	Font   layout.Font
	Scale  float64
	Colors MenuColors

	Separator               bool
	Text, Key               string
	Icon                    windows.Handle
	IconSide                int
	Radio, Checked, Submenu bool
	Highlighted, Disabled   bool
}

// A row's highlight inset from its sides and its top and bottom, the
// arrow's stroke and a separator's thickness, at 100%.
const (
	menuRowHoverX   = 4.0
	menuRowHoverY   = 1.0
	menuArrowStroke = 1.5
	menuLineWidth   = 1.0
)

// The arrow's chevron and a choice's dot, in fractions of their box.
const (
	menuArrowLeft   = 0.35
	menuArrowRight  = 0.7
	menuArrowTop    = 0.15
	menuArrowBottom = 0.85
	menuDotInset    = 0.32
	menuDotSize     = 0.36
)

// MenuRowSize is the size a row needs, in whole pixels.
func (r *Renderer) MenuRowSize(look MenuRowLook) (width, height int) {
	s := look.Scale
	if look.Separator {
		return 0, int(contextmenu.SeparatorHeight(s))
	}
	key := 0.0
	if look.Key != "" {
		key = r.Width(look.Key, look.Font)
	}
	w := contextmenu.RowWidth(r.Width(look.Text, look.Font), key, s)
	return int(math.Ceil(w)), int(contextmenu.RowHeight(r.LineHeight(look.Font), s))
}

// DrawMenuRow paints a row over a whole bitmap of its size, opaque: the
// background, the highlight, the mark or icon, the text, the key on its own
// background, clear of the arrows' column, and a submenu's arrow.
func (r *Renderer) DrawMenuRow(target *win32.DIB, look MenuRowLook) error {
	c, err := r.canvasOn(target)
	if err != nil {
		return err
	}
	// Pixel x covers x..x+1, so the background covers every pixel whole and
	// the picture, put on the menu as it is, has no half-lit edge.
	if err := call(procSetPixelOffsetMode, c.graphics, pixelOffsetHalf); err != nil {
		c.close()
		return err
	}
	bounds := layout.Rect{W: float64(target.Width), H: float64(target.Height)}
	row, err := r.drawRow(c.graphics, bounds, look)
	c.close()
	if err != nil || look.Separator || look.Checked || look.Icon == 0 {
		return err
	}
	// The icon is a bitmap of its own, laid over after GDI+ is done with
	// the pixels.
	x := row.Mark.X + (row.Mark.W-float64(look.IconSide))/2
	y := row.Mark.Y + (row.Mark.H-float64(look.IconSide))/2
	win32.BlendBitmap(target, look.Icon, int32(math.Round(x)), int32(math.Round(y)), look.IconSide)
	return nil
}

// drawRow draws a row but its icon, and tells where its parts went.
func (r *Renderer) drawRow(g uintptr, bounds layout.Rect, look MenuRowLook) (contextmenu.Row, error) {
	s, colors := look.Scale, look.Colors
	if err := fillRect(g, colors.Back, bounds); err != nil {
		return contextmenu.Row{}, err
	}
	if look.Separator {
		y := math.Floor(bounds.H / 2)
		line := layout.Rect{X: menuRowHoverX * s, Y: y, W: bounds.W - 2*menuRowHoverX*s, H: math.Max(math.Round(menuLineWidth*s), 1)}
		return contextmenu.Row{}, fillRect(g, colors.Line, line)
	}
	if look.Highlighted && !look.Disabled {
		hover := layout.Rect{X: menuRowHoverX * s, Y: menuRowHoverY * s, W: bounds.W - 2*menuRowHoverX*s, H: bounds.H - 2*menuRowHoverY*s}
		if err := newShape().rounded(hover, barRadius*s).fill(g, colors.Hover); err != nil {
			return contextmenu.Row{}, err
		}
	}
	text := colors.Text
	if look.Disabled {
		text = colors.Disabled
	}
	key := 0.0
	if look.Key != "" {
		key = r.Width(look.Key, look.Font)
	}
	row := contextmenu.LayRow(bounds, key, r.LineHeight(look.Font), s)
	if look.Checked {
		if err := drawMenuMark(g, row.Mark, look.Radio, text, s); err != nil {
			return row, err
		}
	}
	if err := r.writeFitted(g, look.Text, look.Font, row.Text, text); err != nil {
		return row, err
	}
	if look.Key != "" {
		if err := newShape().rounded(row.Key, barRadius*s).fill(g, colors.KeyBack); err != nil {
			return row, err
		}
		keyText := colors.KeyText
		if look.Disabled {
			keyText = colors.Disabled
		}
		if err := r.writeLine(g, look.Key, look.Font, row.KeyText, false, keyText); err != nil {
			return row, err
		}
	}
	if look.Submenu {
		a := row.Arrow
		arrow := newShape().
			line(a.X+a.W*menuArrowLeft, a.Y+a.H*menuArrowTop, a.X+a.W*menuArrowRight, a.Y+a.H/2).
			line(a.X+a.W*menuArrowRight, a.Y+a.H/2, a.X+a.W*menuArrowLeft, a.Y+a.H*menuArrowBottom)
		if err := arrow.stroke(g, text, menuArrowStroke*s); err != nil {
			return row, err
		}
	}
	return row, nil
}

// drawMenuMark draws a checked item's mark in its box: a check, or the dot
// of the choice made.
func drawMenuMark(g uintptr, box layout.Rect, radio bool, color appearance.Color, s float64) error {
	if radio {
		inner := layout.Rect{X: box.X + box.W*menuDotInset, Y: box.Y + box.H*menuDotInset, W: box.W * menuDotSize, H: box.H * menuDotSize}
		return newShape().ellipse(inner).fill(g, color)
	}
	return newShape().
		line(box.X+box.W*checkLeftX, box.Y+box.H*checkLeftY, box.X+box.W*checkLowX, box.Y+box.H*checkLowY).
		line(box.X+box.W*checkLowX, box.Y+box.H*checkLowY, box.X+box.W*checkRightX, box.Y+box.H*checkRightY).
		stroke(g, color, menuArrowStroke*s)
}
