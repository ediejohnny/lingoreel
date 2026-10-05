// Package subtitleformat reads text subtitle formats (SRT, ASS/SSA, WebVTT)
// into subtitle events. Every input is treated as untrusted data.
package subtitleformat

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// MaxInputBytes bounds the size of a subtitle document accepted by the parsers.
const MaxInputBytes = 32 << 20

// sniffBytes is how much of a document DetectFormat inspects.
const sniffBytes = 4096

var (
	ErrUnsupportedFormat = errors.New("unsupported subtitle format")
	ErrInputTooLarge     = errors.New("subtitle input too large")
	ErrMalformed         = errors.New("malformed subtitle document")
)

// Result holds the parsed events in document order and how many cues were
// dropped for being malformed, empty or duplicated.
type Result struct {
	Events  []subtitle.Event
	Skipped int
}

// Parser reads one subtitle format.
type Parser interface {
	Parse(data []byte) (Result, error)
}

// NewParser returns the parser for a format.
func NewParser(format subtitle.Format) (Parser, error) {
	switch format {
	case subtitle.FormatSRT:
		return SRTParser{}, nil
	case subtitle.FormatASS, subtitle.FormatSSA:
		return ASSParser{}, nil
	case subtitle.FormatWebVTT:
		return WebVTTParser{}, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnsupportedFormat, format)
}

var srtTimingSniffPattern = regexp.MustCompile(`\d+:\d{1,2}:\d{1,2}[,.]\d+\s*-->`)

// DetectFormat identifies the format from the file extension and falls back to
// the content when the extension is unknown.
func DetectFormat(fileName string, data []byte) (subtitle.Format, bool) {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".srt":
		return subtitle.FormatSRT, true
	case ".ass":
		return subtitle.FormatASS, true
	case ".ssa":
		return subtitle.FormatSSA, true
	case ".vtt":
		return subtitle.FormatWebVTT, true
	}

	head := DecodeText(data[:min(len(data), sniffBytes)])
	switch {
	case isWebVTTSignature(strings.TrimSpace(firstLine(head))):
		return subtitle.FormatWebVTT, true
	case strings.Contains(head, "[Script Info]"), strings.Contains(head, "[Events]"):
		return subtitle.FormatASS, true
	case srtTimingSniffPattern.MatchString(head):
		return subtitle.FormatSRT, true
	}
	return "", false
}

func checkInputSize(data []byte) error {
	if len(data) > MaxInputBytes {
		return fmt.Errorf("%w: %d bytes (limit %d)", ErrInputTooLarge, len(data), MaxInputBytes)
	}
	return nil
}

// cue is a parsed entry before normalization.
type cue struct {
	start time.Duration
	end   time.Duration
	text  string
}

// buildResult normalizes cue text, drops invalid or duplicated cues and numbers
// the remaining events in document order.
func buildResult(cues []cue, skipped int) Result {
	events := make([]subtitle.Event, 0, len(cues))
	seen := make(map[cue]struct{}, len(cues))
	for _, c := range cues {
		c.text = subtitle.NormalizeText(c.text)
		event := subtitle.Event{Start: c.start, End: c.end, OriginalText: c.text}
		if !event.Valid() {
			skipped++
			continue
		}
		if _, duplicated := seen[c]; duplicated {
			skipped++
			continue
		}
		seen[c] = struct{}{}
		event.ID = strconv.Itoa(len(events) + 1)
		events = append(events, event)
	}
	return Result{Events: events, Skipped: skipped}
}

// parseTimingLine reads "start --> end [settings]" as used by SRT and WebVTT.
func parseTimingLine(line string) (time.Duration, time.Duration, error) {
	left, right, found := strings.Cut(line, "-->")
	if !found {
		return 0, 0, fmt.Errorf("%w: missing arrow in %q", ErrMalformed, line)
	}
	fields := strings.Fields(right)
	if len(fields) == 0 {
		return 0, 0, fmt.Errorf("%w: missing end time in %q", ErrMalformed, line)
	}
	start, err := ParseTimestamp(left)
	if err != nil {
		return 0, 0, err
	}
	end, err := ParseTimestamp(fields[0])
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}
