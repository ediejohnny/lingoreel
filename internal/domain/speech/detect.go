package speech

import (
	"math"
	"slices"
	"time"
)

// Sensitivity is how readily a doubtful sound counts as voice.
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

// threshold is the speech from which a frame starts a stretch of voice.
func (s Sensitivity) threshold() float32 {
	switch s {
	case SensitivityLow:
		return 0.65
	case SensitivityHigh:
		return 0.35
	}
	return 0.5
}

// speechFall is how far under the threshold the speech must fall before
// the voice ends, so a word that wavers stays whole (as Silero's own
// segmenting does).
const speechFall = 0.15

// hears tells whether a frame is voice, going on from the frame before.
func hears(frame Frame, threshold float32, before bool) bool {
	return frame.Speech >= threshold || (before && frame.Speech >= threshold-speechFall)
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
	// NoVoice: voice fills under 15% of the line.
	NoVoice
	// Steady: voice fills the line and the silence around it, so talk goes
	// on across lines and this one's voice cannot be told apart.
	Steady
	// WrongPace: the voice is far too short or too long for the line's
	// syllables.
	WrongPace
)

var verdictNames = [...]string{"trusted", "missing", "no voice", "steady", "wrong pace"}

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

// The detector's rules. A gap shorter than shortestPause is a stop inside
// a word or between two said together; a burst shorter than shortestVoice
// (two of the model's chunks) is a breath or a click.
const (
	shortestPause   = 100 * time.Millisecond
	shortestVoice   = 64 * time.Millisecond
	leastLineShare  = 0.15 // "How... (2 s)... are you?" speaks in a quarter of its time
	mostWindowShare = 0.98
	slowestPace     = 1.0 // syllables per second of voice
	fastestPace     = 12.0
)

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

	threshold := sensitivity.threshold()
	active := make([]bool, len(frames))
	on := false
	for i, frame := range frames {
		on = hears(frame, threshold, on)
		active[i] = on
	}

	// Voice only in the margin is another line's, or a sound around this one.
	runs := slices.DeleteFunc(voiceRuns(active), func(run [2]int) bool {
		return run[1] <= first-from || run[0] >= last-from
	})
	voiced, inLine := 0, 0
	for _, run := range runs {
		voiced += run[1] - run[0]
		inLine += max(min(run[1], last-from)-max(run[0], first-from), 0)
	}
	switch pace := syllables / (float64(voiced) * FrameLength.Seconds()); {
	case float64(inLine) < leastLineShare*float64(last-first):
		return Detection{Verdict: NoVoice}
	case float64(voiced) > mostWindowShare*float64(len(frames)):
		return Detection{Verdict: Steady}
	case !(pace >= slowestPace && pace <= fastestPace):
		return Detection{Verdict: WrongPace}
	}
	runs = lineOnset(frames, runs, first-from, threshold)
	spans := make([]Span, len(runs))
	for i, run := range runs {
		spans[i] = Span{Start: track.timeOf(from + run[0]), End: track.timeOf(from + run[1])}
	}
	return Detection{Spans: spans, Verdict: Trusted}
}

// Where a line's own voice begins, measured against the voice's start marked
// by ear in five films: the line before often runs up to this one, and the
// model's speech rises a little after the voice does.
const (
	// lineTail: a stretch of voice already going when the line shows that
	// ends this soon after is the end of the line before.
	lineTail = 150 * time.Millisecond
	// alreadySpeaking: voice going this long before the line shows runs into
	// it; the line's voice begins after the deepest dip within onsetReach of
	// the line's start, or with the line when there is none.
	alreadySpeaking = 150 * time.Millisecond
	onsetReach      = 300 * time.Millisecond
	// risingSpeech: the speech a voice starting near the line rises from;
	// its start goes back to there.
	risingSpeech = 0.2
)

// lineOnset moves the start of the first run to where the line's own voice
// begins; first is the frame where the line shows.
func lineOnset(frames []Frame, runs [][2]int, first int, threshold float32) [][2]int {
	if len(runs) > 1 && runs[0][0] < first && runs[0][1] < first+int(lineTail/FrameLength) {
		runs = runs[1:]
	}
	run := &runs[0]
	reach := int(onsetReach / FrameLength)
	if run[0] < first-int(alreadySpeaking/FrameLength) {
		lo, hi := max(first-reach, run[0]), min(first+reach, run[1]-1)
		dip := lo
		for i := lo; i <= hi; i++ {
			if frames[i].Speech < frames[dip].Speech {
				dip = i
			}
		}
		if frames[dip].Speech < threshold {
			run[0] = dip + 1
		} else {
			run[0] = first
		}
		return runs
	}
	if run[0] > first-reach {
		for run[0] > 0 && frames[run[0]-1].Speech >= risingSpeech && frames[run[0]-1].Speech < threshold {
			run[0]--
		}
	}
	return runs
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
