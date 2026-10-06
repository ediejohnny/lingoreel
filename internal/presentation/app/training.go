package app

import (
	"errors"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

var (
	errBadTime     = worded(func() string { return ptbr.T("error.passage.time") })
	errBadPassage  = worded(func() string { return ptbr.T("error.passage.invalid") })
	errBadPlan     = worded(func() string { return ptbr.T("error.passage.plan") })
	errNoLineYet   = worded(func() string { return ptbr.T("error.passage.noLineYet") })
	errNoPassage   = worded(func() string { return ptbr.T("error.passage.notStudying") })
	errBadRepeat   = worded(func() string { return ptbr.T("error.karaoke.lineRepeats") })
	errBadSettings = worded(func() string { return ptbr.T("error.choiceNotSaved") })
)

// SetKaraoke turns the karaoke mode on or off; the choice is kept in the
// active profile.
func (a *API) SetKaraoke(enabled bool) error {
	if err := a.control(func(w *watching) error {
		w.session.SetKaraoke(enabled)
		return nil
	}); err != nil {
		return err
	}
	return a.keep(func(k *settings.KaraokePreferences) { k.Enabled = enabled })
}

// SetKaraokeIndicator chooses "bar" or "countdown" for the previews.
func (a *API) SetKaraokeIndicator(indicator string) error {
	kind := study.Indicator(indicator)
	if !kind.Valid() {
		return errBadSettings
	}
	if err := a.control(func(w *watching) error { return w.session.SetIndicator(kind) }); err != nil {
		return err
	}
	return a.keep(func(k *settings.KaraokePreferences) { k.Indicator = kind })
}

// SetLineRepeats sets how many times each line plays again (0 to 10).
func (a *API) SetLineRepeats(times int) error {
	if times < 0 || times > study.MaxRepeats {
		return errBadRepeat
	}
	if err := a.control(func(w *watching) error { return w.session.SetLineRepeats(times) }); err != nil {
		return err
	}
	return a.keep(func(k *settings.KaraokePreferences) { k.Repeat = times })
}

// keep saves a karaoke mode choice in the active profile.
func (a *API) keep(change func(*settings.KaraokePreferences)) error {
	if err := a.updateProfile(func(profile *settings.Profile) { change(&profile.Study.Karaoke) }); err != nil {
		a.logger.Warn("the karaoke mode choice was not saved", "error", err)
		return errBadSettings
	}
	return nil
}

// ReplayLine trains the line again.
func (a *API) ReplayLine() error {
	return a.control(func(w *watching) error { return w.session.ReplayLine() })
}

// StudyPassage starts studying a passage.
func (a *API) StudyPassage(request PassageRequest) error {
	start, err := study.ParseClock(request.Start)
	if err != nil {
		return errBadTime
	}
	end, err := study.ParseClock(request.End)
	if err != nil {
		return errBadTime
	}
	plan, err := practiceOf(PassagePlan{
		Loop: request.Loop, Gap: request.Gap, Progressive: request.Progressive, Ladder: request.Ladder, Goal: request.Goal,
	})
	if err != nil {
		return err
	}
	current := a.current()
	if current == nil || current.session == nil {
		return errNotWatching
	}
	if err := current.session.StudyPassage(study.Segment{Start: start, End: end}, plan, request.Snap); err != nil {
		if errors.Is(err, study.ErrInvalidSegment) {
			return errBadPassage
		}
		a.logger.Warn("the passage did not start", "error", err)
		return errors.New(ptbr.T("error.noResponse"))
	}
	a.emit(EventPlayback, a.describe(current))
	return nil
}

// StudySavedPassage studies a saved passage as it was kept: its times and
// how it is studied.
func (a *API) StudySavedPassage(name string) error {
	current := a.current()
	if current == nil || current.session == nil {
		return errNotWatching
	}
	a.mu.Lock()
	p := a.projectOf(current)
	var passage project.Passage
	i := -1
	if p != nil {
		i = project.IndexOf(p.value.Passages, name)
	}
	if i >= 0 {
		passage = p.value.Passages[i]
	}
	a.mu.Unlock()
	switch {
	case i < 0:
		return errUnknownPassage
	case passage.Outside(current.duration):
		return errPassageOutside
	}
	if err := current.session.StudySaved(passage.Segment(), passage.Practice()); err != nil {
		if errors.Is(err, study.ErrInvalidSegment) {
			return errBadPassage
		}
		a.logger.Warn("the saved passage did not start", "error", err)
		return errors.New(ptbr.T("error.noResponse"))
	}
	a.emit(EventPlayback, a.describe(current))
	return nil
}

// SetPassageOptions sets how the next passage is studied, the one the end
// key closes included: fitted to the lines or not, and the plan.
func (a *API) SetPassageOptions(options PassageOptions) error {
	plan, err := practiceOf(options.Plan)
	if err != nil {
		return err
	}
	if err := a.control(func(w *watching) error { return w.session.SetPassageOptions(plan, options.Snap) }); err != nil {
		return err
	}
	return nil
}

// practiceOf reads a plan from the window, refusing what the rules refuse.
func practiceOf(plan PassagePlan) (study.PracticePlan, error) {
	mode := study.EndPause
	if plan.Loop {
		mode = study.EndLoop
	}
	practice := study.PracticePlan{Mode: mode, Gap: secondsOf(plan.Gap), Progressive: plan.Progressive, Ladder: plan.Ladder, Goal: plan.Goal}
	if practice.Validate() != nil {
		return study.PracticePlan{}, errBadPlan
	}
	return practice, nil
}

// PassageOfLine is "Esta fala": the times of the line on top being spoken,
// or of the last one, fitted to the lines.
func (a *API) PassageOfLine() (PassageTimes, error) {
	current := a.current()
	if current == nil || current.session == nil {
		return PassageTimes{}, errNotWatching
	}
	segment, err := current.session.LinePassage()
	if errors.Is(err, watch.ErrNoLine) {
		return PassageTimes{}, errNoLineYet
	}
	if err != nil {
		return PassageTimes{}, err
	}
	return PassageTimes{Start: ptbr.PassageClock(segment.Start), End: ptbr.PassageClock(segment.End)}, nil
}

// StopPassage leaves the passage.
func (a *API) StopPassage() error {
	return a.control(func(w *watching) error {
		w.session.StopPassage()
		return nil
	})
}

// ResumePassage goes back into the passage after the video left it.
func (a *API) ResumePassage() error {
	// Checked first: control words every other failure as the player's.
	if current := a.current(); current != nil && current.session != nil && !current.session.Passage().Active {
		return errNoPassage
	}
	return a.control(func(w *watching) error { return w.session.ResumePassage() })
}

func describeKaraoke(s *watch.Session) Karaoke {
	return Karaoke{Enabled: s.Karaoke().Enabled, Indicator: string(s.Indicator()), Repeat: s.LineRepeats(), Voice: voiceMode(s)}
}

func describePassage(status session.PracticeStatus) Passage {
	if !status.Active {
		return Passage{}
	}
	return Passage{
		Active: true, Start: ptbr.PassageClock(status.Segment.Start), End: ptbr.PassageClock(status.Segment.End),
		Lap: status.Lap, Goal: status.Plan.Goal, Suspended: status.Suspended, AtEnd: status.AtEnd,
	}
}
