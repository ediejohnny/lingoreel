package layout_test

import (
	"math"
	"testing"

	"lingoreel/internal/domain/layout"
)

// With monoMeasurer at size 10 every character is 5 pixels wide: "ab cd"
// is 25 wide with 4 visible characters, "ef" 10 wide with 2; 6 in all.
func fillBlock() *layout.Block {
	font := layout.Font{Size: 10}
	return &layout.Block{Font: font, Lines: []layout.Line{
		{Text: "ab cd", Width: 25}, {Text: "ef", Width: 10},
	}}
}

func TestFillWidths(t *testing.T) {
	cases := []struct {
		fraction float64
		want     []float64
	}{
		{0, []float64{0, 0}},
		{-1, []float64{0, 0}},
		{math.NaN(), []float64{0, 0}},
		{1, []float64{25, 10}},
		{2, []float64{25, 10}},
		{4.0 / 6, []float64{25, 0}}, // the first line exactly
		// 3 of 6: "ab" and "c" (the space is skipped): "ab c" is 20 wide.
		{0.5, []float64{20, 0}},
		// 2.5 of 6: half of "c", between "ab " (15) and "ab c" (20).
		{2.5 / 6, []float64{17.5, 0}},
		// 5 of 6: all of the first line and "e" of the second.
		{5.0 / 6, []float64{25, 5}},
	}
	for _, c := range cases {
		got := layout.FillWidths(fillBlock(), c.fraction, monoMeasurer{})
		if len(got) != len(c.want) {
			t.Fatalf("FillWidths(%v) = %v", c.fraction, got)
		}
		for i := range got {
			if math.Abs(got[i]-c.want[i]) > 1e-9 {
				t.Errorf("FillWidths(%v) = %v, want %v", c.fraction, got, c.want)
				break
			}
		}
	}
	if layout.FillWidths(nil, 0.5, monoMeasurer{}) != nil {
		t.Error("a missing block has widths")
	}
}
