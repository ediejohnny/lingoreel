package appearance

import (
	"errors"
	"testing"
)

func customizedStyle() SubtitleStyle {
	style := DefaultPrimaryStyle()
	style.FontFamily = "Segoe UI"
	style.FontSize = 41
	style.VerticalPosition = 0.7
	style.Alignment = AlignLeft
	style.Background = Background{Enabled: true, Color: RGB(1, 2, 3), Padding: 5}
	return style
}

func TestPresetsKeepFontAndPlacement(t *testing.T) {
	for _, preset := range Presets() {
		t.Run(string(preset), func(t *testing.T) {
			before := customizedStyle()
			after, err := preset.Apply(before)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if err := after.Validate(); err != nil {
				t.Fatalf("preset produced an invalid style: %v", err)
			}
			if after.FontFamily != before.FontFamily || after.FontSize != before.FontSize ||
				after.VerticalPosition != before.VerticalPosition || after.Alignment != before.Alignment {
				t.Fatalf("preset changed font or placement: %+v", after)
			}
		})
	}
}

func TestPresetsProduceDistinctLooks(t *testing.T) {
	seen := map[string]Preset{}
	for _, preset := range Presets() {
		style, err := preset.Apply(customizedStyle())
		if err != nil {
			t.Fatalf("Apply(%s): %v", preset, err)
		}
		look := style.Color.String() + style.Outline.Color.String() + style.Shadow.Color.String() + style.Background.Color.String()
		key := look + map[bool]string{true: "bg", false: ""}[style.Background.Enabled]
		if other, duplicated := seen[key]; duplicated {
			t.Fatalf("presets %s and %s look identical", preset, other)
		}
		seen[key] = preset
	}
}

func TestHighContrastAddsBackgroundAndOthersRemoveIt(t *testing.T) {
	highContrast, _ := PresetHighContrast.Apply(customizedStyle())
	if !highContrast.Background.Enabled || highContrast.Color != RGB(0xFF, 0xE6, 0x00) {
		t.Fatalf("high contrast = %+v, want yellow text on a background", highContrast)
	}
	classic, _ := PresetClassic.Apply(customizedStyle())
	if classic.Background.Enabled {
		t.Fatal("classic preset kept the previous background")
	}
}

func TestUnknownPresetIsRejected(t *testing.T) {
	style := customizedStyle()
	got, err := Preset("neon").Apply(style)
	if !errors.Is(err, ErrUnknownPreset) {
		t.Fatalf("error = %v, want ErrUnknownPreset", err)
	}
	if got != style {
		t.Fatal("unknown preset modified the style")
	}
}
