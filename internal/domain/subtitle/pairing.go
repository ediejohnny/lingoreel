package subtitle

import (
	"sort"
	"time"
)

// Placed is a timeline with the offset that puts it on the playback clock.
type Placed struct {
	Timeline *Timeline
	Offset   Offset
}

// span is when an event shows on the playback clock.
func (p Placed) span(event Event) (time.Duration, time.Duration) {
	return p.Offset.PlaybackTime(event.Start), p.Offset.PlaybackTime(event.End)
}

// overlapping lists the last limit events showing at some playback instant
// of [from, to).
func (p Placed) overlapping(from, to time.Duration, limit int) []Event {
	if p.Timeline == nil {
		return nil
	}
	return p.Timeline.LatestOverlapping(p.Offset.TrackTime(from), p.Offset.TrackTime(to), max(limit, 0))
}

// Paired is the translation to show with the study events shown at a playback
// position, so both lines come and go together: each translation event that
// says what a study event shown says (they overlap by at least half of the
// shorter one), for as long as that study event shows; and, on its own time,
// a translation event no study event says (a sign, a song), which has no
// study line to wait for. The result is ordered by Start, and every list
// looked at keeps its latest limit events: a hostile file stacking thousands
// of events must not stall a frame.
func Paired(shown []Event, primary, translation Placed, position time.Duration, limit int) []Event {
	if translation.Timeline == nil {
		return nil
	}
	var found []Event
	if len(shown) > 0 {
		from, to := primary.span(shown[0])
		for _, event := range shown[1:] {
			start, end := primary.span(event)
			from, to = min(from, start), max(to, end)
		}
		for _, event := range translation.overlapping(from, to, limit) {
			if says(translation, event, primary, shown) {
				found = append(found, event)
			}
		}
	}
	for _, event := range translation.Timeline.LatestActiveAt(translation.Offset.TrackTime(position), limit) {
		from, to := translation.span(event)
		if !contains(found, event) && !says(translation, event, primary, primary.overlapping(from, to, limit)) {
			found = append(found, event)
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Start < found[j].Start })
	return latest(found, limit)
}

// latest keeps the last limit events of a list ordered by Start.
func latest(events []Event, limit int) []Event {
	return events[max(len(events)-max(limit, 0), 0):]
}

// says tells a translation event says what one of the study events says.
func says(translation Placed, event Event, primary Placed, study []Event) bool {
	start, end := translation.span(event)
	for _, line := range study {
		lineStart, lineEnd := primary.span(line)
		overlap := min(end, lineEnd) - max(start, lineStart)
		if overlap > 0 && 2*overlap >= min(end-start, lineEnd-lineStart) {
			return true
		}
	}
	return false
}

func contains(events []Event, event Event) bool {
	for _, candidate := range events {
		if candidate.Start == event.Start && candidate.End == event.End && candidate.ID == event.ID {
			return true
		}
	}
	return false
}
