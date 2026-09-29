package subtitleformat

import (
	"fmt"
	"strings"
)

// WebVTTParser reads WebVTT (.vtt) documents. NOTE, STYLE and REGION blocks are
// ignored, as are cue settings and inline markup.
type WebVTTParser struct{}

const webVTTSignature = "WEBVTT"

func isWebVTTSignature(line string) bool {
	if !strings.HasPrefix(line, webVTTSignature) {
		return false
	}
	rest := line[len(webVTTSignature):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

func isWebVTTMetadataBlock(firstLine string) bool {
	for _, keyword := range []string{"NOTE", "STYLE", "REGION"} {
		if firstLine == keyword || strings.HasPrefix(firstLine, keyword+" ") || strings.HasPrefix(firstLine, keyword+"\t") {
			return true
		}
	}
	return false
}

// Parse implements Parser.
func (WebVTTParser) Parse(data []byte) (Result, error) {
	if err := checkInputSize(data); err != nil {
		return Result{}, err
	}
	lines := splitLines(DecodeText(data))
	if len(lines) == 0 || !isWebVTTSignature(strings.TrimSpace(lines[0])) {
		return Result{}, fmt.Errorf("%w: missing %s header", ErrMalformed, webVTTSignature)
	}

	var cues []cue
	skipped := 0
	for i := endOfBlock(lines, 1); i < len(lines); {
		if isBlank(lines[i]) {
			i++
			continue
		}
		blockEnd := endOfBlock(lines, i)
		block := lines[i:blockEnd]
		i = blockEnd

		if isWebVTTMetadataBlock(strings.TrimSpace(block[0])) {
			continue
		}
		timing := 0
		if !strings.Contains(block[0], "-->") {
			timing = 1
		}
		if timing >= len(block) || !strings.Contains(block[timing], "-->") {
			skipped++
			continue
		}
		start, end, err := parseTimingLine(block[timing])
		if err != nil {
			skipped++
			continue
		}
		cues = append(cues, cue{start: start, end: end, text: cleanVTTText(strings.Join(block[timing+1:], "\n"))})
	}
	return buildResult(cues, skipped), nil
}
