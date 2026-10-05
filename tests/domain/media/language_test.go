package media_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

func TestNormalizeLanguage(t *testing.T) {
	cases := []struct {
		bcp47, iso639, name string
		want                media.LanguageTag
	}{
		{"pt-BR", "", "", "pt-BR"},
		{"pt_br", "", "", "pt-BR"},
		{"PT-br", "", "", "pt-BR"},
		{"", "por", "", "pt"},
		{"", "por", "Brazilian Portuguese", "pt-BR"}, // the PRD example
		{"", "por", "Português (Brasil)", "pt-BR"},
		{"", "por", "PT-BR", "pt-BR"},
		{"", "por", "Portuguese (European)", "pt-PT"},
		{"", "eng", "", "en"},
		{"en-US", "eng", "", "en-US"},
		{"pt-BR", "eng", "English", "pt-BR"}, // the declared BCP 47 tag wins
		{"", "eng", "Brazilian", "en"},       // hints only refine the same language
		{"", "eng", "English (SDH)", "en"},
		{"", "und", "English", "en"},
		{"", "", "Português", "pt"},
		{"", "", "日本語", "ja"},
		{"", "spa", "Latin American", "es-419"},
		{"", "spa", "Castilian", "es-ES"},
		{"zh-hant", "", "", "zh-Hant"},
		{"", "chi", "Simplified", "zh-Hans"},
		{"", "fre", "", "fr"},
		{"", "ger", "", "de"},
		{"  ", "por", "", "pt"},
		{"", "und", "", ""},
		{"", "xyz", "", ""},
		{"", "", "Commentary", ""},
		{"english", "", "", ""},
		{"", "eng2", "", ""},
		{"", "", "", ""},
	}
	for _, tc := range cases {
		if got := media.NormalizeLanguage(tc.bcp47, tc.iso639, tc.name); got != tc.want {
			t.Fatalf("NormalizeLanguage(%q, %q, %q) = %q, want %q", tc.bcp47, tc.iso639, tc.name, got, tc.want)
		}
	}
}

func TestLanguageTagParts(t *testing.T) {
	cases := []struct {
		tag          media.LanguageTag
		base, region string
	}{
		{"pt-BR", "pt", "BR"},
		{"en", "en", ""},
		{"zh-Hans", "zh", "Hans"},
		{"", "", ""},
	}
	for _, tc := range cases {
		if tc.tag.Base() != tc.base || tc.tag.Region() != tc.region {
			t.Fatalf("%q parts = %q/%q, want %q/%q", tc.tag, tc.tag.Base(), tc.tag.Region(), tc.base, tc.region)
		}
	}
}
