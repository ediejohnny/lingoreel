package app

import (
	"encoding/base64"
	"errors"
	"math"
	"strings"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/presentation/ptbr"
)

// Sample lines of the preview, as the DESIGN shows them.
const (
	previewPrimary     = "This is a preview."
	previewTranslation = "Esta é uma prévia."
)

// baseDPI is the display scaling of 100%.
const baseDPI = 96

var (
	errNoPreview  = errors.New("A prévia não está disponível.")
	errBadPreview = errors.New("Não foi possível desenhar a prévia.")
	errNoFolder   = errors.New("Não foi possível abrir a pasta.")
)

// previewLanguages are the languages offered for the preferences, in order.
var previewLanguages = []media.LanguageTag{
	"en", "pt-BR", "pt-PT", "es", "fr", "de", "it", "ja", "ko", "zh", "ru", "nl", "pl", "tr", "ar",
}

// Settings is what the settings screen shows: the shared choices, the
// active profile and the players, with the screen's options and limits.
func (a *API) Settings() (SettingsView, error) {
	if _, err := a.context(); err != nil {
		return SettingsView{}, err
	}
	a.mu.Lock()
	config := a.config
	profile := a.profile()
	names := make([]string, len(a.profiles.Items))
	for i, item := range a.profiles.Items {
		names[i] = item.Name
	}
	a.mu.Unlock()
	view := SettingsView{
		General: GeneralSettings{
			Theme: string(config.Theme), SyncNotice: config.SyncNotice, ActiveProfile: profile.Name,
			RecommendationsHidden: config.InstallRecommendations.DismissAll || len(config.InstallRecommendations.Dismissed) > 0,
		},
		Profiles: names, Profile: profileView(profile, a.deps.KeyChar),
		Players: a.playerSettings(config), Options: a.settingsOptions(), Folder: a.deps.SettingsDir,
	}
	return view, nil
}

func (a *API) settingsOptions() SettingsOptions {
	options := SettingsOptions{
		Fonts: []string{}, Presets: []Option{}, Languages: []Option{}, Speeds: []float64{},
		Defaults: profileView(settings.DefaultProfile(), a.deps.KeyChar),
		Limits: SettingsLimits{
			FontSizeMin: appearance.MinFontSize, FontSizeMax: appearance.MaxFontSize,
			OutlineMax: appearance.MaxOutlineWidth, ShadowOffsetMax: appearance.MaxShadowOffset,
			ShadowBlurMax: appearance.MaxShadowBlur, PaddingMax: appearance.MaxPadding,
			LinesMin: appearance.MinLines, LinesMax: appearance.MaxLines, MarginMax: appearance.MaxMarginFraction,
			GapMin: study.MinCueGap.Seconds(), GapMax: study.MaxCueGap.Seconds(),
			LeadMin: study.MinCueLead.Seconds(), LeadMax: study.MaxCueLead.Seconds(), RepeatMax: study.MaxRepeats,
			SyncStepMin: int(subtitle.MinNudgeStep.Milliseconds()), SyncStepMax: int(subtitle.MaxNudgeStep.Milliseconds()),
			SyncFastStepMax: int(subtitle.MaxFastNudgeStep.Milliseconds()), ProfileNameMax: settings.MaxProfileNameRunes,
		},
	}
	if a.deps.Preview != nil {
		if fonts, err := a.deps.Preview.Fonts(); err == nil {
			options.Fonts = fonts
		} else {
			a.logger.Warn("the fonts could not be listed", "error", err)
		}
	}
	for _, preset := range appearance.Presets() {
		options.Presets = append(options.Presets, Option{Value: string(preset), Label: presetName(preset)})
	}
	for _, tag := range previewLanguages {
		options.Languages = append(options.Languages, Option{Value: string(tag), Label: capitalized(ptbr.LanguageName(tag))})
	}
	for _, speed := range playback.SpeedSteps() {
		options.Speeds = append(options.Speeds, float64(speed))
	}
	return options
}

func presetName(preset appearance.Preset) string {
	switch preset {
	case appearance.PresetClassic:
		return "Clássico"
	case appearance.PresetFilm:
		return "Filme"
	case appearance.PresetHighContrast:
		return "Alto contraste"
	case appearance.PresetMinimal:
		return "Minimalista"
	}
	return string(preset)
}

// SaveProfile keeps the screen's choices in the active profile and applies
// them at once to the video being watched.
func (a *API) SaveProfile(view ProfileSettings) error {
	a.mu.Lock()
	current := a.profile()
	if !strings.EqualFold(view.Name, current.Name) {
		a.mu.Unlock()
		return errors.New("O perfil mudou enquanto você editava. Abra as configurações de novo.")
	}
	profile, err := profileFrom(current, view, a.deps.KeyChar)
	if err != nil {
		a.mu.Unlock()
		return err
	}
	profile.Name = current.Name
	next, err := a.profiles.Replace(profile)
	if err == nil {
		err = a.commitProfiles(next)
	}
	a.mu.Unlock()
	if err != nil {
		return err
	}
	a.applyToSession(profile)
	return nil
}

// commitProfiles saves the profiles and, once saved, keeps them. Called with
// a.mu held.
func (a *API) commitProfiles(next settings.Profiles) error {
	if err := next.Validate(); err != nil {
		a.logger.Warn("refused profiles", "error", err)
		return errors.New("Não foi possível salvar o perfil.")
	}
	if a.deps.Settings != nil {
		if err := a.deps.Settings.SaveProfiles(next); err != nil {
			a.logger.Warn("profiles could not be saved", "error", err)
			return errors.New("Não foi possível salvar o perfil.")
		}
	}
	a.profiles = next
	return nil
}

// commitConfig saves the configuration and, once saved, keeps it. Called
// with a.mu held.
func (a *API) commitConfig(next settings.Config) error {
	if err := next.Validate(); err != nil {
		a.logger.Warn("refused settings", "error", err)
		return errors.New("Não foi possível salvar as configurações.")
	}
	if a.deps.Settings != nil {
		if err := a.deps.Settings.SaveConfig(next); err != nil {
			a.logger.Warn("settings could not be saved", "error", err)
			return errors.New("Não foi possível salvar as configurações.")
		}
	}
	a.config = next
	return nil
}

// applyToSession gives the video being watched a profile's choices: the
// look, the keys, the translation key, the sync steps and the Modo Karaokê.
// A choice the session refuses is logged; the profile is saved anyway.
func (a *API) applyToSession(profile settings.Profile) {
	current := a.current()
	if current == nil || current.session == nil {
		return
	}
	s := current.session
	karaoke := profile.Study.Karaoke
	err := errors.Join(
		s.Restyle(watch.LookOf(profile)),
		s.SetShortcuts(profile.Shortcuts),
		s.SetRevealBehavior(profile.Study.Reveal),
		s.SetSyncSteps(profile.Sync.Nudge()),
		s.ApplyKaraoke(session.Karaoke{Enabled: karaoke.Enabled, Rules: karaoke.Rules(), Fill: karaoke.Fill}),
		s.SetIndicator(karaoke.Indicator),
		s.SetLineRepeats(karaoke.Repeat),
	)
	if err != nil {
		a.logger.Warn("the video did not take every new choice", "error", err)
	}
	a.emit(EventPlayback, a.describe(current))
}

// SetSyncNotice shows or leaves out the notice of a sync change on the
// video, for every profile.
func (a *API) SetSyncNotice(show bool) error {
	a.mu.Lock()
	next := a.config
	next.SyncNotice = show
	err := a.commitConfig(next)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	if current := a.current(); current != nil && current.session != nil {
		current.session.SetSyncNotice(show)
	}
	return nil
}

// ApplyPreset returns a style with a preset's colors and effects; font,
// size and placement stay.
func (a *API) ApplyPreset(view StyleSettings, preset string) (StyleSettings, error) {
	style, err := styleFrom(view, "Legenda")
	if err != nil {
		return view, err
	}
	styled, err := appearance.Preset(preset).Apply(style)
	if err != nil {
		return view, errors.New("Estilo desconhecido.")
	}
	return styleView(styled), nil
}

// CheckShortcut checks a captured chord ("Ctrl+Comma") and names it.
func (a *API) CheckShortcut(text string) (ShortcutCheck, error) {
	chord, err := shortcut.Parse(text)
	if err != nil {
		return ShortcutCheck{}, errors.New(ptbr.ShortcutProblem(err, a.deps.KeyChar))
	}
	return ShortcutCheck{Chord: chord.String(), Label: ptbr.ShortcutLabel(chord, a.deps.KeyChar)}, nil
}

// PreviewSubtitles draws the two styles on a sample picture, as a PNG data
// address for an image on the screen.
func (a *API) PreviewSubtitles(request PreviewRequest) (string, error) {
	if a.deps.Preview == nil {
		return "", errNoPreview
	}
	primary, err := styleFrom(request.Primary, "Legenda de cima")
	if err != nil {
		return "", err
	}
	translation, err := styleFrom(request.Translation, "Legenda de baixo")
	if err != nil {
		return "", err
	}
	layout := appearance.Layout{
		MarginLeft: request.Layout.MarginLeft, MarginRight: request.Layout.MarginRight,
		MarginTop: request.Layout.MarginTop, MarginBottom: request.Layout.MarginBottom, MaxWidth: request.Layout.MaxWidth,
	}
	if err := layout.Validate(); err != nil {
		return "", refuse("Margens: %s.", fieldProblem(err))
	}
	scale := request.Scale
	if !(scale >= 0.5 && scale <= 5) {
		scale = 1
	}
	look := session.Look{Layout: layout, Primary: primary, Translation: translation}
	picture, err := a.deps.Preview.Preview(look, request.Width, request.Height, int(math.Round(baseDPI*scale)), previewPrimary, previewTranslation)
	if err != nil {
		a.logger.Warn("the preview was not drawn", "error", err)
		return "", errBadPreview
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(picture), nil
}

// OpenSettingsFolder shows the settings folder, with the log, in the file
// explorer.
func (a *API) OpenSettingsFolder() error {
	if a.deps.OpenFolder == nil || a.deps.SettingsDir == "" {
		return errNoFolder
	}
	if err := a.deps.OpenFolder(a.deps.SettingsDir); err != nil {
		a.logger.Warn("the settings folder did not open", "error", err)
		return errNoFolder
	}
	return nil
}
