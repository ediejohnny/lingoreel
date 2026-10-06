// Package speech finds where people speak in a film's sound: an Analyzer
// has a Model judge the samples and hands out short frames, each with how
// likely someone speaks in it, and DetectLine finds the voice inside a
// subtitle line's time and says whether it can be trusted. Nothing here
// knows where the sound comes from (a decoder ahead of the video, or the
// player heard while it plays).
package speech

import (
	"errors"
	"time"
)

var (
	// ErrChannels reports a sound that is neither mono nor stereo.
	ErrChannels = errors.New("speech: the sound must be mono or stereo")
	// ErrNoStream reports an Analyzer with nothing to judge the sound.
	ErrNoStream = errors.New("speech: no stream to judge the sound")
)

// The sound an Analyzer takes: 16 kHz, as the model hears it, is enough
// for speech (up to 8 kHz).
const (
	SampleRate  = 16000
	FrameLength = 10 * time.Millisecond

	frameSamples = SampleRate / 100
)

// Frame is the sound of one FrameLength.
type Frame struct {
	// Speech is how likely someone speaks in it, from 0 to 1.
	Speech float32
}

// Analyzer turns samples into frames as they come, in pieces of any size.
// The model judges ChunkSamples at a time, so a frame comes out once the
// chunk holding its centre is judged, with that chunk's speech.
type Analyzer struct {
	stream Stream
	stereo bool
	// left waits for its right sample when a piece ends between the two.
	left    float32
	hasLeft bool

	chunk  [ChunkSamples]float32
	filled int // samples in chunk
	judged int // samples in the chunks judged
	framed int // frames handed out
}

// NewAnalyzer takes 16 kHz float samples, interleaved when stereo, and has
// a stream of the model judge them.
func NewAnalyzer(channels int, stream Stream) (*Analyzer, error) {
	if channels != 1 && channels != 2 {
		return nil, ErrChannels
	}
	if stream == nil {
		return nil, ErrNoStream
	}
	return &Analyzer{stream: stream, stereo: channels == 2}, nil
}

// Write adds samples and appends the frames they complete.
func (a *Analyzer) Write(samples []float32, frames []Frame) []Frame {
	for _, sample := range samples {
		if a.stereo {
			if !a.hasLeft {
				a.left, a.hasLeft = sample, true
				continue
			}
			sample, a.hasLeft = (a.left+sample)/2, false
		}
		a.chunk[a.filled] = sample
		if a.filled++; a.filled == ChunkSamples {
			frames = a.judge(frames, a.judged+ChunkSamples)
		}
	}
	return frames
}

// Flush ends the sound: it judges what is left, short of a chunk, as if
// silence followed it, and appends the frames whose centre it holds.
func (a *Analyzer) Flush(frames []Frame) []Frame {
	if a.filled == 0 {
		return frames
	}
	heard := a.judged + a.filled
	clear(a.chunk[a.filled:])
	return a.judge(frames, heard)
}

// judge has the stream judge the chunk and appends the frames whose centre
// falls in it before heard.
func (a *Analyzer) judge(frames []Frame, heard int) []Frame {
	speech := a.stream.Speech(a.chunk[:])
	a.judged += ChunkSamples
	a.filled = 0
	for a.framed*frameSamples+frameSamples/2 < heard {
		frames = append(frames, Frame{Speech: speech})
		a.framed++
	}
	return frames
}
