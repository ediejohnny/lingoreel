package subtitle

import (
	"strings"
	"unicode"
)

// MaxTextRunes bounds the text of a single event; longer input is truncated so
// a hostile file cannot flood the overlay.
const MaxTextRunes = 1000

// byteOrderMark sometimes survives decoding in the middle of concatenated text.
const byteOrderMark rune = 0xFEFF

// NormalizeText turns raw subtitle text into displayable plain text: newlines
// unified, control characters removed, lines trimmed and blank lines dropped.
func NormalizeText(raw string) string {
	raw = strings.ToValidUTF8(raw, string(unicode.ReplacementChar))
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	var builder strings.Builder
	for _, r := range raw {
		switch {
		case r == '\n':
			builder.WriteRune(r)
		case r == '\t':
			builder.WriteRune(' ')
		case unicode.IsControl(r), r == byteOrderMark:
			continue
		default:
			builder.WriteRune(r)
		}
	}

	var lines []string
	for line := range strings.SplitSeq(builder.String(), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return truncateRunes(strings.Join(lines, "\n"), MaxTextRunes)
}

func truncateRunes(text string, limit int) string {
	count := 0
	for index := range text {
		if count == limit {
			return text[:index]
		}
		count++
	}
	return text
}
