package watch_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
)

func TestTheVideoStartsAtTheProfileSpeed(t *testing.T) {
	r := newRig(t)
	r.request.Speed = 0.75
	r.start(t)
	r.running.mu.Lock()
	speeds := slices.Clone(r.running.speeds)
	r.running.mu.Unlock()
	if len(speeds) != 1 || speeds[0] != 0.75 {
		t.Fatalf("speeds asked = %v, want [0.75]", speeds)
	}
	// The normal speed, or none, asks nothing.
	for _, speed := range []playback.Speed{0, 1} {
		plain := newRig(t)
		plain.request.Speed = speed
		plain.start(t)
		if len(plain.running.speeds) != 0 {
			t.Fatalf("speed %v asked %v", speed, plain.running.speeds)
		}
	}
	// A player that refuses the speed ends the start, closing it.
	failing := newRig(t)
	failing.request.Speed = 1.25
	failing.running.speedErr = errors.New("no answer")
	if _, err := watch.Start(t.Context(), failing.ports, failing.request, nil, nil); err == nil || failing.running.closes() != 1 {
		t.Fatalf("Start = %v, closes %d", err, failing.running.closes())
	}
}

func TestTheSyncShortcutsUseTheProfileChords(t *testing.T) {
	r, window, _ := hostedRig(t)
	r.request.Shortcuts = shortcut.Map{shortcut.SyncPrimaryEarlier: {Modifiers: shortcut.Ctrl | shortcut.Shift, Key: "KeyJ"}}
	s := r.start(t)
	want := []string{"Ctrl+Shift+KeyJ", "Shift+Period", "Ctrl+Comma", "Ctrl+Period"}
	if got := chordTexts(r.chords[0]); !slices.Equal(got, want) {
		t.Fatalf("chords = %v, want %v", got, want)
	}
	window.mu.Lock()
	keys := window.keys
	window.mu.Unlock()
	// The window gets every command, the defaults included.
	if keys.Of(shortcut.SyncPrimaryEarlier).Key != "KeyJ" || len(keys) != len(shortcut.Commands()) {
		t.Fatalf("window keys = %+v", keys)
	}
	if s.Shortcuts().Of(shortcut.PlayPause).Key != shortcut.KeySpace {
		t.Fatalf("session shortcuts = %+v", s.Shortcuts())
	}
}

func TestChangingTheShortcutsWhilePlaying(t *testing.T) {
	r, window, _ := hostedRig(t)
	s := r.start(t)

	// A key of the window only: the sync shortcuts stay as they are.
	if err := s.SetShortcuts(shortcut.Map{shortcut.Mute: {Key: "KeyU"}}); err != nil {
		t.Fatal(err)
	}
	window.mu.Lock()
	mute := window.keys.Of(shortcut.Mute)
	window.mu.Unlock()
	if mute.Key != "KeyU" || len(r.chords) != 1 || r.shortcuts.closed {
		t.Fatalf("window mute %v, %d starts, first closed %v", mute, len(r.chords), r.shortcuts.closed)
	}

	// A sync chord: the shortcuts start again on the new chords, and the
	// new ones work.
	if err := s.SetShortcuts(shortcut.Map{shortcut.SyncTranslationLater: {Modifiers: shortcut.Alt, Key: "Period"}}); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	starts, next := len(r.chords), r.later
	last := chordTexts(r.chords[len(r.chords)-1])
	r.mu.Unlock()
	if starts != 2 || !r.shortcuts.closed || last[3] != "Alt+Period" || len(next) != 1 {
		t.Fatalf("%d starts, first closed %v, chords %v", starts, r.shortcuts.closed, last)
	}
	next[0].presses <- watch.Press{Index: 3}
	eventually(t, "the translation later", func() bool {
		return s.Offset(session.LineTranslation) == subtitle.Offset(subtitle.DefaultNudgeStep)
	})

	// A conflict is refused and changes nothing.
	if err := s.SetShortcuts(shortcut.Map{shortcut.Mute: {Key: shortcut.KeySpace}}); err == nil {
		t.Fatal("a conflict was taken")
	}
	if s.Shortcuts().Of(shortcut.Mute).Key != shortcut.KeyM {
		// Mute went back to M with the last set, which left it out.
		t.Fatalf("mute = %v", s.Shortcuts().Of(shortcut.Mute))
	}

	// The end of the session closes the shortcuts in use.
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if !next[0].closed {
		t.Fatal("the restarted shortcuts stayed open")
	}
	// After the end, a change starts nothing.
	if err := s.SetShortcuts(shortcut.Map{shortcut.SyncTranslationLater: {Modifiers: shortcut.Ctrl, Key: "KeyK"}}); err != nil {
		t.Fatal(err)
	}
	if len(r.chords) != 2 {
		t.Fatalf("%d starts after the end", len(r.chords))
	}
}

func TestTheTranslationKey(t *testing.T) {
	r := newRig(t)
	r.request.Mode = study.ModeOnDemand
	s := r.start(t)
	if s.TranslationVisible() {
		t.Fatal("on demand, the translation started shown")
	}
	// Toggle (the default): each press flips it; the release changes nothing.
	s.Reveal(true)
	s.Reveal(false)
	if !s.TranslationVisible() {
		t.Fatal("a press did not show the translation")
	}
	s.Reveal(true)
	if s.TranslationVisible() {
		t.Fatal("a second press did not hide it")
	}
	s.Reveal(false)

	if err := s.SetRevealBehavior(study.RevealHold); err != nil {
		t.Fatal(err)
	}
	s.Reveal(true)
	if !s.TranslationVisible() {
		t.Fatal("holding did not show it")
	}
	s.Reveal(false)
	if s.TranslationVisible() {
		t.Fatal("letting go did not hide it")
	}
	if err := s.SetRevealBehavior("wink"); err == nil || s.RevealBehavior() != study.RevealHold {
		t.Fatal("an unknown behavior was taken")
	}

	// Without a translation the key does nothing.
	alone := newRig(t)
	alone.request.Tracks.Translation = 0
	only := alone.start(t)
	only.SetTranslationVisible(false)
	only.Reveal(true)
	if only.TranslationVisible() {
		t.Fatal("the key showed a translation that is not there")
	}
}

func TestTheSyncNoticeCanBeLeftOut(t *testing.T) {
	r := newRig(t)
	r.request.NoSyncNotice = true
	s := r.start(t)
	s.Nudge(session.LinePrimary, true)
	if _, _, notices, _ := r.display.last(); len(notices) != 0 {
		t.Fatalf("notices = %v, want none", notices)
	}
	s.SetSyncNotice(true)
	s.Nudge(session.LinePrimary, true)
	if _, _, notices, _ := r.display.last(); len(notices) != 1 {
		t.Fatalf("notices = %v, want one", notices)
	}
}

func TestSyncStepsChangeWhilePlaying(t *testing.T) {
	s := newRig(t).start(t)
	if err := s.SetSyncSteps(subtitle.NudgeSteps{Step: 200 * time.Millisecond, FastStep: time.Second}); err != nil {
		t.Fatal(err)
	}
	if offset, _ := s.Nudge(session.LinePrimary, true); offset != subtitle.Offset(200*time.Millisecond) {
		t.Fatalf("offset = %v, want 200ms", offset)
	}
	if err := s.SetSyncSteps(subtitle.NudgeSteps{Step: -1, FastStep: time.Second}); err == nil {
		t.Fatal("a negative step was taken")
	}
	if offset, _ := s.Nudge(session.LinePrimary, true); offset != subtitle.Offset(400*time.Millisecond) {
		t.Fatalf("a refused step changed the steps: offset %v", offset)
	}
}

func TestRestyleReachesTheWindow(t *testing.T) {
	r, window, _ := hostedRig(t)
	s := r.start(t)
	profile := settings.DefaultProfile()
	profile.Primary.Color = appearance.RGB(0x12, 0x34, 0x56)
	look := watch.LookOf(profile)
	if err := s.Restyle(look); err != nil {
		t.Fatal(err)
	}
	window.mu.Lock()
	looks := slices.Clone(window.looks)
	window.mu.Unlock()
	if len(looks) != 1 || looks[0].Primary.Color != profile.Primary.Color || looks[0].FillColor != profile.Study.Karaoke.FillColor {
		t.Fatalf("looks = %+v", looks)
	}
	// A display that cannot change its look is left alone.
	if err := newRig(t).start(t).Restyle(look); err != nil {
		t.Fatal(err)
	}
}

func chordTexts(chords []shortcut.Chord) []string {
	texts := make([]string, len(chords))
	for i, chord := range chords {
		texts[i] = chord.String()
	}
	return texts
}
