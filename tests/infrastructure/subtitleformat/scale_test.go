package subtitleformat_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/infrastructure/subtitleformat"
)

// PRD §58: 50 000 lines with no noticeable cost. The whole way a subtitle
// file takes, read, ordered and looked up, stays far below a quadratic cost
// (minutes at this size); the ceiling is generous for a loaded machine and
// the race detector.
func TestFiftyThousandLinesFromAFile(t *testing.T) {
	const lines = 50_000
	var file bytes.Buffer
	for i := range lines {
		start := time.Duration(i) * 2 * time.Second
		fmt.Fprintf(&file, "%d\r\n%s --> %s\r\nLine number %d, with some words.\r\n\r\n", i+1, srt(start), srt(start+1500*time.Millisecond), i)
	}
	began := time.Now()
	parser, err := subtitleformat.NewParser(subtitle.FormatSRT)
	if err != nil {
		t.Fatal(err)
	}
	result, err := parser.Parse(file.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := subtitle.NewTimeline(result.Events)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < lines; i += 7 {
		at := time.Duration(i)*2*time.Second + time.Second
		if active := timeline.ActiveAt(at); len(active) != 1 || active[0].Text() != fmt.Sprintf("Line number %d, with some words.", i) {
			t.Fatalf("at %v: %+v", at, active)
		}
	}
	elapsed := time.Since(began)
	t.Logf("%d lines (%d KB) read, ordered and looked up in %v", lines, file.Len()/1024, elapsed)
	if len(result.Events) != lines || elapsed > 10*time.Second {
		t.Fatalf("%d lines in %v", len(result.Events), elapsed)
	}
}

func srt(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d:%02d,%03d", d/time.Hour, d%time.Hour/time.Minute, d%time.Minute/time.Second, d%time.Second/time.Millisecond)
}
