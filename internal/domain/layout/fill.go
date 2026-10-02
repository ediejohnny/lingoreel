package layout

import (
	"math"
	"unicode"
)

// FillWidths is how much of each line of a block is painted when a share of
// its visible characters (spaces left out) has been spoken, measured from
// each line's left edge: whole lines before, a measured prefix in the line
// being spoken, nothing after.
func FillWidths(block *Block, fraction float64, m Measurer) []float64 {
	if block == nil {
		return nil
	}
	widths := make([]float64, len(block.Lines))
	if math.IsNaN(fraction) || fraction <= 0 {
		return widths
	}
	total := 0
	for _, line := range block.Lines {
		total += visibleRunes(line.Text)
	}
	target := math.Min(fraction, 1) * float64(total)
	for i, line := range block.Lines {
		count := float64(visibleRunes(line.Text))
		if target >= count {
			widths[i] = line.Width
			target -= count
			continue
		}
		widths[i] = prefixWidth(line, target, block.Font, m)
		break
	}
	return widths
}

// prefixWidth measures a line up to a number of visible characters, the
// last one counted by its share.
func prefixWidth(line Line, visible float64, font Font, m Measurer) float64 {
	if visible <= 0 {
		return 0
	}
	runes := []rune(line.Text)
	seen := 0
	for i, r := range runes {
		if unicode.IsSpace(r) {
			continue
		}
		if float64(seen+1) > visible {
			before := m.Width(string(runes[:i]), font)
			with := m.Width(string(runes[:i+1]), font)
			return math.Min(before+(with-before)*(visible-float64(seen)), line.Width)
		}
		seen++
	}
	return line.Width
}

func visibleRunes(text string) int {
	count := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}
