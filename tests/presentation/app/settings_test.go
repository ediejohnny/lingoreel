package app_test

import (
	"encoding/base64"
	"math"
	"slices"
	"strings"
	"testing"

	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/presentation/app"
)

func settingsOf(t *testing.T, r *rig) app.SettingsView {
	t.Helper()
	view, err := r.api.Settings()
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestSettingsShowTheActiveProfileAndTheOptions(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r)
	p := view.Profile
	if p.Name != "Inglês" || p.Primary.FontFamily != "Arial" || p.Primary.FontSize != 32 || !p.Primary.Responsive ||
		p.Primary.Color != "#FFFFFF" || p.Primary.Position != 0.82 || p.Translation.Opacity != 0.9 {
		t.Fatalf("profile = %+v", p)
	}
	if p.Karaoke.MinGap != 10 || p.Karaoke.Lead != 5 || p.Karaoke.FillColor != "#FFD23F" || p.Reveal != "toggle" ||
		p.SyncStep != 50 || p.SyncFastStep != 500 || p.Speed != 1 {
		t.Fatalf("karaoke %+v reveal %q steps %d/%d speed %v", p.Karaoke, p.Reveal, p.SyncStep, p.SyncFastStep, p.Speed)
	}
	if len(p.Shortcuts) != len(shortcut.Commands()) || p.Shortcuts[0].Name != "Reproduzir / pausar" || p.Shortcuts[0].Label != "Espaço" ||
		p.Shortcuts[0].Chord != "Space" || p.Shortcuts[0].Category != "Reprodução" {
		t.Fatalf("shortcuts = %+v", p.Shortcuts)
	}
	if !view.General.SyncNotice || view.General.Theme != "system" || view.General.ActiveProfile != "Inglês" ||
		!slices.Equal(view.Profiles, []string{"Inglês"}) || view.Folder == "" {
		t.Fatalf("general %+v profiles %v folder %q", view.General, view.Profiles, view.Folder)
	}
	o := view.Options
	if !slices.Equal(o.Fonts, []string{"Arial", "Verdana"}) || len(o.Presets) != 4 || o.Presets[2].Label != "Alto contraste" ||
		o.Limits.FontSizeMax != 200 || o.Limits.LeadMax != 10 || o.Limits.SyncFastStepMax != 5000 || len(o.Speeds) == 0 {
		t.Fatalf("options = %+v", o)
	}
	if i := slices.IndexFunc(o.Languages, func(l app.Option) bool { return l.Value == "pt-BR" }); i < 0 || o.Languages[i].Label != "Português do Brasil" {
		t.Fatalf("languages = %+v", o.Languages)
	}
	// The supported player first, then the others found.
	if len(view.Players) != 2 || view.Players[0].ID != "fake" || !view.Players[0].Supported || view.Players[0].Path != `C:\fake\fake.exe` ||
		view.Players[1].ID != "potplayer" || view.Players[1].Supported {
		t.Fatalf("players = %+v", view.Players)
	}
}

func TestSaveProfileKeepsTheChoicesAndAppliesThemToTheVideo(t *testing.T) {
	r := startWatching(t)
	view := settingsOf(t, r).Profile
	view.Primary.FontSize = 40
	view.Primary.Color = "#FFE600"
	view.Translation.Background = true
	view.Translation.BackgroundColor = "#000000C8"
	view.Layout.MarginBottom = 0.1
	view.Karaoke.Lead = 3
	view.Karaoke.FillColor = "#00FF00"
	view.SyncStep = 100
	view.Reveal = "hold"
	view.Speed = 0.75
	view.Shortcuts[0].Chord = "KeyP"
	view.Shortcuts[shortcutAt(t, view, shortcut.Mute)].Chord = "" // no shortcut
	view.PrecisionStep = 250
	if view.Mode != "on-demand" {
		t.Fatalf("mode = %q, want the default on-demand", view.Mode)
	}
	view.Mode = "dual"
	if err := r.api.SaveProfile(view); err != nil {
		t.Fatal(err)
	}
	saved := r.settings.profiles.Items[0]
	if saved.Primary.FontSize != 40 || saved.Primary.Color.String() != "#FFE600" || !saved.Translation.Background.Enabled ||
		saved.Translation.Background.Color.A != 0xC8 || saved.Layout.MarginBottom != 0.1 || saved.Study.Karaoke.Lead.Std().Seconds() != 3 ||
		saved.Sync.OffsetStep.Std().Milliseconds() != 100 || saved.Study.Reveal != "hold" || saved.Speed != 0.75 ||
		saved.Shortcuts.Of(shortcut.PlayPause).Key != "KeyP" || !saved.Shortcuts.Of(shortcut.Mute).IsZero() ||
		saved.Study.Mode != "dual" || saved.Sync.PrecisionSeekStep.Std().Milliseconds() != 250 {
		t.Fatalf("saved = %+v", saved)
	}
	// The video took the changed mode.
	if state := r.api.PlaybackState(); state.Mode != "dual" || !state.TranslationVisible {
		t.Fatalf("playback mode %q, translation visible %v", state.Mode, state.TranslationVisible)
	}
	// The video being watched took the new look at once.
	look, looks := r.display.lastLook()
	if looks != 1 || look.Primary.FontSize != 40 || look.FillColor.String() != "#00FF00" || look.Layout.MarginBottom != 0.1 {
		t.Fatalf("display looks %d, last %+v", looks, look)
	}
	again := settingsOf(t, r).Profile
	mute := again.Shortcuts[shortcutAt(t, again, shortcut.Mute)]
	if again.Shortcuts[0].Label != "P" || again.Primary.Color != "#FFE600" || mute.Chord != "" || mute.Label != "" {
		t.Fatalf("settings after saving = %+v, mute %+v", again.Shortcuts[0], mute)
	}
}

// shortcutAt is where a command is on the settings screen.
func shortcutAt(t *testing.T, p app.ProfileSettings, command shortcut.Command) int {
	t.Helper()
	i := slices.IndexFunc(p.Shortcuts, func(s app.ShortcutSetting) bool { return s.Command == string(command) })
	if i < 0 {
		t.Fatalf("no %s on the screen", command)
	}
	return i
}

func TestSaveProfileRefusesBadChoicesInPortuguese(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	base := settingsOf(t, r).Profile
	cases := []struct {
		name   string
		change func(*app.ProfileSettings)
		want   string
	}{
		{"font too big", func(p *app.ProfileSettings) { p.Primary.FontSize = 500 }, "Legenda de cima: o tamanho da fonte vai de 8 a 200."},
		{"NaN font", func(p *app.ProfileSettings) { p.Translation.FontSize = math.NaN() }, "Legenda de baixo: o tamanho da fonte"},
		{"bad color", func(p *app.ProfileSettings) { p.Primary.OutlineColor = "red" }, `a cor "red" não é válida`},
		{"no font", func(p *app.ProfileSettings) { p.Primary.FontFamily = " " }, "escolha uma fonte"},
		{"opacity", func(p *app.ProfileSettings) { p.Primary.Opacity = 1.5 }, "a opacidade vai de 0% a 100%"},
		{"margin", func(p *app.ProfileSettings) { p.Layout.MarginLeft = 0.3 }, "Margens: cada margem vai de 0% a 25%."},
		{"lead", func(p *app.ProfileSettings) { p.Karaoke.Lead = 20 }, "a antecedência de 2 a 10 s"},
		{"NaN gap", func(p *app.ProfileSettings) { p.Karaoke.MinGap = math.Inf(1) }, "a lacuna mínima vai de 3 a 60 s"},
		{"indicator", func(p *app.ProfileSettings) { p.Karaoke.Indicator = "dots" }, "escolha a barra ou a contagem"},
		{"repeat", func(p *app.ProfileSettings) { p.Karaoke.Repeat = 11 }, "de 0 a 10 vezes"},
		{"speed", func(p *app.ProfileSettings) { p.Speed = 3 }, "Escolha uma das velocidades"},
		{"NaN speed", func(p *app.ProfileSettings) { p.Speed = math.NaN() }, "Escolha uma das velocidades"},
		{"sync step", func(p *app.ProfileSettings) { p.SyncStep = 5 }, "o passo vai de 10 a 1000 ms"},
		{"fast below step", func(p *app.ProfileSettings) { p.SyncStep, p.SyncFastStep = 200, 100 }, "Passos da sincronização"},
		{"reveal", func(p *app.ProfileSettings) { p.Reveal = "wink" }, "tecla da tradução"},
		{"language", func(p *app.ProfileSettings) { p.Languages.Primary = "english!" }, "Confira os idiomas"},
		{"Alt+Shift", func(p *app.ProfileSettings) {
			p.Shortcuts[shortcutAt(t, *p, shortcut.SeekBack)].Chord = "Alt+Shift+KeyA"
		}, "Atalho de “Voltar 5 s”: Alt+Shift troca o idioma"},
		{"conflict", func(p *app.ProfileSettings) {
			p.Shortcuts[shortcutAt(t, *p, shortcut.Fullscreen)].Chord = "Space"
		}, "“Espaço” já é o atalho de “Reproduzir / pausar”"},
		{"modifiers alone", func(p *app.ProfileSettings) { p.Shortcuts[0].Chord = "Ctrl+" }, "Falta a tecla principal"},
		{"mode", func(p *app.ProfileSettings) { p.Mode = "passive" }, "Escolha um modo de estudo."},
		{"no mode", func(p *app.ProfileSettings) { p.Mode = "" }, "Escolha um modo de estudo."},
		{"precision", func(p *app.ProfileSettings) { p.PrecisionStep = 5 }, "Passo de precisão: vai de 10 a 10000 ms."},
		{"precision too big", func(p *app.ProfileSettings) { p.PrecisionStep = 10001 }, "Passo de precisão"},
		{"unknown command", func(p *app.ProfileSettings) { p.Shortcuts[0].Command = "launch-rockets" }, "comando desconhecido"},
	}
	for _, c := range cases {
		view := base
		view.Shortcuts = slices.Clone(base.Shortcuts)
		c.change(&view)
		saves := r.settings.saves
		err := r.api.SaveProfile(view)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: SaveProfile = %v, want %q", c.name, err, c.want)
		}
		if r.settings.saves != saves {
			t.Errorf("%s: a refused profile was saved", c.name)
		}
	}
	if r.settings.profiles.Items[0].Primary.FontSize != 32 {
		t.Fatal("a refused change reached the profile")
	}
	// A profile renamed elsewhere meanwhile is not overwritten.
	stale := base
	stale.Name = "Francês"
	if err := r.api.SaveProfile(stale); err == nil {
		t.Fatal("saved over another profile's name")
	}
}

func TestProfilesCanBeCreatedSwitchedRenamedAndRemoved(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r).Profile
	view.Primary.FontSize = 44
	if err := r.api.SaveProfile(view); err != nil {
		t.Fatal(err)
	}
	if err := r.api.CreateProfile("Francês"); err != nil {
		t.Fatal(err)
	}
	got := settingsOf(t, r)
	// A copy of the active profile, made active.
	if got.General.ActiveProfile != "Francês" || got.Profile.Primary.FontSize != 44 || !slices.Equal(got.Profiles, []string{"Inglês", "Francês"}) ||
		r.settings.config.ActiveProfile != "Francês" {
		t.Fatalf("after create: %+v %v", got.General, got.Profiles)
	}
	for name, want := range map[string]string{"inglês": "Já existe", "": "Dê ao perfil um nome", " x": "Dê ao perfil um nome"} {
		if err := r.api.CreateProfile(name); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("CreateProfile(%q) = %v", name, err)
		}
	}
	if err := r.api.SwitchProfile("INGLÊS"); err != nil || settingsOf(t, r).General.ActiveProfile != "Inglês" {
		t.Fatalf("SwitchProfile = %v", err)
	}
	if err := r.api.SwitchProfile("Alemão"); err == nil {
		t.Fatal("switched to a missing profile")
	}
	// Renaming the active profile keeps it active.
	if err := r.api.RenameProfile("Inglês", "English"); err != nil {
		t.Fatal(err)
	}
	if got := settingsOf(t, r); got.General.ActiveProfile != "English" || r.settings.config.ActiveProfile != "English" {
		t.Fatalf("after rename: %+v", got.General)
	}
	if err := r.api.RenameProfile("English", "francês"); err == nil {
		t.Fatal("renamed onto another profile")
	}
	// Removing the active profile activates the one left; the last stays.
	if err := r.api.DeleteProfile("English"); err != nil {
		t.Fatal(err)
	}
	if got := settingsOf(t, r); got.General.ActiveProfile != "Francês" || len(got.Profiles) != 1 {
		t.Fatalf("after delete: %+v %v", got.General, got.Profiles)
	}
	if err := r.api.DeleteProfile("Francês"); err == nil || !strings.Contains(err.Error(), "pelo menos um perfil") {
		t.Fatalf("deleting the last = %v", err)
	}
}

func TestTheProfileCannotChangeUnderAVideo(t *testing.T) {
	r := startWatching(t)
	if err := r.api.CreateProfile("Francês"); err == nil || !strings.Contains(err.Error(), "Encerre o vídeo") {
		t.Fatalf("CreateProfile while watching = %v", err)
	}
	if err := r.api.SwitchProfile("Inglês"); err == nil {
		t.Fatal("switched while watching")
	}
	if err := r.api.DeleteProfile("Inglês"); err == nil {
		t.Fatal("deleted the profile in use")
	}
}

func TestResetProfileKeepsTheName(t *testing.T) {
	r := startWatching(t)
	if err := r.api.RenameProfile("Inglês", "Meu"); err != nil {
		t.Fatal(err)
	}
	view := settingsOf(t, r).Profile
	view.Primary.FontSize = 70
	if err := r.api.SaveProfile(view); err != nil {
		t.Fatal(err)
	}
	if err := r.api.ResetProfile(); err != nil {
		t.Fatal(err)
	}
	got := settingsOf(t, r).Profile
	if got.Name != "Meu" || got.Primary.FontSize != 32 {
		t.Fatalf("after reset: %q %v", got.Name, got.Primary.FontSize)
	}
	if look, _ := r.display.lastLook(); look.Primary.FontSize != 32 {
		t.Fatalf("the video kept %v", look.Primary.FontSize)
	}
}

func TestSyncNoticeAndPresetsAndShortcutChecks(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if err := r.api.SetSyncNotice(false); err != nil || r.settings.config.SyncNotice || settingsOf(t, r).General.SyncNotice {
		t.Fatalf("SetSyncNotice(false) = %v", err)
	}
	style := settingsOf(t, r).Profile.Primary
	contrast, err := r.api.ApplyPreset(style, "high-contrast")
	if err != nil || !contrast.Background || contrast.Color != "#FFE600" || contrast.FontSize != style.FontSize || contrast.Position != style.Position {
		t.Fatalf("ApplyPreset = %+v, %v", contrast, err)
	}
	if _, err := r.api.ApplyPreset(style, "neon"); err == nil {
		t.Fatal("an unknown preset was applied")
	}
	check, err := r.api.CheckShortcut("Shift+Ctrl+Comma")
	if err != nil || check.Chord != "Ctrl+Shift+Comma" || check.Label != "Ctrl+Shift+," {
		t.Fatalf("CheckShortcut = %+v, %v", check, err)
	}
	if _, err := r.api.CheckShortcut("Alt+F4"); err == nil || !strings.Contains(err.Error(), "do próprio Windows") {
		t.Fatalf("CheckShortcut(Alt+F4) = %v", err)
	}
}

func TestThePreviewIsAPictureOfTheChoices(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r).Profile
	view.Primary.FontSize = 50
	request := app.PreviewRequest{Primary: view.Primary, Translation: view.Translation, Layout: view.Layout, Width: 600, Height: 338, Scale: 1.5}
	picture, err := r.api.PreviewSubtitles(request)
	if err != nil || picture != "data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("PNG!")) {
		t.Fatalf("PreviewSubtitles = %q, %v", picture, err)
	}
	call := r.preview.requests[0]
	if call.width != 600 || call.height != 338 || call.dpi != 144 || call.look.Primary.FontSize != 50 ||
		call.primary != "This is a preview." || call.translated != "Esta é uma prévia." {
		t.Fatalf("preview call = %+v", call)
	}
	// A scaling no screen has is read as 100%.
	request.Scale = math.NaN()
	if _, err := r.api.PreviewSubtitles(request); err != nil || r.preview.requests[1].dpi != 96 {
		t.Fatalf("NaN scale = %v, dpi %d", err, r.preview.requests[1].dpi)
	}
	request.Primary.Color = "#12"
	if _, err := r.api.PreviewSubtitles(request); err == nil || len(r.preview.requests) != 2 {
		t.Fatal("a bad color reached the preview")
	}
}

func TestPlayerPaths(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if err := r.api.SetPlayerPath("fake", `D:\Apps\Fake\fake.exe`); err != nil || r.settings.config.PlayerPaths["fake"] != `D:\Apps\Fake\fake.exe` {
		t.Fatalf("SetPlayerPath = %v", err)
	}
	if !settingsOf(t, r).Players[0].Custom {
		t.Fatal("a path set by the user is not marked")
	}
	for _, c := range []struct{ id, path string }{{"fake", `fake.exe`}, {"fake", `D:\Apps\fake.bat`}, {"potplayer", `D:\pot.exe`}, {"../x", `D:\x.exe`}} {
		if err := r.api.SetPlayerPath(c.id, c.path); err == nil {
			t.Errorf("SetPlayerPath(%q, %q) was taken", c.id, c.path)
		}
	}
	r.ui.program = `E:\Fake\fake.exe`
	if path, err := r.api.ChoosePlayerProgram("fake"); err != nil || path != `E:\Fake\fake.exe` || r.settings.config.PlayerPaths["fake"] != path {
		t.Fatalf("ChoosePlayerProgram = %q, %v", path, err)
	}
	r.ui.program = ""
	if path, err := r.api.ChoosePlayerProgram("fake"); err != nil || path != "" || r.settings.config.PlayerPaths["fake"] != `E:\Fake\fake.exe` {
		t.Fatalf("a cancelled choice = %q, %v", path, err)
	}
	if err := r.api.ClearPlayerPath("fake"); err != nil || len(r.settings.config.PlayerPaths) != 0 {
		t.Fatalf("ClearPlayerPath = %v, paths %v", err, r.settings.config.PlayerPaths)
	}
}

func TestTheWindowPlaceIsKept(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	bounds := settings.WindowBounds{X: -1500, Y: 40, Width: 1100, Height: 760, Maximized: true}
	r.lifecycle.SaveWindow(bounds)
	if r.lifecycle.WindowBounds() != bounds || r.settings.config.Window != bounds {
		t.Fatalf("window = %+v, saved %+v", r.lifecycle.WindowBounds(), r.settings.config.Window)
	}
	r.lifecycle.SaveWindow(settings.WindowBounds{Width: 1 << 20, Height: 600})
	if r.lifecycle.WindowBounds() != bounds {
		t.Fatal("an impossible window was kept")
	}
}

func TestTheVideoOpensWithTheProfileChoices(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r).Profile
	view.Speed = 0.75
	if err := r.api.SaveProfile(view); err != nil {
		t.Fatal(err)
	}
	if err := r.api.SetSyncNotice(false); err != nil {
		t.Fatal(err)
	}
	if _, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	running := r.plugin.current()
	running.mu.Lock()
	speeds := slices.Clone(running.speeds)
	running.mu.Unlock()
	if len(speeds) != 1 || speeds[0] != 0.75 {
		t.Fatalf("speeds = %v, want the profile's 0.75", speeds)
	}
	// The sync notice is off in the settings: a nudge shows none, until it
	// is turned on again, at once.
	if err := r.api.Nudge("primary", true); err != nil || r.display.noticeCount() != 0 {
		t.Fatalf("Nudge = %v, %d notices", err, r.display.noticeCount())
	}
	if err := r.api.SetSyncNotice(true); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Nudge("primary", true); err != nil || r.display.noticeCount() != 1 {
		t.Fatalf("Nudge = %v, %d notices", err, r.display.noticeCount())
	}
}

func TestOpenSettingsFolder(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if err := r.api.OpenSettingsFolder(); err != nil {
		t.Fatal(err)
	}
	if len(r.ui.urls) != 1 || r.ui.urls[0] != `folder:C:\Users\x\AppData\Roaming\LingoReel` {
		t.Fatalf("opened %v", r.ui.urls)
	}
}
