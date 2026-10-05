package watch

import (
	"fmt"
	"maps"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// SetLine puts another subtitle on a line while the video plays, a track of
// the video or a file (number and what was loaded for it), or takes the
// line's subtitle away (number 0). The line comes with its saved edits and
// offset (saved); the editing on that line starts over, its selection, undo
// and redo, since the new file numbers its lines afresh; and "Repetir cada
// fala" counts afresh too.
func (s *Session) SetLine(line session.Line, number uint64, loaded analysis.LoadedTrack, saved project.Track) error {
	if line != session.LinePrimary && line != session.LineTranslation {
		return fmt.Errorf("unknown subtitle line %d", line)
	}
	if number != 0 && loaded.Timeline == nil {
		return fmt.Errorf("track %d has no subtitles loaded", number)
	}
	s.editor.mu.Lock()
	if chosen := s.editor.selected; chosen != nil && chosen.line == line {
		s.editor.selected = nil
	}
	s.editor.history.Forget(int(line))
	s.editor.lines[line], s.editor.stale[line] = nil, 0
	var track *session.Track
	if number != 0 {
		timeline, err := s.editor.prepareLine(line, loaded.Timeline, saved)
		if err != nil {
			s.editor.mu.Unlock()
			return fmt.Errorf("applying the saved edits: %w", err)
		}
		track = &session.Track{Timeline: timeline, Offset: subtitle.NewOffset(saved.Offset.Duration())}
	}
	s.editor.mu.Unlock()

	if number != 0 {
		s.loadedMu.Lock()
		next := maps.Clone(s.loaded)
		if next == nil {
			next = map[uint64]analysis.LoadedTrack{}
		}
		next[number] = loaded
		s.loaded = next
		s.loadedMu.Unlock()
	}
	if line == session.LineTranslation {
		s.hasTranslation.Store(track != nil)
	}
	s.subtitles.SetTrack(line, track)
	s.practice.LinesChanged()
	return nil
}
