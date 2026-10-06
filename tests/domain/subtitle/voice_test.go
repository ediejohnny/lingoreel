package subtitle_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// dialogue is a film's spoken lines: a second to three each, half a second
// to six apart, the same ones for the same seed.
func dialogue(length time.Duration, seed uint64) []speech.Span {
	random := rand.New(rand.NewPCG(seed, 3))
	var lines []speech.Span
	for at := 20 * time.Second; at < length-10*time.Second; {
		said := time.Second + time.Duration(random.Int64N(int64(2*time.Second)))
		lines = append(lines, speech.Span{Start: at, End: at + said})
		at += said + 500*time.Millisecond + time.Duration(random.Int64N(int64(5500*time.Millisecond)))
	}
	return lines
}

// heardOf is what the model hears of a dialogue in the stretches a video is
// heard: sure inside the lines, a little doubt around, now and then a sound
// taken for voice.
func heardOf(lines []speech.Span, duration time.Duration) []speech.Track {
	random := rand.New(rand.NewPCG(9, 9))
	var heard []speech.Track
	for _, stretch := range subtitle.HearingStretches(duration) {
		track := speech.Track{Start: stretch.From}
		for at := stretch.From; at < stretch.From+stretch.Length; at += speech.FrameLength {
			p := float32(0.05 + 0.2*random.Float64())
			for _, line := range lines {
				if at >= line.Start+50*time.Millisecond && at < line.End-50*time.Millisecond {
					p = 0.9
				}
			}
			track.Frames = append(track.Frames, speech.Frame{Speech: p})
		}
		heard = append(heard, track)
	}
	return heard
}

// moved is a file of the lines timed otherwise: shown at t*rate+offset in
// the video, so the file holds (t-offset)/rate.
func moved(lines []speech.Span, rate float64, offset time.Duration) []speech.Span {
	file := make([]speech.Span, len(lines))
	for i, line := range lines {
		file[i] = speech.Span{
			Start: time.Duration(float64(line.Start-offset) / rate),
			End:   time.Duration(float64(line.End-offset) / rate),
		}
	}
	return file
}

// A file timed for this film is found by its voice, however it is shifted
// or sped up for another frame rate; a file of another film is not.
func TestTheFileThatFitsIsFoundByTheVoice(t *testing.T) {
	t.Parallel()
	const film = 90 * time.Minute
	lines := dialogue(film, 1)
	heard := heardOf(lines, film)
	pal := 25 / (24000.0 / 1001)
	cases := []struct {
		name   string
		rate   float64
		offset time.Duration
	}{
		{"as it is", 1, 0},
		{"shifted", 1, 2500 * time.Millisecond},
		{"another frame rate", pal, -900 * time.Millisecond},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := subtitle.AlignToVoice(heard, moved(lines, c.rate, c.offset), film)
			if got.Verdict != subtitle.VerdictMatch || (got.Offset-c.offset).Abs() > 60*time.Millisecond || got.Rate != c.rate {
				t.Fatalf("got %+v, want a match at %v, rate %v", got, c.offset, c.rate)
			}
		})
	}
	if got := subtitle.AlignToVoice(heard, dialogue(film, 2), film); got.Verdict != subtitle.VerdictNoMatch {
		t.Fatalf("another film's file: %+v", got)
	}
}

// Too few lines heard tell nothing.
func TestAFewLinesHeardTellNothing(t *testing.T) {
	t.Parallel()
	const film = 90 * time.Minute
	lines := dialogue(film, 4)
	if got := subtitle.AlignToVoice(heardOf(lines, film), lines[:15], film); got.Verdict != subtitle.VerdictUnknown {
		t.Fatalf("15 lines: %+v", got)
	}
}
