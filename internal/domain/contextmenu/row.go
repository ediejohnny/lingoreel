package contextmenu

import (
	"math"

	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

// A row of the menu as the app draws it, at 100% (scale 1): the side
// margin, the column of the mark or icon, the gap after it, the least room
// between the text and the key, the key's background around its text, the
// gap before the arrows' column and that column, which every row keeps so
// that the keys end on one line, clear of the arrows; and a separator's
// height.
const (
	rowMargin       = 8.0
	rowMark         = 16.0
	rowMarkGap      = 8.0
	rowKeyGap       = 24.0
	rowKeyPadX      = 6.0
	rowKeyPadY      = 2.0
	rowArrowGap     = 6.0
	rowArrow        = 12.0
	rowPadY         = 4.0
	separatorHeight = 9.0
)

// Row is where a row's parts go: the mark (a check, a choice dot or the
// icon), the text, the key's background and its text inside, and the
// submenu's arrow. Key and KeyText are empty for a row without a key.
type Row struct {
	Mark, Text, Key, KeyText, Arrow layout.Rect
}

// RowWidth is the width a row needs for its text and its key (keyWidth 0
// for none), at a scale.
func RowWidth(textWidth, keyWidth, scale float64) float64 {
	width := 2*rowMargin + rowMark + rowMarkGap + rowArrowGap + rowArrow
	if keyWidth > 0 {
		width += rowKeyGap + keyWidth + 2*rowKeyPadX
	}
	return width*scale + textWidth
}

// RowHeight is a row's height for a line of text of a height, at a scale:
// the mark or the line, whichever is taller, and a margin above and below.
func RowHeight(lineHeight, scale float64) float64 {
	return math.Ceil(math.Max(rowMark*scale, lineHeight+2*rowKeyPadY*scale) + 2*rowPadY*scale)
}

// SeparatorHeight is a separator's height at a scale.
func SeparatorHeight(scale float64) float64 {
	return math.Ceil(separatorHeight * scale)
}

// LayRow places a row's parts in its bounds. The key ends where the arrows'
// column begins, so the keys of a menu, whatever their widths, end on one
// line, and the text takes what is left before the key.
func LayRow(bounds layout.Rect, keyWidth, lineHeight, scale float64) Row {
	middle := bounds.Y + bounds.H/2
	mark := rowMark * scale
	row := Row{Mark: layout.Rect{X: bounds.X + rowMargin*scale, Y: middle - mark/2, W: mark, H: mark}}
	arrow := rowArrow * scale
	row.Arrow = layout.Rect{X: bounds.Right() - rowMargin*scale - arrow, Y: middle - arrow/2, W: arrow, H: arrow}
	textLeft := row.Mark.Right() + rowMarkGap*scale
	textRight := row.Arrow.X - rowArrowGap*scale
	if keyWidth > 0 {
		height := lineHeight + 2*rowKeyPadY*scale
		width := keyWidth + 2*rowKeyPadX*scale
		row.Key = layout.Rect{X: textRight - width, Y: middle - height/2, W: width, H: height}
		row.KeyText = layout.Rect{X: row.Key.X + rowKeyPadX*scale, Y: middle - lineHeight/2, W: keyWidth, H: lineHeight}
		textRight = row.Key.X - rowKeyGap*scale
	}
	row.Text = layout.Rect{X: textLeft, Y: middle - lineHeight/2, W: math.Max(textRight-textLeft, 0), H: lineHeight}
	return row
}
