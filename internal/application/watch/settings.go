package watch

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// The changes the settings screen makes while a video plays: each one
// reaches the video at once, without opening it again.

// startSpeed puts the video at the profile's speed when it opens; a player
// without speed control keeps its own.
func (s *Session) startSpeed(speed playback.Speed) error {
	if speed == 0 || speed == playback.NormalSpeed {
		return nil
	}
	err := s.controls.SetSpeed(speed)
	if errors.Is(err, session.ErrNotSupported) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("starting at %v×: %w", speed, err)
	}
	return nil
}

// Reveal is the translation key going down (pressed) or up: it shows or
// hides the translation as the study mode and the reveal behavior say.
func (s *Session) Reveal(pressed bool) {
	if !s.hasTranslation.Load() {
		return
	}
	visible := s.TranslationVisible()
	if revealed := s.Mode().Reveal(s.RevealBehavior(), visible, pressed); revealed != visible {
		s.SetTranslationVisible(revealed)
	}
}

// Mode is the study mode.
func (s *Session) Mode() study.Mode {
	mode, _ := s.mode.Load().(study.Mode)
	return mode
}

// SetMode changes the study mode while the video plays: the translation
// shows or hides as the mode starts.
func (s *Session) SetMode(mode study.Mode) error {
	if !mode.Valid() {
		return fmt.Errorf("unknown study mode %q", mode)
	}
	s.mode.Store(mode)
	s.translationVisible.Store(mode.StartsRevealed())
	s.redraw()
	return nil
}

// PrecisionStep is the jump of the precision shortcuts.
func (s *Session) PrecisionStep() time.Duration {
	return time.Duration(s.precision.Load())
}

// SetPrecisionStep changes the jump of the precision shortcuts.
func (s *Session) SetPrecisionStep(step time.Duration) error {
	if step <= 0 {
		return fmt.Errorf("precision step %v is not positive", step)
	}
	s.precision.Store(int64(step))
	return nil
}

// RevealBehavior is how the translation key works.
func (s *Session) RevealBehavior() study.RevealBehavior {
	behavior, _ := s.reveal.Load().(study.RevealBehavior)
	return behavior
}

// SetRevealBehavior changes how the translation key works.
func (s *Session) SetRevealBehavior(behavior study.RevealBehavior) error {
	if !behavior.Valid() {
		return fmt.Errorf("unknown reveal behavior %q", behavior)
	}
	s.reveal.Store(behavior)
	return nil
}

// Shortcuts are the keys of the commands, every command included.
func (s *Session) Shortcuts() shortcut.Map {
	if keys := s.keys.Load(); keys != nil {
		return *keys
	}
	return shortcut.Defaults()
}

// SetShortcuts changes the keys: the player window's at once, and the sync
// shortcuts, started again on their new chords when those changed.
func (s *Session) SetShortcuts(keys shortcut.Map) error {
	if err := keys.Validate(); err != nil {
		return err
	}
	resolved := keys.Resolved()
	previous := s.Shortcuts()
	s.keys.Store(&resolved)
	if window, isWindow := s.display.(Window); isWindow {
		window.SetShortcuts(resolved)
	}
	if s.restart == nil || slices.Equal(GlobalChords(previous), GlobalChords(resolved)) {
		return nil
	}
	s.shortcutsMu.Lock()
	defer s.shortcutsMu.Unlock()
	if s.ending {
		return nil // the session is ending: no new shortcuts to close
	}
	var closeErr error
	if s.shortcuts != nil {
		closeErr = s.shortcuts.Close()
	}
	next, err := s.restart(GlobalChords(resolved))
	if err != nil {
		s.shortcuts = nil
		return errors.Join(closeErr, fmt.Errorf("starting the global shortcuts again: %w", err))
	}
	s.shortcuts = next
	go s.listenShortcuts(next)
	return closeErr
}

// LookOf is how a profile draws the subtitles and the marks around them. The
// Modo Karaokê's fill is painted over the line on top, so a fill too close to
// that line's color gives way to one that shows (turning it off is a setting
// of its own).
func LookOf(profile settings.Profile) session.Look {
	karaoke, marks := profile.Study.Karaoke, profile.Marks
	fill, _ := appearance.ReadableFill(profile.Primary.Color, karaoke.FillColor)
	return session.Look{
		Layout: profile.Layout, Primary: profile.Primary, Translation: profile.Translation,
		PreviewColor: karaoke.PreviewColor, FillColor: fill, Cue: karaoke.Cue,
		Notice: marks.Notice, NoticeDuration: time.Duration(marks.NoticeDuration), Selection: marks.Selection,
	}
}

// Restyle draws the subtitles with a new look, when the display can.
func (s *Session) Restyle(look session.Look) error {
	restyler, can := s.display.(session.Restyler)
	if !can {
		return nil
	}
	return restyler.Restyle(look)
}

// ApplyKaraoke replaces the Modo Karaokê's state, its rules and its fill.
func (s *Session) ApplyKaraoke(karaoke session.Karaoke) error {
	if err := karaoke.Rules.Validate(); err != nil {
		return err
	}
	s.subtitles.SetKaraoke(karaoke)
	return nil
}

// SetSyncNotice shows or leaves out the notice of a sync change.
func (s *Session) SetSyncNotice(show bool) {
	s.quiet.Store(!show)
}

// SetSyncSteps changes the steps of the sync shortcuts and buttons.
func (s *Session) SetSyncSteps(steps subtitle.NudgeSteps) error {
	return s.sync.SetSteps(steps)
}
