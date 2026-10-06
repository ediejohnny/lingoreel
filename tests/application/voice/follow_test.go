package voice_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// film is 6 s of a quiet room where someone says "Hi" (0.3-0.8 s), then
// "How..." (2.0-2.25 s) and, after two seconds, "are you?" (4.25-4.75 s).
func film() []float32 {
	return testkit.Mix(
		testkit.Noise(6*time.Second, 100, 7000, -65, 30),
		testkit.Then(testkit.Silence(300*ms), testkit.Voiced(500*ms, -20, 31), testkit.Silence(1200*ms),
			testkit.Voiced(250*ms, -20, 32), testkit.Silence(2*time.Second), testkit.Voiced(500*ms, -20, 33)),
	)
}

var howAreYou = voice.Line{Text: "How... are you?", Start: 1900 * ms, End: 4900 * ms}

// clock plays the film from start at normal speed.
type clock struct{ start time.Time }

func (c clock) PositionAt(at time.Time) time.Duration { return at.Sub(c.start) }

func (c clock) State() playback.Sample {
	return playback.Sample{Status: playback.StatusPlaying, Speed: playback.NormalSpeed}
}

// tap hands over the packets a test feeds, one at a time.
type tap struct {
	packets chan voice.Packet
	heard   chan struct{}
}

func newTap() *tap { return &tap{packets: make(chan voice.Packet), heard: make(chan struct{})} }

func (t *tap) Listen(ctx context.Context, _ int, hear func(voice.Packet)) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case packet := <-t.packets:
			hear(packet)
			t.heard <- struct{}{}
		}
	}
}

// play feeds the film's stereo sound from one position to another, in 10 ms
// packets stamped by the clock.
func (t *tap) play(start time.Time, sound []float32, from, to time.Duration) {
	for at := from; at < to; at += 10 * ms {
		i := int(at.Seconds() * 16000)
		packet := voice.Packet{Channels: 2, At: start.Add(at)}
		for _, s := range sound[i : i+160] {
			packet.Samples = append(packet.Samples, s, s)
		}
		t.packets <- packet
		<-t.heard
	}
}

// decoder decodes the film, or fails.
type decoder struct {
	sound []float32
	fail  bool
}

func (d decoder) Decode(_ context.Context, stretch voice.Stretch, hear func([]float32, int)) error {
	if d.fail {
		return errors.New("broken")
	}
	from := min(int(stretch.From.Seconds()*16000), len(d.sound))
	to := min(from+int(stretch.Length.Seconds()*16000), len(d.sound))
	for at := from; at < to; at += 1600 {
		hear(d.sound[at:min(at+1600, to)], 1)
	}
	return nil
}

func fill(t *testing.T, follower *voice.Follower, line voice.Line, at time.Duration) float64 {
	t.Helper()
	share, ok := follower.Fill(line, at)
	if !ok {
		t.Fatalf("at %v the line is painted by the estimate (mode %v)", at, follower.Mode())
	}
	return share * 13 // characters of "How... are you?"
}

func within(t *testing.T, what string, got, low, high float64) {
	t.Helper()
	if got < low-0.01 || got > high+0.01 {
		t.Fatalf("%s: %.2f characters painted, want %.1f..%.1f", what, got, low, high)
	}
}

// Heard as it plays, the line follows the voice and holds in the pause;
// trained again, it is painted from the sound heard the first time.
func TestTheVoiceIsFollowedLiveAndKeptForTrainingAgain(t *testing.T) {
	t.Parallel()
	sound, player, start := film(), newTap(), time.Now()
	follower := voice.Follow(context.Background(), voice.Sources{Tap: player, PID: 1}, clock{start}, voice.Settings{Sensitivity: speech.SensitivityNormal})
	defer follower.Close()

	player.play(start, sound, 0, 1900*ms)
	if follower.Mode() != voice.ModeLive {
		t.Fatalf("mode = %v, want live", follower.Mode())
	}
	fill(t, follower, howAreYou, 1900*ms) // the line shows
	player.play(start, sound, 1900*ms, 2300*ms)
	said := fill(t, follower, howAreYou, 2300*ms)
	within(t, `"How..." said`, said, 3, 6)
	player.play(start, sound, 2300*ms, 4200*ms)
	within(t, "in the pause", fill(t, follower, howAreYou, 4200*ms), said, said)
	player.play(start, sound, 4200*ms, 4900*ms)
	within(t, `"are you?" said`, fill(t, follower, howAreYou, 4890*ms), 11.5, 13)

	follower.Seeked() // "Treinar de novo"
	within(t, "trained again, in the pause", fill(t, follower, howAreYou, 3*time.Second), 5.5, 6)
	within(t, `trained again, during "are you?"`, fill(t, follower, howAreYou, 4500*ms), 7, 11)
}

// Decoded ahead, a line is painted along its voice before it is played;
// a song is left to the estimate.
func TestTheVoiceIsDecodedAhead(t *testing.T) {
	t.Parallel()
	follower := voice.Follow(context.Background(), voice.Sources{Decoder: decoder{sound: film()}, Voice: voice.Stretch{File: "film.mkv"}},
		clock{time.Now()}, voice.Settings{Sensitivity: speech.SensitivityNormal})
	defer follower.Close()
	testkit.Eventually(t, "the line decoded", func() bool {
		_, ok := follower.Fill(howAreYou, 0)
		return ok
	})
	if follower.Mode() != voice.ModeAhead {
		t.Fatalf("mode = %v, want ahead", follower.Mode())
	}
	within(t, "in the pause", fill(t, follower, howAreYou, 3*time.Second), 5.5, 6)
	within(t, "after the line", fill(t, follower, howAreYou, 4900*ms), 13, 13)
	if _, ok := follower.Fill(voice.Line{Text: "♪ How are you ♪", Start: 1900 * ms, End: 4900 * ms}, 3*time.Second); ok {
		t.Fatal("a song followed the voice")
	}
}

// An FFmpeg that keeps failing, with no player to hear, leaves the estimate.
func TestABrokenDecoderFallsBackToTheEstimate(t *testing.T) {
	t.Parallel()
	follower := voice.Follow(context.Background(), voice.Sources{Decoder: decoder{fail: true}, Voice: voice.Stretch{File: "film.mkv"}},
		clock{time.Now()}, voice.Settings{Sensitivity: speech.SensitivityNormal})
	defer follower.Close()
	testkit.Eventually(t, "the estimate", func() bool { return follower.Mode() == voice.ModeEstimate })
	if _, ok := follower.Fill(howAreYou, 3*time.Second); ok {
		t.Fatal("followed a voice nobody could decode")
	}
}
