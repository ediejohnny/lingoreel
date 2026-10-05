package subtitle_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func TestNormalizeText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"keeps plain text", "I'm going home.", "I'm going home."},
		{"unifies CRLF and CR", "line one\r\nline two\rline three", "line one\nline two\nline three"},
		{"trims lines and drops blanks", "  hello  \n\n\n  world ", "hello\nworld"},
		{"removes control characters", "bell\a and\x00 null", "bell and null"},
		{"turns tabs into spaces", "a\tb", "a b"},
		{"keeps accents", "Você não vai para lá.", "Você não vai para lá."},
		{"removes byte order mark", string(rune(0xFEFF)) + "text", "text"},
		{"empty stays empty", " \n \t ", ""},
		{"nothing at all", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := subtitle.NormalizeText(tc.raw); got != tc.want {
				t.Fatalf("NormalizeText(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNormalizeTextRepairsInvalidUTF8(t *testing.T) {
	t.Parallel()
	got := subtitle.NormalizeText("bad \xff byte")
	if !utf8.ValidString(got) || !strings.HasPrefix(got, "bad ") || !strings.HasSuffix(got, " byte") {
		t.Fatalf("NormalizeText returned %q", got)
	}
}

func TestNormalizeTextTruncatesHugeText(t *testing.T) {
	t.Parallel()
	cases := map[int]int{
		subtitle.MaxTextRunes - 1:   subtitle.MaxTextRunes - 1,
		subtitle.MaxTextRunes:       subtitle.MaxTextRunes,
		subtitle.MaxTextRunes + 1:   subtitle.MaxTextRunes,
		subtitle.MaxTextRunes + 500: subtitle.MaxTextRunes,
	}
	for length, want := range cases {
		got := subtitle.NormalizeText(strings.Repeat("é", length))
		if count := utf8.RuneCountInString(got); count != want || !utf8.ValidString(got) {
			t.Fatalf("input of %d runes kept %d runes, want %d", length, count, want)
		}
	}
}
