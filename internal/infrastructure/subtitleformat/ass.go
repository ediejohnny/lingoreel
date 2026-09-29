package subtitleformat

import (
	"fmt"
	"strings"
)

// ASSParser reads Advanced SubStation Alpha (.ass) and SubStation Alpha (.ssa)
// documents. Only the [Events] section matters; styling is ignored because the
// overlay applies its own appearance.
type ASSParser struct{}

const assEventsSection = "[events]"

// defaultASSEventFormat is used when a document has no Format line.
var defaultASSEventFormat = []string{"layer", "start", "end", "style", "name", "marginl", "marginr", "marginv", "effect", "text"}

// Parse implements Parser.
func (ASSParser) Parse(data []byte) (Result, error) {
	if err := checkInputSize(data); err != nil {
		return Result{}, err
	}

	var cues []cue
	skipped := 0
	section := ""
	sawEvents := false
	format := defaultASSEventFormat
	for _, line := range splitLines(DecodeText(data)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.ToLower(trimmed)
			sawEvents = sawEvents || section == assEventsSection
			continue
		}
		if section != assEventsSection {
			continue
		}

		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "format":
			format = parseASSFormat(value)
		case "dialogue":
			if parsed, ok := parseASSDialogue(value, format); ok {
				cues = append(cues, parsed)
			} else {
				skipped++
			}
		}
	}
	if !sawEvents {
		return Result{}, fmt.Errorf("%w: no [Events] section", ErrMalformed)
	}
	return buildResult(cues, skipped), nil
}

func parseASSFormat(value string) []string {
	fields := strings.Split(value, ",")
	for i := range fields {
		fields[i] = strings.ToLower(strings.TrimSpace(fields[i]))
	}
	return fields
}

// parseASSDialogue splits a Dialogue line by its Format; the text field keeps
// any commas because it receives the remainder of the line.
func parseASSDialogue(value string, format []string) (cue, bool) {
	fields := strings.SplitN(value, ",", len(format))
	if len(fields) != len(format) {
		return cue{}, false
	}

	var parsed cue
	var hasStart, hasEnd, hasText bool
	for i, name := range format {
		switch name {
		case "start":
			start, err := ParseTimestamp(fields[i])
			parsed.start, hasStart = start, err == nil
		case "end":
			end, err := ParseTimestamp(fields[i])
			parsed.end, hasEnd = end, err == nil
		case "text":
			parsed.text, hasText = assToPlainText(fields[i]), true
		}
	}
	return parsed, hasStart && hasEnd && hasText
}
