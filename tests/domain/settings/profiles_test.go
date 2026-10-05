package settings_test

import (
	"errors"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
)

func twoProfiles(t *testing.T) settings.Profiles {
	t.Helper()
	profiles, err := settings.DefaultProfiles().Add("Francês", settings.DefaultProfile())
	if err != nil {
		t.Fatal(err)
	}
	return profiles
}

func TestAddCopiesTheBaseUnderANewName(t *testing.T) {
	base := settings.DefaultProfile()
	base.Speed = 0.75
	base.Shortcuts = shortcut.Map{shortcut.Mute: {Key: "KeyU"}}
	before := settings.DefaultProfiles()
	profiles, err := before.Add("Francês", base)
	if err != nil {
		t.Fatal(err)
	}
	added, found := profiles.Find("francês")
	if !found || added.Name != "Francês" || added.Speed != 0.75 || len(profiles.Items) != 2 || profiles.Items[1].Name != "Francês" {
		t.Fatalf("profiles = %+v", profiles.Items)
	}
	if len(before.Items) != 1 {
		t.Fatal("Add changed the profiles it was called on")
	}
	// The copy does not share the shortcuts with its base.
	added.Shortcuts[shortcut.Mute] = shortcut.Chord{Key: "KeyZ"}
	if base.Shortcuts[shortcut.Mute].Key != "KeyU" {
		t.Fatal("the copy shares its shortcuts with the base")
	}
	if err := profiles.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAddRefusesTakenAndBadNames(t *testing.T) {
	profiles := twoProfiles(t)
	for name, want := range map[string]error{
		"FRANCÊS":                settings.ErrProfileExists,
		"inglês":                 settings.ErrProfileExists,
		"":                       settings.ErrInvalidSettings,
		"  Alemão":               settings.ErrInvalidSettings,
		string(make([]byte, 61)): settings.ErrInvalidSettings,
	} {
		if _, err := profiles.Add(name, settings.DefaultProfile()); !errors.Is(err, want) {
			t.Errorf("Add(%q) = %v, want %v", name, err, want)
		}
	}
}

func TestRenameKeepsNamesUnique(t *testing.T) {
	profiles := twoProfiles(t)
	renamed, err := profiles.Rename("francês", "Francês (Paris)")
	if err != nil {
		t.Fatal(err)
	}
	if _, found := renamed.Find("Francês"); found {
		t.Fatal("the old name is still there")
	}
	if _, found := renamed.Find("Francês (Paris)"); !found || profiles.Items[1].Name != "Francês" {
		t.Fatalf("renamed = %+v; original = %+v", renamed.Items, profiles.Items)
	}
	// Only the case changes: allowed.
	if cased, err := profiles.Rename("Francês", "FRANCÊS"); err != nil || cased.Items[1].Name != "FRANCÊS" {
		t.Fatalf("case-only rename = %v", err)
	}
	if _, err := profiles.Rename("Francês", "inglês"); !errors.Is(err, settings.ErrProfileExists) {
		t.Fatalf("rename onto another profile = %v", err)
	}
	if _, err := profiles.Rename("Alemão", "Novo"); !errors.Is(err, settings.ErrProfileNotFound) {
		t.Fatalf("rename of a missing profile = %v", err)
	}
	if _, err := profiles.Rename("Francês", " "); !errors.Is(err, settings.ErrInvalidSettings) {
		t.Fatalf("rename to blank = %v", err)
	}
}

func TestRemoveKeepsTheLastProfile(t *testing.T) {
	profiles := twoProfiles(t)
	removed, err := profiles.Remove("INGLÊS")
	if err != nil {
		t.Fatal(err)
	}
	if len(removed.Items) != 1 || removed.Items[0].Name != "Francês" || len(profiles.Items) != 2 {
		t.Fatalf("removed = %+v, original = %+v", removed.Items, profiles.Items)
	}
	if _, err := removed.Remove("Francês"); !errors.Is(err, settings.ErrLastProfile) {
		t.Fatalf("removing the last = %v", err)
	}
	if _, err := profiles.Remove("Alemão"); !errors.Is(err, settings.ErrProfileNotFound) {
		t.Fatalf("removing a missing one = %v", err)
	}
}

func TestReplaceSwapsTheProfileOfTheSameName(t *testing.T) {
	profiles := twoProfiles(t)
	changed := settings.DefaultProfile()
	changed.Name = "Francês"
	changed.Speed = 1.25
	replaced, err := profiles.Replace(changed)
	if err != nil || replaced.Items[1].Speed != 1.25 || profiles.Items[1].Speed != 1 || replaced.Items[0].Speed != 1 {
		t.Fatalf("Replace = %v, %+v", err, replaced.Items)
	}
	changed.Name = "Alemão"
	if _, err := profiles.Replace(changed); !errors.Is(err, settings.ErrProfileNotFound) {
		t.Fatalf("Replace of a missing one = %v", err)
	}
}

func TestProfileValidateChecksTheShortcuts(t *testing.T) {
	profile := settings.DefaultProfile()
	profile.Shortcuts = shortcut.Map{shortcut.Mute: {Key: shortcut.KeySpace}}
	var conflict *shortcut.ConflictError
	if err := profile.Validate(); !errors.Is(err, settings.ErrInvalidSettings) || !errors.As(err, &conflict) {
		t.Fatalf("a conflicting profile = %v", err)
	}
	profile.Shortcuts = nil // an old profile: the defaults apply
	if err := profile.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigWindowBounds(t *testing.T) {
	cases := []struct {
		name   string
		window settings.WindowBounds
		valid  bool
		saved  bool
	}{
		{"none yet", settings.WindowBounds{}, true, false},
		{"a window", settings.WindowBounds{X: 100, Y: 50, Width: 1024, Height: 720}, true, true},
		{"on a monitor at the left", settings.WindowBounds{X: -1920, Y: 0, Width: 800, Height: 600, Maximized: true}, true, true},
		{"width only", settings.WindowBounds{Width: 800}, true, false},
		{"negative width", settings.WindowBounds{Width: -1, Height: 600}, false, false},
		{"huge height", settings.WindowBounds{Width: 800, Height: 1 << 20}, false, true},
		{"far away", settings.WindowBounds{X: 1 << 20, Width: 800, Height: 600}, false, true},
		{"far above", settings.WindowBounds{Y: -(1 << 20), Width: 800, Height: 600}, false, true},
	}
	for _, c := range cases {
		config := settings.DefaultConfig()
		config.Window = c.window
		if err := config.Validate(); (err == nil) != c.valid {
			t.Errorf("%s: Validate = %v, want valid %v", c.name, err, c.valid)
		}
		if c.window.Saved() != c.saved {
			t.Errorf("%s: Saved = %v", c.name, c.window.Saved())
		}
	}
	if !settings.DefaultConfig().SyncNotice {
		t.Fatal("the sync notice starts off")
	}
}
