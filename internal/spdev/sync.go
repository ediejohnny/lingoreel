package spdev

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/subtitle"
)

// languageNames are the languages named in the notices.
var languageNames = map[string]string{
	"en": "inglês", "pt": "português", "es": "espanhol", "fr": "francês",
	"de": "alemão", "it": "italiano", "ja": "japonês", "ko": "coreano", "zh": "chinês",
}

// lineNamesPair describes each line by its place on screen and language.
type lineNamesPair [2]string

// lineNames names the lines as in "Legenda de cima (inglês)".
func lineNames(info media.Info, primary, translation uint64) lineNamesPair {
	name := func(place string, number uint64) string {
		track, found := info.Track(number)
		if !found || track.Language == "" {
			return place
		}
		language, known := languageNames[track.Language.Base()]
		if !known {
			language = string(track.Language)
		}
		return fmt.Sprintf("%s (%s)", place, language)
	}
	return lineNamesPair{name("Legenda de cima", primary), name("Legenda de baixo", translation)}
}

// notice words a sync change: "Legenda de cima (inglês): −0,150 s".
func (n lineNamesPair) notice(line session.Line, offset subtitle.Offset) string {
	if line != session.LinePrimary && line != session.LineTranslation {
		return formatOffset(offset)
	}
	return fmt.Sprintf("%s: %s", n[line], formatOffset(offset))
}

// formatOffset shows an offset as "+0,150 s" or "−0,150 s".
func formatOffset(offset subtitle.Offset) string {
	text := strings.Replace(fmt.Sprintf("%+.3f s", offset.Duration().Seconds()), ".", ",", 1)
	return strings.Replace(text, "-", "−", 1)
}

// resetCommand on the console puts both lines back at their files' timing.
const resetCommand = "z"

// readConsole calls run with each line typed on the console, trimmed and in
// lower case, until the context ends or the input closes. A pending read of
// the console cannot be interrupted: the goroutine ends with the process.
func readConsole(ctx context.Context, in io.Reader, run func(string)) {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		run(strings.ToLower(strings.TrimSpace(scanner.Text())))
	}
}
