package settingsfile_test

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/jsonstore"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/settingsfile"
)

func TestSettingsStartWithTheDefaults(t *testing.T) {
	t.Parallel()
	files := settingsfile.Open(t.TempDir())
	config, err := files.LoadConfig()
	if err != nil || config.Theme != settings.ThemeSystem || config.PlayerPaths == nil {
		t.Fatalf("LoadConfig on a first run = %+v, %v", config, err)
	}
	profiles, err := files.LoadProfiles()
	if err != nil || len(profiles.Items) != 1 || profiles.Items[0].Name != settings.DefaultProfileName {
		t.Fatalf("LoadProfiles on a first run = %+v, %v", profiles, err)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := settingsfile.Open(dir)
	config := settings.DefaultConfig()
	config.Theme = settings.ThemeDark
	config.PlayerPaths["vlc"] = `D:\Apps\VLC\vlc.exe`
	config.InstallRecommendations.Dismissed = []string{"mpv"}
	if err := files.SaveConfig(config); err != nil {
		t.Fatal(err)
	}
	profiles := settings.DefaultProfiles()
	profiles.Items[0].PreferredPlayer = "vlc"
	profiles.Items[0].Translation.Background.Enabled = true
	if err := files.SaveProfiles(profiles); err != nil {
		t.Fatal(err)
	}

	again := settingsfile.Open(dir)
	gotConfig, err := again.LoadConfig()
	if err != nil || gotConfig.Theme != settings.ThemeDark || gotConfig.PlayerPaths["vlc"] != `D:\Apps\VLC\vlc.exe` ||
		!gotConfig.InstallRecommendations.IsDismissed("mpv") {
		t.Fatalf("config after a round trip = %+v, %v", gotConfig, err)
	}
	gotProfiles, err := again.LoadProfiles()
	if err != nil || !reflect.DeepEqual(gotProfiles, profiles) {
		t.Fatalf("profiles after a round trip = %+v, %v\nwant %+v", gotProfiles, err, profiles)
	}
	// People may edit the file by hand: times and colors are readable.
	data, err := os.ReadFile(filepath.Join(dir, "profiles.json"))
	if err != nil || !strings.Contains(string(data), `"precisionSeekStep": "100ms"`) || !strings.Contains(string(data), `"color": "#FFFFFF"`) {
		t.Fatalf("profiles.json is not readable by people: %v\n%s", err, data)
	}
}

// Profiles saved before the karaoke mode (version 1) get its defaults; the
// rest of each profile stays as the user left it. Two profiles: decoding
// over the default list, the first one would inherit the defaults anyway.
func TestOldProfilesGetTheKaraokeDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	profile := `{"name": "NAME", "preferredPlayer": "vlc",
		"languages": {"audio": "fr", "primary": "fr", "translation": "pt-BR"},
		"primaryStyle": PRIMARY, "translationStyle": TRANSLATION, "layout": LAYOUT,
		"study": {"mode": "dual", "reveal": "hold"}, "speed": 0.75,
		"sync": {"precisionSeekStep": "100ms", "offsetStep": "50ms", "offsetLargeStep": "500ms"}}`
	old := `{"version": 1, "profiles": [` + strings.Replace(profile, "NAME", "Francês", 1) + `,` +
		strings.Replace(profile, "NAME", "Espanhol", 1) + `]}`
	defaults := settings.DefaultProfile()
	for placeholder, value := range map[string]any{"PRIMARY": defaults.Primary, "TRANSLATION": defaults.Translation, "LAYOUT": defaults.Layout} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		old = strings.ReplaceAll(old, placeholder, string(data))
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	profiles, err := settingsfile.Open(dir).LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles of a version 1 file: %v", err)
	}
	if len(profiles.Items) != 2 {
		t.Fatalf("%d profiles after the migration", len(profiles.Items))
	}
	for i, name := range []string{"Francês", "Espanhol"} {
		got := profiles.Items[i]
		if got.Name != name || got.Study.Mode != "dual" || got.Speed != 0.75 || got.PreferredPlayer != "vlc" {
			t.Fatalf("the migration changed profile %d: %+v", i, got)
		}
		if got.Study.Karaoke != settings.DefaultKaraoke() {
			t.Fatalf("profile %d karaoke = %+v, want the defaults %+v", i, got.Study.Karaoke, settings.DefaultKaraoke())
		}
		// Saved before the shortcuts could change: every one is the default.
		if !maps.Equal(got.Shortcuts.Resolved(), shortcut.Defaults()) {
			t.Fatalf("profile %d shortcuts = %+v", i, got.Shortcuts.Resolved())
		}
	}
}

// A config saved before the sync notice could be turned off keeps it on;
// one that turned it off keeps it off, with the window where it was.
func TestConfigFieldsAddedLaterKeepTheirMeaning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	old := `{"version":1,"theme":"dark","activeProfile":"Inglês","playerPaths":{},"installRecommendations":{"dismissAll":false,"dismissed":[]}}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	files := settingsfile.Open(dir)
	config, err := files.LoadConfig()
	defaults := settings.DefaultConfig()
	if err != nil || !config.SyncNotice || !config.AlwaysOnTop || config.Theme != settings.ThemeDark || config.Window.Saved() ||
		config.Bar != defaults.Bar || config.Edit != defaults.Edit || config.Interface != defaults.Interface || config.Voice != defaults.Voice {
		t.Fatalf("old config = %+v, %v", config, err)
	}
	config.SyncNotice = false
	config.Window = settings.WindowBounds{X: -800, Y: 20, Width: 900, Height: 640, Maximized: true}
	if err := files.SaveConfig(config); err != nil {
		t.Fatal(err)
	}
	again, err := settingsfile.Open(dir).LoadConfig()
	if err != nil || again.SyncNotice || again.Window != config.Window {
		t.Fatalf("config after a round trip = %+v, %v", again, err)
	}
}

func TestChangedShortcutsSurviveARoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	profiles := settings.DefaultProfiles()
	profiles.Items[0].Shortcuts = shortcut.Map{shortcut.Mute: {Modifiers: shortcut.Ctrl, Key: "KeyU"}}
	if err := settingsfile.Open(dir).SaveProfiles(profiles); err != nil {
		t.Fatal(err)
	}
	got, err := settingsfile.Open(dir).LoadProfiles()
	if err != nil || got.Items[0].Shortcuts.Of(shortcut.Mute).String() != "Ctrl+KeyU" || got.Items[0].Shortcuts.Of(shortcut.PlayPause).Key != shortcut.KeySpace {
		t.Fatalf("profiles = %+v, %v", got.Items, err)
	}
	// A hand-edited conflict is refused: the defaults are used instead.
	data := `{"version": 2, "profiles": [` + profileJSON(t, "Inglês", `{"mute": "Space"}`) + `]}`
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := settingsfile.Open(dir).LoadProfiles(); !errors.Is(err, jsonstore.ErrInvalid) {
		t.Fatalf("a conflicting file = %v, want ErrInvalid", err)
	}
}

// A profile that gave Ctrl+F9 or Ctrl+F10 to a command of its own before
// those keys marked passages keeps them there: the marks start without a
// shortcut (or with the other key), and the profile still loads. Without the
// migration the whole profile would be refused and replaced by the defaults.
func TestProfilesUsingThePassageKeysKeepThem(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data := `{"version": 3, "profiles": [` +
		profileJSON(t, "Mine", `{"undo": "Ctrl+F9", "redo": "Ctrl+F10"}`) + `,` +
		profileJSON(t, "Only one", `{"mute": "Ctrl+F10"}`) + `,` +
		profileJSON(t, "No keys", `{}`) + `,` +
		profileJSON(t, "Had it already", `{"undo": "Ctrl+F9", "passage-mark-start": "Ctrl+KeyB"}`) + `]}`
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	profiles, err := settingsfile.Open(dir).LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles of a version 3 file: %v", err)
	}
	want := []struct {
		name                                 string
		undo, redo, mute, markStart, markEnd string
	}{
		{"Mine", "Ctrl+F9", "Ctrl+F10", "KeyM", "", ""},
		{"Only one", "Ctrl+KeyZ", "Ctrl+KeyY", "Ctrl+F10", "Ctrl+F9", ""},
		{"No keys", "Ctrl+KeyZ", "Ctrl+KeyY", "KeyM", "Ctrl+F9", "Ctrl+F10"},
		{"Had it already", "Ctrl+F9", "Ctrl+KeyY", "KeyM", "Ctrl+KeyB", "Ctrl+F10"},
	}
	if len(profiles.Items) != len(want) {
		t.Fatalf("%d profiles after the migration", len(profiles.Items))
	}
	for i, w := range want {
		keys := profiles.Items[i].Shortcuts
		got := []string{keys.Of(shortcut.Undo).String(), keys.Of(shortcut.Redo).String(), keys.Of(shortcut.Mute).String(),
			keys.Of(shortcut.PassageMarkStart).String(), keys.Of(shortcut.PassageMarkEnd).String()}
		expected := []string{w.undo, w.redo, w.mute, w.markStart, w.markEnd}
		if profiles.Items[i].Name != w.name || !slices.Equal(got, expected) {
			t.Errorf("profile %q: undo, redo, mute, marks = %q, want %q", profiles.Items[i].Name, got, expected)
		}
		if err := keys.Validate(); err != nil {
			t.Errorf("profile %q conflicts after the migration: %v", w.name, err)
		}
	}
}

// profileJSON is a default profile named name with the given shortcuts.
func profileJSON(t *testing.T, name, shortcuts string) string {
	t.Helper()
	profile := settings.DefaultProfile()
	profile.Name = name
	profile.Shortcuts = nil
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Replace(string(data), `"shortcuts":null`, `"shortcuts":`+shortcuts, 1)
}

// Profiles saved before the subtitles moved to the app's own font (version
// 2) follow, unless the user had picked a font of their own. Two profiles:
// decoding over the default list, the first one would inherit the defaults
// anyway.
func TestOldProfilesFollowTheAppFont(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	defaults := settings.DefaultProfile()
	styles := func(family string) (string, string) {
		primary, translation := defaults.Primary, defaults.Translation
		primary.FontFamily, translation.FontFamily = family, family
		first, err := json.Marshal(primary)
		if err != nil {
			t.Fatal(err)
		}
		second, err := json.Marshal(translation)
		if err != nil {
			t.Fatal(err)
		}
		return string(first), string(second)
	}
	profile := func(name, family string) string {
		primary, translation := styles(family)
		layout, err := json.Marshal(defaults.Layout)
		if err != nil {
			t.Fatal(err)
		}
		return `{"name": "` + name + `", "languages": {"audio": "en", "primary": "en", "translation": "pt-BR"},
			"primaryStyle": ` + primary + `, "translationStyle": ` + translation + `, "layout": ` + string(layout) + `,
			"study": {"mode": "dual", "reveal": "hold", "karaoke": ` + karaokeJSON(t) + `}, "speed": 1,
			"sync": {"precisionSeekStep": "100ms", "offsetStep": "50ms", "offsetLargeStep": "500ms"}}`
	}
	old := `{"version": 2, "profiles": [` + profile("Padrão", appearance.PreviousDefaultFont) + `,` +
		profile("Escolhida", "Comic Sans MS") + `]}`
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	profiles, err := settingsfile.Open(dir).LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles of a version 2 file: %v", err)
	}
	if len(profiles.Items) != 2 {
		t.Fatalf("%d profiles after the migration", len(profiles.Items))
	}
	for i, want := range []string{appearance.AppFont, "Comic Sans MS"} {
		got := profiles.Items[i]
		if got.Primary.FontFamily != want || got.Translation.FontFamily != want {
			t.Fatalf("profile %q fonts = %q and %q, want %q",
				got.Name, got.Primary.FontFamily, got.Translation.FontFamily, want)
		}
	}
}

// Profiles saved before the looks over the video could change (version 4)
// get them as they were drawn: the line spacing, the gap, the marks and the
// karaoke's bar in the profile's own preview color. A value a profile has
// stays. Two profiles and more: decoding over the default list, only the
// first one would inherit the defaults anyway.
func TestOldProfilesKeepTheLooksTheyHad(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	version4 := func(name, preview string, gap any) map[string]any {
		var profile map[string]any
		data, err := json.Marshal(settings.DefaultProfile())
		if err == nil {
			err = json.Unmarshal(data, &profile)
		}
		if err != nil {
			t.Fatal(err)
		}
		profile["name"] = name
		delete(profile, "marks")
		delete(profile["primaryStyle"].(map[string]any), "lineSpacing")
		delete(profile["translationStyle"].(map[string]any), "lineSpacing")
		delete(profile["layout"].(map[string]any), "lineGap")
		if gap != nil {
			profile["layout"].(map[string]any)["lineGap"] = gap
		}
		karaoke := profile["study"].(map[string]any)["karaoke"].(map[string]any)
		delete(karaoke, "cue")
		karaoke["previewColor"] = preview
		return profile
	}
	old, err := json.Marshal(map[string]any{"version": 4, "profiles": []any{
		version4("Padrão", "#C8CDD4", nil), version4("Azul", "#3366FF", nil), version4("Afastado", "#C8CDD4", 1.5),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), old, 0o600); err != nil {
		t.Fatal(err)
	}
	profiles, err := settingsfile.Open(dir).LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles of a version 4 file: %v", err)
	}
	defaults := settings.DefaultProfile()
	for i, want := range []struct {
		bar string
		gap float64
	}{{"#C8CDD4", 0.2}, {"#3366FF", 0.2}, {"#C8CDD4", 1.5}} {
		got := profiles.Items[i]
		cue := got.Study.Karaoke.Cue
		if cue.BarColor.String() != want.bar || cue.BarThickness != defaults.Study.Karaoke.Cue.BarThickness ||
			cue.CountdownSize != defaults.Study.Karaoke.Cue.CountdownSize {
			t.Errorf("profile %q: karaoke cue %+v, want the default with a %s bar", got.Name, cue, want.bar)
		}
		if got.Layout.LineGap != want.gap || got.Primary.LineSpacing != 1 || got.Translation.LineSpacing != 1 {
			t.Errorf("profile %q: gap %v, spacings %v/%v; want gap %v and spacing 1",
				got.Name, got.Layout.LineGap, got.Primary.LineSpacing, got.Translation.LineSpacing, want.gap)
		}
		if got.Marks != defaults.Marks {
			t.Errorf("profile %q: marks %+v, want the defaults", got.Name, got.Marks)
		}
	}
}

// A profile saved with the old frame around the line picked for a
// correction gets the new, cleaner one; a frame the user changed stays.
func TestOldProfilesGetTheCleanerSelection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	version5 := func(name string, selection appearance.SelectionLook) map[string]any {
		profile := settings.DefaultProfile()
		profile.Name, profile.Marks.Selection = name, selection
		var object map[string]any
		data, err := json.Marshal(profile)
		if err == nil {
			err = json.Unmarshal(data, &object)
		}
		if err != nil {
			t.Fatal(err)
		}
		return object
	}
	mine := appearance.PreviousSelectionLook()
	mine.Width = 3
	old, err := json.Marshal(map[string]any{"version": 5, "profiles": []any{
		version5("Padrão", appearance.PreviousSelectionLook()), version5("Mine", mine),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), old, 0o600); err != nil {
		t.Fatal(err)
	}
	profiles, err := settingsfile.Open(dir).LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles of a version 5 file: %v", err)
	}
	if got := profiles.Items[0].Marks.Selection; got != appearance.DefaultSelectionLook() {
		t.Errorf("the old default frame became %+v, want the new default", got)
	}
	if got := profiles.Items[1].Marks.Selection; got != mine {
		t.Errorf("the user's frame became %+v, want it kept", got)
	}
}
func karaokeJSON(t *testing.T) string {
	t.Helper()
	data, err := json.Marshal(settings.DefaultKaraoke())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A document without study preferences is refused, not silently patched.
func TestBrokenOldProfilesAreRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(`{"version": 1, "profiles": [{"name": "X"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := settingsfile.Open(dir).LoadProfiles(); err == nil {
		t.Fatal("a profile without study preferences loaded")
	}
}

func TestSettingsRefuseBadValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := settingsfile.Open(dir)
	config := settings.DefaultConfig()
	config.PlayerPaths["vlc"] = `vlc.exe`
	if err := files.SaveConfig(config); !errors.Is(err, settingsfile.ErrInvalidPlayerPath) {
		t.Fatalf("saving a relative player path = %v, want ErrInvalidPlayerPath", err)
	}
	config = settings.DefaultConfig()
	config.Theme = "purple"
	if err := files.SaveConfig(config); err == nil {
		t.Fatal("saved an unknown theme")
	}
	if err := files.SaveProfiles(settings.Profiles{}); err == nil {
		t.Fatal("saved no profile at all")
	}

	// A hand-edited file pointing VLC at a script falls back to the defaults.
	edited := `{"version":1,"theme":"dark","activeProfile":"Inglês","playerPaths":{"vlc":"C:\\x\\run.bat"},"installRecommendations":{"dismissAll":false,"dismissed":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := files.LoadConfig()
	if !errors.Is(err, settingsfile.ErrInvalidPlayerPath) || got.Theme != settings.ThemeSystem {
		t.Fatalf("LoadConfig of a hostile file = %+v, %v; want the defaults and ErrInvalidPlayerPath", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	profiles, err := files.LoadProfiles()
	if !errors.Is(err, jsonstore.ErrCorrupt) || len(profiles.Items) != 1 {
		t.Fatalf("LoadProfiles of a corrupt file = %+v, %v; want the defaults and ErrCorrupt", profiles, err)
	}
}
