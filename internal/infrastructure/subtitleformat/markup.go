package subtitleformat

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	// htmlTagPattern matches simple markup such as <i>, </b> or <font color="red">.
	htmlTagPattern = regexp.MustCompile(`</?[a-zA-Z][^<>]*>`)
	// vttTimestampTagPattern matches WebVTT karaoke timestamps such as <00:01.500>.
	vttTimestampTagPattern = regexp.MustCompile(`<\d[^<>]*>`)
	// assOverrideInTextPattern matches ASS override blocks that leak into SRT files, such as {\an8}.
	assOverrideInTextPattern = regexp.MustCompile(`\{\\[^{}]*\}`)
	// assDrawingTagPattern matches the \p<n> tag that switches vector drawing mode.
	assDrawingTagPattern = regexp.MustCompile(`\\p(\d+)`)
)

// cleanSRTText removes HTML-like tags and stray ASS override blocks.
func cleanSRTText(text string) string {
	return assOverrideInTextPattern.ReplaceAllString(htmlTagPattern.ReplaceAllString(text, ""), "")
}

// cleanVTTText removes cue markup and decodes character references.
func cleanVTTText(text string) string {
	text = htmlTagPattern.ReplaceAllString(text, "")
	text = vttTimestampTagPattern.ReplaceAllString(text, "")
	return html.UnescapeString(text)
}

// assToPlainText drops ASS override blocks, vector drawings and escapes,
// keeping only the visible text. It walks bytes but only splits on ASCII, so
// multi-byte characters stay intact.
func assToPlainText(text string) string {
	var builder strings.Builder
	drawing := false
	emit := func(s string) {
		if !drawing {
			builder.WriteString(s)
		}
	}

	for i := 0; i < len(text); {
		switch {
		case text[i] == '{':
			closing := strings.IndexByte(text[i:], '}')
			if closing < 0 {
				emit(text[i:])
				i = len(text)
				continue
			}
			if level, found := drawingLevel(text[i+1 : i+closing]); found {
				drawing = level > 0
			}
			i += closing + 1
		case strings.HasPrefix(text[i:], `\N`), strings.HasPrefix(text[i:], `\n`):
			emit("\n")
			i += 2
		case strings.HasPrefix(text[i:], `\h`):
			emit(" ")
			i += 2
		default:
			emit(text[i : i+1])
			i++
		}
	}
	return builder.String()
}

// drawingLevel returns the last \p<n> value of an override block.
func drawingLevel(block string) (int, bool) {
	matches := assDrawingTagPattern.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		return 0, false
	}
	level, err := strconv.Atoi(matches[len(matches)-1][1])
	if err != nil {
		return 0, false
	}
	return level, true
}
