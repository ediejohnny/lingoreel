package speech_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The karaoke mode's line comes after a long silence, and training it
// again seeks to it: the Listener still hears it from its first syllable,
// measured against the voice heard before.
func TestTheListenerHearsALineAfterASilenceAndASeek(t *testing.T) {
	t.Parallel()
	sound := testkit.Mix(
		testkit.Noise(9*time.Second, 100, 7000, -65, 20),
		testkit.Then(testkit.Silence(500*ms), testkit.Voiced(time.Second, -20, 21),
			testkit.Silence(5500*ms), testkit.Voiced(time.Second, -20, 22)),
	)
	frames := analyze(t, 1, sound).Frames
	listener := speech.NewListener(speech.SensitivityNormal)
	var first, last time.Duration
	for i, frame := range frames {
		at := time.Duration(i) * speech.FrameLength
		if at == 4*time.Second {
			listener.Reset() // a seek
		}
		voice, sure := listener.Hear(frame)
		switch {
		case at >= 7*time.Second && !sure:
			t.Fatalf("at %v the listener is not sure", at)
		case voice && at > 1600*ms && at < 6900*ms:
			t.Fatalf("voice heard at %v, in the silence", at)
		case voice && at >= 6900*ms && first == 0:
			first = at
		case voice:
			last = at
		}
	}
	if first < 6980*ms || first > 7040*ms || last < 7900*ms || last > 8030*ms {
		t.Fatalf("the line was heard from %v to %v, want about 7s to 8s", first, last)
	}
}

// A steady sound never tells voice from silence.
func TestTheListenerIsNotSureOfASteadySound(t *testing.T) {
	t.Parallel()
	listener := speech.NewListener(speech.SensitivityNormal)
	for _, frame := range analyze(t, 1, testkit.Noise(5*time.Second, 100, 7000, -40, 23)).Frames {
		if _, sure := listener.Hear(frame); sure {
			t.Fatal("sure of a steady noise")
		}
	}
}
