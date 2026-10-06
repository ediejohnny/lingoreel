package app_test

import (
	"encoding/base64"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
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
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r)
	p := view.Profile
	if p.Name != "Inglês" || p.Primary.FontFamily != appearance.AppFont || p.Primary.FontSize != 32 || !p.Primary.Responsive ||
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
	t.Parallel()
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
	view.Primary.LineSpacing, view.Layout.LineGap = 1.4, 1
	view.Marks.Notice.Side, view.Marks.NoticeDuration, view.Marks.Selection.Width = "right", 3, 3
	view.Karaoke.BarThickness, view.Karaoke.CountdownSize = 9, 1.2
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
	if looks != 1 || look.Primary.FontSize != 40 || look.FillColor.String() != "#00FF00" || look.Layout.MarginBottom != 0.1 ||
		look.Primary.LineSpacing != 1.4 || look.Layout.LineGap != 1 || look.Notice.Side != appearance.AlignRight ||
		look.NoticeDuration != 3*time.Second || look.Selection.Width != 3 || look.Cue.BarThickness != 9 || look.Cue.CountdownSize != 1.2 {
		t.Fatalf("display looks %d, last %+v", looks, look)
	}
	again := settingsOf(t, r).Profile
	mute := again.Shortcuts[shortcutAt(t, again, shortcut.Mute)]
	if again.Shortcuts[0].Label != "P" || again.Primary.Color != "#FFE600" || mute.Chord != "" || mute.Label != "" {
		t.Fatalf("settings after saving = %+v, mute %+v", again.Shortcuts[0], mute)
	}
}

// While the settings are edited the video is the preview: the screen's
// choices reach it at once, nothing is written, and leaving them unsaved puts
// the saved look back (user report: a style picked in the panel never showed
// on the video).
func TestThePreviewShowsTheChoicesOnTheVideoWithoutKeepingThem(t *testing.T) {
	t.Parallel()
	r := startWatching(t)
	view := settingsOf(t, r).Profile
	savedSize := view.Primary.FontSize
	r.settings.mu.Lock()
	saves := r.settings.saves
	r.settings.mu.Unlock()

	view.Primary.FontSize = 40
	view.Translation.Color = "#FFE600"
	if err := r.api.PreviewLook(view); err != nil {
		t.Fatal(err)
	}
	look, looks := r.display.lastLook()
	if looks != 1 || look.Primary.FontSize != 40 || look.Translation.Color.String() != "#FFE600" {
		t.Fatalf("display looks %d, last %+v", looks, look)
	}
	r.settings.mu.Lock()
	written, kept := r.settings.saves, r.settings.profiles.Items[0].Primary.FontSize
	r.settings.mu.Unlock()
	if written != saves || kept != savedSize || settingsOf(t, r).Profile.Primary.FontSize != savedSize {
		t.Fatalf("the preview was kept: %d saves (was %d), saved size %v", written, saves, kept)
	}

	// A choice the profile refuses leaves the video as it is.
	view.Primary.FontSize = -1
	if err := r.api.PreviewLook(view); err == nil {
		t.Fatal("a refused choice was previewed")
	}
	if _, after := r.display.lastLook(); after != 1 {
		t.Fatalf("the refused choice reached the video (%d looks)", after)
	}

	r.api.EndPreview()
	look, looks = r.display.lastLook()
	if looks != 2 || look.Primary.FontSize != savedSize {
		t.Fatalf("after the preview: %d looks, size %v (saved %v)", looks, look.Primary.FontSize, savedSize)
	}
}

// The karaoke mode of the video being watched (from its project or the bar)
// is the video's: saving the settings changes only what changed in the
// profile.
func TestSavingTheSettingsKeepsTheVideosKaraoke(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	kept := project.New()
	kept.Last.Karaoke = &project.Karaoke{Enabled: true, MinGap: 10_000, Lead: 5_000, Indicator: study.IndicatorCountdown, Fill: true, Repeat: 2}
	if err := r.projects.Save(movie, kept); err != nil {
		t.Fatal(err)
	}
	r.attach(t)
	if _, err := r.api.OpenVideo(movie); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	testkit.Eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	want := app.Karaoke{Enabled: true, Indicator: "countdown", Repeat: 2}
	if got := r.api.PlaybackState().Karaoke; got != want {
		t.Fatalf("karaoke on opening = %+v, want the video's %+v", got, want)
	}
	save := func(change func(*app.ProfileSettings)) {
		t.Helper()
		view := settingsOf(t, r).Profile
		change(&view)
		if err := r.api.SaveProfile(view); err != nil {
			t.Fatal(err)
		}
	}
	save(func(v *app.ProfileSettings) { v.Primary.FontSize = 40 })
	if got := r.api.PlaybackState().Karaoke; got != want {
		t.Fatalf("after saving a font, karaoke = %+v, want the video's %+v", got, want)
	}
	// What changes in the profile still reaches the video.
	save(func(v *app.ProfileSettings) { v.Karaoke.Repeat = 4 })
	if got := r.api.PlaybackState().Karaoke; got != (app.Karaoke{Enabled: true, Indicator: "countdown", Repeat: 4}) {
		t.Fatalf("after changing the repeats, karaoke = %+v", got)
	}
	save(func(v *app.ProfileSettings) { v.Karaoke.Indicator = "countdown"; v.Karaoke.Enabled = true })
	save(func(v *app.ProfileSettings) { v.Karaoke.Indicator = "bar"; v.Karaoke.Enabled = false })
	if got := r.api.PlaybackState().Karaoke; got != (app.Karaoke{Enabled: false, Indicator: "bar", Repeat: 4}) {
		t.Fatalf("after turning it off with a bar, karaoke = %+v", got)
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

// Each refusal names, in Portuguese, the field it is about; the limits in
// the messages come from the domain, which tests them.
func TestSaveProfileRefusesBadChoicesInPortuguese(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	base := settingsOf(t, r).Profile
	cases := []struct {
		name   string
		change func(*app.ProfileSettings)
		want   string
	}{
		{"font too big", func(p *app.ProfileSettings) { p.Primary.FontSize = 500 }, "Legenda de cima: o tamanho da fonte"},
		{"NaN font", func(p *app.ProfileSettings) { p.Translation.FontSize = math.NaN() }, "Legenda de baixo: o tamanho da fonte"},
		{"bad color", func(p *app.ProfileSettings) { p.Primary.OutlineColor = "red" }, `a cor "red" não é válida`},
		{"no font", func(p *app.ProfileSettings) { p.Primary.FontFamily = " " }, "escolha uma fonte"},
		{"opacity", func(p *app.ProfileSettings) { p.Primary.Opacity = 1.5 }, "a opacidade"},
		{"margin", func(p *app.ProfileSettings) { p.Layout.MarginLeft = 0.3 }, "Margens: cada margem"},
		{"lead", func(p *app.ProfileSettings) { p.Karaoke.Lead = 20 }, "Modo Karaokê: a lacuna mínima"},
		{"indicator", func(p *app.ProfileSettings) { p.Karaoke.Indicator = "dots" }, "escolha a barra ou a contagem"},
		{"repeat", func(p *app.ProfileSettings) { p.Karaoke.Repeat = 11 }, "Modo Karaokê: repetir cada fala"},
		{"speed", func(p *app.ProfileSettings) { p.Speed = 3 }, "Escolha uma das velocidades"},
		{"sync step", func(p *app.ProfileSettings) { p.SyncStep = 5 }, "Passos da sincronização"},
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
		{"precision", func(p *app.ProfileSettings) { p.PrecisionStep = 5 }, "Passo de precisão"},
		{"unknown command", func(p *app.ProfileSettings) { p.Shortcuts[0].Command = "launch-rockets" }, "comando desconhecido"},
		{"line spacing", func(p *app.ProfileSettings) { p.Primary.LineSpacing = 3 }, "Legenda de cima: a entrelinha"},
		{"gap", func(p *app.ProfileSettings) { p.Layout.LineGap = math.NaN() }, "Margens: a distância entre as legendas"},
		{"notice", func(p *app.ProfileSettings) { p.Marks.Notice.FontSize = 0 }, "Avisos no vídeo: o tamanho da fonte"},
		{"notice time", func(p *app.ProfileSettings) { p.Marks.NoticeDuration = 0.1 }, "Avisos no vídeo: a duração"},
		{"selection", func(p *app.ProfileSettings) { p.Marks.Selection.Edge = "preto" }, `Fala selecionada: a cor "preto"`},
		{"bar", func(p *app.ProfileSettings) { p.Karaoke.BarThickness = 50 }, "Modo Karaokê: a espessura da barra"},
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
		call.screen.Primary != "This is a preview." || call.screen.Translation != "Esta é uma prévia." ||
		!call.screen.TranslationVisible || call.screen.Highlighted {
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

func TestThePreviewMarksTheLineBeingEdited(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r).Profile
	request := app.PreviewRequest{
		Primary: view.Primary, Translation: view.Translation, Layout: view.Layout, Marks: view.Marks, Width: 640, Height: 360, Scale: 1,
	}
	for i, c := range []struct {
		highlight string
		want      session.Line
	}{{"primary", session.LinePrimary}, {"translation", session.LineTranslation}} {
		request.Highlight = c.highlight
		if _, err := r.api.PreviewSubtitles(request); err != nil {
			t.Fatalf("Highlight %q: %v", c.highlight, err)
		}
		if screen := r.preview.requests[i].screen; !screen.Highlighted || screen.Highlight != c.want {
			t.Fatalf("Highlight %q drew %+v", c.highlight, screen)
		}
	}
}

func TestThePreviewRefusesAnUnknownLine(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r).Profile
	request := app.PreviewRequest{Primary: view.Primary, Translation: view.Translation, Layout: view.Layout, Width: 640, Height: 360, Scale: 1}
	for _, highlight := range []string{"middle", "Primary", " primary", "1"} {
		request.Highlight = highlight
		if _, err := r.api.PreviewSubtitles(request); err == nil {
			t.Errorf("Highlight %q was drawn", highlight)
		}
	}
	if len(r.preview.requests) != 0 {
		t.Fatalf("%d unknown lines reached the preview", len(r.preview.requests))
	}
}

func TestThePreviewShowsTheKaraokeFill(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r).Profile
	request := app.PreviewRequest{Primary: view.Primary, Translation: view.Translation, Layout: view.Layout, Width: 640, Height: 360, Scale: 1}
	request.View, request.Karaoke.FillColor = "fill", "#FF8C00"
	if _, err := r.api.PreviewSubtitles(request); err != nil {
		t.Fatalf("fill view: %v", err)
	}
	orange, _ := appearance.ParseColor("#FF8C00")
	call := r.preview.requests[0]
	if call.look.FillColor != orange || call.screen.Fill != 0.5 || call.screen.Cue != nil {
		t.Fatalf("fill view drew look %+v, screen %+v", call.look, call.screen)
	}
	// Between lines nothing is spoken, whatever color comes along.
	request.View, request.Karaoke.FillColor = "", "#12"
	if _, err := r.api.PreviewSubtitles(request); err != nil || r.preview.requests[1].screen.Fill != 0 {
		t.Fatalf("plain view: %v, fill %v", err, r.preview.requests[1].screen.Fill)
	}
}

// Over the high-contrast yellow the karaoke yellow cannot be seen: the screen
// is told which color stands in for it, and the preview paints that one.
func TestTheFillHiddenByTheLineShowsItsStandIn(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	check, err := r.api.CheckFill("#FFE600", "#FFD23F")
	if err != nil || !check.Changed || check.Used == "#FFD23F" {
		t.Fatalf("CheckFill = %+v, %v", check, err)
	}
	if kept, err := r.api.CheckFill("#FFFFFF", "#FFD23F"); err != nil || kept.Changed || kept.Used != "#FFD23F" {
		t.Fatalf("over white: %+v, %v", kept, err)
	}
	for _, c := range [][2]string{{"#12", "#FFD23F"}, {"#FFE600", "amarelo"}} {
		if _, err := r.api.CheckFill(c[0], c[1]); err == nil {
			t.Errorf("CheckFill(%q, %q) was accepted", c[0], c[1])
		}
	}
	view := settingsOf(t, r).Profile
	request := app.PreviewRequest{Primary: view.Primary, Translation: view.Translation, Layout: view.Layout, Width: 640, Height: 360, Scale: 1}
	request.Primary.Color, request.View, request.Karaoke.FillColor = "#FFE600", "fill", "#FFD23F"
	if _, err := r.api.PreviewSubtitles(request); err != nil {
		t.Fatal(err)
	}
	if painted := r.preview.requests[0].look.FillColor.String(); painted != check.Used {
		t.Fatalf("the preview painted %s, the screen says %s", painted, check.Used)
	}
}

func TestThePreviewRefusesAnUnknownViewOrFill(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r).Profile
	request := app.PreviewRequest{Primary: view.Primary, Translation: view.Translation, Layout: view.Layout, Width: 640, Height: 360, Scale: 1}
	for _, c := range []struct{ view, fill string }{
		{"karaoke", "#FF8C00"}, {"Fill", "#FF8C00"}, {" fill", "#FF8C00"}, {"Cue", "#FF8C00"},
		{"fill", "#12"}, {"fill", ""}, {"fill", "amarelo"},
	} {
		request.View, request.Karaoke.FillColor = c.view, c.fill
		if _, err := r.api.PreviewSubtitles(request); err == nil {
			t.Errorf("view %q with fill %q was drawn", c.view, c.fill)
		}
	}
	if len(r.preview.requests) != 0 {
		t.Fatalf("%d refused previews reached the renderer", len(r.preview.requests))
	}
}

func TestThePreviewShowsTheMarksAsTheDraftHasThem(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r).Profile
	view.Karaoke.Indicator, view.Karaoke.BarThickness, view.Karaoke.BarColor = "countdown", 12, "#FF8C00"
	view.Marks.Notice.Side, view.Marks.Selection.Width = "right", 4
	request := app.PreviewRequest{
		Primary: view.Primary, Translation: view.Translation, Layout: view.Layout, Width: 640, Height: 360, Scale: 1,
		Karaoke: view.Karaoke, Marks: view.Marks,
	}
	request.View = "cue"
	if _, err := r.api.PreviewSubtitles(request); err != nil {
		t.Fatalf("cue view: %v", err)
	}
	orange, _ := appearance.ParseColor("#FF8C00")
	call := r.preview.requests[0]
	if cue := call.screen.Cue; cue == nil || cue.Indicator != study.IndicatorCountdown || cue.Seconds <= 0 ||
		call.look.Cue.BarThickness != 12 || call.look.Cue.BarColor != orange || call.notice != "" {
		t.Fatalf("cue view drew look %+v, screen %+v, notice %q", call.look.Cue, call.screen.Cue, call.notice)
	}
	request.View, request.Highlight = "notice", "translation"
	if _, err := r.api.PreviewSubtitles(request); err != nil {
		t.Fatalf("notice view: %v", err)
	}
	call = r.preview.requests[1]
	if call.notice == "" || call.look.Notice.Side != appearance.AlignRight || call.look.Selection.Width != 4 || call.screen.Cue != nil {
		t.Fatalf("notice view drew notice %q, look %+v / %+v", call.notice, call.look.Notice, call.look.Selection)
	}
	// Each part is read only by the view that shows it.
	request.View, request.Highlight, request.Karaoke.BarColor = "notice", "", "laranja"
	if _, err := r.api.PreviewSubtitles(request); err != nil {
		t.Fatalf("a bad bar color stopped the notice view: %v", err)
	}
	request.View = "cue"
	if _, err := r.api.PreviewSubtitles(request); err == nil || len(r.preview.requests) != 3 {
		t.Fatal("a cue with a bad bar color was drawn")
	}
}

func TestPlayerPaths(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	if err := r.api.OpenSettingsFolder(); err != nil {
		t.Fatal(err)
	}
	if len(r.ui.urls) != 1 || r.ui.urls[0] != `folder:C:\Users\x\AppData\Roaming\LingoReel` {
		t.Fatalf("opened %v", r.ui.urls)
	}
}

// The interface's language: the app lists the ones it speaks, keeps the
// one chosen for the next start, or none for Windows', and refuses one it
// does not speak.
func TestTheInterfaceLanguage(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	view := settingsOf(t, r)
	if view.General.Language != "" || len(view.General.Languages) == 0 || view.General.Languages[0].Value != "pt-BR" ||
		view.General.Languages[0].Label != "Português (Brasil)" {
		t.Fatalf("languages = %q / %+v", view.General.Language, view.General.Languages)
	}
	if got := r.api.Preferences().Language; got != "pt-BR" {
		t.Fatalf("the interface speaks %q", got)
	}
	if err := r.api.SetLanguage("pt-BR"); err != nil || r.settings.config.Language != "pt-BR" {
		t.Fatalf("SetLanguage(pt-BR) = %v, kept %q", err, r.settings.config.Language)
	}
	if err := r.api.SetLanguage("xx"); err == nil || r.settings.config.Language != "pt-BR" {
		t.Fatalf("SetLanguage(xx) = %v, kept %q", err, r.settings.config.Language)
	}
	if err := r.api.SetLanguage(""); err != nil || r.settings.config.Language != "" {
		t.Fatalf("SetLanguage(\"\") = %v, kept %q", err, r.settings.config.Language)
	}
}
