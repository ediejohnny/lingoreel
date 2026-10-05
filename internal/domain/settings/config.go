// Package settings models the user's persisted preferences: the global
// configuration and the study profiles.
package settings

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
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
	// SyncNotice shows on the video which subtitle a sync change moved.
	SyncNotice bool `json:"syncNotice"`
	// Window is where the app's window was when it closed.
	Window WindowBounds `json:"window"`
	// Panel is how the panel beside the video was left.
	Panel PanelBounds `json:"panel"`
	// Language is the interface's language, a BCP 47 tag such as "pt-BR";
	// empty means the Windows display language when the app speaks it.
	Language string `json:"language"`
}

// PanelBounds is the panel beside the video: open or closed, and its width
// in pixels at 100% scaling; a zero width means the default one.
type PanelBounds struct {
	Open  bool    `json:"open"`
	Width float64 `json:"width"`
}

// The panel's width, in pixels at 100% scaling.
const (
	PanelMinWidth = 280
	PanelMaxWidth = 1600
)

// Valid reports whether the width is the default (zero) or a panel's.
func (p PanelBounds) Valid() bool {
	return p.Width == 0 || (p.Width >= PanelMinWidth && p.Width <= PanelMaxWidth)
}

// WindowBounds are the app window's position and size in screen pixels;
// a zero size means none was saved yet.
type WindowBounds struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximized bool `json:"maximized"`
}

// maxWindowSize bounds a saved window side, beyond any real screen.
const maxWindowSize = 1 << 15

// Saved reports whether the bounds hold a window.
func (w WindowBounds) Saved() bool {
	return w.Width > 0 && w.Height > 0
}

// DefaultConfig returns the configuration used on the first run.
func DefaultConfig() Config {
	return Config{
		Theme:         ThemeSystem,
		ActiveProfile: DefaultProfileName,
		PlayerPaths:   map[string]string{},
		SyncNotice:    true,
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
	w := c.Window
	if w.Width < 0 || w.Height < 0 || w.Width > maxWindowSize || w.Height > maxWindowSize ||
		w.X < -maxWindowSize || w.X > maxWindowSize || w.Y < -maxWindowSize || w.Y > maxWindowSize {
		return fmt.Errorf("%w: window %+v", ErrInvalidSettings, w)
	}
	if !c.Panel.Valid() {
		return fmt.Errorf("%w: panel width %v", ErrInvalidSettings, c.Panel.Width)
	}
	if c.Language != "" && !languageTagPattern.MatchString(c.Language) {
		return fmt.Errorf("%w: language %q", ErrInvalidSettings, c.Language)
	}
	return nil
}
