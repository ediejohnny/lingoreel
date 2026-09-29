package subtitle

import (
	"errors"
	"fmt"
	"math"
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

	timeline := &Timeline{events: sorted}
	timeline.buildEndTree()
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
	started := t.countStartedBy(at)
	if started == 0 {
		return nil
	}
	return t.collectActive(1, 0, t.leaves, started, at, nil)
}

// NextChangeAfter returns the earliest instant after at when the set of active
// events changes, or false when nothing changes anymore.
func (t *Timeline) NextChangeAfter(at time.Duration) (time.Duration, bool) {
	var next time.Duration
	found := false

	if started := t.countStartedBy(at); started < len(t.events) {
		next, found = t.events[started].Start, true
	}
	for _, event := range t.ActiveAt(at) {
		if !found || event.End < next {
			next, found = event.End, true
		}
	}
	return next, found
}

// countStartedBy returns how many events start at or before the instant.
func (t *Timeline) countStartedBy(at time.Duration) int {
	return sort.Search(len(t.events), func(i int) bool {
		return t.events[i].Start > at
	})
}

// collectActive walks the segment tree over events[0:started), skipping every
// node whose latest End is not after the query instant.
func (t *Timeline) collectActive(node, low, high, started int, at time.Duration, found []Event) []Event {
	if low >= started || t.endTree[node] <= at {
		return found
	}
	if high-low == 1 {
		return append(found, t.events[low])
	}
	middle := (low + high) / 2
	found = t.collectActive(2*node, low, middle, started, at, found)
	return t.collectActive(2*node+1, middle, high, started, at, found)
}
