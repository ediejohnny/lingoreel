package voice

import (
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Keeping the study line on the voice while the film plays: every KeepEvery
// the voice heard around the playhead is read against the line as it is
// shown, and the line moves as subtitle.DriftTracker says. What the voice
// moves is the session's alone, apart from the user's own offset.

// KeepEvery is how often the line is read against the voice.
const KeepEvery = 5 * time.Second

// How much voice a reading takes around the playhead, and how far beyond it
// the lines read with it reach: a reading looks that far for its lines.
const (
	keepBehind = 100 * time.Second
	keepAhead  = 20 * time.Second
	keepReach  = 15 * time.Second
)

// HeardAround gives the voice heard around a moment of the film, up to
// before it and after it (Follower.Heard); false when none was heard there.
type HeardAround func(at, before, after time.Duration) (speech.Track, bool)

// KeptLine is the line kept on the voice: its track's number (zero for
// none), its lines and the user's own offset.
type KeptLine struct {
	Number   uint64
	Timeline *subtitle.Timeline
	Offset   subtitle.Offset
}

// LineKeeper keeps one line on the voice through a session. Another track
// on the line starts over; a seek takes the shift settled for that part of
// the film. The user's word wins: a move of the line's own offset is the
// voice's new base, or, against a step just taken, leaves the line alone for
// the session, as putting the line back to its file's timing (zero) does.
// It is safe for concurrent use.
type LineKeeper struct {
	mu      sync.Mutex
	tracker subtitle.DriftTracker
	number  uint64
	own     subtitle.Offset
	seeks   uint64
	shift   time.Duration
}

// Turn reads the line against the voice heard around the playhead at now;
// seeks is the playback clock's count of jumps. It returns the shift the
// line is to be shown with, on top of its own offset, and whether it
// changed.
func (k *LineKeeper) Turn(now time.Time, position time.Duration, seeks uint64, line KeptLine, heard HeardAround) (time.Duration, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	before := k.shift
	if line.Number != k.number {
		k.tracker, k.number, k.own, k.seeks, k.shift = subtitle.DriftTracker{}, line.Number, line.Offset, seeks, 0
	}
	if line.Offset != k.own {
		if line.Offset == 0 {
			k.tracker.Stop()
		} else {
			k.tracker.Nudged(line.Offset.Duration()-k.own.Duration(), now)
		}
		k.own = line.Offset
	}
	if seeks != k.seeks {
		k.seeks, k.shift = seeks, k.tracker.Seek(position)
	}
	if line.Number == 0 || line.Timeline == nil {
		return k.shift, k.shift != before
	}
	if heard != nil {
		if window, found := heard(position, keepBehind, keepAhead); found && len(window.Frames) > 0 {
			end := window.Start + time.Duration(len(window.Frames))*speech.FrameLength
			shown := spansShown(line.Timeline, line.Offset.Shift(k.shift), window.Start-keepReach, end+keepReach)
			k.tracker.Heard(subtitle.ReadDrift(window, shown), position, end, now)
		}
	}
	if _, moved := k.tracker.Step(now); moved {
		k.shift = k.tracker.Auto()
	}
	return k.shift, k.shift != before
}

// Forget starts over with no shift, as when keeping the line is turned off;
// it tells whether the line had one to take away.
func (k *LineKeeper) Forget() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	had := k.shift != 0
	k.tracker, k.number, k.shift = subtitle.DriftTracker{}, 0, 0
	return had
}

// spansShown are the spoken lines of a timeline shown at an offset, between
// two moments of the film.
func spansShown(timeline *subtitle.Timeline, offset subtitle.Offset, from, to time.Duration) []speech.Span {
	var spans []speech.Span
	for _, event := range timeline.Overlapping(offset.TrackTime(from), offset.TrackTime(to)) {
		if event.Text() == "" {
			continue
		}
		spans = append(spans, speech.Span{Start: offset.PlaybackTime(event.Start), End: offset.PlaybackTime(event.End)})
	}
	return spans
}
