package app

import (
	"errors"
	"fmt"
	"math"
	"time"

	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/presentation/ptbr"
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
		Speed:        float64(profile.Speed),
		SyncStep:     int(profile.Sync.OffsetStep.Std() / time.Millisecond),
		SyncFastStep: int(profile.Sync.OffsetLargeStep.Std() / time.Millisecond),
		Karaoke: KaraokeSettings{
			Enabled: karaoke.Enabled, MinGap: karaoke.MinGap.Std().Seconds(), Lead: karaoke.Lead.Std().Seconds(),
			Indicator: string(karaoke.Indicator), PreviewColor: karaoke.PreviewColor.String(),
			Fill: karaoke.Fill, FillColor: karaoke.FillColor.String(), Repeat: karaoke.Repeat,
		},
		Reveal:    string(profile.Study.Reveal),
		Shortcuts: []ShortcutSetting{},
	}
	keys := profile.Shortcuts.Resolved()
	for _, command := range shortcut.Commands() {
		chord := keys[command]
		view.Shortcuts = append(view.Shortcuts, ShortcutSetting{
			Command: string(command), Name: ptbr.CommandName(command), Category: ptbr.CategoryName(command.Category()),
			Chord: chord.String(), Label: ptbr.ShortcutLabel(chord, char),
		})
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

func refuse(format string, args ...any) error {
	return settingsError{message: fmt.Sprintf(format, args...)}
}

// profileFrom puts the screen's choices on a profile, checked; what the
// screen does not show (the study mode, the precision step) stays as base has
// it.
func profileFrom(base settings.Profile, view ProfileSettings, char ptbr.KeyChar) (settings.Profile, error) {
	primary, err := styleFrom(view.Primary, "Legenda de cima")
	if err != nil {
		return settings.Profile{}, err
	}
	translation, err := styleFrom(view.Translation, "Legenda de baixo")
	if err != nil {
		return settings.Profile{}, err
	}
	layout := appearance.Layout{
		MarginLeft: view.Layout.MarginLeft, MarginRight: view.Layout.MarginRight,
		MarginTop: view.Layout.MarginTop, MarginBottom: view.Layout.MarginBottom, MaxWidth: view.Layout.MaxWidth,
	}
	if err := layout.Validate(); err != nil {
		return settings.Profile{}, refuse("Margens: %s.", fieldProblem(err))
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
	profile.Study.Reveal, profile.Study.Karaoke = study.RevealBehavior(view.Reveal), karaoke
	profile.Speed = playback.Speed(view.Speed)
	profile.Sync.OffsetStep = settings.Duration(time.Duration(view.SyncStep) * time.Millisecond)
	profile.Sync.OffsetLargeStep = settings.Duration(time.Duration(view.SyncFastStep) * time.Millisecond)
	profile.Shortcuts = keys
	switch {
	case !profile.Speed.Valid():
		return settings.Profile{}, refuse("Escolha uma das velocidades da lista.")
	case profile.Sync.Nudge().Validate() != nil:
		return settings.Profile{}, refuse("Passos da sincronização: o passo vai de %d a %d ms, e o passo segurando vai do passo até %d ms.",
			subtitle.MinNudgeStep.Milliseconds(), subtitle.MaxNudgeStep.Milliseconds(), subtitle.MaxFastNudgeStep.Milliseconds())
	case !profile.Study.Reveal.Valid():
		return settings.Profile{}, refuse("Escolha como a tecla da tradução funciona.")
	}
	if err := profile.Validate(); err != nil {
		return settings.Profile{}, refuse("Confira os idiomas preferidos e o player preferido.")
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
			return style, refuse("%s: a cor %q não é válida.", line, color.text)
		}
		*color.target = parsed
	}
	if err := style.Validate(); err != nil {
		return style, refuse("%s: %s.", line, fieldProblem(err))
	}
	return style, nil
}

// fieldProblem words which value of a style or layout is out of range.
func fieldProblem(err error) string {
	var field *appearance.FieldError
	if !errors.As(err, &field) {
		return "um dos valores está fora do limite"
	}
	switch field.Field {
	case "fontFamily":
		return "escolha uma fonte"
	case "fontSize":
		return fmt.Sprintf("o tamanho da fonte vai de %g a %g", appearance.MinFontSize, appearance.MaxFontSize)
	case "sizeMode":
		return "escolha tamanho fixo ou responsivo"
	case "opacity":
		return "a opacidade vai de 0% a 100%"
	case "outline.width":
		return fmt.Sprintf("a espessura do contorno vai de 0 a %g", appearance.MaxOutlineWidth)
	case "shadow.offsetX", "shadow.offsetY":
		return fmt.Sprintf("o deslocamento da sombra vai de −%g a %g", appearance.MaxShadowOffset, appearance.MaxShadowOffset)
	case "shadow.blur":
		return fmt.Sprintf("o desfoque da sombra vai de 0 a %g", appearance.MaxShadowBlur)
	case "background.padding":
		return fmt.Sprintf("a margem do fundo vai de 0 a %g", appearance.MaxPadding)
	case "verticalPosition":
		return "a posição vai de 1% a 100% da altura"
	case "alignment":
		return "escolha esquerda, centro ou direita"
	case "maxLines":
		return fmt.Sprintf("o máximo de linhas vai de %d a %d", appearance.MinLines, appearance.MaxLines)
	case "marginLeft", "marginRight", "marginTop", "marginBottom":
		return fmt.Sprintf("cada margem vai de 0%% a %g%%", appearance.MaxMarginFraction*100)
	case "maxWidth":
		return "a largura máxima vai de 1% a 100%"
	}
	return "um dos valores está fora do limite"
}

func karaokeFrom(view KaraokeSettings) (settings.KaraokePreferences, error) {
	preview, err := appearance.ParseColor(view.PreviewColor)
	if err != nil {
		return settings.KaraokePreferences{}, refuse("Modo Karaokê: a cor da prévia não é válida.")
	}
	fill, err := appearance.ParseColor(view.FillColor)
	if err != nil {
		return settings.KaraokePreferences{}, refuse("Modo Karaokê: a cor do preenchimento não é válida.")
	}
	karaoke := settings.KaraokePreferences{
		Enabled: view.Enabled, MinGap: seconds(view.MinGap), Lead: seconds(view.Lead),
		Indicator: study.Indicator(view.Indicator), PreviewColor: preview, Fill: view.Fill, FillColor: fill, Repeat: view.Repeat,
	}
	switch {
	case karaoke.Rules().Validate() != nil:
		return karaoke, refuse("Modo Karaokê: a lacuna mínima vai de %g a %g s e a antecedência de %g a %g s.",
			study.MinCueGap.Seconds(), study.MaxCueGap.Seconds(), study.MinCueLead.Seconds(), study.MaxCueLead.Seconds())
	case !karaoke.Indicator.Valid():
		return karaoke, refuse("Modo Karaokê: escolha a barra ou a contagem regressiva.")
	case karaoke.Repeat < 0 || karaoke.Repeat > study.MaxRepeats:
		return karaoke, refuse("Modo Karaokê: repetir cada fala vai de 0 a %d vezes.", study.MaxRepeats)
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
			return nil, refuse("Atalhos: comando desconhecido.")
		}
		chord, err := shortcut.Parse(view.Chord)
		if err != nil {
			return nil, refuse("Atalho de “%s”: %s", ptbr.CommandName(command), ptbr.ShortcutProblem(err, char))
		}
		keys[command] = chord
	}
	if err := keys.Validate(); err != nil {
		return nil, refuse("%s", ptbr.ShortcutProblem(err, char))
	}
	return keys.Resolved(), nil
}
