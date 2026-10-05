package app

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// profileView turns a profile into what the settings screen shows.
func profileView(profile settings.Profile, char ptbr.KeyChar) ProfileSettings {
	karaoke := profile.Study.Karaoke
	view := ProfileSettings{
		Name: profile.Name, PreferredPlayer: profile.PreferredPlayer,
		Languages: LanguageSettings{
			Audio: profile.Languages.Audio, Primary: profile.Languages.Primary, Translation: profile.Languages.Translation,
		},
		Primary: styleView(profile.Primary), Translation: styleView(profile.Translation),
		Layout: LayoutSettings{
			MarginLeft: profile.Layout.MarginLeft, MarginRight: profile.Layout.MarginRight,
			MarginTop: profile.Layout.MarginTop, MarginBottom: profile.Layout.MarginBottom, MaxWidth: profile.Layout.MaxWidth,
		},
		Speed:         float64(profile.Speed),
		SyncStep:      int(profile.Sync.OffsetStep.Std() / time.Millisecond),
		SyncFastStep:  int(profile.Sync.OffsetLargeStep.Std() / time.Millisecond),
		PrecisionStep: int(profile.Sync.PrecisionSeekStep.Std() / time.Millisecond),
		Mode:          string(profile.Study.Mode),
		Karaoke: KaraokeSettings{
			Enabled: karaoke.Enabled, MinGap: karaoke.MinGap.Std().Seconds(), Lead: karaoke.Lead.Std().Seconds(),
			Indicator: string(karaoke.Indicator), PreviewColor: karaoke.PreviewColor.String(),
			Fill: karaoke.Fill, FillColor: karaoke.FillColor.String(), Repeat: karaoke.Repeat,
		},
		Reveal:    string(profile.Study.Reveal),
		Shortcuts: shortcutViews(profile.Shortcuts.Resolved(), char),
	}
	return view
}

func styleView(style appearance.SubtitleStyle) StyleSettings {
	return StyleSettings{
		FontFamily: style.FontFamily, FontSize: style.FontSize, Responsive: style.SizeMode == appearance.SizeResponsive,
		Bold: style.Bold, Italic: style.Italic, Color: style.Color.String(), Opacity: style.Opacity,
		Outline: style.Outline.Enabled, OutlineColor: style.Outline.Color.String(), OutlineWidth: style.Outline.Width,
		Shadow: style.Shadow.Enabled, ShadowColor: style.Shadow.Color.String(),
		ShadowX: style.Shadow.OffsetX, ShadowY: style.Shadow.OffsetY, ShadowBlur: style.Shadow.Blur,
		Background: style.Background.Enabled, BackgroundColor: style.Background.Color.String(),
		BackgroundMargin: style.Background.Padding,
		Position:         style.VerticalPosition, Alignment: string(style.Alignment), MaxLines: style.MaxLines,
	}
}

// settingsError is a choice the screen sent that the profile refuses, worded
// for the user.
type settingsError struct{ message string }

func (e settingsError) Error() string { return e.message }

func refuse(message string) error {
	return settingsError{message: message}
}

// profileFrom puts the screen's choices on a profile, checked; what the
// screen does not show stays as base has it.
func profileFrom(base settings.Profile, view ProfileSettings, char ptbr.KeyChar) (settings.Profile, error) {
	primary, err := styleFrom(view.Primary, ptbr.T("line.primary"))
	if err != nil {
		return settings.Profile{}, err
	}
	translation, err := styleFrom(view.Translation, ptbr.T("line.translation"))
	if err != nil {
		return settings.Profile{}, err
	}
	layout := appearance.Layout{
		MarginLeft: view.Layout.MarginLeft, MarginRight: view.Layout.MarginRight,
		MarginTop: view.Layout.MarginTop, MarginBottom: view.Layout.MarginBottom, MaxWidth: view.Layout.MaxWidth,
	}
	if err := layout.Validate(); err != nil {
		return settings.Profile{}, refuse(ptbr.T("error.settings.margins", "problem", fieldProblem(err)))
	}
	karaoke, err := karaokeFrom(view.Karaoke)
	if err != nil {
		return settings.Profile{}, err
	}
	keys, err := shortcutsFrom(view.Shortcuts, char)
	if err != nil {
		return settings.Profile{}, err
	}
	profile := base
	profile.Name, profile.PreferredPlayer = view.Name, view.PreferredPlayer
	profile.Languages = settings.LanguagePreferences{
		Audio: view.Languages.Audio, Primary: view.Languages.Primary, Translation: view.Languages.Translation,
	}
	profile.Primary, profile.Translation, profile.Layout = primary, translation, layout
	profile.Study.Mode = study.Mode(view.Mode)
	profile.Study.Reveal, profile.Study.Karaoke = study.RevealBehavior(view.Reveal), karaoke
	profile.Speed = playback.Speed(view.Speed)
	profile.Sync.OffsetStep = settings.Duration(time.Duration(view.SyncStep) * time.Millisecond)
	profile.Sync.OffsetLargeStep = settings.Duration(time.Duration(view.SyncFastStep) * time.Millisecond)
	profile.Sync.PrecisionSeekStep = settings.Duration(time.Duration(view.PrecisionStep) * time.Millisecond)
	profile.Shortcuts = keys
	switch {
	case !profile.Study.Mode.Valid():
		return settings.Profile{}, refuse(ptbr.T("error.unknownMode"))
	case !profile.Speed.Valid():
		return settings.Profile{}, refuse(ptbr.T("error.settings.speed"))
	case view.PrecisionStep < int(settings.MinPrecisionSeekStep.Milliseconds()) || view.PrecisionStep > int(settings.MaxPrecisionSeekStep.Milliseconds()):
		return settings.Profile{}, refuse(ptbr.T("error.settings.precisionStep",
			"min", settings.MinPrecisionSeekStep.Milliseconds(), "max", settings.MaxPrecisionSeekStep.Milliseconds()))
	case profile.Sync.Nudge().Validate() != nil:
		return settings.Profile{}, refuse(ptbr.T("error.settings.syncSteps", "min", subtitle.MinNudgeStep.Milliseconds(),
			"max", subtitle.MaxNudgeStep.Milliseconds(), "fastMax", subtitle.MaxFastNudgeStep.Milliseconds()))
	case !profile.Study.Reveal.Valid():
		return settings.Profile{}, refuse(ptbr.T("error.settings.reveal"))
	}
	if err := profile.Validate(); err != nil {
		return settings.Profile{}, refuse(ptbr.T("error.settings.profile"))
	}
	return profile, nil
}

func styleFrom(view StyleSettings, line string) (appearance.SubtitleStyle, error) {
	style := appearance.SubtitleStyle{
		FontFamily: view.FontFamily, FontSize: view.FontSize, SizeMode: appearance.SizeFixed,
		Bold: view.Bold, Italic: view.Italic, Opacity: view.Opacity,
		Outline:          appearance.Outline{Enabled: view.Outline, Width: view.OutlineWidth},
		Shadow:           appearance.Shadow{Enabled: view.Shadow, OffsetX: view.ShadowX, OffsetY: view.ShadowY, Blur: view.ShadowBlur},
		Background:       appearance.Background{Enabled: view.Background, Padding: view.BackgroundMargin},
		VerticalPosition: view.Position, Alignment: appearance.Alignment(view.Alignment), MaxLines: view.MaxLines,
	}
	if view.Responsive {
		style.SizeMode = appearance.SizeResponsive
	}
	for _, color := range []struct {
		text   string
		target *appearance.Color
	}{
		{view.Color, &style.Color}, {view.OutlineColor, &style.Outline.Color},
		{view.ShadowColor, &style.Shadow.Color}, {view.BackgroundColor, &style.Background.Color},
	} {
		parsed, err := appearance.ParseColor(color.text)
		if err != nil {
			return style, refuse(ptbr.T("error.settings.color", "line", line, "color", fmt.Sprintf("%q", color.text)))
		}
		*color.target = parsed
	}
	if err := style.Validate(); err != nil {
		return style, refuse(ptbr.T("error.settings.style", "line", line, "problem", fieldProblem(err)))
	}
	return style, nil
}

// fieldProblem words which value of a style or layout is out of range.
func fieldProblem(err error) string {
	var field *appearance.FieldError
	if !errors.As(err, &field) {
		return ptbr.T("settings.problem.outOfRange")
	}
	limit := func(value float64) string { return ptbr.Number(value, -1) }
	switch field.Field {
	case "fontFamily":
		return ptbr.T("settings.problem.fontFamily")
	case "fontSize":
		return ptbr.T("settings.problem.fontSize", "min", limit(appearance.MinFontSize), "max", limit(appearance.MaxFontSize))
	case "sizeMode":
		return ptbr.T("settings.problem.sizeMode")
	case "opacity":
		return ptbr.T("settings.problem.opacity")
	case "outline.width":
		return ptbr.T("settings.problem.outlineWidth", "max", limit(appearance.MaxOutlineWidth))
	case "shadow.offsetX", "shadow.offsetY":
		return ptbr.T("settings.problem.shadowOffset", "max", limit(appearance.MaxShadowOffset))
	case "shadow.blur":
		return ptbr.T("settings.problem.shadowBlur", "max", limit(appearance.MaxShadowBlur))
	case "background.padding":
		return ptbr.T("settings.problem.padding", "max", limit(appearance.MaxPadding))
	case "verticalPosition":
		return ptbr.T("settings.problem.position")
	case "alignment":
		return ptbr.T("settings.problem.alignment")
	case "maxLines":
		return ptbr.T("settings.problem.maxLines", "min", appearance.MinLines, "max", appearance.MaxLines)
	case "marginLeft", "marginRight", "marginTop", "marginBottom":
		return ptbr.T("settings.problem.margin", "max", limit(appearance.MaxMarginFraction*100))
	case "maxWidth":
		return ptbr.T("settings.problem.maxWidth")
	}
	return ptbr.T("settings.problem.outOfRange")
}

func karaokeFrom(view KaraokeSettings) (settings.KaraokePreferences, error) {
	preview, err := appearance.ParseColor(view.PreviewColor)
	if err != nil {
		return settings.KaraokePreferences{}, refuse(ptbr.T("error.karaoke.previewColor"))
	}
	fill, err := appearance.ParseColor(view.FillColor)
	if err != nil {
		return settings.KaraokePreferences{}, refuse(ptbr.T("error.karaoke.fillColor"))
	}
	karaoke := settings.KaraokePreferences{
		Enabled: view.Enabled, MinGap: seconds(view.MinGap), Lead: seconds(view.Lead),
		Indicator: study.Indicator(view.Indicator), PreviewColor: preview, Fill: view.Fill, FillColor: fill, Repeat: view.Repeat,
	}
	switch {
	case karaoke.Rules().Validate() != nil:
		return karaoke, refuse(ptbr.T("error.karaoke.timing",
			"gapMin", ptbr.Number(study.MinCueGap.Seconds(), -1), "gapMax", ptbr.Number(study.MaxCueGap.Seconds(), -1),
			"leadMin", ptbr.Number(study.MinCueLead.Seconds(), -1), "leadMax", ptbr.Number(study.MaxCueLead.Seconds(), -1)))
	case !karaoke.Indicator.Valid():
		return karaoke, refuse(ptbr.T("error.karaoke.indicator"))
	case karaoke.Repeat < 0 || karaoke.Repeat > study.MaxRepeats:
		return karaoke, refuse(ptbr.T("error.karaoke.repeat", "max", study.MaxRepeats))
	}
	return karaoke, nil
}

// seconds converts the screen's seconds, NaN and infinities becoming zero
// (which every range then refuses).
func seconds(value float64) settings.Duration {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > maxSeekSeconds {
		return 0
	}
	return settings.Duration(time.Duration(value * float64(time.Second)))
}

func shortcutsFrom(views []ShortcutSetting, char ptbr.KeyChar) (shortcut.Map, error) {
	keys := shortcut.Map{}
	for _, view := range views {
		command := shortcut.Command(view.Command)
		if !command.Known() {
			return nil, refuse(ptbr.T("error.shortcut.unknownCommand"))
		}
		if strings.TrimSpace(view.Chord) == "" {
			keys[command] = shortcut.Chord{} // no shortcut
			continue
		}
		chord, err := shortcut.Parse(view.Chord)
		if err != nil {
			return nil, refuse(ptbr.T("error.shortcut.problem",
				"command", ptbr.CommandName(command), "problem", ptbr.ShortcutProblem(err, char)))
		}
		keys[command] = chord
	}
	if err := keys.Validate(); err != nil {
		return nil, refuse(ptbr.ShortcutProblem(err, char))
	}
	return keys.Resolved(), nil
}
