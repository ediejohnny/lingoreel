package media

import (
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// A subtitle file's name is often wrong or missing: users rename files and
// move them around ("legenda.srt"), and a title's word can read as a code. So
// what a file says inside decides; its name only helps when the text cannot.

const (
	// soundShare is the least share of lines describing sounds ("(BIRDS
	// CHIRPING)", "[DOOR SLAMS]") that marks a subtitle made for the hearing
	// impaired; ordinary subtitles have a few.
	soundShare = 0.1
	// forcedDensity is the most lines a minute of a forced subtitle, which
	// only carries the lines spoken in another language and the signs; a
	// full subtitle has 8 to 16 a minute.
	forcedDensity = 1.5
	// minDensityLength is the shortest video whose line count tells a forced
	// subtitle from a full one.
	minDensityLength = 20 * time.Minute
)

// SubtitleLanguage is a subtitle file's language: what its text's frequent
// words say, else what its name says. A name in the same base language still
// gives the region ("pt-BR" over the text's "pt").
func SubtitleLanguage(named LanguageTag, text string) LanguageTag {
	detected := DetectLanguage(text)
	if detected == "" || named.Base() == detected.Base() {
		return named
	}
	return detected
}

// DescribesSounds reports a subtitle made for the hearing impaired by its
// lines: enough of them describe sounds.
func DescribesSounds(events []subtitle.Event) bool {
	if len(events) == 0 {
		return false
	}
	described := 0
	for _, event := range events {
		if describesSound(event.Text()) {
			described++
		}
	}
	return float64(described) >= soundShare*float64(len(events))
}

// describesSound reports a line with a sound in brackets anywhere ("[DOOR
// SLAMS]") or in parentheses at its start ("(GASPS)", "- (laughs) Yes"); a
// parenthesis in the middle of a sentence is just a sentence.
func describesSound(text string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-"))
		if strings.HasPrefix(line, "(") || strings.Contains(line, "[") {
			return true
		}
	}
	return false
}

// LooksForced reports a subtitle that carries too few lines for its video to
// be a full one: a forced subtitle, whatever its name says.
func LooksForced(events []subtitle.Event, videoLength time.Duration) bool {
	return videoLength >= minDensityLength && float64(len(events)) < forcedDensity*videoLength.Minutes()
}
