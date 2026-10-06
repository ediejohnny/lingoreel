package appearance

import "math"

// Lab is a color in CIELAB under the D65 white, the space color differences
// are measured in.
type Lab struct {
	L, A, B float64
}

// D65 reference white, as sRGB defines it.
const (
	whiteX = 0.95047
	whiteY = 1.0
	whiteZ = 1.08883
)

// Lab converts the color, alpha left out, from sRGB to CIELAB.
func (c Color) Lab() Lab {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	x := 0.4124564*r + 0.3575761*g + 0.1804375*b
	y := 0.2126729*r + 0.7151522*g + 0.0721750*b
	z := 0.0193339*r + 0.1191920*g + 0.9503041*b
	fx, fy, fz := labF(x/whiteX), labF(y/whiteY), labF(z/whiteZ)
	return Lab{L: 116*fy - 16, A: 500 * (fx - fy), B: 200 * (fy - fz)}
}

func linear(channel uint8) float64 {
	v := float64(channel) / 255
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func labF(t float64) float64 {
	const delta = 6.0 / 29
	if t > delta*delta*delta {
		return math.Cbrt(t)
	}
	return t/(3*delta*delta) + 4.0/29
}

// DeltaE2000 is how different two colors look (CIEDE2000, with the
// parametric factors at 1): about 1 is the smallest difference an eye
// notices side by side, and the karaoke yellow over the high-contrast yellow
// measures 7.6.
func DeltaE2000(first, second Lab) float64 {
	c1, c2 := math.Hypot(first.A, first.B), math.Hypot(second.A, second.B)
	meanC := (c1 + c2) / 2
	g := 0.5 * (1 - math.Sqrt(pow7(meanC)/(pow7(meanC)+pow7(25))))
	a1, a2 := (1+g)*first.A, (1+g)*second.A
	c1p, c2p := math.Hypot(a1, first.B), math.Hypot(a2, second.B)
	h1p, h2p := hueDegrees(first.B, a1), hueDegrees(second.B, a2)

	deltaL := second.L - first.L
	deltaC := c2p - c1p
	var deltaH float64
	if c1p*c2p != 0 {
		d := h2p - h1p
		switch {
		case d > 180:
			d -= 360
		case d < -180:
			d += 360
		}
		deltaH = 2 * math.Sqrt(c1p*c2p) * math.Sin(radians(d/2))
	}

	meanL := (first.L + second.L) / 2
	meanCp := (c1p + c2p) / 2
	meanH := h1p + h2p
	if c1p*c2p != 0 {
		switch {
		case math.Abs(h1p-h2p) <= 180:
			meanH /= 2
		case h1p+h2p < 360:
			meanH = (meanH + 360) / 2
		default:
			meanH = (meanH - 360) / 2
		}
	}
	t := 1 - 0.17*math.Cos(radians(meanH-30)) + 0.24*math.Cos(radians(2*meanH)) +
		0.32*math.Cos(radians(3*meanH+6)) - 0.20*math.Cos(radians(4*meanH-63))
	rotation := 30 * math.Exp(-math.Pow((meanH-275)/25, 2))
	rc := 2 * math.Sqrt(pow7(meanCp)/(pow7(meanCp)+pow7(25)))
	lightness := (meanL - 50) * (meanL - 50)
	sl := 1 + 0.015*lightness/math.Sqrt(20+lightness)
	sc := 1 + 0.045*meanCp
	sh := 1 + 0.015*meanCp*t
	rt := -math.Sin(radians(2*rotation)) * rc

	l, c, h := deltaL/sl, deltaC/sc, deltaH/sh
	return math.Sqrt(l*l + c*c + h*h + rt*c*h)
}

func pow7(v float64) float64 {
	v2 := v * v
	return v2 * v2 * v2 * v
}

func hueDegrees(b, a float64) float64 {
	if a == 0 && b == 0 {
		return 0
	}
	h := math.Atan2(b, a) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return h
}

func radians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

// MinFillContrast is the least difference (CIEDE2000) between the text and
// the karaoke mode's fill for the spoken part to show: the karaoke yellow
// over the high-contrast yellow measures 7.6 and cannot be seen, over white
// it measures 29.1.
const MinFillContrast = 15

// fillStandIns are the colors a fill too close to its text gives way to, in
// order of preference: cyan first, the classic karaoke contrast with yellow
// and white.
var fillStandIns = []Color{
	RGB(0x00, 0xE5, 0xFF),
	RGB(0xFF, 0x5C, 0xA8),
	RGB(0xFF, 0x8C, 0x00),
	RGB(0x7C, 0xFF, 0x6B),
}

// ReadableFill returns the color the spoken part is painted in over text of
// the given color: the chosen fill when the two differ enough, otherwise the
// first stand-in that clearly differs from the text (or the one that differs
// most), and whether it was replaced. Turning the fill off is a setting of
// its own, so a fill that matches the text is never the way to hide it.
func ReadableFill(text, fill Color) (Color, bool) {
	textLab := text.Lab()
	if DeltaE2000(textLab, fill.Lab()) >= MinFillContrast {
		return fill, false
	}
	best, bestDistance := fillStandIns[0], -1.0
	for _, standIn := range fillStandIns {
		distance := DeltaE2000(textLab, standIn.Lab())
		if distance >= 2*MinFillContrast {
			return standIn.WithAlpha(fill.A), true
		}
		if distance > bestDistance {
			best, bestDistance = standIn, distance
		}
	}
	return best.WithAlpha(fill.A), true
}
