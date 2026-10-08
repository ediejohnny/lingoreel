package app_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The interface's language changes at once, with a video playing: what the
// app words from then on, the bar and the menu over the video, the name in
// Windows' "Abrir com" and what the screens were sent. Not parallel: the
// language is the whole app's, and the parallel tests start only once this
// one has put Portuguese back.
func TestTheLanguageChangesAtOnce(t *testing.T) {
	system := "es-ES"
	r, stage := stageRigWith(t, func(deps *app.Deps) { deps.SystemLanguage = func() string { return system } })
	t.Cleanup(func() {
		if _, err := ptbr.UseLanguage("pt-BR", ""); err != nil {
			t.Error(err)
		}
	})
	r.associations.on = true
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	t.Cleanup(func() { r.api.StopWatching() })
	if got := r.api.PlaybackState().Lines.Primary; !strings.HasPrefix(got, "Legenda de estudo") {
		t.Fatalf("the study line is named %q before the change", got)
	}
	chromes := len(stage.styled())

	spoken, err := r.api.SetLanguage("en-US")
	if err != nil || spoken != "en-US" || r.settings.config.Language != "en-US" {
		t.Fatalf("SetLanguage(en-US) = %q, %v; kept %q", spoken, err, r.settings.config.Language)
	}
	if got := ptbr.EditHint(); !strings.HasPrefix(got, "Enter saves") {
		t.Errorf("the correction box's hint = %q", got)
	}
	if got := r.api.Preferences().Language; got != "en-US" {
		t.Errorf("the screens are told %q", got)
	}
	if len(stage.styled()) <= chromes {
		t.Error("the bar over the video was not drawn again")
	}
	if !slices.Contains(r.associations.calls, "enable") {
		t.Errorf("the association was not written again in English: %v", r.associations.calls)
	}
	if got := r.api.PlaybackState().Lines.Primary; !strings.HasPrefix(got, "Study subtitle") {
		t.Errorf("the study line is named %q after the change", got)
	}
	if state, ok := r.ui.last(app.EventPlayback); !ok || !strings.HasPrefix(state.(app.Playback).Lines.Primary, "Study subtitle") {
		t.Errorf("the panel was not sent the session in English: %+v", state)
	}
	sent, ok := r.ui.last(app.EventVideo)
	if !ok || !slices.ContainsFunc(sent.(app.Video).Modes, func(mode app.Option) bool { return mode.Label == "Translation on demand" }) {
		t.Errorf("the session screen was not sent the video in English: %+v", sent)
	}
	// None chosen: the Windows language when the app speaks it, else English.
	if spoken, err := r.api.SetLanguage(""); err != nil || spoken != "en-US" || r.settings.config.Language != "" {
		t.Fatalf("SetLanguage(\"\") on a Spanish Windows = %q, %v; kept %q", spoken, err, r.settings.config.Language)
	}
	system = "pt-PT"
	if spoken, err := r.api.SetLanguage(""); err != nil || spoken != "pt-BR" {
		t.Fatalf("SetLanguage(\"\") on a Portuguese Windows = %q, %v", spoken, err)
	}
	if got := ptbr.EditHint(); !strings.HasPrefix(got, "Enter salva") {
		t.Errorf("the correction box's hint = %q", got)
	}
	// One the app does not speak is refused, and nothing changes.
	if _, err := r.api.SetLanguage("xx"); err == nil || r.settings.config.Language != "" || ptbr.Language() != "pt-BR" {
		t.Fatalf("SetLanguage(xx) = %v; kept %q, speaking %q", err, r.settings.config.Language, ptbr.Language())
	}
}
