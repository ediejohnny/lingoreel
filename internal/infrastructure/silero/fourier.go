package silero

import (
	"fmt"
	"math"
	"math/bits"
)

// The model computes its spectrum as a convolution with a stored basis, 264
// thousand products a chunk. The basis is a windowed Fourier basis, so a
// fast transform gives the same magnitudes for about a twentieth of the work;
// the window is taken from the basis itself, and the rest of the basis is
// checked against it when the model is read.

// basisTolerance is how far a stored basis value may be from the windowed
// Fourier one: the rounding of a float32 below 1.
const basisTolerance = 1e-5

// fourier is the fast transform of one frame, windowed as the model does.
type fourier struct {
	window   [stftWindow]float64
	cos, sin [stftWindow / 2]float64
	reverse  [stftWindow]uint8
}

// fourierOf checks the stored basis: each real row is the window times a
// cosine of its frequency, each imaginary row the window times its sine
// (with one sign for the row). It returns the transform, or an error for a
// basis of any other kind.
func fourierOf(basis []float32) (*fourier, error) {
	f := &fourier{}
	for n := range stftWindow {
		// The first row is the window times cos(0).
		f.window[n] = float64(basis[n])
	}
	for bin := range stftBins {
		re := basis[bin*stftWindow : (bin+1)*stftWindow]
		im := basis[(bin+stftBins)*stftWindow : (bin+stftBins+1)*stftWindow]
		wave := func(n int) (float64, float64) {
			angle := 2 * math.Pi * float64(bin*n%stftWindow) / stftWindow
			return f.window[n] * math.Cos(angle), f.window[n] * math.Sin(angle)
		}
		// The row's sign, from where its sine is clearest; magnitudes do not
		// depend on it.
		sign, clearest := 1.0, 0.0
		for n := range stftWindow {
			if _, s := wave(n); math.Abs(s) > clearest {
				clearest = math.Abs(s)
				sign = math.Copysign(1, float64(im[n])*s)
			}
		}
		for n := range stftWindow {
			c, s := wave(n)
			if math.Abs(float64(re[n])-c) > basisTolerance || math.Abs(float64(im[n])-sign*s) > basisTolerance {
				return nil, fmt.Errorf("%w: the spectrum's basis is not a windowed Fourier basis (bin %d)", errMalformed, bin)
			}
		}
	}
	for k := range stftWindow / 2 {
		angle := 2 * math.Pi * float64(k) / stftWindow
		f.cos[k], f.sin[k] = math.Cos(angle), math.Sin(angle)
	}
	for n := range stftWindow {
		f.reverse[n] = bits.Reverse8(uint8(n))
	}
	return f, nil
}

// magnitudes writes the magnitude of each bin of one frame, a stftWindow
// long piece of sound, into out at out[bin*stride].
func (f *fourier) magnitudes(frame []float32, out []float32, stride int) {
	var re, im [stftWindow]float64
	for n, v := range frame[:stftWindow] {
		re[f.reverse[n]] = float64(v) * f.window[n]
	}
	for size := 2; size <= stftWindow; size *= 2 {
		half, step := size/2, stftWindow/size
		for start := 0; start < stftWindow; start += size {
			for k := range half {
				c, s := f.cos[k*step], f.sin[k*step]
				i, j := start+k, start+k+half
				// The odd half turned by e^(−iθ).
				tr := re[j]*c + im[j]*s
				ti := im[j]*c - re[j]*s
				re[j], im[j] = re[i]-tr, im[i]-ti
				re[i], im[i] = re[i]+tr, im[i]+ti
			}
		}
	}
	for bin := range stftBins {
		out[bin*stride] = float32(math.Sqrt(re[bin]*re[bin] + im[bin]*im[bin]))
	}
}
