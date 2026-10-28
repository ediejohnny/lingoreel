//go:build windows && amd64

package playerwindow

import (
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
)

// AppStage is a stage as the app drives it, in the settings' own terms: the
// video's place, and the look of the controls over it.
type AppStage struct{ *Stage }

// Place shows the video, or hides it, in the room the app's controls leave.
func (s AppStage) Place(visible bool, insets layout.Insets) {
	s.SetLayout(StageLayout{Visible: visible, Insets: insets})
}

// SetChrome gives the controls over the video the bar, the correction box
// and the theme the settings hold; the system's theme follows Windows.
func (s AppStage) SetChrome(bar settings.BarPreferences, edit appearance.EditStyle, theme settings.Theme) {
	shade := map[settings.Theme]Theme{settings.ThemeLight: ThemeLight, settings.ThemeDark: ThemeDark}[theme]
	s.Stage.SetChrome(Chrome{Bar: bar.Style, HideDelay: bar.HideDelay.Std(), Edit: edit, Theme: shade})
}
