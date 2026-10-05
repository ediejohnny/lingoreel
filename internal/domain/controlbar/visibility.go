package controlbar

import "time"

// Visibility decides when the bar shows: while the mouse is in the bottom
// zone or over the bar, while the user drags the track or the volume, while
// the speed list is open, and for Delay after that. It keeps no clock: the
// caller passes the time.
type Visibility struct {
	// Delay is how long the bar stays after the mouse leaves its zone; zero
	// hides it at once.
	Delay      time.Duration
	shown      bool
	lastActive time.Time
}

// Update records whether the bar is wanted now (active) and returns whether
// it shows, and when to ask again if it may hide later; zero means no need.
func (v *Visibility) Update(now time.Time, active bool) (shown bool, recheck time.Duration) {
	if active {
		v.shown, v.lastActive = true, now
		return true, 0
	}
	if !v.shown {
		return false, 0
	}
	if left := v.Delay - now.Sub(v.lastActive); left > 0 {
		return true, left
	}
	v.shown = false
	return false, 0
}

// Hide takes the bar away at once, as when the mouse leaves the window
// during fullscreen or the window closes.
func (v *Visibility) Hide() {
	v.shown = false
}

// Shown reports whether the bar is showing.
func (v *Visibility) Shown() bool {
	return v.shown
}
