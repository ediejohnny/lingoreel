package voice_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

// filmPeriod is how often the film's 6 s repeat in a long film.
const filmPeriod = 6 * time.Second

// howAreYouAt is the line in the film's repeat that starts at the given
// time.
func howAreYouAt(repeat time.Duration) voice.Line {
	line := howAreYou
	line.Start += repeat
	line.End += repeat
	return line
}

// clock plays the film at normal speed from start, until a test pauses it
// or seeks; it counts the seeks, as the session's does.
type clock struct {
	mu       sync.Mutex
	anchor   time.Time
	position time.Duration
	paused   bool
	seeks    uint64
}

func newClock(start time.Time) *clock { return &clock{anchor: start} }

func (c *clock) PositionAt(at time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		return c.position
	}
	return c.position + at.Sub(c.anchor)
}

func (c *clock) State() playback.Sample {
	c.mu.Lock()
	defer c.mu.Unlock()
	status := playback.StatusPlaying
	if c.paused {
		status = playback.StatusPaused
	}
	return playback.Sample{Status: status, Speed: playback.NormalSpeed}
}

func (c *clock) Seeks() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seeks
}

// seek jumps to a position at an instant, paused there or playing on.
func (c *clock) seek(to time.Duration, at time.Time, paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.position, c.anchor, c.paused = to, at, paused
	c.seeks++
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

// decoder decodes the film, or fails. Given a length, the film's 6 s
// repeat until it; given a pace, each 100 ms piece takes that long.
type decoder struct {
	sound  []float32
	fail   bool
	length time.Duration
	pace   time.Duration
}

func (d decoder) Decode(ctx context.Context, stretch voice.Stretch, hear func([]float32, int)) error {
	if d.fail {
		return errors.New("broken")
	}
	total := len(d.sound)
	if d.length > 0 {
		total = int(d.length.Seconds() * 16000)
	}
	from := min(int(stretch.From.Seconds()*16000), total)
	to := min(from+int(stretch.Length.Seconds()*16000), total)
	piece := make([]float32, 0, 1600)
	for at := from; at < to; at += 1600 {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(d.pace)
		piece = piece[:0]
		for i := at; i < min(at+1600, to); i++ {
			piece = append(piece, d.sound[i%len(d.sound)])
		}
		hear(piece, 1)
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
// trained again (paused, back to its start), it is painted from the sound
// heard the first time, not left whole.
func TestTheVoiceIsFollowedLiveAndKeptForTrainingAgain(t *testing.T) {
	t.Parallel()
	sound, player, start := film(), newTap(), time.Now()
	playing := newClock(start)
	follower := voice.Follow(context.Background(), voice.Sources{Model: testkit.LoudModel{}, Tap: player, PID: 1}, playing, voice.Settings{Sensitivity: speech.SensitivityNormal})
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

	// Trained again: the player is paused and taken back, and says nothing
	// until it plays again.
	playing.seek(howAreYou.Start, time.Now(), true)
	within(t, "trained again, in the pause", fill(t, follower, howAreYou, 3*time.Second), 5.5, 6)
	within(t, `trained again, during "are you?"`, fill(t, follower, howAreYou, 4500*ms), 7, 11)
}

// Decoded ahead, a line is painted along its voice before it is played;
// a song is left to the estimate.
func TestTheVoiceIsDecodedAhead(t *testing.T) {
	t.Parallel()
	follower := voice.Follow(context.Background(), voice.Sources{Model: testkit.LoudModel{}, Decoder: decoder{sound: film()}, Voice: voice.Stretch{File: "film.mkv"}},
		newClock(time.Now()), voice.Settings{Sensitivity: speech.SensitivityNormal})
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

// askedDecoder tells whether it was asked to decode.
type askedDecoder struct{ asked *atomic.Bool }

func (d askedDecoder) Decode(context.Context, voice.Stretch, func([]float32, int)) error {
	d.asked.Store(true)
	return nil
}

// Without the model (switched off, or on battery) nothing is decoded or
// heard: every line keeps the estimate.
func TestWithoutTheModelEveryLineKeepsTheEstimate(t *testing.T) {
	t.Parallel()
	var asked atomic.Bool
	sources := voice.Sources{Decoder: askedDecoder{&asked}, Voice: voice.Stretch{File: "film.mkv"}, Tap: newTap(), PID: 1}
	follower := voice.Follow(context.Background(), sources, newClock(time.Now()), voice.Settings{Sensitivity: speech.SensitivityNormal})
	if follower.Mode() != voice.ModeEstimate {
		t.Fatalf("mode = %v, want the estimate", follower.Mode())
	}
	if _, ok := follower.Fill(howAreYou, 3*time.Second); ok {
		t.Fatal("followed a voice with no model")
	}
	follower.Close()
	if asked.Load() {
		t.Fatal("decoded the film with no model")
	}
}

// An FFmpeg that keeps failing, with no player to hear, leaves the estimate.
func TestABrokenDecoderFallsBackToTheEstimate(t *testing.T) {
	t.Parallel()
	follower := voice.Follow(context.Background(), voice.Sources{Model: testkit.LoudModel{}, Decoder: decoder{fail: true}, Voice: voice.Stretch{File: "film.mkv"}},
		newClock(time.Now()), voice.Settings{Sensitivity: speech.SensitivityNormal})
	defer follower.Close()
	testkit.Eventually(t, "the estimate", func() bool { return follower.Mode() == voice.ModeEstimate })
	if _, ok := follower.Fill(howAreYou, 3*time.Second); ok {
		t.Fatal("followed a voice nobody could decode")
	}
}

// A seek while a window is decoded on a slow computer (8 s a window)
// decodes the new place at once, not after the window.
func TestASeekDuringAWindowDecodesTheNewPlaceAtOnce(t *testing.T) {
	t.Parallel()
	playing := newClock(time.Now())
	slow := decoder{sound: film(), length: 20 * time.Minute, pace: 13 * ms}
	follower := voice.Follow(context.Background(), voice.Sources{Model: testkit.LoudModel{}, Decoder: slow, Voice: voice.Stretch{File: "film.mkv"}},
		playing, voice.Settings{Sensitivity: speech.SensitivityNormal})
	defer follower.Close()

	time.Sleep(300 * ms) // the first window is being decoded
	later := 30 * filmPeriod
	playing.seek(later, time.Now(), false)
	testkit.Eventually(t, "the line after the seek decoded", func() bool {
		_, ok := follower.Fill(howAreYouAt(later), later)
		return ok
	})
}

// After the end of the film was decoded, a seek back past what is kept
// decodes again.
func TestASeekBackAfterTheEndDecodesAgain(t *testing.T) {
	t.Parallel()
	length := 40 * filmPeriod // 4 min, past the 3 min kept
	playing := newClock(time.Now())
	playing.seek(length-10*time.Second, time.Now(), false)
	follower := voice.Follow(context.Background(), voice.Sources{Model: testkit.LoudModel{}, Decoder: decoder{sound: film(), length: length}, Voice: voice.Stretch{File: "film.mkv"}},
		playing, voice.Settings{Sensitivity: speech.SensitivityNormal})
	defer follower.Close()
	end := length - filmPeriod
	testkit.Eventually(t, "the end decoded", func() bool {
		_, ok := follower.Fill(howAreYouAt(end), end)
		return ok
	})

	playing.seek(0, time.Now(), false)
	testkit.Eventually(t, "the start decoded again", func() bool {
		_, ok := follower.Fill(howAreYou, 0)
		return ok
	})
}
