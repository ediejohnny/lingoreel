package watch_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
)

// at puts a paused video at a position at the normal speed, and forgets
// the calls that took.
func at(t *testing.T, r *rig, s *watch.Session, position time.Duration) {
	t.Helper()
	controls := s.Controls()
	if err := errors.Join(controls.SeekTo(position), controls.Pause(), controls.SetSpeed(playback.NormalSpeed)); err != nil {
		t.Fatal(err)
	}
	r.running.took()
}

// playingRig is a session whose player stepped frames and told its length.
func playingRig(t *testing.T) (*rig, *watch.Session) {
	t.Helper()
	r := newRig(t)
	r.running.frames = true
	s := r.start(t)
	r.running.samples <- playback.Sample{Position: 20 * time.Second, Duration: time.Minute, Status: playback.StatusPaused, Speed: 1}
	eventually(t, "the first sample", func() bool { return s.State().Duration == time.Minute })
	return r, s
}

func TestEveryCommandAsksThePlayer(t *testing.T) {
	r, s := playingRig(t)
	// From 20 s, paused, at 1×. The only line on top is 1 s–3 s, so the
	// phrase starts at 1 s minus the 0.5 s lead-in: 0.5 s.
	cases := []struct {
		command shortcut.Command
		want    []string
	}{
		{shortcut.PlayPause, []string{"play"}},
		{shortcut.SeekBack, []string{"seek 15s"}},
		{shortcut.SeekForward, []string{"seek 25s"}},
		{shortcut.SeekBack10, []string{"seek 10s"}},
		{shortcut.SeekForward10, []string{"seek 30s"}},
		{shortcut.SeekBack30, []string{"seek 0s"}}, // never before the start
		{shortcut.SeekForward30, []string{"seek 50s"}},
		{shortcut.PrecisionBack, []string{"seek 19.9s"}},
		{shortcut.PrecisionForward, []string{"seek 20.1s"}},
		{shortcut.FrameBack, []string{"frame back"}},
		{shortcut.FrameForward, []string{"frame forward"}},
		{shortcut.SpeedDown, []string{"speed 0.9"}},
		{shortcut.SpeedUp, []string{"speed 1.1"}},
		{shortcut.RepeatLast3, []string{"seek 17s", "play"}},
		{shortcut.RepeatLast5, []string{"seek 15s", "play"}},
		{shortcut.RepeatLast10, []string{"seek 10s", "play"}},
		{shortcut.RepeatPhrase, []string{"seek 500ms", "play"}},
	}
	for _, c := range cases {
		at(t, r, s, 20*time.Second)
		if err := s.Run(c.command, false); err != nil {
			t.Fatalf("%s: %v", c.command, err)
		}
		if got := r.running.took(); !slices.Equal(got, c.want) {
			t.Errorf("%s asked %q, want %q", c.command, got, c.want)
		}
	}

	// Back to 1× from another speed.
	at(t, r, s, 20*time.Second)
	if err := errors.Join(s.Controls().SetSpeed(0.75), s.Run(shortcut.SpeedNormal, false)); err != nil {
		t.Fatal(err)
	}
	if got := r.running.took(); !slices.Equal(got, []string{"speed 0.75", "speed 1"}) {
		t.Errorf("1× asked %q", got)
	}

	// Playing already, a repeat only seeks.
	at(t, r, s, 20*time.Second)
	if err := errors.Join(s.Controls().Play(), s.Run(shortcut.RepeatLast3, false)); err != nil {
		t.Fatal(err)
	}
	if got := r.running.took(); !slices.Equal(got, []string{"play", "seek 17s"}) {
		t.Errorf("repeat while playing asked %q", got)
	}
}

func TestCommandsTheSessionDoesNotRun(t *testing.T) {
	r, s := playingRig(t)
	at(t, r, s, 20*time.Second)
	for _, command := range []shortcut.Command{shortcut.Fullscreen, shortcut.ExitFullscreen, shortcut.Translation} {
		if err := s.Run(command, false); !errors.Is(err, watch.ErrNotRunnable) {
			t.Errorf("%s: %v, want ErrNotRunnable", command, err)
		}
	}
	if err := s.Run("dance", false); !errors.Is(err, shortcut.ErrUnknownCommand) {
		t.Errorf("unknown command: %v", err)
	}
	// The fake player has no volume: the controls say so.
	if err := s.Run(shortcut.Mute, false); !errors.Is(err, session.ErrNotSupported) {
		t.Errorf("mute: %v, want ErrNotSupported", err)
	}
	if got := r.running.took(); len(got) != 0 {
		t.Errorf("the player was asked %q", got)
	}
	// Every known command is either run or refused on purpose.
	for _, command := range shortcut.Commands() {
		at(t, r, s, 20*time.Second)
		if err := s.Run(command, false); errors.Is(err, shortcut.ErrUnknownCommand) {
			t.Errorf("%s is not handled", command)
		}
	}
}

func TestRepeatPhraseBeforeTheFirstLine(t *testing.T) {
	r, s := playingRig(t)
	at(t, r, s, 500*time.Millisecond) // the first line starts at 1 s
	if err := s.Run(shortcut.RepeatPhrase, false); err != nil {
		t.Fatal(err)
	}
	if got := r.running.took(); len(got) != 0 {
		t.Fatalf("asked %q before any line", got)
	}
	// Inside the line, it goes to its start too.
	at(t, r, s, 2*time.Second)
	if err := s.Run(shortcut.RepeatPhrase, false); err != nil {
		t.Fatal(err)
	}
	if got := r.running.took(); !slices.Equal(got, []string{"seek 500ms", "play"}) {
		t.Fatalf("inside the line asked %q", got)
	}
}

func TestTheSyncCommandsStepTheLines(t *testing.T) {
	r, s := playingRig(t)
	if err := s.Run(shortcut.SyncTranslationEarlier, false); err != nil {
		t.Fatal(err)
	}
	if s.Offset(session.LineTranslation) != -subtitle.Offset(subtitle.DefaultNudgeStep) {
		t.Fatalf("translation offset = %v", s.Offset(session.LineTranslation))
	}
	if err := s.Run(shortcut.SyncPrimaryLater, false); err != nil {
		t.Fatal(err)
	}
	if s.Offset(session.LinePrimary) != subtitle.Offset(subtitle.DefaultNudgeStep) {
		t.Fatalf("primary offset = %v", s.Offset(session.LinePrimary))
	}
	if _, _, notices, _ := r.display.last(); len(notices) != 2 {
		t.Fatalf("notices = %v, want two", notices)
	}
}

func TestThePrecisionStep(t *testing.T) {
	r, s := playingRig(t)
	if s.PrecisionStep() != watch.DefaultPrecisionStep {
		t.Fatalf("default step = %v", s.PrecisionStep())
	}
	if err := s.SetPrecisionStep(250 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	for _, step := range []time.Duration{0, -time.Millisecond} {
		if err := s.SetPrecisionStep(step); err == nil {
			t.Errorf("step %v was taken", step)
		}
	}
	at(t, r, s, 20*time.Second)
	if err := s.Run(shortcut.PrecisionForward, false); err != nil {
		t.Fatal(err)
	}
	if got := r.running.took(); !slices.Equal(got, []string{"seek 20.25s"}) {
		t.Fatalf("asked %q", got)
	}

	// The request's step, and a negative one refused before the player opens.
	given := newRig(t)
	given.request.PrecisionStep = time.Second
	if step := given.start(t).PrecisionStep(); step != time.Second {
		t.Fatalf("requested step = %v", step)
	}
	refused := newRig(t)
	refused.request.PrecisionStep = -time.Second
	if _, err := watch.Start(t.Context(), refused.ports, refused.request, nil, nil); err == nil || len(refused.plugin.requests) != 0 {
		t.Fatalf("Start = %v, %d launches", err, len(refused.plugin.requests))
	}
}

func TestTheStudyModes(t *testing.T) {
	start := func(mode study.Mode) (*rig, *watch.Session) {
		r := newRig(t)
		r.request.Mode = mode
		return r, r.start(t)
	}

	// No mode is dual: both lines, and the key hides and shows the
	// translation in turns, even when the profile holds it.
	_, dual := start("")
	if dual.Mode() != study.ModeDual || !dual.TranslationVisible() {
		t.Fatalf("mode %q, visible %v", dual.Mode(), dual.TranslationVisible())
	}
	if err := dual.SetRevealBehavior(study.RevealHold); err != nil {
		t.Fatal(err)
	}
	dual.Reveal(true)
	if dual.TranslationVisible() {
		t.Fatal("dual: the press itself did not hide the translation")
	}
	dual.Reveal(false)
	if dual.TranslationVisible() {
		t.Fatal("dual: the release brought the translation back")
	}
	dual.Reveal(true)
	if !dual.TranslationVisible() {
		t.Fatal("dual: a second press did not show it")
	}

	// Always shown: nothing hides it.
	always, alwaysOn := start(study.ModeAlwaysTranslated)
	alwaysOn.Reveal(true)
	alwaysOn.SetTranslationVisible(false)
	if err := alwaysOn.Run(shortcut.PrimaryOnly, false); err != nil {
		t.Fatal(err)
	}
	if !alwaysOn.TranslationVisible() {
		t.Fatal("the translation was hidden in the mode that always shows it")
	}
	always.running.samples <- playback.Sample{Position: 2 * time.Second, Duration: time.Minute, Status: playback.StatusPaused, Speed: 1}
	eventually(t, "a screen", func() bool { _, shown, _, _ := always.display.last(); return shown > 0 })
	if screen, _, _, _ := always.display.last(); !screen.TranslationVisible {
		t.Fatal("the screen hid the translation")
	}

	// On demand, Mostrar ambas and Só a legenda de cima set it either way.
	_, onDemand := start(study.ModeOnDemand)
	if err := onDemand.Run(shortcut.ShowBoth, false); err != nil || !onDemand.TranslationVisible() {
		t.Fatalf("show both: %v, visible %v", err, onDemand.TranslationVisible())
	}
	if err := onDemand.Run(shortcut.PrimaryOnly, false); err != nil || onDemand.TranslationVisible() {
		t.Fatalf("primary only: %v, visible %v", err, onDemand.TranslationVisible())
	}

	// An unknown mode never opens the player.
	refused := newRig(t)
	refused.request.Mode = "passive"
	if _, err := watch.Start(t.Context(), refused.ports, refused.request, nil, nil); err == nil || len(refused.plugin.requests) != 0 {
		t.Fatalf("Start = %v, %d launches", err, len(refused.plugin.requests))
	}
}

func TestChangingTheModeWhilePlaying(t *testing.T) {
	r, s := playingRig(t) // dual
	eventually(t, "a screen", func() bool { _, shown, _, _ := r.display.last(); return shown > 0 })
	if err := s.SetMode(study.ModeOnDemand); err != nil {
		t.Fatal(err)
	}
	if s.Mode() != study.ModeOnDemand || s.TranslationVisible() {
		t.Fatalf("mode %q, visible %v", s.Mode(), s.TranslationVisible())
	}
	if screen, _, _, _ := r.display.last(); screen.TranslationVisible {
		t.Fatal("the screen still shows the translation")
	}
	if err := s.SetMode(study.ModeAlwaysTranslated); err != nil || !s.TranslationVisible() {
		t.Fatalf("always: %v, visible %v", err, s.TranslationVisible())
	}
	if err := s.SetMode("passive"); err == nil || s.Mode() != study.ModeAlwaysTranslated {
		t.Fatalf("an unknown mode was taken: %v, %q", err, s.Mode())
	}
}
