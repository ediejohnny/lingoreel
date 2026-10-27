package watch

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// DefaultPrecisionStep is the precision shortcuts' jump when none is given.
const DefaultPrecisionStep = 100 * time.Millisecond

// ErrNotRunnable reports a command the session does not run: the player
// window takes care of the fullscreen, and the translation key goes through
// Reveal, which hears its release too.
var ErrNotRunnable = errors.New("command not run by the session")

// Run carries out a shortcut's command, the same way for the player
// window, the app's panel and the sync shortcuts; repeat tells the key is
// held, which makes a sync step the large one.
func (s *Session) Run(command shortcut.Command, repeat bool) error {
	if jump, jumps := command.Jump(); jumps {
		return s.controls.SeekBy(jump)
	}
	if span, rewinds := command.Rewind(); rewinds {
		return s.playFrom(s.controls.Position() - span)
	}
	if index := slices.Index(syncCommands, command); index >= 0 {
		target := SyncShortcuts[index]
		s.nudge(target.Line, target.Later, repeat)
		return nil
	}
	switch command {
	case shortcut.PlayPause:
		return s.controls.TogglePause()
	case shortcut.VolumeUp, shortcut.VolumeDown:
		return s.tellVolume(s.controls.StepVolume(s.now(), command == shortcut.VolumeUp, repeat))
	case shortcut.Mute:
		return s.tellVolume(s.controls.ToggleMute())
	case shortcut.PrecisionBack:
		return s.controls.SeekBy(-s.PrecisionStep())
	case shortcut.PrecisionForward:
		return s.controls.SeekBy(s.PrecisionStep())
	case shortcut.FrameBack:
		return s.controls.StepFrame(false)
	case shortcut.FrameForward:
		return s.controls.StepFrame(true)
	case shortcut.SpeedDown:
		return s.controls.Slower()
	case shortcut.SpeedUp:
		return s.controls.Faster()
	case shortcut.SpeedNormal:
		return s.controls.NormalSpeed()
	case shortcut.ShowBoth:
		s.SetTranslationVisible(true)
		return nil
	case shortcut.PrimaryOnly:
		s.SetTranslationVisible(false)
		return nil
	case shortcut.ReplayLine:
		return s.ReplayLine()
	case shortcut.RepeatPhrase:
		return s.RepeatPhrase()
	case shortcut.PassageMarkStart:
		s.MarkPassageStart()
		return nil
	case shortcut.PassageMarkEnd:
		_, err := s.MarkPassageEnd()
		// A missing or too close start is told on the video, not an error.
		if errors.Is(err, ErrNoPassageStart) || errors.Is(err, study.ErrInvalidSegment) {
			return nil
		}
		return err
	case shortcut.PassageStop:
		s.StopPassage()
		return nil
	case shortcut.PassageResume:
		if err := s.ResumePassage(); !errors.Is(err, session.ErrNoPassage) {
			return err
		}
		return nil
	case shortcut.Undo:
		_, err := s.Undo()
		return err
	case shortcut.Redo:
		_, err := s.Redo()
		return err
	case shortcut.Fullscreen, shortcut.ExitFullscreen, shortcut.Translation:
		return fmt.Errorf("%w: %s", ErrNotRunnable, command)
	}
	return fmt.Errorf("%w: %q", shortcut.ErrUnknownCommand, command)
}

// RepeatPhrase goes back to the start of the line on top being spoken, or
// the last one, and plays it ("Repetir frase"). Before the first line, or
// without a line on top, there is nothing to repeat.
func (s *Session) RepeatPhrase() error {
	track := s.subtitles.Track(session.LinePrimary)
	if track == nil {
		return nil
	}
	start, found := study.PhraseStart(track.Timeline, track.Shown(), s.controls.Position())
	if !found {
		return nil
	}
	return s.playFrom(start)
}

// playFrom seeks and plays from there, so what was missed is heard again.
func (s *Session) playFrom(position time.Duration) error {
	if err := s.controls.SeekTo(position); err != nil {
		return err
	}
	if s.clock.State().Status == playback.StatusPlaying {
		return nil
	}
	return s.controls.Play()
}

// tellVolume shows the volume on the video after a change that worked, with
// its level on a bar marked at 100% when the display can draw one.
func (s *Session) tellVolume(err error) error {
	if err != nil || s.display == nil || s.volumeNotice == nil {
		return err
	}
	volume, muted := s.controls.Volume()
	ceiling := s.controls.Capabilities().VolumeCeiling()
	text := s.volumeNotice(volume, ceiling, muted)
	if text == "" {
		return nil
	}
	// A lost notice is cosmetic.
	meters, ok := s.display.(session.MeterDisplay)
	if !ok || ceiling <= 0 {
		_ = s.display.Notify(text)
		return nil
	}
	meter := session.Meter{Level: float64(volume / ceiling), Mark: float64(playback.FullVolume / ceiling)}
	if muted {
		meter.Level = 0
	}
	_ = meters.NotifyLevel(text, meter)
	return nil
}
