package settingsfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/jsonstore"
)

const (
	// appFolder is the app's folder inside the user's settings folder
	// (%APPDATA% on Windows).
	appFolder = "LingoReel"
	// configVersion and profilesVersion are the current document versions.
	configVersion = 1
	// profilesVersion 2 added the karaoke mode, 3 moved the subtitles to the
	// app's own font, 4 made room for the passage marks, 5 added the looks
	// 6 gave the selection its cleaner frame and 7 moved the notices to the
	// right (migrations.go).
	profilesVersion = 7
)

// Dir is where the app keeps its settings: %APPDATA%\LingoReel.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding the settings folder: %w", err)
	}
	return filepath.Join(base, appFolder), nil
}

// Files are the settings documents: config.json and profiles.json. A document
// that could not be read is never written over with the defaults (guard.go):
// its read error is settings.ErrNewerVersion, ErrUnreadable or ErrDamaged.
type Files struct {
	config        *jsonstore.Store[settings.Config]
	profiles      *jsonstore.Store[settings.Profiles]
	configGuard   *guard
	profilesGuard *guard
}

// Open uses the documents in a folder; they are created on the first save.
func Open(dir string) *Files {
	return OpenAt(dir, time.Now)
}

// OpenAt is Open with the clock that names the copy of a damaged document.
func OpenAt(dir string, now func() time.Time) *Files {
	configPath, profilesPath := filepath.Join(dir, "config.json"), filepath.Join(dir, "profiles.json")
	return &Files{
		config:        jsonstore.New(configPath, configVersion, settings.DefaultConfig, nil),
		profiles:      jsonstore.New(profilesPath, profilesVersion, settings.DefaultProfiles, profilesMigrations),
		configGuard:   &guard{path: configPath, now: now},
		profilesGuard: &guard{path: profilesPath, now: now},
	}
}

// FirstRun tells the app has never kept its configuration here: the first
// time it opens on this account.
func (f *Files) FirstRun() bool {
	_, err := os.Stat(f.config.Path())
	return errors.Is(err, fs.ErrNotExist)
}

// LoadConfig reads the configuration, or the defaults on the first run or
// when it cannot be read.
func (f *Files) LoadConfig() (settings.Config, error) {
	config, err := f.readConfig()
	f.configGuard.learn(err)
	if err != nil {
		return settings.DefaultConfig(), err
	}
	return config, nil
}

func (f *Files) readConfig() (settings.Config, error) {
	config, err := f.config.Load()
	if err != nil {
		return config, classify(err)
	}
	if err := CheckPlayerPaths(config); err != nil {
		return config, fmt.Errorf("%w: %s: %w", settings.ErrDamaged, f.config.Path(), err)
	}
	if config.PlayerPaths == nil {
		config.PlayerPaths = map[string]string{}
	}
	return config, nil
}

// SaveConfig checks and writes the configuration: never over a document kept
// as it is (settings.ErrReadOnly), and after a damaged one is copied aside.
func (f *Files) SaveConfig(config settings.Config) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if err := CheckPlayerPaths(config); err != nil {
		return err
	}
	if !f.configGuard.known() {
		_, _ = f.LoadConfig() // learns the document's state
	}
	return f.configGuard.write(func() error { return f.config.Save(config) })
}

// LoadProfiles reads the study profiles, or the defaults on the first run or
// when they cannot be read.
func (f *Files) LoadProfiles() (settings.Profiles, error) {
	profiles, err := f.profiles.Load()
	err = classify(err)
	f.profilesGuard.learn(err)
	if err != nil {
		return settings.DefaultProfiles(), err
	}
	return profiles, nil
}

// SaveProfiles checks and writes the study profiles, as SaveConfig does.
func (f *Files) SaveProfiles(profiles settings.Profiles) error {
	if err := profiles.Validate(); err != nil {
		return err
	}
	if !f.profilesGuard.known() {
		_, _ = f.LoadProfiles() // learns the document's state
	}
	return f.profilesGuard.write(func() error { return f.profiles.Save(profiles) })
}
