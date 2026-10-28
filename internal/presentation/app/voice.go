package app

import (
	"context"
	"errors"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// FFmpeg is the user's own FFmpeg, as the app finds it: the app ships none.
type FFmpeg interface {
	// Find returns an FFmpeg that runs: the one chosen, else one installed
	// where FFmpeg usually is.
	Find(ctx context.Context, chosen string) (string, error)
	// Check tells whether a program is an FFmpeg that runs.
	Check(ctx context.Context, path string) error
	// WingetID is the package the app offers to install, from its authors.
	WingetID() string
}

// ffmpegTimeout bounds looking for FFmpeg from the settings screen;
// chosenTimeout bounds checking the one the person chose, whose first
// opening can take seconds while an antivirus reads a large file.
const (
	ffmpegTimeout = 10 * time.Second
	chosenTimeout = time.Minute
)

// ErrFFmpegSlow is what an FFmpeg reports when it did not answer in time, as
// one opened for the first time or on a busy computer: it may well be there.
var ErrFFmpegSlow = errors.New("ffmpeg: did not answer in time")

var errNotFFmpeg = worded(func() string { return ptbr.T("error.ffmpeg.notFFmpeg") })

// voiceSetup is how a session follows the voice, from the settings.
func voiceSetup(v settings.VoicePreferences) watch.VoiceSetup {
	return watch.VoiceSetup{
		Model: v.Model, ModelOnBattery: v.ModelOnBattery, Follow: v.Follow, Sync: v.Sync, FFmpegPath: v.FFmpegPath,
		Settings: voice.Settings{Sensitivity: v.Sensitivity, Delay: time.Duration(v.Delay), EvenWhenBusy: v.EvenWhenBusy},
	}
}

// voiceView is the settings screen's part on the voice: the choices, the
// FFmpeg in use (looked for now) and whether one can be installed; an FFmpeg
// that did not answer in time is told slow, and none is offered to install.
func (a *API) voiceView(ctx context.Context, v settings.VoicePreferences) VoiceSettings {
	view := VoiceSettings{
		Model: v.Model, ModelOnBattery: v.ModelOnBattery, Battery: a.deps.Ports.Power != nil && a.deps.Ports.Power.HasBattery(),
		Follow: v.Follow, Sync: v.Sync, Sensitivity: string(v.Sensitivity), DelayMs: int(time.Duration(v.Delay) / time.Millisecond),
		EvenWhenBusy: v.EvenWhenBusy, FFmpegPath: v.FFmpegPath, Weak: a.machine().Weak(),
	}
	if a.deps.FFmpeg == nil {
		return view
	}
	ctx, cancel := context.WithTimeout(ctx, ffmpegTimeout)
	defer cancel()
	switch found, err := a.deps.FFmpeg.Find(ctx, v.FFmpegPath); {
	case err == nil:
		view.FFmpegFound = found
	case errors.Is(err, ErrFFmpegSlow):
		view.FFmpegSlow = true
	}
	view.CanInstall = view.FFmpegFound == "" && !view.FFmpegSlow && a.deps.Installer != nil && a.deps.Installer.Available()
	return view
}

// SetVoice changes how the actors' voice is followed, at once and from then
// on; the FFmpeg chosen stays as it is.
func (a *API) SetVoice(view VoiceSettings) error {
	if err := a.changeConfig(func(next *settings.Config) error {
		next.Voice.Model, next.Voice.ModelOnBattery = view.Model, view.ModelOnBattery
		next.Voice.Follow, next.Voice.Sync, next.Voice.EvenWhenBusy = view.Follow, view.Sync, view.EvenWhenBusy
		next.Voice.Sensitivity = speech.Sensitivity(view.Sensitivity)
		next.Voice.Delay = settings.Duration(time.Duration(view.DelayMs) * time.Millisecond)
		return nil
	}); err != nil {
		return err
	}
	a.applyVoice()
	return nil
}

// ChooseFFmpeg asks where the user's FFmpeg is and keeps it once it runs;
// it returns the path, or empty when the user cancelled. One that has not
// answered within chosenTimeout is kept too: it is checked again where it is
// used, and a program that is not an FFmpeg is refused at once.
func (a *API) ChooseFFmpeg() (string, error) {
	ctx, err := a.context()
	if err != nil {
		return "", err
	}
	ui := a.window()
	if ui == nil {
		return "", ErrNotReady
	}
	path, err := ui.OpenProgramDialog(ptbr.T("dialog.ffmpeg"))
	if err != nil || path == "" {
		return "", err
	}
	checked, cancel := context.WithTimeout(ctx, chosenTimeout)
	defer cancel()
	if a.deps.FFmpeg == nil {
		return "", errNotFFmpeg
	}
	if err := a.deps.FFmpeg.Check(checked, path); err != nil && !errors.Is(err, ErrFFmpegSlow) {
		return "", errNotFFmpeg
	}
	if err := a.keepFFmpeg(path); err != nil {
		return "", err
	}
	return path, nil
}

// ClearFFmpeg forgets the FFmpeg chosen: the app looks for one again.
func (a *API) ClearFFmpeg() error {
	return a.keepFFmpeg("")
}

func (a *API) keepFFmpeg(path string) error {
	if err := a.changeConfig(func(next *settings.Config) error {
		next.Voice.FFmpegPath = path
		return nil
	}); err != nil {
		return err
	}
	a.applyVoice()
	return nil
}

// InstallFFmpeg installs, with winget, the FFmpeg the app offers, in the
// background; the window hears the result through the player:install event,
// as "ffmpeg".
func (a *API) InstallFFmpeg() error {
	ctx, err := a.context()
	if err != nil {
		return err
	}
	if a.deps.FFmpeg == nil || a.deps.Installer == nil || !a.deps.Installer.Available() {
		return errors.New(ptbr.T("error.install.ffmpegUnavailable"))
	}
	// A video open finds the FFmpeg installed at once.
	a.installInBackground(ctx, ffmpegInstallID, a.deps.FFmpeg.WingetID(), a.applyVoice)
	return nil
}

// ffmpegInstallID names FFmpeg among the installations the window hears of.
const ffmpegInstallID = "ffmpeg"

// applyVoice gives the video being watched the voice settings.
func (a *API) applyVoice() {
	current := a.current()
	if current == nil || current.session == nil {
		return
	}
	a.mu.Lock()
	setup := voiceSetup(a.config.Voice)
	a.mu.Unlock()
	current.session.SetVoiceSetup(setup)
}

// voiceMode words how the voice of the line is followed now, for the panel:
// why the voice model rests, if it does; empty when it is not followed.
func voiceMode(s *watch.Session) string {
	mode, following := s.VoiceMode()
	switch rest := s.ModelRest(); {
	case !following:
		return ""
	case rest != watch.ModelAtWork:
		return string(rest)
	}
	return mode.String()
}
