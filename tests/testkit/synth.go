package testkit

import (
	"math"
	"math/rand/v2"
	"time"
)

// Sounds for the speech tests, at 16 kHz, built from sines so each test
// knows exactly where its voice is. Levels are RMS in dB of full scale.
const synthRate = 16000

func synthSamples(d time.Duration) int { return int(d.Seconds() * synthRate) }

// scaled sets a sound's RMS to a level.
func scaled(sound []float32, level float64) []float32 {
	sum := 0.0
	for _, s := range sound {
		sum += float64(s) * float64(s)
	}
	if sum == 0 {
		return sound
	}
	gain := math.Pow(10, level/20) / math.Sqrt(sum/float64(len(sound)))
	for i := range sound {
		sound[i] = float32(float64(sound[i]) * gain)
	}
	return sound
}

// sines sums sines of the given frequencies, at random phases from seed,
// each weighed by amplitude(frequency).
func sines(d time.Duration, frequencies []float64, amplitude func(float64) float64, seed uint64) []float32 {
	random := rand.New(rand.NewPCG(seed, 1))
	phases := make([]float64, len(frequencies))
	for i := range phases {
		phases[i] = random.Float64() * 2 * math.Pi
	}
	sound := make([]float32, synthSamples(d))
	for n := range sound {
		t := float64(n) / synthRate
		sum := 0.0
		for i, f := range frequencies {
			sum += amplitude(f) * math.Sin(2*math.Pi*f*t+phases[i])
		}
		sound[n] = float32(sum)
	}
	return sound
}

// noiseBetween is noise limited to a range of frequencies: many sines at
// random frequencies inside it.
func noiseBetween(d time.Duration, from, to float64, seed uint64) []float32 {
	random := rand.New(rand.NewPCG(seed, 2))
	frequencies := make([]float64, 80)
	for i := range frequencies {
		frequencies[i] = from + random.Float64()*(to-from)
	}
	return sines(d, frequencies, func(float64) float64 { return 1 }, seed)
}

// syllables shapes a sound into syllables of 250 ms, each rising from
// silence and falling back to it, as loudness does in speech.
func syllables(sound []float32) []float32 {
	per := synthSamples(250 * time.Millisecond)
	for n := range sound {
		s := math.Sin(math.Pi * float64(n%per) / float64(per))
		sound[n] *= float32(s * s)
	}
	return sound
}

// Voiced is someone speaking aloud: the harmonics of a 140 Hz voice up to
// 3.4 kHz, in syllables of 250 ms.
func Voiced(d time.Duration, level float64, seed uint64) []float32 {
	var harmonics []float64
	for f := 140.0; f <= 3400; f += 140 {
		harmonics = append(harmonics, f)
	}
	return scaled(syllables(sines(d, harmonics, func(f float64) float64 { return 140 / f }, seed)), level)
}

// Whispered is someone whispering: breath noise from 500 Hz to 6 kHz, in
// syllables, with no voice under it.
func Whispered(d time.Duration, level float64, seed uint64) []float32 {
	return scaled(syllables(noiseBetween(d, 500, 6000, seed)), level)
}

// Noise is a steady sound between two frequencies: room tone, rumble.
func Noise(d time.Duration, from, to, level float64, seed uint64) []float32 {
	return scaled(noiseBetween(d, from, to, seed), level)
}

// Chord is a held chord from a root note, like music under a scene.
func Chord(d time.Duration, root, level float64, seed uint64) []float32 {
	notes := []float64{root, root * 1.26, root * 1.5, root * 2, root * 2.52, root * 3}
	return scaled(sines(d, notes, func(float64) float64 { return 1 }, seed), level)
}

// Silence is digital silence.
func Silence(d time.Duration) []float32 { return make([]float32, synthSamples(d)) }

// Then joins sounds one after another.
func Then(sounds ...[]float32) []float32 {
	var out []float32
	for _, s := range sounds {
		out = append(out, s...)
	}
	return out
}

// Mix plays sounds together from the start; the result is as long as the
// longest.
func Mix(sounds ...[]float32) []float32 {
	var out []float32
	for _, s := range sounds {
		if len(s) > len(out) {
			out = append(out, make([]float32, len(s)-len(out))...)
		}
		for i, v := range s {
			out[i] += v
		}
	}
	return out
}

// Stereo interleaves a sound mixed to the middle with one on each side.
func Stereo(center, left, right []float32) []float32 {
	n := max(len(center), len(left), len(right))
	at := func(s []float32, i int) float32 {
		if i < len(s) {
			return s[i]
		}
		return 0
	}
	out := make([]float32, 0, 2*n)
	for i := range n {
		out = append(out, at(center, i)+at(left, i), at(center, i)+at(right, i))
	}
	return out
}
