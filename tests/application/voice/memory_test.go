package voice_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

const ms = time.Millisecond

// hear adds length of sound from a position in packets of 160 ms, each
// placed by a clock that wavers by up to 20 ms; every frame's Speech is its
// own position in seconds, so a test can tell which frames came back.
func hear(memory *voice.Memory, from, length time.Duration) {
	waver := []time.Duration{0, 20 * ms, -15 * ms, 5 * ms}
	for at, i := from, 0; at < from+length; at, i = at+160*ms, i+1 {
		frames := make([]speech.Frame, 16)
		for j := range frames {
			frames[j].Speech = float32((at + time.Duration(j)*speech.FrameLength).Seconds())
		}
		memory.Add(at+waver[i%len(waver)], frames)
	}
}

// A line trained again comes back with the sound around it; what was heard
// after a seek does not mix with what came before; old sound is forgotten.
func TestTheMemoryKeepsTheRecentSoundByTheFilmsClock(t *testing.T) {
	t.Parallel()
	memory := voice.NewMemory(5 * time.Second)
	hear(memory, 10*time.Second, 3200*ms)

	track, ok := memory.Track(11*time.Second, 12*time.Second)
	if !ok || track.Start != 10500*ms || len(track.Frames) != 200 {
		t.Fatalf("line 11s-12s: ok=%v from %v, %d frames; want from 10.5s, 200 frames", ok, track.Start, len(track.Frames))
	}
	if got := track.Frames[50].Speech; got < 10.999 || got > 11.001 {
		t.Fatalf("the frame at 11s holds the sound of %.3fs", got)
	}

	hear(memory, 50*time.Second, time.Second) // a seek
	if _, ok := memory.Track(12800*ms, 13500*ms); ok {
		t.Fatal("a line past the sound heard before the seek came back")
	}
	if _, ok := memory.Track(50100*ms, 50500*ms); !ok {
		t.Fatal("the sound after the seek is missing")
	}

	hear(memory, 51*time.Second, 2*time.Second) // 1 s of the first run left
	if _, ok := memory.Track(10500*ms, 11*time.Second); ok {
		t.Fatal("sound older than the memory's length came back")
	}
	if _, ok := memory.Track(12500*ms, 13*time.Second); !ok {
		t.Fatal("the newest second of the first run was forgotten")
	}
}
