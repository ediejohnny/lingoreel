package subtitle_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func ms(value int) time.Duration {
	return time.Duration(value) * time.Millisecond
}

func event(id string, start, end int) subtitle.Event {
	return subtitle.Event{ID: id, Start: ms(start), End: ms(end), OriginalText: "text " + id}
}

func activeIDs(events []subtitle.Event) []string {
	ids := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	return ids
}

func mustTimeline(t testing.TB, events ...subtitle.Event) *subtitle.Timeline {
	t.Helper()
	timeline, err := subtitle.NewTimeline(events)
	if err != nil {
		t.Fatalf("NewTimeline: %v", err)
	}
	return timeline
}
