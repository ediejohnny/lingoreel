package subtitleformat

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// exportMode is an exported file's permission, as any program saves the
// user's files.
const exportMode = 0o644

// WriteFile writes the events to a file the user picked: Advanced
// SubStation for a name ending in .ass, in any case, else SubRip. A name
// that is not a subtitle's gets ".srt" added, so a video picked by mistake
// is never written over. It returns the path written.
func WriteFile(path string, events []subtitle.Event) (string, error) {
	write := WriteSRT
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ass":
		write = WriteASS
	case ".srt":
	default:
		path += ".srt"
	}
	var body bytes.Buffer
	if err := write(&body, events); err != nil {
		return path, fmt.Errorf("writing %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, body.Bytes(), exportMode); err != nil {
		return path, fmt.Errorf("saving the subtitle: %w", err)
	}
	return path, nil
}

// The exported files start with utf8BOM: older Windows players need it to
// read accents as UTF-8.

// newline ends every line of an exported file, as Windows tools expect.
const newline = "\r\n"

// WriteSRT writes the events as SubRip, in time order and numbered from 1,
// with their displayed text (the user's edit when there is one). Events that
// are not valid are left out.
func WriteSRT(w io.Writer, events []subtitle.Event) error {
	out := bufio.NewWriter(w)
	out.Write(utf8BOM)
	for i, event := range exportable(events) {
		fmt.Fprintf(out, "%d%s%s --> %s%s", i+1, newline, srtTime(event.Start), srtTime(event.End), newline)
		for _, row := range textRows(event.Text()) {
			out.WriteString(row + newline)
		}
		out.WriteString(newline)
	}
	return out.Flush()
}

// assHeader is the script around the exported lines: one plain style, since
// the overlay's own look is not part of the subtitle.
const assHeader = "[Script Info]" + newline +
	"; Exportado pelo LingoReel" + newline +
	"ScriptType: v4.00+" + newline +
	"WrapStyle: 0" + newline +
	"ScaledBorderAndShadow: yes" + newline +
	"PlayResX: 1920" + newline +
	"PlayResY: 1080" + newline + newline +
	"[V4+ Styles]" + newline +
	"Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, " +
	"Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, " +
	"MarginV, Encoding" + newline +
	"Style: Default,Arial,64,&H00FFFFFF,&H000000FF,&H00000000,&H64000000,0,0,0,0,100,100,0,0,1,3,1,2,40,40,60,1" + newline + newline +
	"[Events]" + newline +
	"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text" + newline

// assSafe keeps ASS commands out of the text: braces open style overrides
// and a backslash starts a break, so they become their full-width forms.
var assSafe = strings.NewReplacer("{", "｛", "}", "｝", `\`, "＼")

// WriteASS writes the events as an Advanced SubStation Alpha script, in
// time order. ASS keeps centiseconds, so times round to the nearest 10 ms.
func WriteASS(w io.Writer, events []subtitle.Event) error {
	out := bufio.NewWriter(w)
	out.Write(utf8BOM)
	out.WriteString(assHeader)
	for _, event := range exportable(events) {
		rows := textRows(event.Text())
		for i := range rows {
			rows[i] = assSafe.Replace(rows[i])
		}
		fmt.Fprintf(out, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s%s", assTime(event.Start), assTime(event.End), strings.Join(rows, `\N`), newline)
	}
	return out.Flush()
}

// exportable is the valid events with something to show, in time order.
func exportable(events []subtitle.Event) []subtitle.Event {
	valid := make([]subtitle.Event, 0, len(events))
	for _, event := range events {
		if event.Valid() && len(textRows(event.Text())) > 0 {
			valid = append(valid, event)
		}
	}
	slices.SortStableFunc(valid, func(a, b subtitle.Event) int {
		return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End))
	})
	return valid
}

// textRows splits a text into its non-blank rows: a blank row would end an
// SRT cue early.
func textRows(text string) []string {
	var rows []string
	for row := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if row = strings.TrimSpace(row); row != "" {
			rows = append(rows, row)
		}
	}
	return rows
}

func srtTime(d time.Duration) string {
	d = d.Round(time.Millisecond)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", d/time.Hour, d%time.Hour/time.Minute, d%time.Minute/time.Second, d%time.Second/time.Millisecond)
}

func assTime(d time.Duration) string {
	centiseconds := (d + 5*time.Millisecond) / (10 * time.Millisecond)
	return fmt.Sprintf("%d:%02d:%02d.%02d", centiseconds/360_000, centiseconds%360_000/6000, centiseconds%6000/100, centiseconds%100)
}
