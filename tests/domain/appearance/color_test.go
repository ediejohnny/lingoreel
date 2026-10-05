package appearance_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
)

func TestParseColorAcceptsBothForms(t *testing.T) {
	cases := []struct {
		value string
		want  appearance.Color
	}{
		{"#FFFFFF", appearance.Color{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}},
		{"#000000", appearance.Color{A: 0xFF}},
		{"#1a2B3c", appearance.Color{R: 0x1A, G: 0x2B, B: 0x3C, A: 0xFF}},
		{"#11223344", appearance.Color{R: 0x11, G: 0x22, B: 0x33, A: 0x44}},
		{"#00000000", appearance.Color{}},
		{"  #ABCDEF  ", appearance.Color{R: 0xAB, G: 0xCD, B: 0xEF, A: 0xFF}},
	}
	for _, tc := range cases {
		got, err := appearance.ParseColor(tc.value)
		if err != nil || got != tc.want {
			t.Fatalf("ParseColor(%q) = %+v, %v; want %+v", tc.value, got, err, tc.want)
		}
	}
}

func TestParseColorRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "#", "FFFFFF", "#FFF", "#FFFFF", "#1234567", "#123456789", "#GGGGGG", "#12 456", "rgb(1,2,3)", "#-12345", "#+12345"} {
		if _, err := appearance.ParseColor(value); !errors.Is(err, appearance.ErrInvalidColor) {
			t.Fatalf("ParseColor(%q) error = %v, want ErrInvalidColor", value, err)
		}
	}
}

func TestColorStringOmitsAlphaOnlyWhenOpaque(t *testing.T) {
	cases := map[appearance.Color]string{
		appearance.RGB(0xFF, 0x80, 0x00):                 "#FF8000",
		appearance.RGB(0xFF, 0x80, 0x00).WithAlpha(0xFE): "#FF8000FE",
		appearance.RGB(0, 0, 0).WithAlpha(0):             "#00000000",
	}
	for color, want := range cases {
		if got := color.String(); got != want {
			t.Fatalf("%+v.String() = %q, want %q", color, got, want)
		}
		parsed, err := appearance.ParseColor(want)
		if err != nil || parsed != color {
			t.Fatalf("round trip of %q = %+v, %v; want %+v", want, parsed, err, color)
		}
	}
}

func TestColorJSONUsesHexText(t *testing.T) {
	var target struct {
		Value appearance.Color `json:"color"`
	}
	if err := json.Unmarshal([]byte(`{"color":"#12345"}`), &target); !errors.Is(err, appearance.ErrInvalidColor) {
		t.Fatalf("Unmarshal error = %v, want ErrInvalidColor", err)
	}
	if err := json.Unmarshal([]byte(`{"color":"#10203040"}`), &target); err != nil || target.Value != (appearance.Color{R: 0x10, G: 0x20, B: 0x30, A: 0x40}) {
		t.Fatalf("Unmarshal valid color = %+v, %v", target.Value, err)
	}
	encoded, err := json.Marshal(target)
	if err != nil || string(encoded) != `{"color":"#10203040"}` {
		t.Fatalf("Marshal = %s, %v", encoded, err)
	}
}
