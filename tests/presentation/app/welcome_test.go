package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
)

// welcomeRig is a first run on a Windows in a language.
func welcomeRig(t *testing.T, windows string, firstRun bool) *rig {
	t.Helper()
	r := newRigWith(t, func(deps *app.Deps) {
		deps.FirstRun = firstRun
		deps.SystemLanguage = func() string { return windows }
	})
	r.attach(t)
	return r
}

// profileNames lists the stored profiles' names.
func profileNames(r *rig) string {
	r.settings.mu.Lock()
	defer r.settings.mu.Unlock()
	names := make([]string, len(r.settings.profiles.Items))
	for i, profile := range r.settings.profiles.Items {
		names[i] = profile.Name
	}
	return strings.Join(names, ",")
}

// activeStored is the stored active profile.
func activeStored(t *testing.T, r *rig) settings.Profile {
	t.Helper()
	r.settings.mu.Lock()
	defer r.settings.mu.Unlock()
	profile, found := r.settings.profiles.Find(r.settings.config.ActiveProfile)
	if !found {
		t.Fatalf("the active profile %q is not stored", r.settings.config.ActiveProfile)
	}
	return profile
}

// Skipping the first run's welcome keeps what Windows tells: the person
// speaks its language and studies English, or Brazilian Portuguese on an
// English Windows; the first profile is named after the language studied.
func TestSkippingTheWelcomeKeepsWhatWindowsTells(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ windows, name, studied, spoken string }{
		{"pt-BR", "Inglês", "en", "pt-BR"},
		{"en-US", "Português do Brasil", "pt-BR", "en"},
		{"es-ES", "Inglês", "en", "es"},
		{"pt-PT", "Inglês", "en", "pt-PT"},
	} {
		r := welcomeRig(t, c.windows, true)
		view := r.api.Welcome()
		if !view.Pending || view.Studied != c.studied || view.Spoken != c.spoken || view.Mode != "on-demand" {
			t.Fatalf("Windows %s: the welcome suggests %+v", c.windows, view)
		}
		if err := r.api.SkipWelcome(); err != nil {
			t.Fatal(err)
		}
		profile := activeStored(t, r)
		if got := profileNames(r); got != c.name || profile.Languages.Primary != c.studied ||
			profile.Languages.Audio != c.studied || profile.Languages.Translation != c.spoken {
			t.Errorf("Windows %s: profiles %q, active %+v", c.windows, got, profile.Languages)
		}
		if r.api.Welcome().Pending {
			t.Errorf("Windows %s: the welcome is still pending after skipping", c.windows)
		}
	}
}

// The answers make the first profile; closing the app during the welcome
// skips it, so the next opening does not ask again.
func TestTheWelcomesAnswersMakeTheFirstProfile(t *testing.T) {
	t.Parallel()
	r := welcomeRig(t, "pt-BR", true)
	if err := r.api.FinishWelcome(app.WelcomeChoices{Spoken: "pt-BR", Studied: "fr", Mode: "dual", PreferredPlayer: "fake"}); err != nil {
		t.Fatal(err)
	}
	profile := activeStored(t, r)
	if got := profileNames(r); got != "Francês" || profile.Study.Mode != "dual" || profile.PreferredPlayer != "fake" ||
		profile.Languages.Primary != "fr" || profile.Languages.Translation != "pt-BR" {
		t.Fatalf("profiles %q, active %+v", got, profile)
	}
	// Answers that make no profile are refused, and nothing changes.
	for _, bad := range []app.WelcomeChoices{
		{Spoken: "pt-BR", Studied: "en", Mode: "sometimes"},
		{Spoken: "pt-BR", Studied: "not a language", Mode: "dual"},
		{Spoken: "pt-BR", Studied: "en", Mode: "dual", PreferredPlayer: "nope"},
	} {
		if err := r.api.FinishWelcome(bad); err == nil {
			t.Errorf("FinishWelcome(%+v) was taken", bad)
		}
	}
	if got := profileNames(r); got != "Francês" {
		t.Errorf("a refused answer changed the profiles: %q", got)
	}

	closed := welcomeRig(t, "en-US", true)
	closed.lifecycle.Shutdown()
	if got := profileNames(closed); got != "Português do Brasil" {
		t.Errorf("closing during the welcome left the profiles %q", got)
	}
}

// Opened again from the settings, the welcome shows the active profile and
// changes it; another language studied makes a profile of its own, and the
// old one stays. A video being watched keeps its profile.
// A skip that comes while the answers are being written (closing the app
// right after Concluir) finds the welcome answered and changes nothing; an
// answer that could not be written leaves the welcome pending.
func TestASkipNeverWritesOverTheAnswers(t *testing.T) {
	t.Parallel()
	r := welcomeRig(t, "pt-BR", true)
	entered, release := r.settings.hold()
	finished, skipped := make(chan error, 1), make(chan error, 1)
	go func() {
		finished <- r.api.FinishWelcome(app.WelcomeChoices{Spoken: "pt-BR", Studied: "fr", Mode: "dual"})
	}()
	<-entered
	go func() { skipped <- r.api.SkipWelcome() }()
	release()
	if err := errors.Join(<-finished, <-skipped); err != nil {
		t.Fatal(err)
	}
	if profile := activeStored(t, r); profileNames(r) != "Francês" || profile.Languages.Primary != "fr" || profile.Study.Mode != "dual" {
		t.Fatalf("profiles %q, active %+v", profileNames(r), profile.Languages)
	}

	failing := welcomeRig(t, "pt-BR", true)
	failing.settings.mu.Lock()
	failing.settings.saveError = errors.New("disk full")
	failing.settings.mu.Unlock()
	if err := failing.api.FinishWelcome(app.WelcomeChoices{Spoken: "pt-BR", Studied: "fr", Mode: "dual"}); err == nil || !failing.api.Welcome().Pending {
		t.Fatalf("a failed answer: %v, pending %v", err, failing.api.Welcome().Pending)
	}
}

func TestTheWelcomeOpenedAgainChangesTheActiveProfile(t *testing.T) {
	t.Parallel()
	r := welcomeRig(t, "es-ES", false)
	view := r.api.Welcome()
	if view.Pending || view.Studied != "en" || view.Spoken != "pt-BR" {
		t.Fatalf("the welcome opened again suggests %+v, not the active profile", view)
	}
	if err := r.api.SkipWelcome(); err != nil || profileNames(r) != settings.DefaultProfileName {
		t.Fatalf("skipping it again changed the profiles: %v, %q", err, profileNames(r))
	}
	if err := r.api.FinishWelcome(app.WelcomeChoices{Spoken: "pt-BR", Studied: "en", Mode: "always"}); err != nil {
		t.Fatal(err)
	}
	if got := profileNames(r); got != "Inglês" || activeStored(t, r).Study.Mode != "always" {
		t.Fatalf("the same language studied: profiles %q, mode %q", got, activeStored(t, r).Study.Mode)
	}
	if err := r.api.FinishWelcome(app.WelcomeChoices{Spoken: "pt-BR", Studied: "es", Mode: "on-demand"}); err != nil {
		t.Fatal(err)
	}
	profile := activeStored(t, r)
	if got := profileNames(r); got != "Inglês,Espanhol" || profile.Name != "Espanhol" || profile.Languages.Primary != "es" {
		t.Fatalf("another language studied: profiles %q, active %+v", got, profile)
	}

	watching := startWatching(t)
	if err := watching.api.FinishWelcome(app.WelcomeChoices{Spoken: "pt-BR", Studied: "de", Mode: "dual"}); err == nil {
		t.Fatal("the welcome changed the profile of a video being watched")
	}
}

// On a weak computer the first run starts with the voice AI off, and the
// welcome and the voice settings say why; the person may turn it on anyway.
// A strong one, or someone who already uses the app, keeps it as it is.
func TestAWeakComputerStartsWithTheVoiceAIOff(t *testing.T) {
	t.Parallel()
	weak := capacity.Machine{Processors: 2, Memory: 2 << 30}
	strong := capacity.Machine{Processors: 8, Memory: 24 << 30}
	rigOn := func(machine capacity.Machine, firstRun bool) *rig {
		r := newRigWith(t, func(deps *app.Deps) {
			deps.FirstRun = firstRun
			deps.SystemLanguage = func() string { return "pt-BR" }
			deps.Machine = func() capacity.Machine { return machine }
		})
		r.attach(t)
		return r
	}
	voiceOn := func(r *rig) bool {
		r.settings.mu.Lock()
		defer r.settings.mu.Unlock()
		return r.settings.config.Voice.Model
	}

	skipped := rigOn(weak, true)
	if view := skipped.api.Welcome(); !view.Weak || view.VoiceModel {
		t.Fatalf("the welcome on a weak computer = weak %v, voice AI %v", view.Weak, view.VoiceModel)
	}
	if err := skipped.api.SkipWelcome(); err != nil || voiceOn(skipped) {
		t.Fatalf("skipping on a weak computer: %v, voice AI on %v", err, voiceOn(skipped))
	}
	if view := settingsOf(t, skipped); !view.Voice.Weak {
		t.Error("the voice settings do not warn about the weak computer")
	}

	risked := rigOn(weak, true)
	if err := risked.api.FinishWelcome(app.WelcomeChoices{Spoken: "pt-BR", Studied: "en", Mode: "on-demand", VoiceModel: true}); err != nil || !voiceOn(risked) {
		t.Fatalf("turning it on at one's own risk: %v, voice AI on %v", err, voiceOn(risked))
	}

	fine := rigOn(strong, true)
	if view := fine.api.Welcome(); view.Weak || !view.VoiceModel {
		t.Fatalf("the welcome on a strong computer = weak %v, voice AI %v", view.Weak, view.VoiceModel)
	}
	if err := fine.api.SkipWelcome(); err != nil || !voiceOn(fine) {
		t.Fatalf("skipping on a strong computer: %v, voice AI on %v", err, voiceOn(fine))
	}

	using := rigOn(weak, false)
	if err := using.api.SkipWelcome(); err != nil || !voiceOn(using) {
		t.Fatalf("someone already using the app had the voice AI changed: %v, on %v", err, voiceOn(using))
	}
}

// Each line of the settings' preview speaks its line's language.
func TestThePreviewSpeaksEachLinesLanguage(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r)
	request := app.PreviewRequest{
		Primary: view.Profile.Primary, Translation: view.Profile.Translation, Layout: view.Profile.Layout,
		Width: 640, Height: 360, Scale: 1, Languages: app.LanguageSettings{Audio: "fr", Primary: "fr", Translation: "de"},
	}
	if _, err := r.api.PreviewSubtitles(request); err != nil {
		t.Fatal(err)
	}
	// Without a translation line, the one below speaks the app's language.
	request.Languages = app.LanguageSettings{Audio: "ja", Primary: "ja"}
	if _, err := r.api.PreviewSubtitles(request); err != nil {
		t.Fatal(err)
	}
	r.preview.mu.Lock()
	defer r.preview.mu.Unlock()
	want := [][2]string{{"Ceci est un aperçu.", "Das ist eine Vorschau."}, {"This is a preview.", "Esta é uma prévia."}}
	if len(r.preview.requests) != len(want) {
		t.Fatalf("%d previews drawn", len(r.preview.requests))
	}
	for i, call := range r.preview.requests {
		if got := [2]string{call.screen.Primary, call.screen.Translation}; got != want[i] {
			t.Errorf("preview %d drew %q, want %q", i, got, want[i])
		}
	}
}
