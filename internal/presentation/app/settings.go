package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// minPreviewScale and maxPreviewScale bound the display scaling a preview is
// drawn at; outside them it is drawn at 100%.
const (
	minPreviewScale = 0.5
	maxPreviewScale = 5
)

// The preview's views besides the plain one. The karaoke mode's fill paints
// the line on top as half spoken, so the fill color shows next to the
// line's own; its cue shows that line before its time, halfway through the
// bar or three seconds before it; the notice shows a notice as a sync
// shortcut would.
const (
	previewViewFill   = "fill"
	previewViewCue    = "cue"
	previewViewNotice = "notice"
	previewSpoken     = 0.5
	previewCueLeft    = 0.5
	previewCueSeconds = 3
	// previewNoticeOffset is the sample notice's sync step.
	previewNoticeOffset = subtitle.Offset(-150 * time.Millisecond)
)

// previewCue shows the line on top before its time, as the karaoke mode
// does, with the indicator and the look the screen sent.
func previewCue(view KaraokeSettings, look *session.Look, screen *session.Screen) error {
	color, err := appearance.ParseColor(view.PreviewColor)
	if err != nil {
		return refuse(ptbr.T("error.karaoke.previewColor"))
	}
	indicator := study.Indicator(view.Indicator)
	if !indicator.Valid() {
		return refuse(ptbr.T("error.karaoke.indicator"))
	}
	if look.Cue, err = cueFrom(view); err != nil {
		return err
	}
	look.PreviewColor = color
	screen.Cue = &session.CueScreen{Indicator: indicator, Progress: previewCueLeft, Seconds: previewCueSeconds}
	return nil
}

var (
	errNoPreview  = worded(func() string { return ptbr.T("error.preview.unavailable") })
	errBadPreview = worded(func() string { return ptbr.T("error.preview.failed") })
	errNoFolder   = worded(func() string { return ptbr.T("error.folder") })
)

// previewLanguages are the languages offered for the preferences, in order.
var previewLanguages = []media.LanguageTag{
	"en", "pt-BR", "pt-PT", "es", "fr", "de", "it", "ja", "ko", "zh", "ru", "nl", "pl", "tr", "ar",
}

// Settings is what the settings screen shows: the shared choices, the
// active profile and the players, with the screen's options and limits.
func (a *API) Settings() (SettingsView, error) {
	ctx, err := a.context()
	if err != nil {
		return SettingsView{}, err
	}
	a.mu.Lock()
	config := a.config
	profile := a.profile()
	names := make([]string, len(a.profiles.Items))
	for i, item := range a.profiles.Items {
		names[i] = item.Name
	}
	notes := make([]string, len(a.settingsNotes))
	for i, note := range a.settingsNotes {
		notes[i] = note()
	}
	a.mu.Unlock()
	view := SettingsView{
		General: GeneralSettings{
			Theme: string(config.Theme), SyncNotice: config.SyncNotice, ActiveProfile: profile.Name, AlwaysOnTop: config.AlwaysOnTop,
			RecommendationsHidden: config.InstallRecommendations.DismissAll || len(config.InstallRecommendations.Dismissed) > 0,
			Language:              config.Language, Languages: interfaceLanguages(),
		},
		Profiles: names, Profile: profileView(profile, a.deps.KeyChar), Appearance: appearanceView(config.Bar, config.Edit),
		Players: a.playerSettings(config), Voice: a.voiceView(ctx, config.Voice), Options: a.settingsOptions(),
		Thumbnails: a.thumbnailsView(config.Thumbnails), Folder: a.deps.SettingsDir, Notes: notes,
	}
	return view, nil
}

func (a *API) settingsOptions() SettingsOptions {
	options := SettingsOptions{
		Fonts: []string{}, Presets: []Option{}, Languages: []Option{}, Speeds: []float64{}, Modes: modeOptions(),
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
			PrecisionStepMin: int(settings.MinPrecisionSeekStep.Milliseconds()),
			PrecisionStepMax: int(settings.MaxPrecisionSeekStep.Milliseconds()),
			LineSpacingMin:   appearance.MinLineSpacing, LineSpacingMax: appearance.MaxLineSpacing,
			LineGapMax: appearance.MaxLineGap, NoticeSizeMax: appearance.MaxNoticeFontSize,
			NoticeMarginMax: appearance.MaxNoticeMargin, NoticeWidthMin: appearance.MinNoticeMaxWidth,
			NoticeDurationMin: settings.MinNoticeDuration.Seconds(), NoticeDurationMax: settings.MaxNoticeDuration.Seconds(),
			SelectionWidthMin: appearance.MinSelectionWidth, SelectionWidthMax: appearance.MaxSelectionWidth,
			SelectionGapMax: appearance.MaxSelectionGap, SelectionRadiusMax: appearance.MaxSelectionCurve,
			BarThicknessMin: appearance.MinCueThickness, BarThicknessMax: appearance.MaxCueThickness,
			CountdownSizeMin: appearance.MinCountdownSize, CountdownSizeMax: appearance.MaxCountdownSize,
			BarSizeMin: appearance.MinBarSize, BarSizeMax: appearance.MaxBarSize,
			HideDelayMin: settings.MinBarHideDelay.Seconds(), HideDelayMax: settings.MaxBarHideDelay.Seconds(),
			HintSizeMin: appearance.MinEditHintSize, HintSizeMax: appearance.MaxEditHintSize,
			TextSizeMin: appearance.MinTextSize, TextSizeMax: appearance.MaxTextSize,
			VolumeStepMin: playback.MinVolumeTouch.Percent(), VolumeStepMax: playback.MaxVolumeTouch.Percent(),
		},
		AppearanceDefaults: appearanceView(settings.DefaultBar(), appearance.DefaultEditStyle()),
		EditHint:           ptbr.EditHint(),
		InterfaceDefaults:  interfaceView(appearance.DefaultInterfaceStyle()),
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
		options.Languages = append(options.Languages, Option{Value: string(tag), Label: ptbr.Capitalized(ptbr.LanguageName(tag))})
	}
	for _, speed := range playback.SpeedSteps() {
		options.Speeds = append(options.Speeds, float64(speed))
	}
	return options
}

// modeOptions are the study modes, in the order the screens offer them.
func modeOptions() []Option {
	options := []Option{}
	for _, mode := range study.Modes() {
		options = append(options, Option{Value: string(mode), Label: ptbr.ModeName(mode)})
	}
	return options
}

func presetName(preset appearance.Preset) string {
	switch preset {
	case appearance.PresetClassic:
		return ptbr.T("preset.classic")
	case appearance.PresetFilm:
		return ptbr.T("preset.film")
	case appearance.PresetHighContrast:
		return ptbr.T("preset.highContrast")
	case appearance.PresetMinimal:
		return ptbr.T("preset.minimal")
	}
	return string(preset)
}

// SaveProfile keeps the screen's choices in the active profile and applies
// them at once to the video being watched.
func (a *API) SaveProfile(view ProfileSettings) error {
	var current, profile settings.Profile
	err := a.changeProfiles(func(next *settings.Profiles) error {
		current = a.profile()
		if !strings.EqualFold(view.Name, current.Name) {
			return errors.New(ptbr.T("error.profile.changed"))
		}
		var err error
		if profile, err = profileFrom(current, view, a.deps.KeyChar); err != nil {
			return err
		}
		profile.Name = current.Name
		*next, err = next.Replace(profile)
		return err
	})
	if err != nil {
		return err
	}
	a.applyToSession(current, profile)
	return nil
}

// PreviewLook shows the screen's choices on the video being watched without
// keeping them: while the settings are edited, the video is the preview.
// Choices the profile would refuse are refused here too, and the video keeps
// its look. Without a video it does nothing.
func (a *API) PreviewLook(view ProfileSettings) error {
	a.mu.Lock()
	current := a.profile()
	a.mu.Unlock()
	profile, err := profileFrom(current, view, a.deps.KeyChar)
	if err != nil {
		return err
	}
	profile.Name = current.Name
	if err := profile.Validate(); err != nil {
		a.logger.Warn("refused preview", "error", err)
		return errors.New(ptbr.T("error.profile.notSaved"))
	}
	a.restyle(profile)
	return nil
}

// EndPreview puts the saved profile's look back on the video, once the
// screen's choices were left unsaved.
func (a *API) EndPreview() {
	a.mu.Lock()
	profile := a.profile()
	a.mu.Unlock()
	a.restyle(profile)
}

// restyle gives the video being watched a profile's look; a refusal is only
// logged.
func (a *API) restyle(profile settings.Profile) {
	if current := a.current(); current != nil && current.session != nil {
		if err := current.session.Restyle(watch.LookOf(profile)); err != nil {
			a.logger.Warn("the video did not take the look", "error", err)
		}
	}
}

// What a settings change writes.
const (
	writeConfig = 1 << iota
	writeProfiles
)

// changeConfig changes the configuration: change edits a copy of it, with
// a.mu held, and the copy is written without a.mu, then kept. Called without
// a.mu.
func (a *API) changeConfig(change func(next *settings.Config) error) error {
	return a.changeSettings(writeConfig, func(config *settings.Config, _ *settings.Profiles) error { return change(config) })
}

// changeProfiles changes the profiles as changeConfig does the
// configuration. Called without a.mu.
func (a *API) changeProfiles(change func(next *settings.Profiles) error) error {
	return a.changeSettings(writeProfiles, func(_ *settings.Config, profiles *settings.Profiles) error { return change(profiles) })
}

// changeSettings changes the configuration and the profiles: change edits
// copies of both, with a.mu held, and what writes names is written without
// a.mu, the profiles first, one settings change at a time; the copies are
// kept once written, so a slow disk never holds the screens or the keys.
// When the configuration cannot be written after the profiles were, the
// profiles are put back, so the configuration never names a profile the
// file lacks. Called without a.mu.
func (a *API) changeSettings(writes int, change func(config *settings.Config, profiles *settings.Profiles) error) error {
	a.settingsWrites.Lock()
	defer a.settingsWrites.Unlock()
	a.mu.Lock()
	before, beforeProfiles := a.config, a.profiles
	config, profiles := a.config, settings.Profiles{Items: slices.Clone(a.profiles.Items)}
	err := change(&config, &profiles)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	// Both are checked before either is written, so a refused configuration
	// never leaves the profiles written alone.
	if writes&writeConfig != 0 {
		if err := config.Validate(); err != nil {
			a.logger.Warn("refused settings", "error", err)
			return errSettingsNotSaved
		}
	}
	if writes&writeProfiles != 0 {
		if err := profiles.Validate(); err != nil {
			a.logger.Warn("refused profiles", "error", err)
			return errProfilesNotSaved
		}
	}
	if writes&writeProfiles != 0 {
		if err := a.writeProfiles(profiles); err != nil {
			return err
		}
	}
	if writes&writeConfig != 0 {
		if err := a.writeConfig(config); err != nil {
			if writes&writeProfiles != 0 {
				if undo := a.writeProfiles(beforeProfiles); undo != nil {
					a.logger.Warn("the profiles could not be put back", "error", undo)
				}
			}
			return err
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if writes&writeConfig == 0 {
		config = before
	}
	if writes&writeProfiles == 0 {
		profiles = beforeProfiles
	}
	a.config, a.profiles = config, profiles
	return nil
}

// writeProfiles checks and writes the profiles; a file kept as it is takes
// them in memory only, until the app closes, and the settings screen says
// why. Called with a.settingsWrites held, without a.mu.
func (a *API) writeProfiles(next settings.Profiles) error {
	if err := next.Validate(); err != nil {
		a.logger.Warn("refused profiles", "error", err)
		return errProfilesNotSaved
	}
	if a.deps.Settings == nil {
		return nil
	}
	if err := a.deps.Settings.SaveProfiles(next); err != nil && !errors.Is(err, settings.ErrReadOnly) {
		a.logger.Warn("profiles could not be saved", "error", err)
		return a.because(errProfilesNotSaved, err)
	}
	return nil
}

// writeConfig checks and writes the configuration, as writeProfiles does
// the profiles. Called with a.settingsWrites held, without a.mu.
func (a *API) writeConfig(next settings.Config) error {
	if err := next.Validate(); err != nil {
		a.logger.Warn("refused settings", "error", err)
		return errSettingsNotSaved
	}
	if a.deps.Settings == nil {
		return nil
	}
	if err := a.deps.Settings.SaveConfig(next); err != nil && !errors.Is(err, settings.ErrReadOnly) {
		a.logger.Warn("settings could not be saved", "error", err)
		return a.because(errSettingsNotSaved, err)
	}
	return nil
}

// applyToSession gives the video being watched a profile's new choices:
// the look, the keys, the translation key, the steps and the karaoke mode;
// the study mode only when it changed from previous, so a mode picked on the
// panel for this video stays. A choice the session refuses is logged; the
// profile is saved anyway.
func (a *API) applyToSession(previous, profile settings.Profile) {
	current := a.current()
	if current == nil || current.session == nil {
		return
	}
	s := current.session
	karaoke, before := profile.Study.Karaoke, previous.Study.Karaoke
	err := errors.Join(
		s.Restyle(watch.LookOf(profile)),
		s.SetShortcuts(profile.Shortcuts),
		s.SetRevealBehavior(profile.Study.Reveal),
		s.SetSyncSteps(profile.Sync.Nudge()),
		s.SetPrecisionStep(profile.Sync.PrecisionSeekStep.Std()),
	)
	// The karaoke mode of the video may come from its project or the bar:
	// only the parts that changed in the profile replace it.
	if karaoke.Enabled != before.Enabled || karaoke.MinGap != before.MinGap || karaoke.Lead != before.Lead || karaoke.Fill != before.Fill {
		err = errors.Join(err, s.ApplyKaraoke(session.Karaoke{Enabled: karaoke.Enabled, Rules: karaoke.Rules(), Fill: karaoke.Fill}))
	}
	if karaoke.Indicator != before.Indicator {
		err = errors.Join(err, s.SetIndicator(karaoke.Indicator))
	}
	if karaoke.Repeat != before.Repeat {
		err = errors.Join(err, s.SetLineRepeats(karaoke.Repeat))
	}
	if profile.Study.Mode != previous.Study.Mode {
		err = errors.Join(err, s.SetMode(profile.Study.Mode))
	}
	s.SetTranslationTiming(profile.Sync.TranslationOwnTiming)
	s.SetVolumeTouch(profile.VolumeTouch)
	if err != nil {
		a.logger.Warn("the video did not take every new choice", "error", err)
	}
	a.emit(EventPlayback, a.describe(current))
}

// SetSyncNotice shows or leaves out the notice of a sync change on the
// video, for every profile.
func (a *API) SetSyncNotice(show bool) error {
	if err := a.changeConfig(func(next *settings.Config) error {
		next.SyncNotice = show
		return nil
	}); err != nil {
		return err
	}
	if current := a.current(); current != nil && current.session != nil {
		current.session.SetSyncNotice(show)
	}
	return nil
}

// SetAlwaysOnTop keeps the app's window in front of the other programs', or
// lets it go behind them, at once and from then on.
func (a *API) SetAlwaysOnTop(on bool) error {
	if err := a.changeConfig(func(next *settings.Config) error {
		next.AlwaysOnTop = on
		return nil
	}); err != nil {
		return err
	}
	a.keepOnTop()
	return nil
}

// keepOnTop gives the window the place the settings ask for, unless it is
// gone.
func (a *API) keepOnTop() {
	a.mu.Lock()
	ui, gone, on := a.ui, a.shutDown, a.config.AlwaysOnTop
	a.mu.Unlock()
	if ui != nil && !gone {
		ui.SetAlwaysOnTop(on)
	}
}

// ApplyPreset returns a style with a preset's colors and effects; font,
// size and placement stay.
func (a *API) ApplyPreset(view StyleSettings, preset string) (StyleSettings, error) {
	style, err := styleFrom(view, ptbr.T("line.any"))
	if err != nil {
		return view, err
	}
	styled, err := appearance.Preset(preset).Apply(style)
	if err != nil {
		return view, errors.New(ptbr.T("error.unknownPreset"))
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

// CheckFill tells which color the karaoke mode's fill is painted in over a
// line of color text: a fill the line would hide gives way to one that shows.
func (a *API) CheckFill(text, fill string) (FillCheck, error) {
	textColor, err := appearance.ParseColor(text)
	if err != nil {
		return FillCheck{}, refuse(ptbr.T("error.settings.color", "line", ptbr.T("line.primary"), "color", fmt.Sprintf("%q", text)))
	}
	fillColor, err := appearance.ParseColor(fill)
	if err != nil {
		return FillCheck{}, refuse(ptbr.T("error.karaoke.fillColor"))
	}
	used, changed := appearance.ReadableFill(textColor, fillColor)
	return FillCheck{Used: used.String(), Changed: changed}, nil
}

// PreviewSubtitles draws the two styles on a sample picture, as a PNG data
// address for an image on the screen.
func (a *API) PreviewSubtitles(request PreviewRequest) (string, error) {
	if a.deps.Preview == nil {
		return "", errNoPreview
	}
	primary, err := styleFrom(request.Primary, ptbr.T("line.primary"))
	if err != nil {
		return "", err
	}
	translation, err := styleFrom(request.Translation, ptbr.T("line.translation"))
	if err != nil {
		return "", err
	}
	margins, err := layoutFrom(request.Layout)
	if err != nil {
		return "", err
	}
	// Without a translation line, the one below speaks the interface's language.
	below := media.LanguageTag(request.Languages.Translation)
	if below == "" {
		below = media.LanguageTag(ptbr.Language())
	}
	screen := session.Screen{
		Primary: media.SamplePhrase(media.LanguageTag(request.Languages.Primary)), Translation: media.SamplePhrase(below),
		TranslationVisible: true,
	}
	look := session.Look{Layout: margins, Primary: primary, Translation: translation}
	if request.Highlight != "" {
		line, err := lineOf(request.Highlight)
		if err != nil {
			return "", err
		}
		if look.Selection, err = selectionFrom(request.Marks.Selection); err != nil {
			return "", err
		}
		screen.Highlighted, screen.Highlight = true, line
	}
	notice := ""
	switch request.View {
	case "":
	case previewViewFill:
		fill, err := appearance.ParseColor(request.Karaoke.FillColor)
		if err != nil {
			return "", refuse(ptbr.T("error.karaoke.fillColor"))
		}
		// The fill as the video paints it over this line.
		look.FillColor, _ = appearance.ReadableFill(primary.Color, fill)
		screen.Fill = previewSpoken
	case previewViewCue:
		if err := previewCue(request.Karaoke, &look, &screen); err != nil {
			return "", err
		}
	case previewViewNotice:
		if look.Notice, err = noticeFrom(request.Marks.Notice); err != nil {
			return "", err
		}
		names := ptbr.LineNames{ptbr.T("line.primary"), ptbr.T("line.translation")}
		notice = names.Notice(session.LinePrimary, previewNoticeOffset)
	default:
		return "", errors.New(ptbr.T("error.preview.unknown"))
	}
	scale := request.Scale
	if !(scale >= minPreviewScale && scale <= maxPreviewScale) {
		scale = 1
	}
	picture, err := a.deps.Preview.Preview(look, request.Width, request.Height, int(math.Round(layout.BaseDPI*scale)), screen, notice)
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
