package subtitle_test

import (
	"encoding/binary"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// rhythm returns n start instants of a film-like dialogue: from 30 s on,
// gaps between 0.8 s and 6 s, reproducible from the seed.
func rhythm(seed uint64, n int) []time.Duration {
	random := rand.New(rand.NewPCG(seed, seed*7+1))
	starts := make([]time.Duration, n)
	at := 30 * time.Second
	for i := range starts {
		starts[i] = at
		at += 800*time.Millisecond + time.Duration(random.Int64N(int64(5200*time.Millisecond)))
	}
	return starts
}

// mapped returns every instant moved by f.
func mapped(instants []time.Duration, f func(time.Duration) time.Duration) []time.Duration {
	moved := make([]time.Duration, len(instants))
	for i, at := range instants {
		moved[i] = f(at)
	}
	return moved
}

// worstResidual is the largest distance between an aligned candidate start
// and the reference start it came from (same index).
func worstResidual(reference, candidate []time.Duration, a subtitle.Alignment) time.Duration {
	worst := time.Duration(0)
	for i := range candidate {
		shown := time.Duration(float64(candidate[i])*a.Rate) + a.Offset
		worst = max(worst, (shown - reference[i]).Abs())
	}
	return worst
}

func expectVerdict(t *testing.T, got subtitle.Alignment, want subtitle.Verdict) {
	t.Helper()
	if got.Verdict != want {
		t.Fatalf("verdict = %s, want %s (%+v)", got.Verdict, want, got)
	}
}

func TestAlignIdenticalFile(t *testing.T) {
	reference := rhythm(1, 800)
	got := subtitle.Align(reference, reference, 0)
	expectVerdict(t, got, subtitle.VerdictMatch)
	if got.Score != 1 || got.Offset.Abs() > time.Millisecond || math.Abs(got.Rate-1) > 1e-6 {
		t.Fatalf("got %+v, want offset 0, rate 1, score 1", got)
	}
}

func TestAlignFindsConstantOffset(t *testing.T) {
	reference := rhythm(2, 800)
	for _, shift := range []time.Duration{1500 * time.Millisecond, -1500 * time.Millisecond, 45 * time.Second, -4 * time.Minute} {
		// The file shows each line shift earlier, so it needs +shift.
		candidate := mapped(reference, func(at time.Duration) time.Duration { return at - shift })
		got := subtitle.Align(reference, candidate, 0)
		expectVerdict(t, got, subtitle.VerdictMatch)
		if (got.Offset - shift).Abs() > 5*time.Millisecond {
			t.Errorf("shift %v: offset = %v", shift, got.Offset)
		}
	}
}

func TestAlignOffsetBeyondTheWindowIsNotFound(t *testing.T) {
	reference := rhythm(3, 800)
	// Six minutes is past the 5-minute search window.
	candidate := mapped(reference, func(at time.Duration) time.Duration { return at + 6*time.Minute })
	got := subtitle.Align(reference, candidate, 0)
	if got.Verdict == subtitle.VerdictMatch {
		t.Fatalf("matched a file 6 minutes off: %+v", got)
	}
}

func TestAlignCorrectsFrameRate(t *testing.T) {
	reference := rhythm(4, 800)
	// The file was timed for 25 fps and the video runs at 23.976: a line at
	// reference time r sits at (r-2s)*23.976/25 in the file, so
	// r = file*25/23.976 + 2s, rate 25*1001/24000 = 1.0427083.
	speed := (24000.0 / 1001) / 25
	candidate := mapped(reference, func(at time.Duration) time.Duration {
		return time.Duration(float64(at-2*time.Second) * speed)
	})
	got := subtitle.Align(reference, candidate, 0)
	expectVerdict(t, got, subtitle.VerdictMatch)
	if want := 25 * 1001 / 24000.0; math.Abs(got.Rate-want) > 1e-5 {
		t.Fatalf("rate = %v, want %v", got.Rate, want)
	}
	if worst := worstResidual(reference, candidate, got); worst > 10*time.Millisecond {
		t.Fatalf("worst residual %v (alignment %+v)", worst, got)
	}
}

func TestAlignRemovesSlowDrift(t *testing.T) {
	reference := rhythm(5, 1000)
	// 0.005% slower: the last line (about an hour in) is ~170 ms off with a
	// single offset; the line fit must bring every line within 10 ms.
	candidate := mapped(reference, func(at time.Duration) time.Duration { return time.Duration(float64(at) * 0.99995) })
	got := subtitle.Align(reference, candidate, 0)
	expectVerdict(t, got, subtitle.VerdictMatch)
	if worst := worstResidual(reference, candidate, got); worst > 10*time.Millisecond {
		t.Fatalf("worst residual %v (alignment %+v)", worst, got)
	}
}

func TestAlignToleratesMissingAndExtraLines(t *testing.T) {
	reference := rhythm(6, 900)
	t.Run("a third of the lines missing", func(t *testing.T) {
		var candidate []time.Duration
		for i, at := range reference {
			if i%3 != 0 {
				candidate = append(candidate, at+700*time.Millisecond)
			}
		}
		got := subtitle.Align(reference, candidate, 0)
		expectVerdict(t, got, subtitle.VerdictMatch)
		if got.Score != 1 || (got.Offset+700*time.Millisecond).Abs() > 5*time.Millisecond {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("30% extra lines (sounds, songs)", func(t *testing.T) {
		candidate := slices.Clone(reference)
		for _, at := range rhythm(99, 270) {
			candidate = append(candidate, at+1700*time.Millisecond)
		}
		// 900 of 1170 lines line up: 0.77, above the 0.6 needed.
		got := subtitle.Align(reference, candidate, 0)
		expectVerdict(t, got, subtitle.VerdictMatch)
	})
}

func TestAlignDoesNotTrustAFileThatIsHalfExtras(t *testing.T) {
	reference := rhythm(13, 600)
	candidate := slices.Clone(reference)
	for _, at := range rhythm(130, 600) {
		candidate = append(candidate, at+2300*time.Millisecond)
	}
	// Every dialogue line is there, but only half the file (0.5 < 0.6)
	// follows the video: too little to choose it without asking.
	if got := subtitle.Align(reference, candidate, 0); got.Verdict == subtitle.VerdictMatch {
		t.Fatalf("matched a file whose lines are half extras: %+v", got)
	}
}

func TestAlignJitteredTimingBeatsASharpFalsePeak(t *testing.T) {
	reference := rhythm(14, 1000)
	random := rand.New(rand.NewPCG(3, 3))
	candidate := make([]time.Duration, 0, len(reference))
	for i, at := range reference {
		if i%10 < 3 {
			// 30% of the lines sit exactly 10 s off: one sharp bucket.
			candidate = append(candidate, at-10*time.Second)
			continue
		}
		// 70% follow the video with up to ±70 ms of jitter. The 50 ms bucket
		// at zero spans 50 of those 140 ms (0.7 × 50/140 = 25% of the lines)
		// and each neighbour 45 ms (22.5%): alone, every bucket loses to the
		// sharp 30%; summed with its neighbours the true peak (70%) wins.
		candidate = append(candidate, at+time.Duration(random.Int64N(int64(140*time.Millisecond)))-70*time.Millisecond)
	}
	got := subtitle.Align(reference, candidate, 0)
	if got.Offset.Abs() > 30*time.Millisecond {
		t.Fatalf("offset %v: the sharp false peak won (%+v)", got.Offset, got)
	}
}

func TestAlignDetectsAnotherCut(t *testing.T) {
	reference := rhythm(7, 800)
	added := func(from int, scene time.Duration) []time.Duration {
		candidate := slices.Clone(reference)
		for i := from; i < len(candidate); i++ {
			candidate[i] += scene
		}
		return candidate
	}
	// A 3-minute scene added halfway: half the lines line up with each offset.
	expectVerdict(t, subtitle.Align(reference, added(400, 3*time.Minute), 0), subtitle.VerdictOtherCut)
	// Added at 80%: the score alone (0.8) would pass; the last part does not.
	expectVerdict(t, subtitle.Align(reference, added(640, 2*time.Minute), 0), subtitle.VerdictOtherCut)
}

func TestAlignFileRunningPastTheVideo(t *testing.T) {
	reference := rhythm(8, 300)
	last := reference[len(reference)-1]
	// The last line may start up to 10 s after the end (credits), not later.
	expectVerdict(t, subtitle.Align(reference, reference, last-10*time.Second), subtitle.VerdictMatch)
	expectVerdict(t, subtitle.Align(reference, reference, last-10*time.Second-time.Millisecond), subtitle.VerdictOtherCut)
	// Timed 2 minutes late: its raw end is past the video, its aligned end is not.
	late := mapped(reference, func(at time.Duration) time.Duration { return at + 2*time.Minute })
	expectVerdict(t, subtitle.Align(reference, late, last+30*time.Second), subtitle.VerdictMatch)
	// Without a reference only the length can be judged.
	expectVerdict(t, subtitle.Align(nil, reference, last-time.Minute), subtitle.VerdictOtherCut)
	expectVerdict(t, subtitle.Align(nil, reference, last+time.Minute), subtitle.VerdictUnknown)
}

// The director's cut of the real case: scenes added all along. The longest
// stretch (the last 30%) wins the alignment and pulls the file's end back
// inside the video; no quarter but the last lines up on its own. Only the
// raw length, 3.5 minutes past the end, gives it away.
func TestAlignExtendedCutWithManyAddedScenes(t *testing.T) {
	reference := rhythm(15, 800)
	candidate := slices.Clone(reference)
	for i := range candidate {
		// Lines 0-559 in seven blocks of 80, each 30 s later than the one
		// before; lines 560-799 (30%) 7 × 30 s = 3.5 min late.
		candidate[i] += time.Duration(min(i/80, 7)) * 30 * time.Second
	}
	duration := reference[len(reference)-1] + 5*time.Second
	got := subtitle.Align(reference, candidate, duration)
	expectVerdict(t, got, subtitle.VerdictOtherCut)
	if (got.Offset + 210*time.Second).Abs() > 50*time.Millisecond {
		t.Fatalf("offset %v, want the last stretch's -3m30s (the case this test is about)", got.Offset)
	}
	expectVerdict(t, subtitle.Align(reference, candidate, 0), subtitle.VerdictNoMatch)
}

func TestAlignUnrelatedFile(t *testing.T) {
	got := subtitle.Align(rhythm(9, 800), rhythm(10, 800), 0)
	expectVerdict(t, got, subtitle.VerdictNoMatch)
	if got.Score >= 0.6 {
		t.Fatalf("score %v for an unrelated file", got.Score)
	}
}

func TestAlignNeedsTwentyLines(t *testing.T) {
	reference := rhythm(11, 20)
	expectVerdict(t, subtitle.Align(reference, reference, 0), subtitle.VerdictMatch)
	expectVerdict(t, subtitle.Align(reference[:19], reference, 0), subtitle.VerdictUnknown)
	expectVerdict(t, subtitle.Align(reference, reference[:19], 0), subtitle.VerdictUnknown)
	expectVerdict(t, subtitle.Align(nil, nil, 0), subtitle.VerdictUnknown)
	if got := subtitle.Align(nil, nil, time.Hour); got.Rate != 1 || got.Offset != 0 {
		t.Fatalf("unknown alignment must be neutral: %+v", got)
	}
}

func TestAlignOrderAndJunkDoNotMatter(t *testing.T) {
	reference := rhythm(12, 500)
	candidate := mapped(reference, func(at time.Duration) time.Duration { return at - time.Second })
	want := subtitle.Align(reference, candidate, 0)

	shuffled := slices.Clone(candidate)
	rand.New(rand.NewPCG(1, 1)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	junk := append(slices.Clone(shuffled), -time.Hour, 200*time.Hour, math.MaxInt64, math.MinInt64)
	before := slices.Clone(junk)
	if got := subtitle.Align(reference, junk, 0); got != want {
		t.Fatalf("shuffled with junk: %+v, want %+v", got, want)
	}
	if !slices.Equal(junk, before) {
		t.Fatal("Align must not change its input")
	}
}

func TestAlignHostileInputsFinish(t *testing.T) {
	same := make([]time.Duration, 50_000)
	got := subtitle.Align(same, same, 0)
	if got.Score < 0 || got.Score > 1 || !(got.Rate > 0) {
		t.Fatalf("50k identical starts: %+v", got)
	}
}

func TestStartsAreSorted(t *testing.T) {
	events := []subtitle.Event{event("b", 3000, 4000), event("a", 1000, 2000), event("c", 2000, 2500)}
	if got, want := subtitle.Starts(events), []time.Duration{ms(1000), ms(2000), ms(3000)}; !slices.Equal(got, want) {
		t.Fatalf("Starts = %v, want %v", got, want)
	}
}

func TestRetimeMovesTimesOnly(t *testing.T) {
	events := []subtitle.Event{event("a", 1000, 2000), event("b", 10_000, 12_000)}
	events[1].OverrideText = "edited"
	got := subtitle.Retime(events, subtitle.Alignment{Rate: 2, Offset: -1500 * time.Millisecond})
	// a: 1000*2-1500 = 500, 2000*2-1500 = 2500; b: 18500 to 22500.
	want := []subtitle.Event{event("a", 500, 2500), event("b", 18_500, 22_500)}
	want[1].OverrideText = "edited"
	if !slices.Equal(got, want) {
		t.Fatalf("Retime = %+v, want %+v", got, want)
	}
	if events[0].Start != ms(1000) {
		t.Fatal("Retime changed its input")
	}
}

func TestRetimeClampsAndDrops(t *testing.T) {
	events := []subtitle.Event{event("gone", 0, 1000), event("cut", 500, 3000), event("kept", 5000, 6000)}
	// Offset -1 s: "gone" ends at 0 (dropped), "cut" starts at -0.5 s (clamped to 0).
	got := subtitle.Retime(events, subtitle.Alignment{Rate: 1, Offset: -time.Second})
	want := []subtitle.Event{event("cut", 0, 2000), event("kept", 4000, 5000)}
	if !slices.Equal(got, want) {
		t.Fatalf("Retime = %+v, want %+v", got, want)
	}
}

func TestRetimeRefusesBadRates(t *testing.T) {
	events := []subtitle.Event{event("a", 1000, 2000)}
	for _, rate := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		got := subtitle.Retime(events, subtitle.Alignment{Rate: rate, Offset: time.Second})
		if want := []subtitle.Event{event("a", 2000, 3000)}; !slices.Equal(got, want) {
			t.Errorf("rate %v: %+v, want rate 1", rate, got)
		}
	}
}

func FuzzAlign(f *testing.F) {
	f.Add([]byte{0, 0, 0, 1, 0, 0, 0, 2}, int64(0))
	f.Add(make([]byte, 400), int64(time.Hour))
	f.Fuzz(func(t *testing.T, data []byte, duration int64) {
		var reference, candidate []time.Duration
		for i := 0; i+4 <= len(data); i += 4 {
			at := time.Duration(int32(binary.BigEndian.Uint32(data[i:]))) * time.Millisecond
			if i%8 == 0 {
				reference = append(reference, at)
			} else {
				candidate = append(candidate, at)
			}
		}
		got := subtitle.Align(reference, candidate, time.Duration(duration))
		if !(got.Score >= 0 && got.Score <= 1) || !(got.Rate > 0) || math.IsInf(got.Rate, 0) {
			t.Fatalf("out of range: %+v", got)
		}
		switch got.Verdict {
		case subtitle.VerdictMatch, subtitle.VerdictOtherCut, subtitle.VerdictNoMatch, subtitle.VerdictUnknown:
		default:
			t.Fatalf("verdict %q", got.Verdict)
		}
	})
}
