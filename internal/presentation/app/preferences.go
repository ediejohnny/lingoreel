package app

import (
	"errors"
	"slices"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
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
	return a.changeConfig(func(next *settings.Config) error {
		next.Theme = settings.Theme(theme)
		// The menu over the video follows the theme, even when it is not kept.
		if a.deps.Stage != nil {
			a.deps.Stage.SetChrome(next.Bar, next.Edit, next.Theme)
		}
		return nil
	})
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
// for the Windows display language, and speaks it at once: the bar and the
// menu over the video, the name in Windows' "Abrir com" and what the screens
// were sent. It returns the language now spoken, for the screens.
func (a *API) SetLanguage(tag string) (string, error) {
	if _, speaks := ptbr.Languages()[tag]; tag != "" && !speaks {
		return "", errors.New(ptbr.T("error.unknownLanguage"))
	}
	if err := a.changeConfig(func(next *settings.Config) error {
		next.Language = tag
		return nil
	}); err != nil {
		return "", err
	}
	spoken, err := ptbr.UseLanguage(tag, a.systemLanguage())
	if err != nil {
		a.logger.Warn("the interface's catalog did not load", "language", tag, "error", err)
		return ptbr.Language(), nil // kept for the next opening; the screens stay as they are
	}
	a.logger.Info("interface language", "language", spoken)
	a.speakAgain()
	return spoken, nil
}

// speakAgain words again what was worded before the language changed: the
// association's names, the bar over the video and the line names of the
// session, the video sent to the screens and the session's state.
func (a *API) speakAgain() {
	if a.deps.Associations != nil {
		if on, err := a.deps.Associations.Enabled(); err == nil && on {
			if err := a.deps.Associations.Enable(); err != nil {
				a.logger.Warn("the association was not renamed", "error", err)
			}
		}
	}
	a.mu.Lock()
	if a.deps.Stage != nil {
		a.deps.Stage.SetChrome(a.config.Bar, a.config.Edit, a.config.Theme)
	}
	video, opened := a.video, a.project
	mode := a.profile().Study.Mode
	var suggested media.Selection
	if video != nil {
		suggested = video.Suggested
		if opened != nil && opened.video == video.Path {
			suggested, mode = opened.remembered(video.Info, suggested, mode)
		} else {
			opened = nil
		}
	}
	a.mu.Unlock()
	if current := a.current(); current != nil && current.session != nil && video != nil {
		a.renameLines(current, video.Info)
		a.emit(EventPlayback, a.describe(current))
	}
	if video != nil {
		a.emit(EventVideo, a.describeVideo(*video, opened, suggested, mode))
	}
}
