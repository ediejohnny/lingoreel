package app

import (
	"errors"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/presentation/ptbr"
)

var (
	errBadTime     = errors.New("Use horas, minutos e segundos, como 1:10:23 ou 10:23.")
	errBadPassage  = errors.New("O trecho precisa ter pelo menos 1 segundo e terminar depois de começar, dentro do vídeo.")
	errBadPlan     = errors.New("Confira o intervalo (0 a 5 s) e a meta de repetições (0 a 99).")
	errNoLineYet   = errors.New("Nenhuma fala apareceu ainda.")
	errNoPassage   = errors.New("Nenhum trecho está sendo estudado.")
	errBadRepeat   = errors.New("Escolha de 0 a 10 repetições.")
	errBadSettings = errors.New("Não foi possível guardar a escolha.")
)

// SetKaraoke turns the Modo Karaokê on or off; the choice is kept in the
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

// keep saves a Modo Karaokê choice in the active profile.
func (a *API) keep(change func(*settings.KaraokePreferences)) error {
	if err := a.updateProfile(func(profile *settings.Profile) { change(&profile.Study.Karaoke) }); err != nil {
		a.logger.Warn("the Modo Karaokê choice was not saved", "error", err)
		return errBadSettings
	}
	return nil
}

// ReplayLine is "Treinar de novo".
func (a *API) ReplayLine() error {
	return a.control(func(w *watching) error { return w.session.ReplayLine() })
}

// StudyPassage starts "Estudar trecho".
func (a *API) StudyPassage(request PassageRequest) error {
	start, err := study.ParseClock(request.Start)
	if err != nil {
		return errBadTime
	}
	end, err := study.ParseClock(request.End)
	if err != nil {
		return errBadTime
	}
	mode := study.EndPause
	if request.Loop {
		mode = study.EndLoop
	}
	plan := study.PracticePlan{
		Mode: mode, Gap: secondsOf(request.Gap), Progressive: request.Progressive, Ladder: request.Ladder, Goal: request.Goal,
	}
	if plan.Validate() != nil {
		return errBadPlan
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
		return errors.New("O player não respondeu ao comando.")
	}
	a.emit(EventPlayback, a.describe(current))
	return nil
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
	return Karaoke{Enabled: s.Karaoke().Enabled, Indicator: string(s.Indicator()), Repeat: s.LineRepeats()}
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
