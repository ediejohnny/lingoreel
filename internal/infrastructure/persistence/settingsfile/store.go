package settingsfile

import (
	"fmt"
	"os"
	"path/filepath"

	"lingoreel/internal/domain/settings"
	"lingoreel/internal/infrastructure/persistence/jsonstore"
)

const (
	// appFolder is the app's folder inside the user's settings folder
	// (%APPDATA% on Windows).
	appFolder = "LingoReel"
	// configVersion and profilesVersion are the current document versions.
	configVersion = 1
	// profilesVersion 2 added the Modo Karaokê (migrations.go).
	profilesVersion = 2
)

// Dir is where the app keeps its settings: %APPDATA%\LingoReel.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding the settings folder: %w", err)
	}
	return filepath.Join(base, appFolder), nil
}

// Files are the settings documents: config.json and profiles.json.
type Files struct {
	config   *jsonstore.Store[settings.Config]
	profiles *jsonstore.Store[settings.Profiles]
}

// Open uses the documents in a folder; they are created on the first save.
func Open(dir string) *Files {
	return &Files{
		config:   jsonstore.New(filepath.Join(dir, "config.json"), configVersion, settings.DefaultConfig, nil),
		profiles: jsonstore.New(filepath.Join(dir, "profiles.json"), profilesVersion, settings.DefaultProfiles, profilesMigrations),
	}
}

// LoadConfig reads the configuration, or the defaults on the first run.
func (f *Files) LoadConfig() (settings.Config, error) {
	config, err := f.config.Load()
	if err != nil {
		return settings.DefaultConfig(), err
	}
	if err := CheckPlayerPaths(config); err != nil {
		return settings.DefaultConfig(), fmt.Errorf("%s: %w", f.config.Path(), err)
	}
	if config.PlayerPaths == nil {
		config.PlayerPaths = map[string]string{}
	}
	return config, nil
}

// SaveConfig checks and writes the configuration.
func (f *Files) SaveConfig(config settings.Config) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if err := CheckPlayerPaths(config); err != nil {
		return err
	}
	return f.config.Save(config)
}

// LoadProfiles reads the study profiles, or the defaults on the first run.
func (f *Files) LoadProfiles() (settings.Profiles, error) {
	profiles, err := f.profiles.Load()
	if err != nil {
		return settings.DefaultProfiles(), err
	}
	return profiles, nil
}

// SaveProfiles checks and writes the study profiles.
func (f *Files) SaveProfiles(profiles settings.Profiles) error {
	if err := profiles.Validate(); err != nil {
		return err
	}
	return f.profiles.Save(profiles)
}
