package speech_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// Heard as it plays, the voice is told only after a few frames, from the
// start and again after a seek; a word the model doubts halfway through
// stays voice, as in DetectLine.
func TestTheListenerWarmsUpAndForgetsOnASeek(t *testing.T) {
	t.Parallel()
	listener := speech.NewListener(speech.SensitivityNormal)
	hear := func(speeches ...float32) (voices []bool, sure bool) {
		for _, s := range speeches {
			var voice bool
			voice, sure = listener.Hear(speech.Frame{Speech: s})
			voices = append(voices, voice)
		}
		return voices, sure
	}
	repeat := func(s float32, n int) []float32 {
		out := make([]float32, n)
		for i := range out {
			out[i] = s
		}
		return out
	}

	if _, sure := hear(repeat(voice, 10)...); sure {
		t.Fatal("sure within the first 100 ms")
	}
	voices, sure := hear(voice, unsure, quiet, unsure)
	if !sure || !voices[0] || !voices[1] || voices[2] || voices[3] {
		t.Fatalf("voice %v, sure %v; want voice held while doubtful, then gone", voices, sure)
	}

	hear(voice)
	listener.Reset() // a seek
	if _, sure := hear(repeat(unsure, 10)...); sure {
		t.Fatal("sure within 100 ms of a seek")
	}
	if voices, sure := hear(unsure); !sure || voices[0] {
		t.Fatalf("voice %v, sure %v; want no voice carried over the seek", voices, sure)
	}
}
