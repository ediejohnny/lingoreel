package subtitle_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// stacked is a timeline of n events with random, often overlapping times
// over a minute, the same ones for the same seed.
func stacked(t *testing.T, n int, seed uint64) *subtitle.Timeline {
	t.Helper()
	random := rand.New(rand.NewPCG(seed, seed+7))
	events := make([]subtitle.Event, 0, n)
	for i := range n {
		start := time.Duration(random.Int64N(int64(time.Minute)))
		length := time.Duration(1+random.Int64N(int64(8*time.Second))) + time.Millisecond
		events = append(events, subtitle.Event{ID: fmt.Sprint(i), Start: start, End: start + length, OriginalText: "x"})
	}
	timeline, err := subtitle.NewTimeline(events)
	if err != nil {
		t.Fatal(err)
	}
	return timeline
}

func tail(events []subtitle.Event, limit int) []subtitle.Event {
	return events[max(len(events)-limit, 0):]
}

// The latest-N queries give exactly the last N of the full ones, for any
// instant, span and limit, without looking at the rest.
func TestTheLatestQueriesKeepTheTailOfTheFullOnes(t *testing.T) {
	t.Parallel()
	for seed := range uint64(20) {
		timeline := stacked(t, 1+int(seed)*13, seed)
		random := rand.New(rand.NewPCG(seed, 99))
		for range 200 {
			at := time.Duration(random.Int64N(int64(70 * time.Second)))
			span := time.Duration(random.Int64N(int64(5 * time.Second)))
			limit := random.IntN(6)
			if got, want := timeline.LatestActiveAt(at, limit), tail(timeline.ActiveAt(at), limit); !slices.Equal(got, want) {
				t.Fatalf("seed %d: LatestActiveAt(%v, %d) = %d events, want the last %d of ActiveAt", seed, at, limit, len(got), len(want))
			}
			if got, want := timeline.LatestOverlapping(at, at+span, limit), tail(timeline.Overlapping(at, at+span), limit); !slices.Equal(got, want) {
				t.Fatalf("seed %d: LatestOverlapping(%v, +%v, %d) = %d events, want the last %d of Overlapping", seed, at, span, limit, len(got), len(want))
			}
		}
	}
}

// The next change is the earliest of the next start and the ends of what
// shows now, as a walk over every event finds it.
func TestTheNextChangeIsTheEarliestStartOrEnd(t *testing.T) {
	t.Parallel()
	for seed := range uint64(20) {
		timeline := stacked(t, 1+int(seed)*7, seed)
		events := timeline.Events()
		random := rand.New(rand.NewPCG(seed, 5))
		for range 200 {
			at := time.Duration(random.Int64N(int64(70 * time.Second)))
			var want time.Duration
			found := false
			for _, e := range events {
				for _, change := range []time.Duration{e.Start, e.End} {
					if change > at && (e.Start > at || change == e.End) && (!found || change < want) {
						want, found = change, true
					}
				}
			}
			got, ok := timeline.NextChangeAfter(at)
			if ok != found || got != want {
				t.Fatalf("seed %d: NextChangeAfter(%v) = %v, %v; want %v, %v", seed, at, got, ok, want, found)
			}
		}
	}
}
