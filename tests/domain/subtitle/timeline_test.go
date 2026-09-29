package subtitle_test

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"testing"
	"time"

	"lingoreel/internal/domain/subtitle"
)

func TestActiveAtBoundaries(t *testing.T) {
	timeline := mustTimeline(t, event("a", 10_000, 12_000), event("b", 12_000, 15_000))

	cases := []struct {
		name string
		at   int
		want []string
	}{
		{"before first event", 9_999, []string{}},
		{"exact start is active", 10_000, []string{"a"}},
		{"inside event", 11_500, []string{"a"}},
		{"end is exclusive and next starts", 12_000, []string{"b"}},
		{"last millisecond of last event", 14_999, []string{"b"}},
		{"exact end of last event", 15_000, []string{}},
		{"far after the end", 90_000, []string{}},
		{"negative position", -500, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := activeIDs(timeline.ActiveAt(ms(tc.at)))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("ActiveAt(%dms) = %v, want %v", tc.at, got, tc.want)
			}
		})
	}
}

func TestActiveAtGapBetweenEvents(t *testing.T) {
	timeline := mustTimeline(t, event("a", 1_000, 2_000), event("b", 5_000, 6_000))

	if got := timeline.ActiveAt(ms(3_000)); len(got) != 0 {
		t.Fatalf("seek into a gap returned %v, want nothing", activeIDs(got))
	}
}

func TestActiveAtOverlappingEvents(t *testing.T) {
	timeline := mustTimeline(t,
		event("sign", 0, 60_000),
		event("line1", 10_000, 12_000),
		event("line2", 11_000, 13_000),
	)

	got := activeIDs(timeline.ActiveAt(ms(11_500)))
	if want := []string{"sign", "line1", "line2"}; !slices.Equal(got, want) {
		t.Fatalf("ActiveAt with overlaps = %v, want %v", got, want)
	}
}

func TestNewTimelineSortsUnorderedInput(t *testing.T) {
	timeline := mustTimeline(t, event("c", 5_000, 6_000), event("a", 1_000, 2_000), event("b", 3_000, 4_000))

	got := activeIDs(timeline.Events())
	if want := []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("Events() order = %v, want %v", got, want)
	}
	if timeline.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", timeline.Len())
	}
}

func TestNewTimelineOrdersEqualStartsByEnd(t *testing.T) {
	timeline := mustTimeline(t, event("long", 1_000, 9_000), event("short", 1_000, 2_000))

	if got := activeIDs(timeline.Events()); !slices.Equal(got, []string{"short", "long"}) {
		t.Fatalf("Events() order = %v, want short before long", got)
	}
}

func TestNewTimelineRejectsInvalidEvents(t *testing.T) {
	cases := map[string]subtitle.Event{
		"zero duration":    event("z", 1_000, 1_000),
		"end before start": event("r", 2_000, 1_000),
		"negative start":   event("n", -1, 1_000),
		"no text to show":  {ID: "e", Start: ms(0), End: ms(1_000)},
	}
	for name, invalid := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := subtitle.NewTimeline([]subtitle.Event{event("ok", 0, 500), invalid})
			if !errors.Is(err, subtitle.ErrInvalidEvent) {
				t.Fatalf("NewTimeline error = %v, want ErrInvalidEvent", err)
			}
		})
	}
}

func TestTimelineDoesNotAliasInputOrOutput(t *testing.T) {
	input := []subtitle.Event{event("a", 0, 1_000)}
	timeline := mustTimeline(t, input...)

	input[0].OriginalText = "changed input"
	timeline.Events()[0].OriginalText = "changed output"
	if got := timeline.ActiveAt(ms(500))[0].OriginalText; got != "text a" {
		t.Fatalf("timeline changed through a shared slice: %q", got)
	}
}

func TestEmptyTimeline(t *testing.T) {
	timeline := mustTimeline(t)

	if got := timeline.ActiveAt(0); len(got) != 0 {
		t.Fatalf("empty timeline returned %v", got)
	}
	if _, ok := timeline.NextChangeAfter(0); ok {
		t.Fatal("empty timeline reported a next change")
	}
}

func TestNextChangeAfter(t *testing.T) {
	timeline := mustTimeline(t,
		event("a", 1_000, 3_000),
		event("b", 2_000, 5_000),
		event("c", 8_000, 9_000),
	)

	cases := []struct {
		at     int
		want   int
		wantOK bool
	}{
		{-5_000, 1_000, true}, // before everything
		{0, 1_000, true},      // next start
		{1_000, 2_000, true},  // b starts before a ends
		{2_500, 3_000, true},  // a ends first
		{3_000, 5_000, true},  // b ends
		{5_000, 8_000, true},  // gap until c
		{8_500, 9_000, true},  // c ends
		{9_000, 0, false},     // nothing left
	}
	for _, tc := range cases {
		got, ok := timeline.NextChangeAfter(ms(tc.at))
		if ok != tc.wantOK || (ok && got != ms(tc.want)) {
			t.Fatalf("NextChangeAfter(%dms) = %v,%v want %dms,%v", tc.at, got, ok, tc.want, tc.wantOK)
		}
	}
}

// Compares the indexed lookup against a brute-force scan on random data,
// including very long events that span most of the timeline.
func TestActiveAtMatchesLinearScan(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	events := make([]subtitle.Event, 0, 2_000)
	for i := range 2_000 {
		start := random.Intn(600_000)
		length := 1 + random.Intn(4_000)
		if i%250 == 0 {
			length = 300_000
		}
		events = append(events, event(fmt.Sprint(i), start, start+length))
	}
	timeline := mustTimeline(t, events...)
	sorted := timeline.Events()

	for range 5_000 {
		at := ms(random.Intn(700_000) - 10_000)
		want := []string{}
		for _, e := range sorted {
			if e.ActiveAt(at) {
				want = append(want, e.ID)
			}
		}
		if got := activeIDs(timeline.ActiveAt(at)); !slices.Equal(got, want) {
			t.Fatalf("ActiveAt(%v) = %v, linear scan = %v", at, got, want)
		}
	}
}

// Checks NextChangeAfter against a brute-force oracle: the earliest start or
// end strictly after the instant.
func TestNextChangeAfterMatchesLinearScan(t *testing.T) {
	random := rand.New(rand.NewSource(99))
	events := make([]subtitle.Event, 0, 500)
	for i := range 500 {
		start := random.Intn(100_000)
		events = append(events, event(fmt.Sprint(i), start, start+1+random.Intn(3_000)))
	}
	timeline := mustTimeline(t, events...)

	for range 2_000 {
		at := ms(random.Intn(110_000) - 5_000)
		var want time.Duration
		found := false
		for _, e := range events {
			for _, edge := range []time.Duration{e.Start, e.End} {
				if edge > at && (!found || edge < want) {
					want, found = edge, true
				}
			}
		}
		got, ok := timeline.NextChangeAfter(at)
		if ok != found || (ok && got != want) {
			t.Fatalf("NextChangeAfter(%v) = %v,%v; oracle %v,%v", at, got, ok, want, found)
		}
	}
}

func BenchmarkActiveAt50kEvents(b *testing.B) {
	events := make([]subtitle.Event, 0, 50_000)
	for i := range 50_000 {
		start := i * 2_500
		events = append(events, event(fmt.Sprint(i), start, start+2_000))
	}
	timeline := mustTimeline(b, events...)
	positions := make([]time.Duration, 1024)
	random := rand.New(rand.NewSource(7))
	for i := range positions {
		positions[i] = ms(random.Intn(50_000 * 2_500))
	}

	b.ResetTimer()
	for i := range b.N {
		timeline.ActiveAt(positions[i%len(positions)])
	}
}
