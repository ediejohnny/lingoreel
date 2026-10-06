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

// ffmpegTimeout bounds looking for FFmpeg from the settings screen.
const ffmpegTimeout = 10 * time.Second

var errNotFFmpeg = worded(func() string { return ptbr.T("error.ffmpeg.notFFmpeg") })

// voiceSetup is how a session follows the voice, from the settings.
func voiceSetup(v settings.VoicePreferences) watch.VoiceSetup {
	return watch.VoiceSetup{
		Model: v.Model, ModelOnBattery: v.ModelOnBattery, Follow: v.Follow, FFmpegPath: v.FFmpegPath,
		Settings: voice.Settings{Sensitivity: v.Sensitivity, Delay: time.Duration(v.Delay), EvenWhenBusy: v.EvenWhenBusy},
	}
}

// voiceView is the settings screen's part on the voice: the choices, the
// FFmpeg in use (looked for now) and whether one can be installed.
func (a *API) voiceView(ctx context.Context, v settings.VoicePreferences) VoiceSettings {
	view := VoiceSettings{
		Model: v.Model, ModelOnBattery: v.ModelOnBattery, Battery: a.deps.Ports.Power != nil && a.deps.Ports.Power.HasBattery(),
		Follow: v.Follow, Sensitivity: string(v.Sensitivity), DelayMs: int(time.Duration(v.Delay) / time.Millisecond),
		EvenWhenBusy: v.EvenWhenBusy, FFmpegPath: v.FFmpegPath,
	}
	if a.deps.FFmpeg == nil {
		return view
	}
	ctx, cancel := context.WithTimeout(ctx, ffmpegTimeout)
	defer cancel()
	if found, err := a.deps.FFmpeg.Find(ctx, v.FFmpegPath); err == nil {
		view.FFmpegFound = found
	}
	view.CanInstall = view.FFmpegFound == "" && a.deps.Installer != nil && a.deps.Installer.Available()
	return view
}

// SetVoice changes how the actors' voice is followed, at once and from then
// on; the FFmpeg chosen stays as it is.
func (a *API) SetVoice(view VoiceSettings) error {
	a.mu.Lock()
	next := a.config
	next.Voice.Model, next.Voice.ModelOnBattery = view.Model, view.ModelOnBattery
	next.Voice.Follow, next.Voice.EvenWhenBusy = view.Follow, view.EvenWhenBusy
	next.Voice.Sensitivity = speech.Sensitivity(view.Sensitivity)
	next.Voice.Delay = settings.Duration(time.Duration(view.DelayMs) * time.Millisecond)
	err := a.commitConfig(next)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	a.applyVoice()
	return nil
}

// ChooseFFmpeg asks where the user's FFmpeg is and keeps it once it runs;
// it returns the path, or empty when the user cancelled.
func (a *API) ChooseFFmpeg() (string, error) {
	ctx, err := a.context()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	ui := a.ui
	a.mu.Unlock()
	if ui == nil {
		return "", ErrNotReady
	}
	path, err := ui.OpenProgramDialog(ptbr.T("dialog.ffmpeg"))
	if err != nil || path == "" {
		return "", err
	}
	checked, cancel := context.WithTimeout(ctx, ffmpegTimeout)
	defer cancel()
	if a.deps.FFmpeg == nil || a.deps.FFmpeg.Check(checked, path) != nil {
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
	a.mu.Lock()
	next := a.config
	next.Voice.FFmpegPath = path
	err := a.commitConfig(next)
	a.mu.Unlock()
	if err != nil {
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
		return errors.New(ptbr.T("error.install.unavailable"))
	}
	go func() {
		progress := InstallProgress{ID: ffmpegInstallID, Done: true}
		if err := a.deps.Installer.Install(ctx, a.deps.FFmpeg.WingetID()); err != nil {
			a.logger.Warn("FFmpeg installation failed", "error", err)
			progress.Error = ptbr.T("error.install.failed")
		} else {
			a.applyVoice() // a video open finds it now
		}
		a.emit(EventInstall, progress)
	}()
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
