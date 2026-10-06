package watch

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Editing a line: with the video paused, a line on
// screen is selected, then edited; the file's text is never lost, and every
// change can be undone.

var (
	// ErrPlaying refuses an edit while the video plays.
	ErrPlaying = errors.New("pause the video to edit")
	// ErrNothingSelected reports an edit without a selected line.
	ErrNothingSelected = errors.New("no line selected")
	// ErrNoLineHere reports a selection where no line is on screen.
	ErrNoLineHere = errors.New("no line on screen")
	// ErrEmptyText refuses a line left without text.
	ErrEmptyText = errors.New("a line needs text")
	// ErrBadTimes refuses a line that would not end after it starts, or
	// would start before the video.
	ErrBadTimes = errors.New("a line must start at zero or later and end after it starts")
)

// Selection is the line being edited.
type Selection struct {
	Line session.Line
	// Event is the line as it shows now; Original as its file has it.
	Event, Original subtitle.Event
	// Offset turns the line's times into playback times, as the user sees
	// them: Offset.PlaybackTime(Event.Start).
	Offset subtitle.Offset
}

// Edited reports whether the line differs from its file.
func (s Selection) Edited() bool {
	_, changed := project.Diff(s.Original, s.Event)
	return changed
}

// EditChange is what the user changes on the selected line; nil keeps it.
// Times are on the playback clock, as the user sees them.
type EditChange struct {
	Text       *string
	Start, End *time.Duration
}

// lineEdits holds one line's events: as the file has them and as edited.
type lineEdits struct {
	file  map[string]subtitle.Event
	now   map[string]subtitle.Event
	order []string
	edits map[string]project.Edit
}

// editing is the session's editing state, guarded by its mutex.
type editing struct {
	mu       sync.Mutex
	lines    [2]*lineEdits
	history  *project.History
	selected *selected
	// stale counts, by line, the saved edits that no longer apply to their
	// file.
	stale [2]int
}

type selected struct {
	line session.Line
	id   string
}

// prepareLine applies a line's saved edits to the events its file gave and
// returns the timeline to show; the editing state keeps both.
func (e *editing) prepareLine(line session.Line, timeline *subtitle.Timeline, saved project.Track) (*subtitle.Timeline, error) {
	if timeline == nil {
		return nil, nil
	}
	files := timeline.Events()
	applied, stale := saved.Apply(files)
	edits := &lineEdits{
		file: make(map[string]subtitle.Event, len(files)), now: make(map[string]subtitle.Event, len(files)),
		order: make([]string, len(files)), edits: map[string]project.Edit{},
	}
	for i, event := range files {
		edits.file[event.ID], edits.now[event.ID], edits.order[i] = event, applied[i], event.ID
		if edit, changed := project.Diff(event, applied[i]); changed {
			edits.edits[event.ID] = edit
		}
	}
	e.lines[line] = edits
	e.stale[line] = stale
	if len(edits.edits) == 0 {
		return timeline, nil // nothing edited: the file's own timeline
	}
	return subtitle.NewTimeline(applied)
}

// timeline is a line's events as edited.
func (l *lineEdits) timeline() (*subtitle.Timeline, error) {
	events := make([]subtitle.Event, len(l.order))
	for i, id := range l.order {
		events[i] = l.now[id]
	}
	return subtitle.NewTimeline(events)
}

// Select picks the line on screen at the current position for editing: on
// the line given, the earliest event shown. The video must be paused.
func (s *Session) Select(line session.Line) (Selection, error) {
	if s.playing() {
		return Selection{}, ErrPlaying
	}
	var events []subtitle.Event
	visible := s.subtitles.At(s.clock.Position())
	switch line {
	case session.LinePrimary:
		events = visible.Primary
	case session.LineTranslation:
		events = visible.Translation
	}
	if len(events) == 0 {
		return Selection{}, ErrNoLineHere
	}
	s.editor.mu.Lock()
	s.editor.selected = &selected{line: line, id: events[0].ID}
	selection, _ := s.selectionLocked()
	s.editor.mu.Unlock()
	s.redraw()
	return selection, nil
}

// Selection is the line being edited, if any.
func (s *Session) Selection() (Selection, bool) {
	s.editor.mu.Lock()
	defer s.editor.mu.Unlock()
	return s.selectionLocked()
}

func (s *Session) selectionLocked() (Selection, bool) {
	chosen := s.editor.selected
	if chosen == nil || s.editor.lines[chosen.line] == nil {
		return Selection{}, false
	}
	edits := s.editor.lines[chosen.line]
	return Selection{
		Line: chosen.line, Event: edits.now[chosen.id], Original: edits.file[chosen.id],
		Offset: s.subtitles.Offset(chosen.line),
	}, true
}

// ClearSelection leaves the editing.
func (s *Session) ClearSelection() {
	if s.leaveEditing() {
		s.redraw()
	}
}

// leaveEditing lets the selected line go, without drawing; it tells whether
// a line was selected.
func (s *Session) leaveEditing() bool {
	s.editor.mu.Lock()
	defer s.editor.mu.Unlock()
	had := s.editor.selected != nil
	s.editor.selected = nil
	return had
}

// Edit changes the selected line. The video must be paused; an empty text
// or times out of order are refused, and the line stays as it was.
func (s *Session) Edit(change EditChange) error {
	if s.playing() {
		return ErrPlaying
	}
	s.editor.mu.Lock()
	selection, found := s.selectionLocked()
	if !found {
		s.editor.mu.Unlock()
		return ErrNothingSelected
	}
	after := selection.Event
	if change.Text != nil {
		text := strings.ReplaceAll(*change.Text, "\r\n", "\n")
		if strings.TrimSpace(text) == "" {
			s.editor.mu.Unlock()
			return ErrEmptyText
		}
		after.OverrideText = text
		if text == after.OriginalText {
			after.OverrideText = ""
		}
	}
	if change.Start != nil {
		after.Start = selection.Offset.TrackTime(*change.Start)
	}
	if change.End != nil {
		after.End = selection.Offset.TrackTime(*change.End)
	}
	if after.Start < 0 || after.End <= after.Start {
		s.editor.mu.Unlock()
		return ErrBadTimes
	}
	done, err := s.replaceLocked(selection.Line, selection.Event, after, true)
	s.editor.mu.Unlock()
	done()
	return err
}

// RestoreOriginal puts the selected line back as its file has it.
func (s *Session) RestoreOriginal() error {
	if s.playing() {
		return ErrPlaying
	}
	s.editor.mu.Lock()
	selection, found := s.selectionLocked()
	if !found {
		s.editor.mu.Unlock()
		return ErrNothingSelected
	}
	done, err := s.replaceLocked(selection.Line, selection.Event, selection.Original, true)
	s.editor.mu.Unlock()
	done()
	return err
}

// Undo takes back the latest edit; false when there is none.
func (s *Session) Undo() (bool, error) {
	return s.step(func(h *project.History) (project.Change, bool) { return h.Undo() }, true)
}

// Redo makes the latest undone edit again; false when there is none.
func (s *Session) Redo() (bool, error) {
	return s.step(func(h *project.History) (project.Change, bool) { return h.Redo() }, false)
}

func (s *Session) step(take func(*project.History) (project.Change, bool), undo bool) (bool, error) {
	if s.playing() {
		return false, ErrPlaying
	}
	s.editor.mu.Lock()
	change, found := take(s.editor.history)
	if !found {
		s.editor.mu.Unlock()
		return false, nil
	}
	from, to := change.After, change.Before
	if !undo {
		from, to = change.Before, change.After
	}
	done, err := s.replaceLocked(session.Line(change.Line), from, to, false)
	s.editor.mu.Unlock()
	done()
	return true, err
}

// CanUndo and CanRedo tell the editor which buttons work.
func (s *Session) CanUndo() bool {
	s.editor.mu.Lock()
	defer s.editor.mu.Unlock()
	return s.editor.history.CanUndo()
}

func (s *Session) CanRedo() bool {
	s.editor.mu.Lock()
	defer s.editor.mu.Unlock()
	return s.editor.history.CanRedo()
}

// StaleEdits counts the saved edits that did not apply: their line is gone
// or its text changed in the file.
func (s *Session) StaleEdits() int {
	s.editor.mu.Lock()
	defer s.editor.mu.Unlock()
	return s.editor.stale[session.LinePrimary] + s.editor.stale[session.LineTranslation]
}

// Edits are a line's edits as the project keeps them.
func (s *Session) Edits(line session.Line) map[string]project.Edit {
	s.editor.mu.Lock()
	defer s.editor.mu.Unlock()
	if !knownEditLine(line) || s.editor.lines[line] == nil {
		return map[string]project.Edit{}
	}
	return maps.Clone(s.editor.lines[line].edits)
}

// Export is a line's events as the user sees them, ready for a file: the
// edits in, the offset applied to the times, and what would start before
// the video cut at zero or left out.
func (s *Session) Export(line session.Line) []subtitle.Event {
	track := s.subtitles.Track(line)
	if track == nil || track.Timeline == nil {
		return nil
	}
	var events []subtitle.Event
	for _, event := range track.Timeline.Events() {
		event.Start = max(track.Offset.PlaybackTime(event.Start), 0)
		event.End = track.Offset.PlaybackTime(event.End)
		if event.End > event.Start {
			events = append(events, event)
		}
	}
	return events
}

// replaceLocked shows a line's event as to instead of from, keeps the edit
// for the project and, when record, in the history. Called with the mutex
// held; done, called once it is released, tells the listener and redraws
// the outline (the listener may call the session back).
func (s *Session) replaceLocked(line session.Line, from, to subtitle.Event, record bool) (done func(), err error) {
	nothing := func() {}
	edits := s.editor.lines[line]
	if !knownEditLine(line) || edits == nil || from.ID != to.ID {
		return nothing, fmt.Errorf("%w: line %d, event %q", ErrNothingSelected, line, to.ID)
	}
	if from == to {
		return nothing, nil
	}
	edits.now[to.ID] = to
	timeline, err := edits.timeline()
	if err != nil {
		edits.now[to.ID] = from
		return nothing, fmt.Errorf("%w: %w", ErrBadTimes, err)
	}
	if edit, changed := project.Diff(edits.file[to.ID], to); changed {
		edits.edits[to.ID] = edit
	} else {
		delete(edits.edits, to.ID)
	}
	if record {
		s.editor.history.Record(project.Change{Line: int(line), Before: from, After: to})
	}
	s.subtitles.SetTimeline(line, timeline)
	saved := maps.Clone(edits.edits)
	// SetLine changes the tracks under the same lock: this is the track edited.
	track := s.tracks.Load().Number(line)
	return func() {
		if s.listener != nil {
			s.listener.Edited(line, track, saved)
		}
		s.redraw()
	}, nil
}

// highlighted is the line to outline on a screen: the selected one, while
// its event shows on that line and the line itself shows.
func (s *Session) highlighted(visible session.Visible, screen session.Screen) (session.Line, bool) {
	s.editor.mu.Lock()
	chosen := s.editor.selected
	s.editor.mu.Unlock()
	if chosen == nil {
		return 0, false
	}
	events, shown := visible.Primary, !screen.PrimaryHidden
	if chosen.line == session.LineTranslation {
		events, shown = visible.Translation, screen.TranslationVisible
	}
	onScreen := slices.ContainsFunc(events, func(event subtitle.Event) bool { return event.ID == chosen.id })
	return chosen.line, shown && onScreen
}

// playing tells the video plays: edits wait for a pause.
func (s *Session) playing() bool {
	return s.clock.State().Status == playback.StatusPlaying
}

func knownEditLine(line session.Line) bool {
	return line == session.LinePrimary || line == session.LineTranslation
}
