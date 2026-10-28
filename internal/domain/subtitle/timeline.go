package subtitle

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"
)

// ErrInvalidEvent reports an event that cannot be placed on a timeline.
var ErrInvalidEvent = errors.New("invalid subtitle event")

// emptyLeafEnd marks unused segment tree leaves so they never match a query.
const emptyLeafEnd = time.Duration(math.MinInt64)

// Timeline answers which events are active at a position in O(log n + k),
// overlapping events included. It is immutable after creation.
type Timeline struct {
	// events are sorted by Start, then End.
	events []Event
	// endTree is a segment tree over events holding the maximum End of each
	// node, which prunes every subtree that finished before the query time.
	endTree []time.Duration
	leaves  int
	// latestEnd[i] is the latest End among events[0..i], so the end of the
	// speech before any instant is one search away.
	latestEnd []time.Duration
}

// NewTimeline sorts a copy of events and indexes them. Every event must be
// valid; parsers are expected to drop malformed cues before this point.
func NewTimeline(events []Event) (*Timeline, error) {
	sorted := make([]Event, len(events))
	copy(sorted, events)
	for i, event := range sorted {
		if !event.Valid() {
			return nil, fmt.Errorf("%w: position %d (id %q)", ErrInvalidEvent, i, event.ID)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Start != sorted[j].Start {
			return sorted[i].Start < sorted[j].Start
		}
		return sorted[i].End < sorted[j].End
	})

	timeline := &Timeline{events: sorted, latestEnd: make([]time.Duration, len(sorted))}
	timeline.buildEndTree()
	for i, event := range sorted {
		timeline.latestEnd[i] = event.End
		if i > 0 {
			timeline.latestEnd[i] = max(event.End, timeline.latestEnd[i-1])
		}
	}
	return timeline, nil
}

func (t *Timeline) buildEndTree() {
	t.leaves = 1
	for t.leaves < len(t.events) {
		t.leaves *= 2
	}
	t.endTree = make([]time.Duration, 2*t.leaves)
	for i := range t.leaves {
		t.endTree[t.leaves+i] = emptyLeafEnd
		if i < len(t.events) {
			t.endTree[t.leaves+i] = t.events[i].End
		}
	}
	for node := t.leaves - 1; node >= 1; node-- {
		t.endTree[node] = max(t.endTree[2*node], t.endTree[2*node+1])
	}
}

// Len returns the number of events.
func (t *Timeline) Len() int {
	return len(t.events)
}

// Events returns a copy of the events in timeline order.
func (t *Timeline) Events() []Event {
	events := make([]Event, len(t.events))
	copy(events, t.events)
	return events
}

// ActiveAt returns the events visible at the given instant, ordered by Start.
func (t *Timeline) ActiveAt(at time.Duration) []Event {
	return t.collect(t.countStartedBy(at), at, -1)
}

// LatestActiveAt is ActiveAt kept to the last limit events, which is all it
// looks at: a file stacking thousands of events costs no more.
func (t *Timeline) LatestActiveAt(at time.Duration, limit int) []Event {
	return t.collect(t.countStartedBy(at), at, max(limit, 0))
}

// Overlapping returns the events showing at some instant of [from, to),
// ordered by Start; nothing for an empty span.
func (t *Timeline) Overlapping(from, to time.Duration) []Event {
	return t.LatestOverlapping(from, to, -1)
}

// LatestOverlapping is Overlapping kept to the last limit events, which is
// all it looks at; a negative limit keeps them all.
func (t *Timeline) LatestOverlapping(from, to time.Duration, limit int) []Event {
	if to <= from {
		return nil
	}
	started := sort.Search(len(t.events), func(i int) bool {
		return t.events[i].Start >= to
	})
	return t.collect(started, from, limit)
}

// collect lists the events among the first started still showing after at,
// ordered by Start: every one with a negative limit, else the last limit.
func (t *Timeline) collect(started int, at time.Duration, limit int) []Event {
	if started == 0 || limit == 0 {
		return nil
	}
	var found []Event
	if limit < 0 {
		t.visitActive(1, 0, t.leaves, started, at, false, func(i int) bool {
			found = append(found, t.events[i])
			return true
		})
		return found
	}
	t.visitActive(1, 0, t.leaves, started, at, true, func(i int) bool {
		found = append(found, t.events[i])
		return len(found) < limit
	})
	slices.Reverse(found)
	return found
}

// NextChangeAfter returns the earliest instant after at when the set of active
// events changes, or false when nothing changes anymore.
func (t *Timeline) NextChangeAfter(at time.Duration) (time.Duration, bool) {
	var next time.Duration
	found := false

	if started := t.countStartedBy(at); started < len(t.events) {
		next, found = t.events[started].Start, true
	}
	if started := t.countStartedBy(at); started > 0 {
		t.visitActive(1, 0, t.leaves, started, at, false, func(i int) bool {
			if end := t.events[i].End; !found || end < next {
				next, found = end, true
			}
			return true
		})
	}
	return next, found
}

// NextStartAfter returns the first event starting strictly after the
// instant; with several starting together, the one ending first.
func (t *Timeline) NextStartAfter(at time.Duration) (Event, bool) {
	if started := t.countStartedBy(at); started < len(t.events) {
		return t.events[started], true
	}
	return Event{}, false
}

// LastStartedBy returns the latest event starting at or before the instant.
func (t *Timeline) LastStartedBy(at time.Duration) (Event, bool) {
	if started := t.countStartedBy(at); started > 0 {
		return t.events[started-1], true
	}
	return Event{}, false
}

// LatestEndBefore returns when the speech before an instant ends: the
// latest End among the events starting before it, or zero when none does.
// The silence before a line is its Start minus this.
func (t *Timeline) LatestEndBefore(start time.Duration) time.Duration {
	before := sort.Search(len(t.events), func(i int) bool {
		return t.events[i].Start >= start
	})
	if before == 0 {
		return 0
	}
	return t.latestEnd[before-1]
}

// countStartedBy returns how many events start at or before the instant.
func (t *Timeline) countStartedBy(at time.Duration) int {
	return sort.Search(len(t.events), func(i int) bool {
		return t.events[i].Start > at
	})
}

// visitActive walks the segment tree over events[0:started), skipping every
// node whose latest End is not after the query instant, and hands each
// event left to visit, in Start order or, backwards, from the latest; it
// stops, telling false, once visit does.
func (t *Timeline) visitActive(node, low, high, started int, at time.Duration, backwards bool, visit func(int) bool) bool {
	if low >= started || t.endTree[node] <= at {
		return true
	}
	if high-low == 1 {
		return visit(low)
	}
	middle := (low + high) / 2
	first, second := [3]int{2 * node, low, middle}, [3]int{2*node + 1, middle, high}
	if backwards {
		first, second = second, first
	}
	return t.visitActive(first[0], first[1], first[2], started, at, backwards, visit) &&
		t.visitActive(second[0], second[1], second[2], started, at, backwards, visit)
}
