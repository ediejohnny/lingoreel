package study

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// ErrInvalidKaraoke reports karaoke mode settings outside their ranges.
var ErrInvalidKaraoke = errors.New("invalid karaoke mode settings")

// Ranges of the karaoke mode. Below a 3 s gap the preview would last under
// 2 s and mix with quick, overlapping dialogue; a 2 s lead is about the time
// to read a short line.
const (
	MinCueGap     = 3 * time.Second
	MaxCueGap     = 60 * time.Second
	DefaultCueGap = 10 * time.Second
	MinCueLead    = 2 * time.Second
	MaxCueLead    = 10 * time.Second
	DefaultLead   = 5 * time.Second
	MaxRepeats    = 10
)

// Indicator is how a preview shows the time left before the line.
type Indicator string

const (
	// IndicatorBar is a thin bar, the width of the text, shrinking to zero.
	IndicatorBar Indicator = "bar"
	// IndicatorCountdown counts the seconds down above the line.
	IndicatorCountdown Indicator = "countdown"
)

// Valid reports a known indicator.
func (i Indicator) Valid() bool {
	return i == IndicatorBar || i == IndicatorCountdown
}

// CueRules say when the next line shows before its time: after a silence of
// at least MinGap, Lead before it starts (never longer than the silence).
type CueRules struct {
	MinGap time.Duration
	Lead   time.Duration
}

// DefaultCueRules are the rules a new profile starts with.
func DefaultCueRules() CueRules {
	return CueRules{MinGap: DefaultCueGap, Lead: DefaultLead}
}

// Validate checks the ranges.
func (r CueRules) Validate() error {
	if r.MinGap < MinCueGap || r.MinGap > MaxCueGap {
		return fmt.Errorf("%w: minimum gap %v outside %v..%v", ErrInvalidKaraoke, r.MinGap, MinCueGap, MaxCueGap)
	}
	if r.Lead < MinCueLead || r.Lead > MaxCueLead {
		return fmt.Errorf("%w: lead %v outside %v..%v", ErrInvalidKaraoke, r.Lead, MinCueLead, MaxCueLead)
	}
	return nil
}

// Cue is a line shown before its time.
type Cue struct {
	Event subtitle.Event
	// Start is when the line is spoken, on the playback clock.
	Start time.Duration
	// Lead is how long before Start the preview began: the rules' lead, or
	// the silence when it is shorter.
	Lead time.Duration
	// Remaining is how long until Start, in video time.
	Remaining time.Duration
}

// Progress is the share of the preview still to go: 1 when it begins, 0 at
// the line. The bar indicator shrinks with it.
func (c Cue) Progress() float64 {
	if c.Lead <= 0 {
		return 0
	}
	return math.Min(math.Max(float64(c.Remaining)/float64(c.Lead), 0), 1)
}

// Countdown is the whole seconds left in real time, rounded up, at a
// playback speed: at 0.75x a second of video lasts longer.
func (c Cue) Countdown(speed float64) int {
	if !(speed > 0) || c.Remaining <= 0 {
		return 0
	}
	wall := float64(c.Remaining) / speed
	return int(math.Ceil(wall / float64(time.Second)))
}

// gapBefore is the silence before an event, from the end of the speech
// before it (or the start of the video); negative for overlapping lines.
func gapBefore(timeline *subtitle.Timeline, event subtitle.Event) time.Duration {
	return event.Start - timeline.LatestEndBefore(event.Start)
}

// leadFor is how long before an event its preview shows, and whether it
// shows at all.
func leadFor(timeline *subtitle.Timeline, event subtitle.Event, rules CueRules) (time.Duration, bool) {
	gap := gapBefore(timeline, event)
	if gap < rules.MinGap || rules.Lead <= 0 {
		return 0, false
	}
	return min(rules.Lead, gap), true
}

// UpcomingCue returns the line to preview at a playback position: the next
// one, when it comes after a silence of at least MinGap and its preview has
// begun. The preview never covers the line before it.
func UpcomingCue(timeline *subtitle.Timeline, offset subtitle.Offset, position time.Duration, rules CueRules) (Cue, bool) {
	if timeline == nil {
		return Cue{}, false
	}
	at := offset.TrackTime(position)
	next, found := timeline.NextStartAfter(at)
	if !found {
		return Cue{}, false
	}
	lead, shows := leadFor(timeline, next, rules)
	if !shows || next.Start-at > lead {
		return Cue{}, false
	}
	return Cue{Event: next, Start: offset.PlaybackTime(next.Start), Lead: lead, Remaining: next.Start - at}, true
}

// cueSearchHorizon bounds the lines NextCueStartAfter looks through: a
// session wakes up at least once a second anyway, so a preview further away
// is found later, and a file of quick lines costs no long scan.
const cueSearchHorizon = time.Minute

// NextCueStartAfter returns when the next preview begins after a position,
// on the playback clock, so a session can wake up for it; false when none
// begins within the search horizon.
func NextCueStartAfter(timeline *subtitle.Timeline, offset subtitle.Offset, position time.Duration, rules CueRules) (time.Duration, bool) {
	if timeline == nil {
		return 0, false
	}
	at := offset.TrackTime(position)
	for event, found := timeline.NextStartAfter(at); found; event, found = timeline.NextStartAfter(event.Start) {
		if event.Start-at > cueSearchHorizon+rules.Lead {
			return 0, false
		}
		if lead, shows := leadFor(timeline, event, rules); shows {
			if begin := event.Start - lead; begin > at {
				return offset.PlaybackTime(begin), true
			}
			continue // its preview already began: the next one counts
		}
	}
	return 0, false
}

// replayLeadIn is how far before a line a replay goes when the
// line has no preview: enough to hear it start.
const replayLeadIn = time.Second

// ReplayTarget is where a replay goes: the start of the preview of
// the line being previewed, spoken or last spoken; for a line without a
// preview, a moment before it. False when no line has come yet.
func ReplayTarget(timeline *subtitle.Timeline, offset subtitle.Offset, position time.Duration, rules CueRules) (time.Duration, bool) {
	if cue, upcoming := UpcomingCue(timeline, offset, position, rules); upcoming {
		return cue.Start - cue.Lead, true
	}
	if timeline == nil {
		return 0, false
	}
	event, found := timeline.LastStartedBy(offset.TrackTime(position))
	if !found {
		return 0, false
	}
	lead, shows := leadFor(timeline, event, rules)
	if !shows {
		lead = min(replayLeadIn, max(gapBefore(timeline, event), 0))
	}
	return max(offset.PlaybackTime(event.Start-lead), 0), true
}

// ReplayTargetIn is ReplayTarget while a passage is studied: the line
// trained again is one of the passage (at its end, the last one spoken in
// it), and the target never goes before the passage's start, which is also
// where it goes when no line of the passage has come yet.
func ReplayTargetIn(timeline *subtitle.Timeline, offset subtitle.Offset, position time.Duration, rules CueRules, passage Segment) time.Duration {
	// Just inside the passage: a preview there has started by then, so the
	// target never lands past the end either.
	at := min(max(position, passage.Start), passage.End-time.Millisecond)
	target, ok := ReplayTarget(timeline, offset, at, rules)
	if !ok {
		return passage.Start
	}
	return max(target, passage.Start)
}

// Speech timing estimate: a line's words are spoken over the first
// speakingShare of its time, each taking its syllables, with a pause after
// a comma, a sentence's end or a change of speaker. Subtitles carry no word
// timing, so this is only an estimate, and the help says so.
const (
	speakingShare = 0.85
	commaPause    = 0.6 // in syllables
	stopPause     = 1.0
)

func pauseWeight(b Break) float64 {
	switch b {
	case BreakComma:
		return commaPause
	case BreakStop, BreakSpeaker:
		return stopPause
	}
	return 0
}

// SpokenFraction is the share of the line's visible characters (spaces
// left out) already spoken at a position of the track's clock: 0 before the
// line, 1 once it is all said. A line where nothing is said aloud (only a
// sound's description, or music) is never painted.
func SpokenFraction(event subtitle.Event, trackTime time.Duration) float64 {
	units := SpeechUnits(event.Text())
	if !units.Spoken() || event.End <= event.Start || trackTime <= event.Start {
		return 0
	}
	// In floating point: a difference of two far-apart times would overflow.
	duration := float64(event.End) - float64(event.Start)
	elapsed := (float64(trackTime) - float64(event.Start)) / (duration * speakingShare)
	if elapsed >= 1 || math.IsNaN(elapsed) {
		return 1
	}
	total, visible := 0.0, 0
	for _, word := range units.Words {
		total += word.Weight + pauseWeight(word.Break)
		visible += word.Chars
	}
	target := elapsed * total
	spent, done := 0.0, 0.0
	for _, word := range units.Words {
		if spent+word.Weight >= target && word.Weight > 0 {
			done += float64(word.Chars) * (target - spent) / word.Weight
			break
		}
		// What takes no time (a sound's description) is painted when reached.
		spent += word.Weight
		done += float64(word.Chars)
		if spent += pauseWeight(word.Break); spent >= target {
			break // held in the pause after this word
		}
	}
	return math.Min(done/float64(visible), 1)
}
