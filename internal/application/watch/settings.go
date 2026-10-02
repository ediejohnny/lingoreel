package watch

import (
	"errors"
	"fmt"
	"slices"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
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
// hides the translation as the reveal behavior says.
func (s *Session) Reveal(pressed bool) {
	if !s.hasTranslation {
		return
	}
	visible := s.TranslationVisible()
	if revealed := s.RevealBehavior().Revealed(visible, pressed); revealed != visible {
		s.SetTranslationVisible(revealed)
	}
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
	if s.restart == nil || slices.Equal(SyncChords(previous), SyncChords(resolved)) {
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
	next, err := s.restart(SyncChords(resolved))
	if err != nil {
		s.shortcuts = nil
		return errors.Join(closeErr, fmt.Errorf("starting the sync shortcuts again: %w", err))
	}
	s.shortcuts = next
	go s.listenShortcuts(next)
	return closeErr
}

// LookOf is how a profile draws the subtitles.
func LookOf(profile settings.Profile) session.Look {
	return session.Look{
		Layout: profile.Layout, Primary: profile.Primary, Translation: profile.Translation,
		PreviewColor: profile.Study.Karaoke.PreviewColor, FillColor: profile.Study.Karaoke.FillColor,
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
