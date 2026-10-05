package app_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
)

func TestThePanelRunsTheShortcuts(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if err := r.api.RunShortcut("seek-back", false); err == nil {
		t.Fatal("a shortcut ran without a video")
	}
	r = startWatching(t)
	eventually(t, "the first sample", func() bool { return r.api.PlaybackState().Position == 30 })
	if err := r.api.RunShortcut("seek-back-10", false); err != nil {
		t.Fatal(err)
	}
	if seek, _ := r.plugin.current().lastSeek(); seek != 20*time.Second {
		t.Fatalf("seek to %v, want 30 s - 10 s", seek)
	}
	// Held, a jump goes on; a repeat goes back once only, as in the window.
	if err := r.api.RunShortcut("seek-back-10", true); err != nil {
		t.Fatal(err)
	}
	if seek, _ := r.plugin.current().lastSeek(); seek != 10*time.Second {
		t.Fatalf("held jump to %v, want 10 s", seek)
	}
	if err := r.api.RunShortcut("repeat-last-3", true); err != nil {
		t.Fatal(err)
	}
	if seek, _ := r.plugin.current().lastSeek(); seek != 10*time.Second {
		t.Fatalf("a held repeat ran: seek to %v", seek)
	}
	refused := map[string]string{
		"fullscreen":      "janela do filme",
		"exit-fullscreen": "janela do filme",
		"translation":     "janela do filme",
		"launch-rockets":  "Atalho desconhecido",
		"":                "Atalho desconhecido",
	}
	for command, want := range refused {
		if err := r.api.RunShortcut(command, false); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", command, err, want)
		}
	}
	// The fake player steps no frames: the panel is told so.
	if err := r.api.RunShortcut("frame-forward", false); err == nil || !strings.Contains(err.Error(), "não permite") {
		t.Fatalf("frame step: %v", err)
	}
}

func TestThePanelTranslationKeyAndTheMode(t *testing.T) {
	r := startWatching(t)
	// The default profile studies on demand: the translation starts hidden.
	if state := r.api.PlaybackState(); state.Mode != "on-demand" || state.TranslationVisible {
		t.Fatalf("mode %q, visible %v", state.Mode, state.TranslationVisible)
	}
	if err := r.api.Reveal(true); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Reveal(false); err != nil {
		t.Fatal(err)
	}
	if !r.api.PlaybackState().TranslationVisible {
		t.Fatal("the key on the panel did not show the translation")
	}
	if err := r.api.SetStudyMode("always"); err != nil {
		t.Fatal(err)
	}
	if err := r.api.SetTranslationVisible(false); err != nil {
		t.Fatal(err)
	}
	if state := r.api.PlaybackState(); state.Mode != "always" || !state.TranslationVisible {
		t.Fatalf("always: mode %q, visible %v", state.Mode, state.TranslationVisible)
	}
	for _, bad := range []string{"passive", ""} {
		if err := r.api.SetStudyMode(bad); err == nil || err.Error() != "Escolha um modo de estudo." {
			t.Errorf("mode %q: %v", bad, err)
		}
	}
	// The panel's choice is the video's: the profile keeps its own.
	if mode := r.settings.profiles.Items[0].Study.Mode; mode != "on-demand" {
		t.Fatalf("the profile's mode became %q", mode)
	}
}

func TestWatchTakesTheModeOfTheSessionScreen(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	video, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`)
	if err != nil {
		t.Fatal(err)
	}
	labels := make([]string, len(video.Modes))
	for i, mode := range video.Modes {
		labels[i] = mode.Value + "=" + mode.Label
	}
	want := []string{"dual=Dupla", "on-demand=Tradução sob demanda", "always=Tradução sempre visível"}
	if video.Mode != "on-demand" || !slices.Equal(labels, want) {
		t.Fatalf("mode %q, modes %v", video.Mode, labels)
	}
	request := app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5, Mode: "passive"}
	if err := r.api.Watch(request); err == nil || err.Error() != "Escolha um modo de estudo." || len(r.plugin.launched) != 0 {
		t.Fatalf("Watch = %v, %d launches", err, len(r.plugin.launched))
	}
	request.Mode = "dual"
	if err := r.api.Watch(request); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	if state := r.api.PlaybackState(); state.Mode != "dual" || !state.TranslationVisible {
		t.Fatalf("mode %q, visible %v", state.Mode, state.TranslationVisible)
	}
}

func TestTheKeysThePanelShows(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	keys := r.api.Shortcuts()
	if len(keys) != len(shortcut.Commands()) {
		t.Fatalf("%d keys for %d commands", len(keys), len(shortcut.Commands()))
	}
	find := func(keys []app.ShortcutSetting, command shortcut.Command) app.ShortcutSetting {
		i := slices.IndexFunc(keys, func(k app.ShortcutSetting) bool { return k.Command == string(command) })
		if i < 0 {
			t.Fatalf("no %s", command)
		}
		return keys[i]
	}
	if back := find(keys, shortcut.SeekBack10); back.Label != "Shift+←" || back.Chord != "Shift+ArrowLeft" || back.Category != "Avançar e voltar" {
		t.Fatalf("10 s back = %+v", back)
	}
	if both := find(keys, shortcut.ShowBoth); both.Label != "" || both.Chord != "" || both.Name != "Mostrar as duas legendas" {
		t.Fatalf("show both = %+v", both)
	}
	// While a video plays, they are the video's keys.
	r = startWatching(t)
	view := settingsOf(t, r).Profile
	view.Shortcuts[shortcutAt(t, view, shortcut.PlayPause)].Chord = "KeyP"
	if err := r.api.SaveProfile(view); err != nil {
		t.Fatal(err)
	}
	if play := find(r.api.Shortcuts(), shortcut.PlayPause); play.Label != "P" {
		t.Fatalf("play/pause while watching = %+v", play)
	}
}
