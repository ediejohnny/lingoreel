package contextmenu_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

// In a menu as wide as its widest row needs, the keys of every row, short or
// long, end on one line, clear of the submenus' arrows, each on its
// background; and no text reaches its key.
func TestTheKeysEndOnOneLineBeforeTheArrows(t *testing.T) {
	const line = 16.0
	for _, scale := range []float64{1, 1.5, 2} {
		rows := []struct{ text, key float64 }{{120, 0}, {80, 30}, {60, 110}, {200, 18}}
		width := 0.0
		for _, r := range rows {
			width = max(width, contextmenu.RowWidth(r.text, r.key, scale))
		}
		bounds := layout.Rect{X: 3, Y: 40, W: width, H: contextmenu.RowHeight(line, scale)}
		arrow := contextmenu.LayRow(bounds, 0, line, scale).Arrow
		end := -1.0
		for _, r := range rows {
			row := contextmenu.LayRow(bounds, r.key, line, scale)
			if r.key == 0 {
				if row.Key != (layout.Rect{}) {
					t.Errorf("at %v a row without a key has a key box %+v", scale, row.Key)
				}
				continue
			}
			if end < 0 {
				end = row.Key.Right()
			}
			if row.Key.Right() != end {
				t.Errorf("at %v a key %v wide ends at %v, another at %v", scale, r.key, row.Key.Right(), end)
			}
			if row.Key.Right() >= arrow.X {
				t.Errorf("at %v a key ends at %v, on the arrows' column from %v", scale, row.Key.Right(), arrow.X)
			}
			if row.KeyText.X <= row.Key.X || row.KeyText.Right() >= row.Key.Right() || row.KeyText.W != r.key {
				t.Errorf("at %v the key's text %+v is not inside its background %+v", scale, row.KeyText, row.Key)
			}
			if row.Text.X+r.text >= row.Key.X {
				t.Errorf("at %v a text %v wide from %v reaches its key at %v", scale, r.text, row.Text.X, row.Key.X)
			}
		}
	}
}
