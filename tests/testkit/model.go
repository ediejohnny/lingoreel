package testkit

import (
	"math"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// LoudModel stands in for the speech model where a test cares about what
// is done with the speech, not how it is told: it takes loud sound for
// speech, so the voices of synth.go are speech and its quiet rooms are not.
// The speech rises from 0 at -60 dB to 1 at -30 dB.
type LoudModel struct{}

func (LoudModel) NewStream() (speech.Stream, error) { return loudStream{}, nil }

type loudStream struct{}

func (loudStream) Speech(chunk []float32) float32 {
	if len(chunk) != speech.ChunkSamples {
		return 0
	}
	sum := 0.0
	for _, s := range chunk {
		sum += float64(s) * float64(s)
	}
	level := 10 * math.Log10(max(sum/float64(len(chunk)), 1e-12))
	return float32(min(max((level+60)/30, 0), 1))
}
