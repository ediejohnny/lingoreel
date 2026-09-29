package session

import (
	"sync"
	"time"

	"lingoreel/internal/domain/subtitle"
)

// Line identifies one of the two subtitle lines.
type Line int

const (
	LinePrimary Line = iota
	LineTranslation
)

// Track is a subtitle timeline shown with an offset.
type Track struct {
	Timeline *subtitle.Timeline
	Offset   subtitle.Offset
}

// Visible holds the events on screen for both lines.
type Visible struct {
	Primary     []subtitle.Event
	Translation []subtitle.Event
}

// Equal reports whether both snapshots show the same text.
func (v Visible) Equal(other Visible) bool {
	return sameEvents(v.Primary, other.Primary) && sameEvents(v.Translation, other.Translation)
}

func sameEvents(a, b []subtitle.Event) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Text() != b[i].Text() {
			return false
		}
	}
	return true
}

// Subtitles decides which events are visible at a playback position.
// It is safe for concurrent use.
type Subtitles struct {
	mu    sync.RWMutex
	lines [2]*Track
}

// NewSubtitles creates the subtitles of a session; either track may be nil.
func NewSubtitles(primary, translation *Track) *Subtitles {
	return &Subtitles{lines: [2]*Track{primary, translation}}
}

// At returns the events visible at a playback position.
func (s *Subtitles) At(position time.Duration) Visible {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Visible{
		Primary:     activeAt(s.lines[LinePrimary], position),
		Translation: activeAt(s.lines[LineTranslation], position),
	}
}

func activeAt(track *Track, position time.Duration) []subtitle.Event {
	if track == nil || track.Timeline == nil {
		return nil
	}
	return track.Timeline.ActiveAt(track.Offset.TrackTime(position))
}

// NextChangeAfter returns the earliest playback position after the given one
// at which either line changes.
func (s *Subtitles) NextChangeAfter(position time.Duration) (time.Duration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var next time.Duration
	found := false
	for _, track := range s.lines {
		if track == nil || track.Timeline == nil {
			continue
		}
		change, ok := track.Timeline.NextChangeAfter(track.Offset.TrackTime(position))
		if !ok {
			continue
		}
		if at := track.Offset.PlaybackTime(change); !found || at < next {
			next, found = at, true
		}
	}
	return next, found
}

// Offset returns the offset of a line.
func (s *Subtitles) Offset(line Line) subtitle.Offset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if track := s.lines[line]; track != nil {
		return track.Offset
	}
	return 0
}

// SetOffset changes the offset of a line; it has no effect on a missing line.
func (s *Subtitles) SetOffset(line Line, offset subtitle.Offset) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if track := s.lines[line]; track != nil {
		updated := *track
		updated.Offset = offset
		s.lines[line] = &updated
	}
}
