package subtitleformat

import (
	"testing"
	"unicode/utf8"

	"lingoreel/internal/domain/subtitle"
)

// checkInvariants holds for any input: no panic, only valid events with valid
// UTF-8 text and unique IDs, and a timeline that accepts them.
func checkInvariants(t *testing.T, parser Parser, data []byte) {
	result, err := parser.Parse(data)
	if err != nil {
		return
	}
	seen := make(map[string]bool, len(result.Events))
	for _, event := range result.Events {
		if !event.Valid() || !utf8.ValidString(event.OriginalText) || seen[event.ID] {
			t.Fatalf("invalid event %+v", event)
		}
		seen[event.ID] = true
	}
	if _, err := subtitle.NewTimeline(result.Events); err != nil {
		t.Fatalf("timeline rejected parser output: %v", err)
	}
}

func FuzzSRT(f *testing.F) {
	f.Add([]byte("1\n00:00:01,000 --> 00:00:02,000\n<i>hi</i>\n\n2\n00:00:02,500 --> 00:00:03,000\nthere\n"))
	f.Add([]byte("00:00:01.5 --> 00:00:02.0\n{\\an8}x\n3\n00:00:04,000 --> 00:00:05,000\ny"))
	f.Add([]byte{0xFF, 0xFE, '1', 0, '\n', 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		checkInvariants(t, SRTParser{}, data)
	})
}

func FuzzASS(f *testing.F) {
	f.Add([]byte(assDocument))
	f.Add([]byte("[Events]\nFormat: Start, End, Text\nDialogue: 0:00:01.00,0:00:02.00,{\\p1}m 0 0{\\p0}x{"))
	f.Fuzz(func(t *testing.T, data []byte) {
		checkInvariants(t, ASSParser{}, data)
	})
}

func FuzzWebVTT(f *testing.F) {
	f.Add([]byte("WEBVTT\n\n00:01.000 --> 00:02.000 line:0\n<v A>hi &amp; <00:01.500>bye\n"))
	f.Add([]byte("WEBVTT\n\nNOTE x\n\nid\n00:00:01.000 --> 00:00:02.000\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		checkInvariants(t, WebVTTParser{}, data)
	})
}
