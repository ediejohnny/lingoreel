// Package speech finds where people speak in a film's sound: an Analyzer
// turns the samples into short frames of loudness, and DetectLine finds the
// voice inside a subtitle line's time and says whether it can be trusted.
// Nothing here knows where the sound comes from (a decoder ahead of the
// video, or the player heard while it plays).
package speech

import (
	"errors"
	"math"
	"time"
)

// ErrChannels reports a sound that is neither mono nor stereo.
var ErrChannels = errors.New("speech: the sound must be mono or stereo")

// The sound an Analyzer takes: 16 kHz is enough for speech (up to 8 kHz)
// and a third of the work of 48 kHz.
const (
	SampleRate  = 16000
	FrameLength = 10 * time.Millisecond

	frameSamples = SampleRate / 100
)

// Frame is the sound of one FrameLength.
type Frame struct {
	// Voice is the loudness, in dB, of 200–3500 Hz: where speech carries.
	Voice float32
	// Hiss is the loudness, in dB, of 2–6 kHz: breath and consonants, what
	// a whisper keeps when the voice band drowns in music or rumble.
	Hiss float32
	// Center is how much of the voice band sits in the middle of a stereo
	// sound, from 0 (only at the sides, or unrelated on each side) to 1:
	// dialogue is mixed to the middle, music and effects spread wide. Always
	// 1 for mono.
	Center float32
}

// centerSmoothing averages the middle and the sides over about 20 ms.
var centerSmoothing = 1 - math.Exp(-float64(FrameLength)/float64(20*time.Millisecond))

// silentDB is the loudness of digital silence, so a log never sees zero.
const silentDB = -120

func decibels(meanSquare float64) float32 {
	return float32(max(10*math.Log10(meanSquare), silentDB))
}

// biquad is a second-order filter (RBJ's cookbook, transposed direct form).
type biquad struct{ b0, b1, b2, a1, a2, z1, z2 float64 }

const butterworthQ = math.Sqrt2 / 2

func newBiquad(highPass bool, cutoff float64) biquad {
	w := 2 * math.Pi * cutoff / SampleRate
	alpha, cos := math.Sin(w)/(2*butterworthQ), math.Cos(w)
	a0 := 1 + alpha
	q := biquad{a1: -2 * cos / a0, a2: (1 - alpha) / a0}
	if highPass {
		q.b0, q.b1 = (1+cos)/2/a0, -(1+cos)/a0
	} else {
		q.b0, q.b1 = (1-cos)/2/a0, (1-cos)/a0
	}
	q.b2 = q.b0
	return q
}

func (q *biquad) step(x float64) float64 {
	y := q.b0*x + q.z1
	q.z1 = q.b1*x - q.a1*y + q.z2
	q.z2 = q.b2*x - q.a2*y
	return y
}

// flush clears a state decayed to almost nothing: in long silence it would
// sink into subnormal numbers, which x86 computes many times slower.
func (q *biquad) flush() {
	const tiny = 1e-25
	if math.Abs(q.z1) < tiny && math.Abs(q.z2) < tiny {
		q.z1, q.z2 = 0, 0
	}
}

// band keeps one range of frequencies.
type band struct{ high, low biquad }

func newBand(from, to float64) band {
	return band{high: newBiquad(true, from), low: newBiquad(false, to)}
}

func (b *band) step(x float64) float64 { return b.low.step(b.high.step(x)) }

func (b *band) flush() { b.high.flush(); b.low.flush() }

// Analyzer turns samples into frames as they come, in pieces of any size.
type Analyzer struct {
	stereo bool
	// left waits for its right sample when a piece ends between the two.
	left    float32
	hasLeft bool

	voice, hiss, side band
	voiceSum, hissSum float64
	sideSum           float64
	count             int
	// The voice band's middle and sides over the last frames: in one frame
	// two notes a little apart still look alike on both sides.
	midEnergy, sideEnergy float64
}

// NewAnalyzer takes 16 kHz float samples, interleaved when stereo.
func NewAnalyzer(channels int) (*Analyzer, error) {
	if channels != 1 && channels != 2 {
		return nil, ErrChannels
	}
	return &Analyzer{
		stereo: channels == 2,
		voice:  newBand(200, 3500),
		hiss:   newBand(2000, 6000),
		side:   newBand(200, 3500),
	}, nil
}

// Write adds samples and appends the frames they complete.
func (a *Analyzer) Write(samples []float32, frames []Frame) []Frame {
	for _, sample := range samples {
		if !a.stereo {
			frames = a.add(float64(sample), 0, frames)
			continue
		}
		if !a.hasLeft {
			a.left, a.hasLeft = sample, true
			continue
		}
		left, right := float64(a.left), float64(sample)
		a.hasLeft = false
		frames = a.add((left+right)/2, (left-right)/2, frames)
	}
	return frames
}

// add takes one sample of the middle of the sound and one of its sides.
func (a *Analyzer) add(mid, side float64, frames []Frame) []Frame {
	voice, hiss := a.voice.step(mid), a.hiss.step(mid)
	a.voiceSum += voice * voice
	a.hissSum += hiss * hiss
	if a.stereo {
		s := a.side.step(side)
		a.sideSum += s * s
	}
	if a.count++; a.count < frameSamples {
		return frames
	}
	center := float32(1)
	a.midEnergy += (a.voiceSum - a.midEnergy) * centerSmoothing
	a.sideEnergy += (a.sideSum - a.sideEnergy) * centerSmoothing
	if total := a.midEnergy + a.sideEnergy; a.stereo && total > 0 {
		center = float32(max((a.midEnergy-a.sideEnergy)/total, 0))
	}
	frames = append(frames, Frame{
		Voice:  decibels(a.voiceSum / frameSamples),
		Hiss:   decibels(a.hissSum / frameSamples),
		Center: center,
	})
	a.voiceSum, a.hissSum, a.sideSum, a.count = 0, 0, 0, 0
	a.voice.flush()
	a.hiss.flush()
	a.side.flush()
	return frames
}
