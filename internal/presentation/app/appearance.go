package app

import (
	"encoding/base64"
	"math"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// The bar's preview shows a video a quarter played.
const (
	previewBarPosition = 25 * time.Minute
	previewBarDuration = 100 * time.Minute
)

// SetAppearance keeps how the controls over the video look, for every
// profile, and applies it at once: to the video being watched, and to the
// next ones.
func (a *API) SetAppearance(view AppearanceSettings) error {
	bar, err := barFrom(view.Bar)
	if err != nil {
		return err
	}
	edit, err := editFrom(view.Edit)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.config
	next.Bar, next.Edit = bar, edit
	if err := a.commitConfig(next); err != nil {
		return err
	}
	if a.deps.Stage != nil {
		a.deps.Stage.SetChrome(bar, edit, next.Theme)
	}
	return nil
}

// SetInterface keeps the app's own look over its theme; the screens apply it
// themselves, at once, like the theme.
func (a *API) SetInterface(view InterfaceSettings) error {
	look, err := interfaceFrom(view)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.config
	next.Interface = look
	return a.commitConfig(next)
}

// PreviewBar draws the control bar in a style, as a PNG data address for an
// image on the screen.
func (a *API) PreviewBar(request BarPreviewRequest) (string, error) {
	if a.deps.Preview == nil {
		return "", errNoPreview
	}
	bar, err := barFrom(request.Bar)
	if err != nil {
		return "", err
	}
	scale := request.Scale
	if !(scale >= minPreviewScale && scale <= maxPreviewScale) {
		scale = 1
	}
	clock := ptbr.BarTime(previewBarPosition, previewBarDuration)
	picture, err := a.deps.Preview.PreviewBar(bar.Style, request.Width, request.Height, int(math.Round(layout.BaseDPI*scale)), clock, ptbr.Speed)
	if err != nil {
		a.logger.Warn("the bar's preview was not drawn", "error", err)
		return "", errBadPreview
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(picture), nil
}

func appearanceView(bar settings.BarPreferences, edit appearance.EditStyle) AppearanceSettings {
	style := bar.Style
	return AppearanceSettings{
		Bar: BarSettings{
			Panel: style.Panel.String(), Menu: style.Menu.String(), Accent: style.Accent.String(),
			Icon: style.Icon.String(), Loud: style.Loud.String(), FontFamily: style.FontFamily, Size: style.Size,
			HideDelay: bar.HideDelay.Std().Seconds(),
		},
		Edit: EditSettings{
			Background: edit.Background.String(), Text: edit.Text.String(), Hint: edit.Hint.String(), HintSize: edit.HintSize,
		},
	}
}

func barFrom(view BarSettings) (settings.BarPreferences, error) {
	part := ptbr.T("part.bar")
	style := appearance.BarStyle{FontFamily: view.FontFamily, Size: view.Size}
	if err := parseColors(part,
		colorField{view.Panel, &style.Panel}, colorField{view.Menu, &style.Menu}, colorField{view.Accent, &style.Accent},
		colorField{view.Icon, &style.Icon}, colorField{view.Loud, &style.Loud},
	); err != nil {
		return settings.BarPreferences{}, err
	}
	if err := style.Validate(); err != nil {
		return settings.BarPreferences{}, refuse(ptbr.T("error.settings.style", "line", part, "problem", fieldProblem(err)))
	}
	delay := seconds(view.HideDelay)
	if !(delay.Std() >= settings.MinBarHideDelay && delay.Std() <= settings.MaxBarHideDelay) {
		return settings.BarPreferences{}, refuse(ptbr.T("error.settings.hideDelay",
			"min", ptbr.Number(settings.MinBarHideDelay.Seconds(), -1), "max", ptbr.Number(settings.MaxBarHideDelay.Seconds(), -1)))
	}
	return settings.BarPreferences{Style: style, HideDelay: delay}, nil
}

func interfaceView(look appearance.InterfaceStyle) InterfaceSettings {
	view := InterfaceSettings{FontFamily: look.FontFamily, TextSize: look.TextSize}
	if look.HasAccent() {
		view.Accent = look.Accent.String()
	}
	return view
}

func interfaceFrom(view InterfaceSettings) (appearance.InterfaceStyle, error) {
	part := ptbr.T("part.interface")
	look := appearance.InterfaceStyle{FontFamily: view.FontFamily, TextSize: view.TextSize}
	if view.Accent != "" {
		if err := parseColors(part, colorField{view.Accent, &look.Accent}); err != nil {
			return look, err
		}
	}
	if err := look.Validate(); err != nil {
		return look, refuse(ptbr.T("error.settings.style", "line", part, "problem", fieldProblem(err)))
	}
	return look, nil
}

func editFrom(view EditSettings) (appearance.EditStyle, error) {
	part := ptbr.T("part.edit")
	edit := appearance.EditStyle{HintSize: view.HintSize}
	if err := parseColors(part,
		colorField{view.Background, &edit.Background}, colorField{view.Text, &edit.Text}, colorField{view.Hint, &edit.Hint},
	); err != nil {
		return edit, err
	}
	if err := edit.Validate(); err != nil {
		return edit, refuse(ptbr.T("error.settings.style", "line", part, "problem", fieldProblem(err)))
	}
	return edit, nil
}
