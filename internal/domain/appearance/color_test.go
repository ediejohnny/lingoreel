package appearance

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseColorAcceptsBothForms(t *testing.T) {
	cases := []struct {
		value string
		want  Color
	}{
		{"#FFFFFF", Color{0xFF, 0xFF, 0xFF, 0xFF}},
		{"#000000", Color{0, 0, 0, 0xFF}},
		{"#1a2B3c", Color{0x1A, 0x2B, 0x3C, 0xFF}},
		{"#11223344", Color{0x11, 0x22, 0x33, 0x44}},
		{"#00000000", Color{0, 0, 0, 0}},
		{"  #ABCDEF  ", Color{0xAB, 0xCD, 0xEF, 0xFF}},
	}
	for _, tc := range cases {
		got, err := ParseColor(tc.value)
		if err != nil || got != tc.want {
			t.Fatalf("ParseColor(%q) = %+v, %v; want %+v", tc.value, got, err, tc.want)
		}
	}
}

func TestParseColorRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "#", "FFFFFF", "#FFF", "#FFFFF", "#1234567", "#123456789", "#GGGGGG", "#12 456", "rgb(1,2,3)", "#-12345"} {
		if _, err := ParseColor(value); !errors.Is(err, ErrInvalidColor) {
			t.Fatalf("ParseColor(%q) error = %v, want ErrInvalidColor", value, err)
		}
	}
}

func TestColorStringOmitsAlphaOnlyWhenOpaque(t *testing.T) {
	cases := map[Color]string{
		RGB(0xFF, 0x80, 0x00):                 "#FF8000",
		RGB(0xFF, 0x80, 0x00).WithAlpha(0xFE): "#FF8000FE",
		RGB(0, 0, 0).WithAlpha(0):             "#00000000",
	}
	for color, want := range cases {
		if got := color.String(); got != want {
			t.Fatalf("%+v.String() = %q, want %q", color, got, want)
		}
		parsed, err := ParseColor(want)
		if err != nil || parsed != color {
			t.Fatalf("round trip of %q = %+v, %v; want %+v", want, parsed, err, color)
		}
	}
}

func TestColorJSONRejectsInvalidText(t *testing.T) {
	var target struct {
		Color Color `json:"color"`
	}
	if err := json.Unmarshal([]byte(`{"color":"#12345"}`), &target); !errors.Is(err, ErrInvalidColor) {
		t.Fatalf("Unmarshal error = %v, want ErrInvalidColor", err)
	}
	if err := json.Unmarshal([]byte(`{"color":"#10203040"}`), &target); err != nil || target.Color != (Color{0x10, 0x20, 0x30, 0x40}) {
		t.Fatalf("Unmarshal valid color = %+v, %v", target.Color, err)
	}
}
