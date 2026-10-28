package study

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// ErrInvalidSegment reports a passage that cannot be studied.
var ErrInvalidSegment = errors.New("invalid passage")

// Limits of a studied passage.
const (
	// MinSegment is the shortest passage: shorter ones would loop faster
	// than anyone can repeat them.
	MinSegment = time.Second
	// SnapLeadIn is the pause kept before the first line when the passage
	// is fitted to the lines, never reaching into the line before.
	SnapLeadIn = 500 * time.Millisecond
	// MaxLoopGap bounds the pause between two laps in "Repetir sem parar".
	MaxLoopGap = 5 * time.Second
	// MaxGoal bounds the laps a goal may ask for.
	MaxGoal = 99
)

// Segment is a passage of the video, on the playback clock.
type Segment struct {
	Start, End time.Duration
}

// Validate checks a passage against the video's duration (zero when
// unknown): it starts at or after 0, lasts at least MinSegment and ends
// inside the video.
func (s Segment) Validate(duration time.Duration) error {
	switch {
	case s.Start < 0:
		return fmt.Errorf("%w: starts before the video", ErrInvalidSegment)
	case s.End-s.Start < MinSegment:
		return fmt.Errorf("%w: shorter than %v", ErrInvalidSegment, MinSegment)
	case duration > 0 && s.End > duration:
		return fmt.Errorf("%w: ends after the video", ErrInvalidSegment)
	}
	return nil
}

// FitTo keeps a passage inside the video: it never starts before 0 nor ends
// after duration (zero when unknown leaves the end alone). Fitting to the
// lines can stretch the end past the video, to a line of the credits.
func (s Segment) FitTo(duration time.Duration) Segment {
	s.Start = max(s.Start, 0)
	if duration > 0 {
		s.End = min(s.End, duration)
	}
	return s
}

// Contains reports a position inside the passage, its end included.
func (s Segment) Contains(position time.Duration) bool {
	return position >= s.Start && position <= s.End
}

// ParseClock reads a time typed by the user: "1:10:23", "10:23" or "23",
// with an optional fraction after a comma or a dot ("10:23,5").
func ParseClock(text string) (time.Duration, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, fmt.Errorf("%w: empty time", ErrInvalidSegment)
	}
	parts := strings.Split(text, ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("%w: %q has too many parts", ErrInvalidSegment, text)
	}
	var total time.Duration
	for i, part := range parts {
		last := i == len(parts)-1
		value, err := clockPart(part, last)
		if err != nil {
			return 0, fmt.Errorf("%w: %q", ErrInvalidSegment, text)
		}
		// Minutes and seconds after the first part stay below 60.
		if i > 0 && value >= 60*time.Second {
			return 0, fmt.Errorf("%w: %q", ErrInvalidSegment, text)
		}
		total = total*60 + value
	}
	if total > maxClock {
		return 0, fmt.Errorf("%w: %q is too long", ErrInvalidSegment, text)
	}
	return total, nil
}

// maxClock bounds typed times to something a video can have.
const maxClock = 100 * time.Hour

// secondsPattern is the last part of a typed time: "23", "23,5", "23.25".
var secondsPattern = regexp.MustCompile(`^[0-9]+([.,][0-9]+)?$`)

// clockPart reads one part in seconds; only the last may have a fraction.
func clockPart(part string, last bool) (time.Duration, error) {
	if part == "" || strings.HasPrefix(part, "+") || strings.HasPrefix(part, "-") {
		return 0, ErrInvalidSegment
	}
	if last {
		// Digits with an optional fraction only: ParseFloat alone would take
		// "1e3", "NaN" or "Inf".
		if !secondsPattern.MatchString(part) {
			return 0, ErrInvalidSegment
		}
		part = strings.Replace(part, ",", ".", 1)
		seconds, err := strconv.ParseFloat(part, 64)
		if err != nil || seconds < 0 || seconds > float64(maxClock/time.Second) {
			return 0, ErrInvalidSegment
		}
		return time.Duration(seconds * float64(time.Second)).Round(time.Millisecond), nil
	}
	whole, err := strconv.Atoi(part)
	if err != nil || whole < 0 || whole > int(maxClock/time.Minute) {
		return 0, ErrInvalidSegment
	}
	return time.Duration(whole) * time.Second, nil
}

// SnapToLines widens a passage so no line is cut: the start moves back to
// the start of a line being spoken there, then SnapLeadIn earlier without
// reaching into the speech before it; the end moves forward to the end of
// the lines spoken there. Times are on the playback clock.
func SnapToLines(segment Segment, track subtitle.Placed) Segment {
	timeline, offset := track.Timeline, track.Offset
	if timeline == nil {
		return segment
	}
	start := offset.TrackTime(segment.Start)
	if active := timeline.ActiveAt(start); len(active) > 0 {
		start = active[0].Start // ordered by start: the earliest one
	}
	if next, found := timeline.NextStartAfter(start); found && next.Start-start < SnapLeadIn && len(timeline.ActiveAt(start)) == 0 {
		start = next.Start // a passage starting just before a line keeps its lead-in from that line
	}
	floor := timeline.LatestEndBefore(start)
	if floor > start {
		floor = start
	}
	start = max(start-SnapLeadIn, floor, 0)

	end := offset.TrackTime(segment.End)
	for _, event := range timeline.ActiveAt(end) {
		end = max(end, event.End)
	}
	return Segment{Start: max(offset.PlaybackTime(start), 0), End: offset.PlaybackTime(end)}
}

// SegmentForLine is the passage of one line, fitted like SnapToLines.
func SegmentForLine(event subtitle.Event, track subtitle.Placed) Segment {
	return SnapToLines(Segment{Start: track.Offset.PlaybackTime(event.Start), End: track.Offset.PlaybackTime(event.End)}, track)
}

// PhraseStart is where repeating the phrase goes: the start of the line
// being spoken at a playback position, or of the last one spoken, with the
// lead-in of SnapToLines. False before the first line.
func PhraseStart(track subtitle.Placed, position time.Duration) (time.Duration, bool) {
	if track.Timeline == nil {
		return 0, false
	}
	line, found := track.Timeline.LastStartedBy(track.Offset.TrackTime(position))
	if !found {
		return 0, false
	}
	return SegmentForLine(line, track).Start, true
}

// EndMode is what happens at the end of the passage.
type EndMode string

const (
	// EndPause pauses; Play then starts the passage again.
	EndPause EndMode = "pause"
	// EndLoop goes back to the start by itself, after the plan's Gap.
	EndLoop EndMode = "loop"
)

// Valid reports a known mode.
func (m EndMode) Valid() bool {
	return m == EndPause || m == EndLoop
}

// progressiveSpeeds are the speeds of the first laps with "Velocidade
// progressiva": understand first, then train at the real pace.
var progressiveSpeeds = []playback.Speed{0.75, 0.9, 1}

// PracticePlan is how a passage is practiced.
type PracticePlan struct {
	Mode EndMode
	// Gap is the pause between laps in EndLoop.
	Gap time.Duration
	// Progressive plays the first laps slower, up to normal speed.
	Progressive bool
	// Ladder takes one help away each lap: both lines, then the primary
	// only, then none.
	Ladder bool
	// Goal ends the practice after that many laps; zero never ends it.
	Goal int
}

// Validate checks the plan.
func (p PracticePlan) Validate() error {
	switch {
	case !p.Mode.Valid():
		return fmt.Errorf("%w: end mode %q", ErrInvalidSegment, p.Mode)
	case p.Gap < 0 || p.Gap > MaxLoopGap:
		return fmt.Errorf("%w: gap %v outside 0..%v", ErrInvalidSegment, p.Gap, MaxLoopGap)
	case p.Goal < 0 || p.Goal > MaxGoal:
		return fmt.Errorf("%w: goal %d outside 0..%d", ErrInvalidSegment, p.Goal, MaxGoal)
	}
	return nil
}

// Lap is how one lap plays.
type Lap struct {
	// Speed is the lap's speed; zero keeps the user's.
	Speed           playback.Speed
	ShowPrimary     bool
	ShowTranslation bool
}

// Lap returns the settings of lap n, counted from 1; laps past the plan's
// steps repeat the last step.
func (p PracticePlan) Lap(n int) Lap {
	n = max(n, 1)
	lap := Lap{ShowPrimary: true, ShowTranslation: true}
	if p.Progressive {
		lap.Speed = progressiveSpeeds[min(n, len(progressiveSpeeds))-1]
	}
	if p.Ladder {
		lap.ShowTranslation = n <= 1
		lap.ShowPrimary = n <= 2
	}
	return lap
}

// Done reports whether the goal is reached after a number of finished laps.
func (p PracticePlan) Done(finished int) bool {
	return p.Goal > 0 && finished >= p.Goal
}
