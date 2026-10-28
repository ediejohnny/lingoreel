package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
)

// someFFmpeg finds the one chosen, or one installed at found; a program is
// an FFmpeg when it is named so.
type someFFmpeg struct{ found string }

func (f someFFmpeg) Find(_ context.Context, chosen string) (string, error) {
	switch {
	case chosen != "":
		return chosen, nil
	case f.found != "":
		return f.found, nil
	}
	return "", errors.New("not found")
}

func (someFFmpeg) Check(_ context.Context, path string) error {
	if !strings.HasSuffix(strings.ToLower(path), `\ffmpeg.exe`) {
		return errors.New("not an FFmpeg")
	}
	return nil
}

func (someFFmpeg) WingetID() string { return "BtbN.FFmpeg.LGPL.8.1" }

// busyFFmpeg is an FFmpeg on a computer too busy to start it in time.
type busyFFmpeg struct{ someFFmpeg }

func (busyFFmpeg) Find(context.Context, string) (string, error) { return "", app.ErrFFmpegSlow }

func (busyFFmpeg) Check(context.Context, string) error { return app.ErrFFmpegSlow }

// An FFmpeg that did not answer in time is never told missing: the settings
// screen says it is slow and offers no installation, and one the person
// chose is kept (its first opening can outlast any wait while an antivirus
// reads it), never called "not an FFmpeg".
func TestAnFFmpegTooBusyToAnswerIsNotToldMissing(t *testing.T) {
	t.Parallel()
	missing := newRigWith(t, func(deps *app.Deps) { deps.FFmpeg = someFFmpeg{} })
	missing.attach(t)
	missing.ui.program = `D:\Tools\notepad.exe`
	_, notFFmpeg := missing.api.ChooseFFmpeg()
	if voice := settingsOf(t, missing).Voice; voice.FFmpegSlow || !voice.CanInstall || notFFmpeg == nil {
		t.Fatalf("with no FFmpeg: %+v, choosing another program: %v", voice, notFFmpeg)
	}

	r := newRigWith(t, func(deps *app.Deps) { deps.FFmpeg = busyFFmpeg{} })
	r.attach(t)
	if voice := settingsOf(t, r).Voice; !voice.FFmpegSlow || voice.CanInstall || voice.FFmpegFound != "" {
		t.Fatalf("an FFmpeg too busy to answer: %+v, want it slow and no installation offered", voice)
	}
	r.ui.program = `D:\Tools\ffmpeg.exe`
	path, err := r.api.ChooseFFmpeg()
	if err != nil || path != r.ui.program || r.settings.config.Voice.FFmpegPath != path {
		t.Fatalf("choosing an FFmpeg slow to answer = %q, %v, kept %q; want it kept", path, err, r.settings.config.Voice.FFmpegPath)
	}
}

// The voice settings are the computer's: kept at once, an FFmpeg chosen only
// once it runs, and a delay out of range refused with nothing changed.
func TestTheVoiceSettings(t *testing.T) {
	t.Parallel()
	r := newRigWith(t, func(deps *app.Deps) { deps.FFmpeg = someFFmpeg{found: `C:\ffmpeg\bin\ffmpeg.exe`} })
	r.attach(t)
	voice := settingsOf(t, r).Voice
	if !voice.Model || voice.ModelOnBattery || !voice.Follow || voice.Sensitivity != "normal" || voice.FFmpegFound != `C:\ffmpeg\bin\ffmpeg.exe` || voice.FFmpegPath != "" {
		t.Fatalf("first voice settings = %+v", voice)
	}

	voice.Follow, voice.Sensitivity, voice.DelayMs, voice.EvenWhenBusy = false, "high", 200, true
	if err := r.api.SetVoice(voice); err != nil {
		t.Fatal(err)
	}
	if kept := r.settings.config.Voice; kept.Follow || kept.Sensitivity != speech.SensitivityHigh ||
		time.Duration(kept.Delay) != 200*time.Millisecond || !kept.EvenWhenBusy {
		t.Fatalf("kept voice settings = %+v", kept)
	}
	voice.DelayMs = 5000
	if err := r.api.SetVoice(voice); err == nil || time.Duration(r.settings.config.Voice.Delay) != 200*time.Millisecond {
		t.Fatalf("a delay of 5 s = %v, kept %v", err, r.settings.config.Voice.Delay)
	}

	r.ui.program = `D:\Tools\notepad.exe`
	if path, err := r.api.ChooseFFmpeg(); err == nil || path != "" || r.settings.config.Voice.FFmpegPath != "" {
		t.Fatalf("choosing a program that is not FFmpeg = %q, %v", path, err)
	}
	r.ui.program = `D:\Tools\ffmpeg.exe`
	if path, err := r.api.ChooseFFmpeg(); err != nil || path != r.ui.program || r.settings.config.Voice.FFmpegPath != path {
		t.Fatalf("choosing FFmpeg = %q, %v", path, err)
	}
	if found := settingsOf(t, r).Voice.FFmpegFound; found != `D:\Tools\ffmpeg.exe` {
		t.Fatalf("the FFmpeg in use is %q, want the one chosen", found)
	}
	if err := r.api.ClearFFmpeg(); err != nil || r.settings.config.Voice.FFmpegPath != "" {
		t.Fatalf("ClearFFmpeg = %v, kept %q", err, r.settings.config.Voice.FFmpegPath)
	}
}
