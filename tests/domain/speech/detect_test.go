package speech_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

const ms = time.Millisecond

// part is a stretch of sound the model gave one speech to.
type part struct {
	length time.Duration
	speech float32
}

// heard is the track of a sound made of parts, from the film's start.
func heard(parts ...part) speech.Track {
	var track speech.Track
	for _, p := range parts {
		for range p.length / speech.FrameLength {
			track.Frames = append(track.Frames, speech.Frame{Speech: p.speech})
		}
	}
	return track
}

// Speech as the model gives it: sure, doubtful, and none.
const (
	voice  = 0.9
	unsure = 0.42
	quiet  = 0.03
)

func expectSpans(t *testing.T, got speech.Detection, want ...speech.Span) {
	t.Helper()
	if got.Verdict != speech.Trusted {
		t.Fatalf("verdict = %v, want trusted", got.Verdict)
	}
	if len(got.Spans) != len(want) {
		t.Fatalf("spans = %v, want %v", got.Spans, want)
	}
	for i := range want {
		if got.Spans[i] != want[i] {
			t.Fatalf("spans = %v, want %v", got.Spans, want)
		}
	}
}

// "How... (2 s)... are you?": the voice is found in two pieces, so the fill
// can hold on "How" through the pause; a click in the pause is no voice,
// and the stop inside "are you" does not split it.
func TestAPauseInTheLineSplitsTheVoice(t *testing.T) {
	t.Parallel()
	track := heard(
		part{time.Second, quiet}, part{250 * ms, voice}, // How...
		part{time.Second, quiet}, part{40 * ms, voice}, part{960 * ms, quiet}, // a click
		part{200 * ms, voice}, part{80 * ms, quiet}, part{220 * ms, voice}, // are you?
		part{750 * ms, quiet},
	)
	got := speech.DetectLine(track, 900*ms, 3900*ms, 3.9, speech.SensitivityNormal)
	expectSpans(t, got, speech.Span{Start: time.Second, End: 1250 * ms}, speech.Span{Start: 3250 * ms, End: 3750 * ms})
}

// startsNear fails when the voice found does not begin near a time.
func startsNear(t *testing.T, got speech.Detection, want time.Duration) {
	t.Helper()
	if got.Verdict != speech.Trusted {
		t.Fatalf("verdict = %v, want trusted", got.Verdict)
	}
	if start := got.Spans[0].Start; (start - want).Abs() > 20*ms {
		t.Fatalf("the voice begins at %v, want about %v (spans %v)", start, want, got.Spans)
	}
}

// Where a line's own voice begins, as marked by ear: the line before often
// runs up to this one, and the model's speech rises a little after the
// voice does.
func TestTheLinesOwnVoiceBeginsWhereItIsSaid(t *testing.T) {
	t.Parallel()
	line := func(parts ...part) speech.Detection {
		return speech.DetectLine(heard(parts...), time.Second, 2500*ms, 6, speech.SensitivityNormal)
	}
	// The line before is said up to this one, with only a dip between them.
	startsNear(t, line(part{500 * ms, quiet}, part{600 * ms, voice}, part{30 * ms, 0.3}, part{20 * ms, 0.1},
		part{30 * ms, 0.3}, part{1400 * ms, voice}, part{time.Second, quiet}), 1150*ms)
	// It ends just after this one shows, before a pause.
	startsNear(t, line(part{500 * ms, quiet}, part{600 * ms, voice}, part{250 * ms, quiet},
		part{1200 * ms, voice}, part{time.Second, quiet}), 1350*ms)
	// Someone is speaking all along: the voice begins with the line.
	startsNear(t, line(part{400 * ms, quiet}, part{2200 * ms, voice}, part{time.Second, quiet}), time.Second)
	// The speech rises over 60 ms before the model is sure.
	startsNear(t, line(part{1100 * ms, quiet}, part{60 * ms, 0.3}, part{1300 * ms, voice}, part{time.Second, quiet}), 1100*ms)
}

// A word the model doubts halfway through stays whole: once the voice
// began, it ends only well under where it begins.
func TestAWordTheModelDoubtsStaysWhole(t *testing.T) {
	t.Parallel()
	word := func(middle float32) speech.Track {
		return heard(part{time.Second, quiet}, part{200 * ms, voice}, part{200 * ms, middle}, part{200 * ms, voice}, part{time.Second, quiet})
	}
	expectSpans(t, speech.DetectLine(word(unsure), 900*ms, 1700*ms, 2, speech.SensitivityNormal),
		speech.Span{Start: time.Second, End: 1600 * ms})
	expectSpans(t, speech.DetectLine(word(quiet), 900*ms, 1700*ms, 2, speech.SensitivityNormal),
		speech.Span{Start: time.Second, End: 1200 * ms}, speech.Span{Start: 1400 * ms, End: 1600 * ms})
}

// The sensitivity sets how sure the model must be: a doubtful voice counts
// only when it is high, a fairly sure one is left out when it is low.
func TestTheSensitivitySetsHowSureTheModelMustBe(t *testing.T) {
	t.Parallel()
	line := func(level float32) speech.Track {
		return heard(part{time.Second, quiet}, part{500 * ms, level}, part{time.Second, quiet})
	}
	cases := []struct {
		level       float32
		sensitivity speech.Sensitivity
		trusted     bool
	}{
		{unsure, speech.SensitivityHigh, true},
		{unsure, speech.SensitivityNormal, false},
		{0.6, speech.SensitivityNormal, true},
		{0.6, speech.SensitivityLow, false},
	}
	for _, c := range cases {
		got := speech.DetectLine(line(c.level), 900*ms, 1600*ms, 2.5, c.sensitivity)
		if (got.Verdict == speech.Trusted) != c.trusted {
			t.Errorf("speech %.2f at %s sensitivity: verdict %v", c.level, c.sensitivity, got.Verdict)
		}
	}
}

// A line whose voice cannot be told keeps the estimate, and the verdict
// says why.
func TestDoubtfulLinesAreNotTrusted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		track      speech.Track
		start, end time.Duration
		syllables  float64
		want       speech.Verdict
	}{
		{"no voice at all", heard(part{3 * time.Second, quiet}), time.Second, 2 * time.Second, 4, speech.NoVoice},
		{"talk with no silence around it", heard(part{4 * time.Second, voice}), time.Second, 3 * time.Second, 8, speech.Steady},
		{"voice only before the line", heard(part{600 * ms, quiet}, part{250 * ms, voice}, part{2 * time.Second, quiet}),
			time.Second, 2 * time.Second, 4, speech.NoVoice},
		{"far too many syllables", heard(part{time.Second, quiet}, part{500 * ms, voice}, part{time.Second, quiet}),
			900 * ms, 1600 * ms, 20, speech.WrongPace},
		{"far too few syllables", heard(part{time.Second, quiet}, part{2 * time.Second, voice}, part{time.Second, quiet}),
			900 * ms, 3100 * ms, 1, speech.WrongPace},
		{"the sound ends before the line", heard(part{2 * time.Second, quiet}), 1500 * ms, 2500 * ms, 4, speech.Missing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := speech.DetectLine(c.track, c.start, c.end, c.syllables, speech.SensitivityNormal)
			if got.Verdict != c.want {
				t.Fatalf("verdict = %v, want %v", got.Verdict, c.want)
			}
		})
	}
}
