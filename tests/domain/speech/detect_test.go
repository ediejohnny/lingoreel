package speech_test

import (
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

const ms = time.Millisecond

func analyze(t testing.TB, channels int, sound []float32) speech.Track {
	t.Helper()
	analyzer, err := speech.NewAnalyzer(channels)
	if err != nil {
		t.Fatal(err)
	}
	return speech.Track{Frames: analyzer.Write(sound, nil)}
}

// near fails when the spans found are not the ones expected, edge by edge.
func near(t *testing.T, got []speech.Span, want []speech.Span, tolerance time.Duration) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("spans = %v, want %v", got, want)
	}
	for i := range want {
		if (got[i].Start-want[i].Start).Abs() > tolerance || (got[i].End-want[i].End).Abs() > tolerance {
			t.Fatalf("spans = %v, want %v (±%v)", got, want, tolerance)
		}
	}
}

// "How... (2 s)... are you?": the voice is found in two pieces, so the fill
// can hold on "How" through the pause.
func TestAPauseInTheLineSplitsTheVoice(t *testing.T) {
	t.Parallel()
	sound := testkit.Mix(
		testkit.Noise(4500*ms, 100, 7000, -65, 1),
		testkit.Then(testkit.Silence(time.Second), testkit.Voiced(250*ms, -20, 2),
			testkit.Silence(2*time.Second), testkit.Voiced(500*ms, -20, 3)),
	)
	got := speech.DetectLine(analyze(t, 1, sound), 900*ms, 3900*ms, 3.9, speech.SensitivityNormal)
	if got.Verdict != speech.Trusted {
		t.Fatalf("verdict = %v, want trusted", got.Verdict)
	}
	near(t, got.Spans, []speech.Span{{Start: time.Second, End: 1250 * ms}, {Start: 3250 * ms, End: 3750 * ms}}, 40*ms)
}

// A whisper over a room's rumble: the voice band drowns, the breath above
// 2 kHz still shows where the words are.
func TestAWhisperIsHeardOverRumble(t *testing.T) {
	t.Parallel()
	sound := testkit.Mix(
		testkit.Noise(3*time.Second, 150, 800, -38, 4),
		testkit.Then(testkit.Silence(time.Second), testkit.Whispered(time.Second, -40, 5)),
	)
	got := speech.DetectLine(analyze(t, 1, sound), 900*ms, 2100*ms, 4, speech.SensitivityNormal)
	if got.Verdict != speech.Trusted {
		t.Fatalf("verdict = %v, want trusted", got.Verdict)
	}
	near(t, got.Spans, []speech.Span{{Start: time.Second, End: 2 * time.Second}}, 60*ms)
}

// Sound spread wide (rain, a crowd, music with room), as loud as the
// dialogue in the middle, does not count as voice; alone it tells nothing.
func TestSoundAtTheSidesIsNotTakenForTheVoice(t *testing.T) {
	t.Parallel()
	left, right := testkit.Noise(3*time.Second, 150, 6000, -26, 7), testkit.Noise(3*time.Second, 150, 6000, -26, 8)
	voice := testkit.Then(testkit.Silence(time.Second), testkit.Voiced(500*ms, -26, 6))

	got := speech.DetectLine(analyze(t, 2, testkit.Stereo(voice, left, right)), 900*ms, 1600*ms, 2.6, speech.SensitivityNormal)
	if got.Verdict != speech.Trusted {
		t.Fatalf("verdict = %v, want trusted", got.Verdict)
	}
	// As loud as the sides, a syllable stands out only some way into its rise.
	near(t, got.Spans, []speech.Span{{Start: time.Second, End: 1500 * ms}}, 70*ms)

	alone := speech.DetectLine(analyze(t, 2, testkit.Stereo(nil, left, right)), 900*ms, 1600*ms, 2.6, speech.SensitivityNormal)
	if alone.Verdict == speech.Trusted {
		t.Fatalf("the sides alone were trusted: %v", alone.Spans)
	}
}

// A line whose voice cannot be told keeps the estimate, and the verdict
// says why.
func TestDoubtfulLinesAreNotTrusted(t *testing.T) {
	t.Parallel()
	room := func(d time.Duration) []float32 { return testkit.Noise(d, 100, 7000, -65, 9) }
	cases := []struct {
		name       string
		sound      []float32
		start, end time.Duration
		syllables  float64
		want       speech.Verdict
	}{
		{"only the room", room(3 * time.Second), time.Second, 2 * time.Second, 4, speech.TooQuiet},
		{"talk with no silence around it", testkit.Voiced(4*time.Second, -20, 10), time.Second, 3 * time.Second, 8, speech.Steady},
		{"voice only before the line", testkit.Mix(room(3*time.Second), testkit.Then(testkit.Silence(600*ms), testkit.Voiced(250*ms, -20, 11))),
			time.Second, 2 * time.Second, 4, speech.NoVoice},
		{"far too many syllables", testkit.Mix(room(3*time.Second), testkit.Then(testkit.Silence(time.Second), testkit.Voiced(500*ms, -20, 12))),
			900 * ms, 1600 * ms, 20, speech.WrongPace},
		{"far too few syllables", testkit.Mix(room(4*time.Second), testkit.Then(testkit.Silence(time.Second), testkit.Voiced(2*time.Second, -20, 13))),
			900 * ms, 3100 * ms, 1, speech.WrongPace},
		{"the sound ends before the line", room(2 * time.Second), 1500 * ms, 2500 * ms, 4, speech.Missing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := speech.DetectLine(analyze(t, 1, c.sound), c.start, c.end, c.syllables, speech.SensitivityNormal)
			if got.Verdict != c.want {
				t.Fatalf("verdict = %v, want %v", got.Verdict, c.want)
			}
		})
	}
}

// The player's sound comes in packets of any size, even one ending between
// the left and the right sample: the frames are the same.
func TestTheAnalyzerTakesTheSoundInAnyPieces(t *testing.T) {
	t.Parallel()
	sound := testkit.Stereo(testkit.Voiced(time.Second, -20, 14), testkit.Chord(time.Second, 220, -30, 15), nil)
	whole := analyze(t, 2, sound).Frames

	analyzer, err := speech.NewAnalyzer(2)
	if err != nil {
		t.Fatal(err)
	}
	var pieces []speech.Frame
	for at, i := 0, 0; at < len(sound); i++ {
		size := min([]int{1, 7, 160, 333, 2}[i%5], len(sound)-at)
		pieces = analyzer.Write(sound[at:at+size], pieces)
		at += size
	}
	if !slices.Equal(pieces, whole) {
		t.Fatalf("%d frames in pieces, %d whole, or they differ", len(pieces), len(whole))
	}
}

// What a weak PC pays to listen: 10 s of stereo.
func BenchmarkAnalyzer(b *testing.B) {
	sound := testkit.Stereo(testkit.Voiced(10*time.Second, -20, 16), testkit.Chord(10*time.Second, 220, -26, 17), nil)
	analyzer, err := speech.NewAnalyzer(2)
	if err != nil {
		b.Fatal(err)
	}
	frames := make([]speech.Frame, 0, 1000)
	for b.Loop() {
		frames = analyzer.Write(sound, frames[:0])
	}
	b.ReportMetric(10/(b.Elapsed().Seconds()/float64(b.N)), "x-realtime")
}
