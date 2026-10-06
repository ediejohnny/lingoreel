package silero_test

// The deterministic 16 kHz signals the reference probabilities in
// testdata/silero_golden.json were computed on, adapted from
// go-mediatoolkit's vad/internal/goldensignals (commit 25d6556,
// Copyright (c) 2026 Daniel Sullivan, MIT: testdata/LICENSE-go-mediatoolkit).
// They must stay sample for sample what the reference runtime was given.

import "math"

const (
	signalRate    = 16000
	signalSeconds = 60
)

// signal is one named test signal.
type signal struct {
	name    string
	samples []float32
}

// signals are the five reference signals, 60 s each: a noise ladder,
// speech-like pulses, the two mixed, silence and hard square edges.
func signals() []signal {
	return []signal{
		{"noise-amp-ladder", noiseAmpLadder()},
		{"speech-pulses", speechPulses()},
		{"snr-mixture", snrMixture()},
		{"silence", make([]float32, signalSeconds*signalRate)},
		{"square-edges", squareEdges()},
	}
}

// prng is splitmix64.
type prng struct{ state uint64 }

func (p *prng) next() uint64 {
	p.state += 0x9E3779B97F4A7C15
	z := p.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// uniform is in [-1, 1).
func (p *prng) uniform() float64 {
	return float64(int64(p.next())) / float64(1<<63)
}

// noiseAmpLadder is white noise whose amplitude steps every 10 s through
// 0.01, 0.03, 0.1, 0.3, 0.6 and 1.
func noiseAmpLadder() []float32 {
	amps := []float64{0.01, 0.03, 0.1, 0.3, 0.6, 1.0}
	rng := prng{state: 0x5EED_0001}
	out := make([]float32, signalSeconds*signalRate)
	seg := len(out) / len(amps)
	for i := range out {
		out[i] = float32(amps[i/seg] * rng.uniform())
	}
	return out
}

// speechGen makes speech-like sound: 1.5 s utterances and 0.7 s pauses, of
// 250 ms syllables (a 50 ms noise burst, then a voiced nucleus whose
// formants sweep) over a falling, slightly vibrato'd pitch.
type speechGen struct {
	rng   prng
	phase float64
	n     int
}

func (g *speechGen) next() float64 {
	t := float64(g.n) / signalRate
	g.n++
	ut := math.Mod(t, 2.2)
	if ut >= 1.5 {
		return 0
	}
	f0 := 130 - 25*(ut/1.5) + 6*math.Sin(2*math.Pi*5*t)
	g.phase += 2 * math.Pi * f0 / signalRate
	syl := math.Mod(ut, 0.25)
	sylIdx := int(ut / 0.25)
	if syl < 0.05 {
		env := math.Sin(math.Pi * syl / 0.05)
		return 0.15 * env * g.rng.uniform()
	}
	fr := (syl - 0.05) / 0.20
	dir := 1.0
	if sylIdx%2 == 1 {
		dir = -1
	}
	f1 := 400 + dir*250*fr
	f2 := 1800 - dir*700*fr
	v := 0.0
	for k := 1; k <= 14; k++ {
		fk := float64(k) * f0
		w := math.Exp(-(fk-f1)*(fk-f1)/(2*150*150)) +
			0.7*math.Exp(-(fk-f2)*(fk-f2)/(2*250*250))
		v += w * math.Sin(float64(k)*g.phase)
	}
	env := math.Min(1, math.Min(fr*8, (1-fr)*4+0.2))
	am := 0.85 + 0.15*math.Sin(2*math.Pi*3.3*t)
	return 0.95 * math.Tanh(0.25*env*am*v)
}

func speechPulses() []float32 {
	out := make([]float32, signalSeconds*signalRate)
	gen := speechGen{rng: prng{state: 0x5EED_0003}}
	for i := range out {
		out[i] = float32(gen.next())
	}
	return out
}

// snrMixture is the speech-like pulses over noise stepping every 10 s
// through about 20, 10, 5, 0, -5 and -10 dB.
func snrMixture() []float32 {
	snrDB := []float64{20, 10, 5, 0, -5, -10}
	const speechRMS = 0.08
	gen := speechGen{rng: prng{state: 0x5EED_0003}}
	rng := prng{state: 0x5EED_0002}
	out := make([]float32, signalSeconds*signalRate)
	seg := len(out) / len(snrDB)
	for i := range out {
		s := gen.next()
		snr := math.Pow(10, snrDB[i/seg]/20)
		a := speechRMS / snr * math.Sqrt(3)
		out[i] = float32(0.65 * (s + a*rng.uniform()))
	}
	return out
}

// squareEdges alternates 0.4 s of a 250 Hz square wave, 0.35 s of silence
// and, every fourth cycle, a ±0.5 step held 0.2 s.
func squareEdges() []float32 {
	out := make([]float32, signalSeconds*signalRate)
	const cycle = 0.4 + 0.35 + 0.2
	for i := range out {
		t := float64(i) / signalRate
		c := math.Mod(t, cycle)
		n := int(t / cycle)
		switch {
		case c < 0.4:
			if math.Mod(t*250, 1) < 0.5 {
				out[i] = 0.8
			} else {
				out[i] = -0.8
			}
		case c < 0.75:
			out[i] = 0
		case n%4 == 0:
			out[i] = 0.5
		case n%4 == 2:
			out[i] = -0.5
		}
	}
	return out
}
