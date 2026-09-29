// Package subtitle models timed subtitle events and the timeline that answers
// which events are visible at a given playback position.
package subtitle

import "time"

// Event is one timed subtitle line. OriginalText is never modified; user edits
// live in OverrideText so the original can always be restored.
type Event struct {
	ID           string
	Start        time.Duration
	End          time.Duration
	OriginalText string
	OverrideText string
}

// Text returns the text that should be displayed.
func (e Event) Text() string {
	if e.OverrideText != "" {
		return e.OverrideText
	}
	return e.OriginalText
}

// Valid reports whether the event has a non-negative start, a positive
// duration and something to display.
func (e Event) Valid() bool {
	return e.Start >= 0 && e.End > e.Start && e.Text() != ""
}

// ActiveAt reports whether the event is visible at t. End is exclusive, so two
// back-to-back events never show together.
func (e Event) ActiveAt(t time.Duration) bool {
	return e.Start <= t && t < e.End
}
