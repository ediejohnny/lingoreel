package subtitleformat

import (
	"regexp"
	"strings"
)

// SRTParser reads SubRip (.srt) documents. It tolerates missing or broken
// indices, missing blank lines between cues and '.' as decimal separator.
type SRTParser struct{}

var srtTimingLinePattern = regexp.MustCompile(`^\s*\d+:\d{1,2}:\d{1,2}(?:[,.]\d+)?\s*-->\s*\d+:\d{1,2}:\d{1,2}(?:[,.]\d+)?`)

func isSRTTimingLine(line string) bool {
	return srtTimingLinePattern.MatchString(line)
}

// Parse implements Parser.
func (SRTParser) Parse(data []byte) (Result, error) {
	if err := checkInputSize(data); err != nil {
		return Result{}, err
	}
	lines := splitLines(DecodeText(data))

	var cues []cue
	skipped := 0
	inGarbage := false
	for i := 0; i < len(lines); {
		if isBlank(lines[i]) {
			inGarbage = false
			i++
			continue
		}

		timing := i
		if !isSRTTimingLine(lines[i]) {
			timing = i + 1
		}
		if timing >= len(lines) || !isSRTTimingLine(lines[timing]) {
			// Counts a run of unreadable lines once, then keeps looking for the
			// next cue even without a blank line in between.
			if !inGarbage {
				skipped++
				inGarbage = true
			}
			i++
			continue
		}
		inGarbage = false

		textEnd := endOfSRTText(lines, timing+1)
		start, end, err := parseTimingLine(lines[timing])
		if err != nil {
			skipped++
		} else {
			text := strings.Join(lines[timing+1:textEnd], "\n")
			cues = append(cues, cue{start: start, end: end, text: cleanSRTText(text)})
		}
		i = textEnd
	}
	return buildResult(cues, skipped), nil
}

// endOfSRTText finds where a cue's text ends: at a blank line, or right before
// the index line of a following cue written without a separating blank line.
func endOfSRTText(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if isBlank(lines[i]) {
			return i
		}
		if isSRTTimingLine(lines[i]) {
			if i > from {
				return i - 1
			}
			return i
		}
	}
	return len(lines)
}
