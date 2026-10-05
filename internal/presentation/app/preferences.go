package app

import (
	"errors"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// Preferences are the settings the window needs at start.
func (a *API) Preferences() Preferences {
	a.mu.Lock()
	theme, panel := a.config.Theme, a.config.Panel
	a.mu.Unlock()
	steps := playback.SpeedSteps()
	speeds := make([]float64, len(steps))
	for i, step := range steps {
		speeds[i] = float64(step)
	}
	return Preferences{Theme: string(theme), Speeds: speeds, Panel: PanelState{Open: panel.Open, Width: panel.Width}}
}

// SetTheme changes the color theme: "system", "light" or "dark".
func (a *API) SetTheme(theme string) error {
	switch settings.Theme(theme) {
	case settings.ThemeSystem, settings.ThemeLight, settings.ThemeDark:
	default:
		return errors.New(ptbr.T("error.unknownTheme"))
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.Theme = settings.Theme(theme)
	return a.saveConfig()
}
