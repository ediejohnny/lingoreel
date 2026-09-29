package subtitle

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeText(t *testing.T) {
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
		{"empty stays empty", " \n \t ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeText(tc.raw); got != tc.want {
				t.Fatalf("NormalizeText(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNormalizeTextRemovesByteOrderMark(t *testing.T) {
	raw := string([]rune{byteOrderMark}) + "text"
	if got := NormalizeText(raw); got != "text" {
		t.Fatalf("NormalizeText kept the byte order mark: %q", got)
	}
}

func TestNormalizeTextRepairsInvalidUTF8(t *testing.T) {
	got := NormalizeText("bad \xff byte")
	if !utf8.ValidString(got) {
		t.Fatalf("NormalizeText returned invalid UTF-8: %q", got)
	}
}

func TestNormalizeTextTruncatesHugeText(t *testing.T) {
	got := NormalizeText(strings.Repeat("é", MaxTextRunes+500))
	if count := utf8.RuneCountInString(got); count != MaxTextRunes {
		t.Fatalf("NormalizeText kept %d runes, want %d", count, MaxTextRunes)
	}
}
