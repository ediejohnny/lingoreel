// Package settings models the user's persisted preferences: the global
// configuration and the study profiles.
package settings

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

// playerIDPattern restricts player identifiers such as "mpv" or "potplayer".
var playerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

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
		if !playerIDPattern.MatchString(playerID) {
			return fmt.Errorf("%w: player id %q", ErrInvalidSettings, playerID)
		}
		if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".exe") {
			return fmt.Errorf("%w: player path for %q must be an absolute .exe path", ErrInvalidSettings, playerID)
		}
	}
	for _, playerID := range c.InstallRecommendations.Dismissed {
		if !playerIDPattern.MatchString(playerID) {
			return fmt.Errorf("%w: dismissed player id %q", ErrInvalidSettings, playerID)
		}
	}
	return nil
}
