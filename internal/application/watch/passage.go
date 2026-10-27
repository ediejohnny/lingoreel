package watch

import (
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// ErrNoPassageStart reports the end of a passage marked before its start.
var ErrNoPassageStart = errors.New("no passage start marked")

// MarkEvent is what a passage mark did, for its notice on the video.
type MarkEvent int

const (
	// MarkStarted: the start was marked; the segment's Start says where.
	MarkStarted MarkEvent = iota + 1
	// MarkNoStart: the end came with no start marked.
	MarkNoStart
	// MarkTooShort: the end came too soon after the start, which stays.
	MarkTooShort
)

// marking holds the start the mark shortcut set and how a passage it closes
// is studied: the options last used, fitted to the lines by default.
type marking struct {
	mu    sync.Mutex
	start time.Duration
	set   bool
	plan  study.PracticePlan
	snap  bool
}

// MarkPassageStart marks where a passage begins: the position now. A start
// marked again moves.
func (s *Session) MarkPassageStart() time.Duration {
	position := s.controls.Position()
	s.marks.mu.Lock()
	s.marks.start, s.marks.set = position, true
	s.marks.mu.Unlock()
	s.tellMark(MarkStarted, study.Segment{Start: position})
	return position
}

// MarkedStart is the start marked for a passage, if any.
func (s *Session) MarkedStart() (time.Duration, bool) {
	s.marks.mu.Lock()
	defer s.marks.mu.Unlock()
	return s.marks.start, s.marks.set
}

// ClearMark forgets the marked start.
func (s *Session) ClearMark() {
	s.marks.mu.Lock()
	s.marks.set = false
	s.marks.mu.Unlock()
}

// MarkPassageEnd closes the passage at the position now, starts studying it
// and tells the listener, which saves it. The marks may come in either
// order on the clock: an end before the start swaps them. Without a start,
// or too soon after it (the start then stays, to try further on), it is
// told on the video and nothing starts.
func (s *Session) MarkPassageEnd() (study.Segment, error) {
	s.marks.mu.Lock()
	start, set, plan, snap := s.marks.start, s.marks.set, s.marks.plan, s.marks.snap
	s.marks.mu.Unlock()
	if !set {
		s.tellMark(MarkNoStart, study.Segment{})
		return study.Segment{}, ErrNoPassageStart
	}
	end := s.controls.Position()
	segment := study.Segment{Start: min(start, end), End: max(start, end)}
	if snap {
		if track := s.subtitles.Track(session.LinePrimary); track != nil {
			segment = study.SnapToLines(segment, track.Timeline, track.Shown())
		}
	}
	duration := s.clock.State().Duration
	segment = segment.FitTo(duration)
	if err := segment.Validate(duration); err != nil {
		s.tellMark(MarkTooShort, segment)
		return segment, err
	}
	if err := s.practice.Start(segment, plan); err != nil {
		return segment, err
	}
	s.ClearMark()
	if s.listener != nil {
		s.listener.PassageMarked(segment, plan)
	}
	return segment, nil
}

// SetPassageOptions sets how a passage closed by the mark shortcut is
// studied: the plan and whether it is fitted to the lines.
func (s *Session) SetPassageOptions(plan study.PracticePlan, snap bool) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	s.marks.mu.Lock()
	s.marks.plan, s.marks.snap = plan, snap
	s.marks.mu.Unlock()
	return nil
}

// PassageOptions is how a passage closed by the mark shortcut is studied.
func (s *Session) PassageOptions() (study.PracticePlan, bool) {
	s.marks.mu.Lock()
	defer s.marks.mu.Unlock()
	return s.marks.plan, s.marks.snap
}

// SavedPassage is a passage the video keeps, as the player window lists it.
type SavedPassage struct {
	Name    string
	Segment study.Segment
	Plan    study.PracticePlan
}

// SetSavedPassages tells the session the video's saved passages, for the
// player window's menu; the app keeps them and calls it on every change.
func (s *Session) SetSavedPassages(list []SavedPassage) {
	copied := slices.Clone(list)
	s.saved.Store(&copied)
}

// SavedPassages are the video's saved passages, as last told.
func (s *Session) SavedPassages() []SavedPassage {
	if list := s.saved.Load(); list != nil {
		return slices.Clone(*list)
	}
	return nil
}

// StudySavedAt studies the saved passage at index, as it was kept.
func (s *Session) StudySavedAt(index int) error {
	list := s.SavedPassages()
	if index < 0 || index >= len(list) {
		return ErrNoSavedPassage
	}
	return s.StudySaved(list[index].Segment, list[index].Plan)
}

// ErrNoSavedPassage reports a saved passage that is not there anymore.
var ErrNoSavedPassage = errors.New("no such saved passage")

// tellMark shows the notice of a mark.
func (s *Session) tellMark(event MarkEvent, segment study.Segment) {
	if s.display == nil || s.markNotice == nil {
		return
	}
	if text := s.markNotice(event, segment); text != "" {
		_ = s.display.Notify(text) // a lost notice is cosmetic
	}
}
