package subtitleformat_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitleformat"
)

// The whole way a subtitle file takes, read, ordered and looked up, at 50 000
// lines. It takes about a second without the race detector; a quadratic step
// would take minutes.
func BenchmarkFiftyThousandLinesFromAFile(b *testing.B) {
	const lines = 50_000
	var file bytes.Buffer
	for i := range lines {
		start := time.Duration(i) * 2 * time.Second
		fmt.Fprintf(&file, "%d\r\n%s --> %s\r\nLine number %d, with some words.\r\n\r\n", i+1, srt(start), srt(start+1500*time.Millisecond), i)
	}
	parser, err := subtitleformat.NewParser(subtitle.FormatSRT)
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		result, err := parser.Parse(file.Bytes())
		if err != nil || len(result.Events) != lines {
			b.Fatalf("Parse = %d lines, %v", len(result.Events), err)
		}
		timeline, err := subtitle.NewTimeline(result.Events)
		if err != nil {
			b.Fatal(err)
		}
		for i := 0; i < lines; i += 7 {
			timeline.ActiveAt(time.Duration(i)*2*time.Second + time.Second)
		}
	}
}

func srt(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d:%02d,%03d", d/time.Hour, d%time.Hour/time.Minute, d%time.Minute/time.Second, d%time.Second/time.Millisecond)
}
