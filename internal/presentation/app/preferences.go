package app

import (
	"errors"
	"slices"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// Preferences are the settings the window needs at start.
func (a *API) Preferences() Preferences {
	a.mu.Lock()
	theme, panel, look := a.config.Theme, a.config.Panel, a.config.Interface
	a.mu.Unlock()
	steps := playback.SpeedSteps()
	speeds := make([]float64, len(steps))
	for i, step := range steps {
		speeds[i] = float64(step)
	}
	return Preferences{
		Theme: string(theme), Speeds: speeds, Panel: PanelState{Open: panel.Open, Width: panel.Width}, Language: ptbr.Language(),
		Interface: interfaceView(look),
	}
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
	// The menu over the video follows the theme, even when it is not kept.
	if a.deps.Stage != nil {
		a.deps.Stage.SetChrome(a.config.Bar, a.config.Edit, a.config.Theme)
	}
	return a.saveConfig()
}

// interfaceLanguages are the languages the app speaks, by tag, each named in
// itself.
func interfaceLanguages() []Option {
	names := ptbr.Languages()
	tags := make([]string, 0, len(names))
	for tag := range names {
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	options := make([]Option, len(tags))
	for i, tag := range tags {
		options[i] = Option{Value: tag, Label: names[tag]}
	}
	return options
}

// SetLanguage keeps the interface's language, a tag the app speaks or empty
// for the Windows display language. It applies the next time the app opens.
func (a *API) SetLanguage(tag string) error {
	if _, speaks := ptbr.Languages()[tag]; tag != "" && !speaks {
		return errors.New(ptbr.T("error.unknownLanguage"))
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.config
	next.Language = tag
	return a.commitConfig(next)
}
