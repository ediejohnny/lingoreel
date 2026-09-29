package subtitleformat

import (
	"slices"
	"testing"
	"time"

	"lingoreel/internal/domain/subtitle"
)

func ms(value int) time.Duration {
	return time.Duration(value) * time.Millisecond
}

type wantEvent struct {
	start int
	end   int
	text  string
}

func assertEvents(t *testing.T, got []subtitle.Event, want []wantEvent) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d events %v, want %d", len(got), got, len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Start != ms(w.start) || g.End != ms(w.end) || g.OriginalText != w.text {
			t.Fatalf("event %d = {%v %v %q}, want {%v %v %q}", i, g.Start, g.End, g.OriginalText, ms(w.start), ms(w.end), w.text)
		}
	}
	ids := make([]string, 0, len(got))
	for _, e := range got {
		ids = append(ids, e.ID)
	}
	slices.Sort(ids)
	if len(slices.Compact(ids)) != len(got) {
		t.Fatalf("event IDs are not unique: %v", ids)
	}
}

func mustParse(t *testing.T, parser Parser, document string) Result {
	t.Helper()
	result, err := parser.Parse([]byte(document))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return result
}
