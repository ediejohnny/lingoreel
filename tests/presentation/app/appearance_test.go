package app_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
)

// How the controls over the video look: kept for every profile, applied at
// once to the video's stage, and applied again when the app starts.

func TestTheAppearanceIsKeptAndReachesTheStage(t *testing.T) {
	t.Parallel()
	r, stage := stageRig(t)
	view := settingsOf(t, r).Appearance
	view.Bar.Accent, view.Bar.Size, view.Bar.HideDelay = "#FF8C00", 1.5, 4
	view.Edit.Background, view.Edit.HintSize = "#102030", 18
	if err := r.api.SetAppearance(view); err != nil {
		t.Fatal(err)
	}
	bar := settings.DefaultBar()
	bar.Style.Accent, bar.Style.Size, bar.HideDelay = appearance.RGB(0xFF, 0x8C, 0x00), 1.5, settings.Duration(4*time.Second)
	edit := appearance.DefaultEditStyle()
	edit.Background, edit.HintSize = appearance.RGB(0x10, 0x20, 0x30), 18
	if r.settings.config.Bar != bar || r.settings.config.Edit != edit {
		t.Fatalf("kept %+v and %+v, want %+v and %+v", r.settings.config.Bar, r.settings.config.Edit, bar, edit)
	}
	styled := stage.styled()
	if got := styled[len(styled)-1]; got.bar != bar || got.edit != edit {
		t.Fatalf("the stage got %+v, want %+v and %+v", got, bar, edit)
	}
	if got := settingsOf(t, r).Appearance; got != view {
		t.Fatalf("the settings show %+v, want %+v", got, view)
	}
}

func TestTheStoredAppearanceReachesTheStageWhenTheAppStarts(t *testing.T) {
	t.Parallel()
	stage := &fakeStage{}
	r := newRigWith(t, func(deps *app.Deps) { deps.Stage = stage })
	r.settings.config.Bar.Style.Accent = appearance.RGB(0xFF, 0x8C, 0x00)
	r.settings.config.Edit.HintSize = 18
	r.attach(t)
	styled := stage.styled()
	if len(styled) != 1 || styled[0].bar != r.settings.config.Bar || styled[0].edit != r.settings.config.Edit {
		t.Fatalf("the stage got %+v, want the kept look once", styled)
	}
}

func TestTheAppearanceRefusesBadChoicesInPortuguese(t *testing.T) {
	t.Parallel()
	r, stage := stageRig(t)
	base := settingsOf(t, r).Appearance
	cases := []struct {
		name   string
		change func(*app.AppearanceSettings)
		want   string
	}{
		{"bar color", func(v *app.AppearanceSettings) { v.Bar.Icon = "branco" }, `Barra de controles: a cor "branco"`},
		{"bar size", func(v *app.AppearanceSettings) { v.Bar.Size = math.NaN() }, "Barra de controles: o tamanho vai de"},
		{"hide delay", func(v *app.AppearanceSettings) { v.Bar.HideDelay = 0.5 }, "Barra de controles: o tempo para sumir"},
		{"hint size", func(v *app.AppearanceSettings) { v.Edit.HintSize = 40 }, "Caixa de correção: o tamanho da dica"},
	}
	styled := len(stage.styled())
	for _, c := range cases {
		view := base
		c.change(&view)
		saves := r.settings.saves
		err := r.api.SetAppearance(view)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: SetAppearance = %v, want %q", c.name, err, c.want)
		}
		if r.settings.saves != saves {
			t.Errorf("%s: a refused look was saved", c.name)
		}
	}
	if got := len(stage.styled()); got != styled {
		t.Fatalf("a refused look reached the stage %d times", got-styled)
	}
}

// The app's own look is kept and comes back with the preferences, which the
// screens apply when the app opens; a see-through accent is refused.
func TestTheAppsLookIsKeptForTheNextStart(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	if got := r.api.Preferences().Interface; got != (app.InterfaceSettings{TextSize: 1}) {
		t.Fatalf("a new app's look = %+v, want the themes' accent and font at 100%%", got)
	}
	chosen := app.InterfaceSettings{Accent: "#FF8C00", FontFamily: "Verdana", TextSize: 1.25}
	if err := r.api.SetInterface(chosen); err != nil {
		t.Fatal(err)
	}
	want := appearance.InterfaceStyle{Accent: appearance.RGB(0xFF, 0x8C, 0x00), FontFamily: "Verdana", TextSize: 1.25}
	if r.settings.config.Interface != want {
		t.Fatalf("kept %+v, want %+v", r.settings.config.Interface, want)
	}
	if got := r.api.Preferences().Interface; got != chosen {
		t.Fatalf("the preferences give %+v, want %+v", got, chosen)
	}
	saves := r.settings.saves
	err := r.api.SetInterface(app.InterfaceSettings{Accent: "#FF8C0080", TextSize: 1})
	if err == nil || !strings.Contains(err.Error(), "Aparência do app: a cor de destaque") || r.settings.saves != saves {
		t.Fatalf("a see-through accent: %v, %d saves", err, r.settings.saves-saves)
	}
}

func TestTheBarsPreviewIsAPictureOfTheStyle(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	bar := settingsOf(t, r).Appearance.Bar
	bar.Panel, bar.Size = "#FF8C0080", 2
	request := app.BarPreviewRequest{Bar: bar, Width: 640, Height: 120, Scale: 1.5}
	if _, err := r.api.PreviewBar(request); err != nil {
		t.Fatal(err)
	}
	call := r.preview.bars[0]
	if call.style.Panel != appearance.RGB(0xFF, 0x8C, 0x00).WithAlpha(0x80) || call.style.Size != 2 ||
		call.width != 640 || call.height != 120 || call.dpi != 144 {
		t.Fatalf("preview call = %+v", call)
	}
	request.Bar.Accent = "#12"
	if _, err := r.api.PreviewBar(request); err == nil || len(r.preview.bars) != 1 {
		t.Fatal("a bad color reached the preview")
	}
}
