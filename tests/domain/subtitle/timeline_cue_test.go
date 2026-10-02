package subtitle_test

import (
	"testing"
	"time"

	"lingoreel/internal/domain/subtitle"
)

func TestTimelineNeighbors(t *testing.T) {
	sec := func(v float64) time.Duration { return time.Duration(v * float64(time.Second)) }
	ev := func(id string, start, end float64) subtitle.Event {
		return subtitle.Event{ID: id, Start: sec(start), End: sec(end), OriginalText: id}
	}
	// b and c start together; c ends first, so it comes first.
	timeline, err := subtitle.NewTimeline([]subtitle.Event{ev("a", 2, 4), ev("b", 10, 20), ev("c", 10, 12), ev("d", 15, 16)})
	if err != nil {
		t.Fatal(err)
	}
	if next, ok := timeline.NextStartAfter(sec(4)); !ok || next.ID != "c" {
		t.Fatalf("NextStartAfter(4) = %v %v, want c", next.ID, ok)
	}
	if next, ok := timeline.NextStartAfter(sec(10)); !ok || next.ID != "d" {
		t.Fatalf("NextStartAfter(10) = %v %v, want d (strictly after)", next.ID, ok)
	}
	if _, ok := timeline.NextStartAfter(sec(15)); ok {
		t.Fatal("NextStartAfter past the last start")
	}
	if last, ok := timeline.LastStartedBy(sec(10)); !ok || last.ID != "b" {
		t.Fatalf("LastStartedBy(10) = %v %v, want b", last.ID, ok)
	}
	if _, ok := timeline.LastStartedBy(sec(1)); ok {
		t.Fatal("LastStartedBy before the first line")
	}
	// The speech before 15 s ends at 20 (b); before 10 s at 4 (a only).
	for at, want := range map[float64]float64{15: 20, 10: 4, 2: 0, 1: 0, 100: 20} {
		if got := timeline.LatestEndBefore(sec(at)); got != sec(want) {
			t.Errorf("LatestEndBefore(%v) = %v, want %v", at, got, sec(want))
		}
	}
	empty, _ := subtitle.NewTimeline(nil)
	if _, ok := empty.NextStartAfter(0); ok || empty.LatestEndBefore(sec(5)) != 0 {
		t.Fatal("an empty timeline has neighbors")
	}
}
