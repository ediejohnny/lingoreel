package watch

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Tracks are the video's audio and subtitle tracks, in the video's order, and
// the ones the session plays (zero: none, or the player's own audio).
type Tracks struct {
	List    []media.Track
	Current media.Selection
}

// Number is the track a subtitle line shows; zero when it shows none.
func (t Tracks) Number(line session.Line) uint64 {
	if line == session.LineTranslation {
		return t.Current.Translation
	}
	return t.Current.Primary
}

// Of lists the tracks of a kind.
func (t Tracks) Of(kind media.TrackType) []media.Track {
	var list []media.Track
	for _, track := range t.List {
		if track.Type == kind {
			list = append(list, track)
		}
	}
	return list
}

// Why a line cannot show a track, for the app to word it.
var (
	// ErrUnknownTrack: not a readable subtitle track of the video.
	ErrUnknownTrack = errors.New("not a readable subtitle track of the video")
	// ErrSameTrack: the other line shows that track already.
	ErrSameTrack = errors.New("the other line shows that track")
	// ErrNeedsPrimary: the line on top always shows a subtitle.
	ErrNeedsPrimary = errors.New("the line on top needs a subtitle")
)

// CheckLine tells whether a line can show a track (zero: none), before it
// is loaded: a readable subtitle of the video that the other line does not
// show, and never none on top.
func (s *Session) CheckLine(line session.Line, number uint64) error {
	if line != session.LinePrimary && line != session.LineTranslation {
		return fmt.Errorf("unknown subtitle line %d", line)
	}
	tracks := s.Tracks()
	if number != 0 && !slices.ContainsFunc(tracks.Of(media.TrackSubtitle), func(track media.Track) bool { return track.Number == number && track.Readable }) {
		return ErrUnknownTrack
	}
	return tracks.pairs(line, number)
}

// pairs tells whether a line can show a track next to what the other line
// shows: never none on top, never the track the other line shows.
func (t Tracks) pairs(line session.Line, number uint64) error {
	if number == 0 {
		if line == session.LinePrimary {
			return ErrNeedsPrimary
		}
		return nil
	}
	other := session.LineTranslation
	if line == session.LineTranslation {
		other = session.LinePrimary
	}
	if t.Number(other) == number {
		return ErrSameTrack
	}
	return nil
}

// newTracks keeps the tracks a session can switch between.
func newTracks(info media.Info, current media.Selection) *Tracks {
	tracks := &Tracks{Current: current}
	for _, track := range info.Tracks {
		if track.Type == media.TrackAudio || track.Type == media.TrackSubtitle {
			tracks.List = append(tracks.List, track)
		}
	}
	return tracks
}

// Tracks are the video's tracks and the ones playing.
func (s *Session) Tracks() Tracks {
	tracks := *s.tracks.Load()
	tracks.List = slices.Clone(tracks.List)
	return tracks
}

// AddTrack lists a subtitle file added to the video while it plays; a track
// already listed is left as it is.
func (s *Session) AddTrack(track media.Track) {
	s.changeTracks(func(tracks *Tracks) {
		if !slices.ContainsFunc(tracks.List, func(listed media.Track) bool { return listed.Number == track.Number }) {
			tracks.List = append(slices.Clone(tracks.List), track)
		}
	})
}

// changeTracks replaces the tracks with a changed copy; readers never wait.
func (s *Session) changeTracks(change func(*Tracks)) {
	s.tracksMu.Lock()
	defer s.tracksMu.Unlock()
	next := *s.tracks.Load()
	change(&next)
	s.tracks.Store(&next)
}

// SelectAudio switches the audio track while the video plays, by its number;
// session.ErrNotSupported when the player cannot.
func (s *Session) SelectAudio(number uint64) error {
	index := slices.IndexFunc(s.Tracks().Of(media.TrackAudio), func(track media.Track) bool { return track.Number == number })
	if index < 0 {
		return fmt.Errorf("unknown audio track %d", number)
	}
	if err := s.controls.SelectAudioTrack(index); err != nil {
		return err
	}
	s.changeTracks(func(tracks *Tracks) { tracks.Current.Audio = number })
	return nil
}

// SetSyncWording changes how a sync change is worded on the video, as the
// lines' names change with their tracks; nil shows none.
func (s *Session) SetSyncWording(notice func(session.Line, subtitle.Offset) string) {
	s.notice.Store(&notice)
}

// SetLine puts another subtitle on a line while the video plays, a track of
// the video or a file (number and what was loaded for it), or takes the
// line's subtitle away (number 0). The line comes with its saved edits and
// offset (saved); the editing on that line starts over, its selection, undo
// and redo, since the new file numbers its lines afresh; and "Repetir cada
// fala" counts afresh too. The pair of lines is checked again here, as the
// other line may have changed while this file loaded.
func (s *Session) SetLine(line session.Line, number uint64, loaded analysis.LoadedTrack, saved project.Track) error {
	if line != session.LinePrimary && line != session.LineTranslation {
		return fmt.Errorf("unknown subtitle line %d", line)
	}
	if number != 0 && loaded.Timeline == nil {
		return fmt.Errorf("track %d has no subtitles loaded", number)
	}
	s.lineMu.Lock()
	defer s.lineMu.Unlock()
	if err := s.Tracks().pairs(line, number); err != nil {
		return err
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
	// Under the editor's lock, so an edit is always told with the track it
	// was made on.
	s.changeTracks(func(tracks *Tracks) {
		if line == session.LineTranslation {
			tracks.Current.Translation = number
		} else {
			tracks.Current.Primary = number
		}
	})
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
