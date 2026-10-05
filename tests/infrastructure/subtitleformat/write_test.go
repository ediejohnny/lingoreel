package subtitleformat_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitleformat"
)

func at(h, m, s, millis int) time.Duration {
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second + time.Duration(millis)*time.Millisecond
}

// exportSample has what an export meets: an edit, two rows, accents, a
// film past 99 minutes, events out of order and events to leave out.
func exportSample() []subtitle.Event {
	return []subtitle.Event{
		{ID: "3", Start: at(1, 45, 2, 7), End: at(1, 45, 4, 999), OriginalText: "Late in the film."},
		{ID: "1", Start: at(0, 0, 1, 120), End: at(0, 0, 3, 800), OriginalText: "I am going to...", OverrideText: "I'm gonna..."},
		{ID: "2", Start: at(0, 10, 41, 0), End: at(0, 10, 43, 500), OriginalText: "Isso está doente!\nQue coração ♥"},
		{ID: "4", Start: at(0, 20, 0, 0), End: at(0, 20, 0, 0), OriginalText: "No time at all."},
		{ID: "5", Start: at(0, 30, 0, 0), End: at(0, 30, 1, 0), OriginalText: " \n "},
	}
}

func parse(t *testing.T, format subtitle.Format, data []byte) []subtitle.Event {
	t.Helper()
	parser, err := subtitleformat.NewParser(format)
	if err != nil {
		t.Fatal(err)
	}
	result, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("the exported %s does not read back: %v\n%s", format, err, data)
	}
	return result.Events
}

func TestSRTReadsBackAsItWasWritten(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := subtitleformat.WriteSRT(&out, exportSample()); err != nil {
		t.Fatal(err)
	}
	data := out.Bytes()
	if !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) || !bytes.Contains(data, []byte("\r\n")) || bytes.Contains(bytes.ReplaceAll(data, []byte("\r\n"), nil), []byte("\n")) {
		t.Fatal("not UTF-8 with BOM and CRLF only")
	}
	// The parser is the oracle: the same lines, in time order, with the
	// edited text and every millisecond.
	want := []struct {
		start, end time.Duration
		text       string
	}{
		{at(0, 0, 1, 120), at(0, 0, 3, 800), "I'm gonna..."},
		{at(0, 10, 41, 0), at(0, 10, 43, 500), "Isso está doente!\nQue coração ♥"},
		{at(1, 45, 2, 7), at(1, 45, 4, 999), "Late in the film."},
	}
	got := parse(t, subtitle.FormatSRT, data)
	if len(got) != len(want) {
		t.Fatalf("read back %d lines, want %d:\n%s", len(got), len(want), data)
	}
	for i, w := range want {
		if got[i].Start != w.start || got[i].End != w.end || got[i].Text() != w.text {
			t.Errorf("line %d = %v–%v %q, want %v–%v %q", i+1, got[i].Start, got[i].End, got[i].Text(), w.start, w.end, w.text)
		}
	}
	if !strings.Contains(string(data), "01:45:02,007 --> 01:45:04,999") {
		t.Fatalf("times past an hour:\n%s", data)
	}
	// The parser drops a line without time or text; the file must not have it.
	if cues := strings.Count(string(data), "-->"); cues != len(want) {
		t.Fatalf("%d cues written, want %d (no blank or timeless line):\n%s", cues, len(want), data)
	}
}

func TestASSReadsBackToTheCentisecond(t *testing.T) {
	t.Parallel()
	events := []subtitle.Event{
		{ID: "1", Start: at(0, 0, 59, 996), End: at(0, 1, 2, 5), OriginalText: "Round up: 59.996 is 1:00.00."},
		{ID: "2", Start: at(2, 0, 0, 4), End: at(2, 0, 1, 0), OriginalText: "Two rows,\nwith, commas."},
		{ID: "3", Start: at(0, 5, 0, 0), End: at(0, 5, 1, 0), OriginalText: `{\i1}Not a tag{\i0} \N`},
	}
	var out bytes.Buffer
	if err := subtitleformat.WriteASS(&out, events); err != nil {
		t.Fatal(err)
	}
	data := out.String()
	// 59.996 s rounds up across the minute; 2:00:00.004 rounds down.
	for _, part := range []string{"Dialogue: 0,0:01:00.00,0:01:02.01,", "Dialogue: 0,2:00:00.00,2:00:01.00,", `Two rows,\Nwith, commas.`} {
		if !strings.Contains(data, part) {
			t.Errorf("missing %q in:\n%s", part, data)
		}
	}
	if strings.Contains(data, "0:00:59.99") {
		t.Errorf("59.996 s was cut instead of rounded:\n%s", data)
	}
	got := parse(t, subtitle.FormatASS, out.Bytes())
	if len(got) != 3 {
		t.Fatalf("read back %d lines:\n%s", len(got), data)
	}
	if got[0].Start != at(0, 1, 0, 0) || got[0].End != at(0, 1, 2, 10) {
		t.Errorf("rounded times = %v–%v", got[0].Start, got[0].End)
	}
	if got[2].Text() != "Two rows,\nwith, commas." {
		t.Errorf("rows = %q", got[2].Text())
	}
	// Braces and backslashes would be commands in ASS: they stay readable.
	if got[1].Text() != "｛＼i1｝Not a tag｛＼i0｝ ＼N" {
		t.Errorf("commands in the text = %q", got[1].Text())
	}
}

func TestNothingToExport(t *testing.T) {
	t.Parallel()
	for name, write := range map[string]func(*bytes.Buffer, []subtitle.Event) error{
		"srt": func(b *bytes.Buffer, e []subtitle.Event) error { return subtitleformat.WriteSRT(b, e) },
		"ass": func(b *bytes.Buffer, e []subtitle.Event) error { return subtitleformat.WriteASS(b, e) },
	} {
		var out bytes.Buffer
		if err := write(&out, nil); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Contains(out.String(), "-->") || strings.Contains(out.String(), "Dialogue:") {
			t.Fatalf("%s: lines written for no events", name)
		}
	}
}
