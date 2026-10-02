package settingsfile_test

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/infrastructure/persistence/jsonstore"
	"lingoreel/internal/infrastructure/persistence/settingsfile"
)

func TestSettingsStartWithTheDefaults(t *testing.T) {
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
	if err != nil || gotProfiles.Items[0].PreferredPlayer != "vlc" {
		t.Fatalf("profiles after a round trip = %+v, %v", gotProfiles, err)
	}
	for name, version := range map[string]string{"config.json": `"version": 1`, "profiles.json": `"version": 2`} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !strings.Contains(string(data), version) {
			t.Fatalf("%s = %s, %v; want %s", name, data, err, version)
		}
	}
}

// Profiles saved before the Modo Karaokê (version 1) get its defaults; the
// rest of each profile stays as the user left it. Two profiles: decoding
// over the default list, the first one would inherit the defaults anyway.
func TestOldProfilesGetTheKaraokeDefaults(t *testing.T) {
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
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	old := `{"version":1,"theme":"dark","activeProfile":"Inglês","playerPaths":{},"installRecommendations":{"dismissAll":false,"dismissed":[]}}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	files := settingsfile.Open(dir)
	config, err := files.LoadConfig()
	if err != nil || !config.SyncNotice || config.Theme != settings.ThemeDark || config.Window.Saved() {
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

// A document without study preferences is refused, not silently patched.
func TestBrokenOldProfilesAreRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(`{"version": 1, "profiles": [{"name": "X"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := settingsfile.Open(dir).LoadProfiles(); err == nil {
		t.Fatal("a profile without study preferences loaded")
	}
}

func TestSettingsRefuseBadValues(t *testing.T) {
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

func TestDirIsInsideTheUserSettings(t *testing.T) {
	dir, err := settingsfile.Dir()
	if err != nil {
		t.Fatal(err)
	}
	base, _ := os.UserConfigDir()
	if filepath.Dir(dir) != base || filepath.Base(dir) != "LingoReel" {
		t.Fatalf("Dir = %q, want %q\\LingoReel", dir, base)
	}
}
