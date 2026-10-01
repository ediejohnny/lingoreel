package settingsfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lingoreel/internal/domain/settings"
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
	for _, name := range []string{"config.json", "profiles.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !strings.Contains(string(data), `"version": 1`) {
			t.Fatalf("%s = %s, %v; want a versioned document", name, data, err)
		}
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
