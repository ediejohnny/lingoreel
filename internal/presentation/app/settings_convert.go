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
			LineGap: profile.Layout.LineGap,
		},
		Marks:         marksView(profile.Marks),
		Speed:         float64(profile.Speed),
		SyncStep:      int(profile.Sync.OffsetStep.Std() / time.Millisecond),
		SyncFastStep:  int(profile.Sync.OffsetLargeStep.Std() / time.Millisecond),
		PrecisionStep: int(profile.Sync.PrecisionSeekStep.Std() / time.Millisecond),
		Mode:          string(profile.Study.Mode),
		Karaoke: KaraokeSettings{
			Enabled: karaoke.Enabled, MinGap: karaoke.MinGap.Std().Seconds(), Lead: karaoke.Lead.Std().Seconds(),
			Indicator: string(karaoke.Indicator), PreviewColor: karaoke.PreviewColor.String(),
			Fill: karaoke.Fill, FillColor: karaoke.FillColor.String(), Repeat: karaoke.Repeat,
			BarThickness: karaoke.Cue.BarThickness, BarColor: karaoke.Cue.BarColor.String(),
			CountdownSize: karaoke.Cue.CountdownSize,
		},
		Reveal:    string(profile.Study.Reveal),
		Shortcuts: shortcutViews(profile.Shortcuts.Resolved(), char),
	}
	return view
}

func marksView(marks settings.MarkPreferences) MarkSettings {
	notice, selection := marks.Notice, marks.Selection
	return MarkSettings{
		Notice: NoticeSettings{
			FontFamily: notice.FontFamily, FontSize: notice.FontSize, Color: notice.Color.String(),
			Background: notice.Background.String(), Side: string(notice.Side), Margin: notice.Margin, MaxWidth: notice.MaxWidth,
		},
		NoticeDuration: marks.NoticeDuration.Std().Seconds(),
		Selection: SelectionSettings{
			Color: selection.Color.String(), Edge: selection.Edge.String(),
			Width: selection.Width, Gap: selection.Gap, Radius: selection.Radius,
		},
	}
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
		LineSpacing: style.LineSpacing,
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
	layout, err := layoutFrom(view.Layout)
	if err != nil {
		return settings.Profile{}, err
	}
	marks, err := marksFrom(view.Marks)
	if err != nil {
		return settings.Profile{}, err
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
	profile.Primary, profile.Translation, profile.Layout, profile.Marks = primary, translation, layout, marks
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
		LineSpacing: view.LineSpacing,
	}
	if view.Responsive {
		style.SizeMode = appearance.SizeResponsive
	}
	if err := parseColors(line,
		colorField{view.Color, &style.Color}, colorField{view.OutlineColor, &style.Outline.Color},
		colorField{view.ShadowColor, &style.Shadow.Color}, colorField{view.BackgroundColor, &style.Background.Color},
	); err != nil {
		return style, err
	}
	if err := style.Validate(); err != nil {
		return style, refuse(ptbr.T("error.settings.style", "line", line, "problem", fieldProblem(err)))
	}
	return style, nil
}

func layoutFrom(view LayoutSettings) (appearance.Layout, error) {
	layout := appearance.Layout{
		MarginLeft: view.MarginLeft, MarginRight: view.MarginRight,
		MarginTop: view.MarginTop, MarginBottom: view.MarginBottom, MaxWidth: view.MaxWidth, LineGap: view.LineGap,
	}
	if err := layout.Validate(); err != nil {
		return layout, refuse(ptbr.T("error.settings.margins", "problem", fieldProblem(err)))
	}
	return layout, nil
}

func marksFrom(view MarkSettings) (settings.MarkPreferences, error) {
	notice, err := noticeFrom(view.Notice)
	if err != nil {
		return settings.MarkPreferences{}, err
	}
	selection, err := selectionFrom(view.Selection)
	if err != nil {
		return settings.MarkPreferences{}, err
	}
	duration := seconds(view.NoticeDuration)
	if !(duration.Std() >= settings.MinNoticeDuration && duration.Std() <= settings.MaxNoticeDuration) {
		return settings.MarkPreferences{}, refuse(ptbr.T("error.settings.noticeDuration",
			"min", ptbr.Number(settings.MinNoticeDuration.Seconds(), -1), "max", ptbr.Number(settings.MaxNoticeDuration.Seconds(), -1)))
	}
	return settings.MarkPreferences{Notice: notice, NoticeDuration: duration, Selection: selection}, nil
}

func noticeFrom(view NoticeSettings) (appearance.NoticeLook, error) {
	part := ptbr.T("part.notice")
	notice := appearance.NoticeLook{
		FontFamily: view.FontFamily, FontSize: view.FontSize, Side: appearance.Alignment(view.Side),
		Margin: view.Margin, MaxWidth: view.MaxWidth,
	}
	if err := parseColors(part, colorField{view.Color, &notice.Color}, colorField{view.Background, &notice.Background}); err != nil {
		return notice, err
	}
	if err := notice.Validate(); err != nil {
		return notice, refuse(ptbr.T("error.settings.style", "line", part, "problem", fieldProblem(err)))
	}
	return notice, nil
}

func selectionFrom(view SelectionSettings) (appearance.SelectionLook, error) {
	part := ptbr.T("part.selection")
	selection := appearance.SelectionLook{Width: view.Width, Gap: view.Gap, Radius: view.Radius}
	if err := parseColors(part, colorField{view.Color, &selection.Color}, colorField{view.Edge, &selection.Edge}); err != nil {
		return selection, err
	}
	if err := selection.Validate(); err != nil {
		return selection, refuse(ptbr.T("error.settings.style", "line", part, "problem", fieldProblem(err)))
	}
	return selection, nil
}

// colorField is a color as the screen sent it, and where it goes once read.
type colorField struct {
	text   string
	target *appearance.Color
}

// parseColors reads each color, refusing the first that is not one; part
// names what the colors are of.
func parseColors(part string, fields ...colorField) error {
	for _, field := range fields {
		parsed, err := appearance.ParseColor(field.text)
		if err != nil {
			return refuse(ptbr.T("error.settings.color", "line", part, "color", fmt.Sprintf("%q", field.text)))
		}
		*field.target = parsed
	}
	return nil
}

// fieldProblem words which value of a style or layout is out of range.
func fieldProblem(err error) string {
	var field *appearance.FieldError
	if !errors.As(err, &field) {
		return ptbr.T("settings.problem.outOfRange")
	}
	limit := func(value float64) string { return ptbr.Number(value, -1) }
	switch field.Field {
	case "fontFamily", "notice.fontFamily":
		return ptbr.T("settings.problem.fontFamily")
	case "fontSize":
		return ptbr.T("settings.problem.fontSize", "min", limit(appearance.MinFontSize), "max", limit(appearance.MaxFontSize))
	case "notice.fontSize":
		return ptbr.T("settings.problem.fontSize", "min", limit(appearance.MinFontSize), "max", limit(appearance.MaxNoticeFontSize))
	case "sizeMode":
		return ptbr.T("settings.problem.sizeMode")
	case "opacity":
		return ptbr.T("settings.problem.opacity")
	case "lineSpacing":
		return ptbr.T("settings.problem.lineSpacing", "min", limit(appearance.MinLineSpacing), "max", limit(appearance.MaxLineSpacing))
	case "lineGap":
		return ptbr.T("settings.problem.lineGap", "max", limit(appearance.MaxLineGap))
	case "notice.margin":
		return ptbr.T("settings.problem.noticeMargin", "max", limit(appearance.MaxNoticeMargin*100))
	case "notice.maxWidth":
		return ptbr.T("settings.problem.noticeWidth", "min", limit(appearance.MinNoticeMaxWidth*100))
	case "selection.width":
		return ptbr.T("settings.problem.selectionWidth",
			"min", limit(appearance.MinSelectionWidth), "max", limit(appearance.MaxSelectionWidth))
	case "selection.gap":
		return ptbr.T("settings.problem.selectionGap", "max", limit(appearance.MaxSelectionGap))
	case "selection.radius":
		return ptbr.T("settings.problem.selectionRadius", "max", limit(appearance.MaxSelectionCurve))
	case "cue.barThickness":
		return ptbr.T("settings.problem.barThickness", "min", limit(appearance.MinCueThickness), "max", limit(appearance.MaxCueThickness))
	case "cue.countdownSize":
		return ptbr.T("settings.problem.countdownSize",
			"min", limit(appearance.MinCountdownSize*100), "max", limit(appearance.MaxCountdownSize*100))
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
	case "alignment", "notice.side":
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
	cue, err := cueFrom(view)
	if err != nil {
		return settings.KaraokePreferences{}, err
	}
	karaoke := settings.KaraokePreferences{
		Enabled: view.Enabled, MinGap: seconds(view.MinGap), Lead: seconds(view.Lead),
		Indicator: study.Indicator(view.Indicator), PreviewColor: preview, Fill: view.Fill, FillColor: fill, Repeat: view.Repeat,
		Cue: cue,
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

// cueFrom reads how the time left before a previewed line looks.
func cueFrom(view KaraokeSettings) (appearance.CueStyle, error) {
	bar, err := appearance.ParseColor(view.BarColor)
	if err != nil {
		return appearance.CueStyle{}, refuse(ptbr.T("error.karaoke.barColor"))
	}
	cue := appearance.CueStyle{BarThickness: view.BarThickness, BarColor: bar, CountdownSize: view.CountdownSize}
	if err := cue.Validate(); err != nil {
		return cue, refuse(ptbr.T("error.settings.style", "line", ptbr.T("part.karaoke"), "problem", fieldProblem(err)))
	}
	return cue, nil
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
