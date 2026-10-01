// Package settings models the user's persisted preferences: the global
// configuration and the study profiles.
package settings

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"lingoreel/internal/domain/player"
)

// ErrInvalidSettings reports a preference outside its allowed values.
var ErrInvalidSettings = errors.New("invalid settings")

// Theme is the application color theme.
type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

// InstallRecommendations remembers which player install suggestions the user
// dismissed.
type InstallRecommendations struct {
	DismissAll bool     `json:"dismissAll"`
	Dismissed  []string `json:"dismissed"`
}

// IsDismissed reports whether the suggestion for a player should stay hidden.
func (r InstallRecommendations) IsDismissed(playerID string) bool {
	return r.DismissAll || slices.Contains(r.Dismissed, playerID)
}

// Config holds the preferences shared by every profile.
type Config struct {
	Theme                  Theme                  `json:"theme"`
	ActiveProfile          string                 `json:"activeProfile"`
	PlayerPaths            map[string]string      `json:"playerPaths"`
	InstallRecommendations InstallRecommendations `json:"installRecommendations"`
}

// DefaultConfig returns the configuration used on the first run.
func DefaultConfig() Config {
	return Config{
		Theme:         ThemeSystem,
		ActiveProfile: DefaultProfileName,
		PlayerPaths:   map[string]string{},
	}
}

// Validate checks the theme, the player overrides and the dismissed players.
func (c Config) Validate() error {
	if c.Theme != ThemeSystem && c.Theme != ThemeLight && c.Theme != ThemeDark {
		return fmt.Errorf("%w: theme %q", ErrInvalidSettings, c.Theme)
	}
	if err := validateProfileName(c.ActiveProfile); err != nil {
		return fmt.Errorf("activeProfile: %w", err)
	}
	for playerID, path := range c.PlayerPaths {
		if !player.ID(playerID).Valid() {
			return fmt.Errorf("%w: player id %q", ErrInvalidSettings, playerID)
		}
		// Whether it is an absolute .exe path is an operating system rule,
		// checked where the settings are stored (settingsfile).
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("%w: empty player path for %q", ErrInvalidSettings, playerID)
		}
	}
	for _, playerID := range c.InstallRecommendations.Dismissed {
		if !player.ID(playerID).Valid() {
			return fmt.Errorf("%w: dismissed player id %q", ErrInvalidSettings, playerID)
		}
	}
	return nil
}
