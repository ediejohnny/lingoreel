package speech

import (
	"math"
	"slices"
	"time"
)

// Sensitivity is how readily quiet sound counts as voice.
type Sensitivity string

const (
	SensitivityLow    Sensitivity = "low"
	SensitivityNormal Sensitivity = "normal"
	SensitivityHigh   Sensitivity = "high"
)

// Valid reports a known sensitivity.
func (s Sensitivity) Valid() bool {
	return s == SensitivityLow || s == SensitivityNormal || s == SensitivityHigh
}

// tuning is where between a line's quiet and its loudest the voice begins,
// and how far apart those two must be to tell anything (in dB).
func (s Sensitivity) tuning() (share, contrast float64) {
	switch s {
	case SensitivityLow:
		return 0.55, 12
	case SensitivityHigh:
		return 0.35, 8
	}
	return 0.45, 10
}

// Track is a run of frames from a point of the film's clock.
type Track struct {
	Start  time.Duration
	Frames []Frame
}

// frameAt is the frame holding a position, before 0 or past the end when
// the track does not reach it.
func (t Track) frameAt(position time.Duration) int {
	return int(math.Floor(float64(position-t.Start) / float64(FrameLength)))
}

func (t Track) timeOf(frame int) time.Duration {
	return t.Start + time.Duration(frame)*FrameLength
}

// Span is a stretch of voice on the film's clock.
type Span struct{ Start, End time.Duration }

// Verdict says whether a line's voice can be trusted, or why not; an
// untrusted line keeps the estimate from its letters.
type Verdict int

const (
	Trusted Verdict = iota
	// Missing: the sound does not cover the line.
	Missing
	// TooQuiet: nothing stands out from the sound around the line.
	TooQuiet
	// NoVoice: voice fills under a quarter of the line.
	NoVoice
	// Steady: sound fills the line and the silence around it, so it is
	// music or noise, not someone speaking.
	Steady
	// WrongPace: the voice is far too short or too long for the line's
	// syllables.
	WrongPace
)

var verdictNames = [...]string{"trusted", "missing", "too quiet", "no voice", "steady", "wrong pace"}

func (v Verdict) String() string {
	if v < 0 || int(v) >= len(verdictNames) {
		return "unknown"
	}
	return verdictNames[v]
}

// Detection is the voice found in a line.
type Detection struct {
	Spans   []Span
	Verdict Verdict
}

// LineMargin is the sound on each side of a line that DetectLine judges it
// by, when the track has it.
const LineMargin = 500 * time.Millisecond

// The detector's rules. Each line is judged on its own sound, a margin
// around it included: its quiet (the 10th percentile) and its loudest (the
// 95th) set where voice begins, so a whisper in a quiet scene counts as
// much as a shout in a loud one. A gap shorter than shortestPause is a stop
// inside a word (the closure before a P or a T); a burst shorter than
// shortestVoice is a click.
const (
	floorPercentile = 0.10
	peakPercentile  = 0.95
	hysteresis      = 2.0 // dB below the threshold before the voice ends
	shortestPause   = 120 * time.Millisecond
	shortestVoice   = 50 * time.Millisecond
	leastLineShare  = 0.15 // "How... (2 s)... are you?" speaks in a quarter of its time
	mostWindowShare = 0.98
	slowestPace     = 1.0 // syllables per second of voice
	fastestPace     = 12.0
)

// leastCenterDB bounds how much a sound spread wide is turned down: near
// zero, the share in the middle of noise swings far in decibels.
const leastCenterDB = -12

func voiceLevel(f Frame) float64 { return float64(f.Voice) + centerDB(f) }

func hissLevel(f Frame) float64 { return float64(f.Hiss) + centerDB(f) }

func centerDB(f Frame) float64 {
	return max(10*math.Log10(float64(f.Center)), leastCenterDB)
}

// DetectLine finds the voice of a line spoken from start to end, whose
// text weighs syllables (study.SpeechUnits).
func DetectLine(track Track, start, end time.Duration, syllables float64, sensitivity Sensitivity) Detection {
	first, last := track.frameAt(start), track.frameAt(end)
	if end <= start || first < 0 || last > len(track.Frames) {
		return Detection{Verdict: Missing}
	}
	from := max(track.frameAt(start-LineMargin), 0)
	to := min(track.frameAt(end+LineMargin), len(track.Frames))
	frames := track.Frames[from:to]

	share, contrast := sensitivity.tuning()
	active := make([]bool, len(frames))
	levels := make([]float64, len(frames))
	sorted := make([]float64, len(frames))
	usable := false
	for _, level := range [...]func(Frame) float64{voiceLevel, hissLevel} {
		for i, frame := range frames {
			levels[i] = level(frame)
		}
		copy(sorted, levels)
		slices.Sort(sorted)
		floor := sorted[int(floorPercentile*float64(len(sorted)-1))]
		peak := sorted[int(peakPercentile*float64(len(sorted)-1))]
		if peak-floor < contrast {
			continue
		}
		usable = true
		threshold := floor + share*(peak-floor)
		on := false
		for i, l := range levels {
			on = l >= threshold || (on && l >= threshold-hysteresis)
			active[i] = active[i] || on
		}
	}
	if !usable {
		return Detection{Verdict: TooQuiet}
	}

	runs := voiceRuns(active)
	voiced, inLine := 0, 0
	spans := make([]Span, len(runs))
	for i, run := range runs {
		voiced += run[1] - run[0]
		inLine += max(min(run[1], last-from)-max(run[0], first-from), 0)
		spans[i] = Span{Start: track.timeOf(from + run[0]), End: track.timeOf(from + run[1])}
	}
	switch pace := syllables / (float64(voiced) * FrameLength.Seconds()); {
	case float64(inLine) < leastLineShare*float64(last-first):
		return Detection{Verdict: NoVoice}
	case float64(voiced) > mostWindowShare*float64(len(frames)):
		return Detection{Verdict: Steady}
	case !(pace >= slowestPace && pace <= fastestPace):
		return Detection{Verdict: WrongPace}
	}
	return Detection{Spans: spans, Verdict: Trusted}
}

// voiceRuns turns the frames heard as voice into runs [from, to), joining
// those split by a stop inside a word and dropping clicks.
func voiceRuns(active []bool) [][2]int {
	pause, burst := int(shortestPause/FrameLength), int(shortestVoice/FrameLength)
	var runs [][2]int
	for i := 0; i < len(active); {
		if !active[i] {
			i++
			continue
		}
		j := i
		for j < len(active) && active[j] {
			j++
		}
		if n := len(runs); n > 0 && i-runs[n-1][1] < pause {
			runs[n-1][1] = j
		} else {
			runs = append(runs, [2]int{i, j})
		}
		i = j
	}
	return slices.DeleteFunc(runs, func(run [2]int) bool { return run[1]-run[0] < burst })
}
