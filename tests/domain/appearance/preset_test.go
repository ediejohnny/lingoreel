package appearance_test

import (
	"errors"
	"testing"

	"lingoreel/internal/domain/appearance"
)

func customizedStyle() appearance.SubtitleStyle {
	style := appearance.DefaultPrimaryStyle()
	style.FontFamily = "Segoe UI"
	style.FontSize = 41
	style.VerticalPosition = 0.7
	style.Alignment = appearance.AlignLeft
	style.Background = appearance.Background{Enabled: true, Color: appearance.RGB(1, 2, 3), Padding: 5}
	return style
}

func TestPresetsKeepFontAndPlacement(t *testing.T) {
	for _, preset := range appearance.Presets() {
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
	seen := map[string]appearance.Preset{}
	for _, preset := range appearance.Presets() {
		style, err := preset.Apply(customizedStyle())
		if err != nil {
			t.Fatalf("Apply(%s): %v", preset, err)
		}
		look := style.Color.String() + "|" + style.Outline.Color.String() + "|" + style.Shadow.Color.String() + "|" + style.Background.Color.String()
		if style.Background.Enabled {
			look += "|background"
		}
		if other, duplicated := seen[look]; duplicated {
			t.Fatalf("presets %s and %s look identical", preset, other)
		}
		seen[look] = preset
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 presets, got %d", len(seen))
	}
}

func TestHighContrastAddsBackgroundAndOthersRemoveIt(t *testing.T) {
	highContrast, _ := appearance.PresetHighContrast.Apply(customizedStyle())
	if !highContrast.Background.Enabled || highContrast.Color != appearance.RGB(0xFF, 0xE6, 0x00) {
		t.Fatalf("high contrast = %+v, want yellow text on a background", highContrast)
	}
	for _, preset := range []appearance.Preset{appearance.PresetClassic, appearance.PresetFilm, appearance.PresetMinimal} {
		style, _ := preset.Apply(customizedStyle())
		if style.Background.Enabled {
			t.Fatalf("preset %s kept the previous background", preset)
		}
	}
}

func TestUnknownPresetIsRejectedWithoutChanges(t *testing.T) {
	style := customizedStyle()
	for _, preset := range []appearance.Preset{"neon", "", "Classic"} {
		got, err := preset.Apply(style)
		if !errors.Is(err, appearance.ErrUnknownPreset) {
			t.Fatalf("Apply(%q) error = %v, want ErrUnknownPreset", preset, err)
		}
		if got != style {
			t.Fatalf("unknown preset %q modified the style", preset)
		}
	}
}
