package appearance_test

import (
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
)

// The published CIEDE2000 test data (Sharma, Wu and Dalal, 2005): every
// branch of the formula (hue wrap-around, zero chroma, the blue rotation)
// has a pair here, with the difference to four decimals.
func TestDeltaE2000MatchesThePublishedPairs(t *testing.T) {
	t.Parallel()
	pairs := []struct {
		first, second appearance.Lab
		want          float64
	}{
		{lab(50, 2.6772, -79.7751), lab(50, 0, -82.7485), 2.0425},
		{lab(50, 3.1571, -77.2803), lab(50, 0, -82.7485), 2.8615},
		{lab(50, 2.8361, -74.0200), lab(50, 0, -82.7485), 3.4412},
		{lab(50, -1.3802, -84.2814), lab(50, 0, -82.7485), 1.0000},
		{lab(50, -1.1848, -84.8006), lab(50, 0, -82.7485), 1.0000},
		{lab(50, -0.9009, -85.5211), lab(50, 0, -82.7485), 1.0000},
		{lab(50, 0, 0), lab(50, -1, 2), 2.3669},
		{lab(50, -1, 2), lab(50, 0, 0), 2.3669},
		{lab(50, 2.4900, -0.0010), lab(50, -2.4900, 0.0009), 7.1792},
		{lab(50, 2.4900, -0.0010), lab(50, -2.4900, 0.0010), 7.1792},
		{lab(50, 2.4900, -0.0010), lab(50, -2.4900, 0.0011), 7.2195},
		{lab(50, 2.4900, -0.0010), lab(50, -2.4900, 0.0012), 7.2195},
		{lab(50, -0.0010, 2.4900), lab(50, 0.0009, -2.4900), 4.8045},
		{lab(50, -0.0010, 2.4900), lab(50, 0.0010, -2.4900), 4.8045},
		{lab(50, -0.0010, 2.4900), lab(50, 0.0011, -2.4900), 4.7461},
		{lab(50, 2.5, 0), lab(50, 0, -2.5), 4.3065},
		{lab(50, 2.5, 0), lab(73, 25, -18), 27.1492},
		{lab(50, 2.5, 0), lab(61, -5, 29), 22.8977},
		{lab(50, 2.5, 0), lab(56, -27, -3), 31.9030},
		{lab(50, 2.5, 0), lab(58, 24, 15), 19.4535},
		{lab(50, 2.5, 0), lab(50, 3.1736, 0.5854), 1.0000},
		{lab(50, 2.5, 0), lab(50, 3.2972, 0), 1.0000},
		{lab(50, 2.5, 0), lab(50, 1.8634, 0.5757), 1.0000},
		{lab(50, 2.5, 0), lab(50, 3.2592, 0.3350), 1.0000},
		{lab(60.2574, -34.0099, 36.2677), lab(60.4626, -34.1751, 39.4387), 1.2644},
		{lab(63.0109, -31.0961, -5.8663), lab(62.8187, -29.7946, -4.0864), 1.2630},
		{lab(61.2901, 3.7196, -5.3901), lab(61.4292, 2.2480, -4.9620), 1.8731},
		{lab(35.0831, -44.1164, 3.7933), lab(35.0232, -40.0716, 1.5901), 1.8645},
		{lab(22.7233, 20.0904, -46.6940), lab(23.0331, 14.9730, -42.5619), 2.0373},
		{lab(36.4612, 47.8580, 18.3852), lab(36.2715, 50.5065, 21.2231), 1.4146},
		{lab(90.8027, -2.0831, 1.4410), lab(91.1528, -1.6435, 0.0447), 1.4441},
		{lab(90.9257, -0.5406, -0.9208), lab(88.6381, -0.8985, -0.7239), 1.5381},
		{lab(6.7747, -0.2908, -2.4247), lab(5.8714, -0.0985, -2.2286), 0.6377},
		{lab(2.0776, 0.0795, -1.1350), lab(0.9033, -0.0636, -0.5514), 0.9082},
	}
	for i, pair := range pairs {
		if got := appearance.DeltaE2000(pair.first, pair.second); math.Abs(got-pair.want) > 0.00005 {
			t.Errorf("pair %d: ΔE00 = %.4f, want %.4f", i+1, got, pair.want)
		}
	}
}

func lab(l, a, b float64) appearance.Lab {
	return appearance.Lab{L: l, A: a, B: b}
}

// The fill is only replaced when it cannot be seen over the text: the
// karaoke yellow over the high-contrast yellow (the user's profile, 7.6
// apart) gives way, over white or the film's warm white it stays.
func TestReadableFillReplacesOnlyAFillTheTextHides(t *testing.T) {
	t.Parallel()
	karaokeYellow := appearance.RGB(0xFF, 0xD2, 0x3F)
	cases := []struct {
		name    string
		text    appearance.Color
		fill    appearance.Color
		changed bool
	}{
		{"high-contrast yellow", appearance.RGB(0xFF, 0xE6, 0x00), karaokeYellow, true},
		{"pure yellow", appearance.RGB(0xFF, 0xFF, 0x00), karaokeYellow, true},
		{"the same color", karaokeYellow, karaokeYellow, true},
		{"white", appearance.RGB(0xFF, 0xFF, 0xFF), karaokeYellow, false},
		{"film's warm white", appearance.RGB(0xF5, 0xF0, 0xE1), karaokeYellow, false},
		{"orange over white", appearance.RGB(0xFF, 0xFF, 0xFF), appearance.RGB(0xFF, 0x8C, 0x00), false},
	}
	for _, c := range cases {
		used, changed := appearance.ReadableFill(c.text, c.fill)
		if changed != c.changed {
			t.Errorf("%s: changed = %v, want %v", c.name, changed, c.changed)
		}
		if !changed && used != c.fill {
			t.Errorf("%s: kept fill = %s, want %s", c.name, used, c.fill)
		}
		distance := appearance.DeltaE2000(c.text.Lab(), used.Lab())
		if distance < appearance.MinFillContrast {
			t.Errorf("%s: the fill used, %s, is only %.1f from the text", c.name, used, distance)
		}
	}
}
