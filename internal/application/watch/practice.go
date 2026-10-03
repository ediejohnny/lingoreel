package watch

import (
	"errors"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/study"
)

// ErrNoLine reports "Esta fala" before any line was spoken.
var ErrNoLine = errors.New("no line spoken yet")

// screen is what the display shows for the visible events: the user's
// translation choice, the ladder of a passage, a preview and a fill.
func (s *Session) screen(visible session.Visible) session.Screen {
	lap := s.currentLap()
	screen := Screen(visible, s.translationVisible.Load() && lap.ShowTranslation)
	screen.PrimaryHidden = !lap.ShowPrimary
	screen.Fill = visible.Fill
	screen.Highlight, screen.Highlighted = s.highlighted(visible, screen)
	if visible.Cue != nil {
		screen.Cue = &session.CueScreen{
			Indicator: s.Indicator(),
			Progress:  visible.Cue.Progress(),
			Seconds:   visible.Cue.Countdown(float64(s.clock.State().Speed)),
		}
	}
	return screen
}

func (s *Session) currentLap() study.Lap {
	if lap := s.lap.Load(); lap != nil {
		return *lap
	}
	return study.Lap{ShowPrimary: true, ShowTranslation: true}
}

// setLap shows the lines of a lap and redraws.
func (s *Session) setLap(lap study.Lap) {
	s.lap.Store(&lap)
	s.redraw()
}

// redraw shows the last visible events again, after a change of how they
// show.
func (s *Session) redraw() {
	if last := s.lastVisible.Load(); last != nil {
		s.show(*last)
	}
}

// practiced shows a notice for what happened to the passage.
func (s *Session) practiced(notice func(session.PracticeEvent, session.PracticeStatus) string) func(session.PracticeEvent, session.PracticeStatus) {
	return func(event session.PracticeEvent, status session.PracticeStatus) {
		if s.display == nil || notice == nil {
			return
		}
		if text := notice(event, status); text != "" {
			_ = s.display.Notify(text) // a lost notice is cosmetic
		}
	}
}

// SetKaraoke turns the Modo Karaokê on or off, keeping its rules.
func (s *Session) SetKaraoke(enabled bool) {
	karaoke := s.subtitles.Karaoke()
	karaoke.Enabled = enabled
	s.subtitles.SetKaraoke(karaoke)
}

// Karaoke is the Modo Karaokê as it is now.
func (s *Session) Karaoke() session.Karaoke {
	return s.subtitles.Karaoke()
}

// SetIndicator changes how previews show the time left.
func (s *Session) SetIndicator(indicator study.Indicator) error {
	if !indicator.Valid() {
		return study.ErrInvalidKaraoke
	}
	s.indicator.Store(indicator)
	s.redraw()
	return nil
}

// Indicator is how previews show the time left.
func (s *Session) Indicator() study.Indicator {
	indicator, _ := s.indicator.Load().(study.Indicator)
	return indicator
}

// SetLineRepeats sets how many times each line plays again (0 is off).
func (s *Session) SetLineRepeats(times int) error {
	return s.practice.SetLineRepeats(times)
}

// LineRepeats is how many times each line plays again.
func (s *Session) LineRepeats() int {
	return s.practice.LineRepeats()
}

// ReplayLine is "Treinar de novo": back to the start of the preview of the
// line being previewed, spoken or last spoken.
func (s *Session) ReplayLine() error {
	return s.practice.ReplayLine()
}

// StudyPassage starts "Estudar trecho"; snap widens the passage so no line
// is cut.
func (s *Session) StudyPassage(segment study.Segment, plan study.PracticePlan, snap bool) error {
	if snap {
		if track := s.subtitles.Track(session.LinePrimary); track != nil {
			segment = study.SnapToLines(segment, track.Timeline, track.Offset)
		}
	}
	return s.practice.Start(segment, plan)
}

// LinePassage is the passage of the line on top being spoken, or the last
// one, fitted to the lines ("Esta fala").
func (s *Session) LinePassage() (study.Segment, error) {
	track := s.subtitles.Track(session.LinePrimary)
	if track == nil || track.Timeline == nil {
		return study.Segment{}, ErrNoLine
	}
	line, found := track.Timeline.LastStartedBy(track.Offset.TrackTime(s.clock.Position()))
	if !found {
		return study.Segment{}, ErrNoLine
	}
	return study.SegmentForLine(line, track.Timeline, track.Offset), nil
}

// StopPassage leaves the passage.
func (s *Session) StopPassage() {
	s.practice.Stop()
}

// ResumePassage goes back into a passage the video left.
func (s *Session) ResumePassage() error {
	return s.practice.Resume()
}

// Passage describes the passage being studied.
func (s *Session) Passage() session.PracticeStatus {
	return s.practice.Status()
}
