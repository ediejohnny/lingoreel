package subtitle

import "time"

// MaxOffset bounds how far a track can be shifted in either direction.
const MaxOffset = 10 * time.Minute

// Offset shifts a subtitle track in time. Positive values show the track later;
// a translation that appears 500 ms late is fixed with -500 ms.
type Offset time.Duration

// NewOffset returns the offset clamped to ±MaxOffset.
func NewOffset(shift time.Duration) Offset {
	return Offset(min(max(shift, -MaxOffset), MaxOffset))
}

// Duration returns the offset as a duration.
func (o Offset) Duration() time.Duration {
	return time.Duration(o)
}

// Shift returns the offset moved by delta, clamped to ±MaxOffset.
func (o Offset) Shift(delta time.Duration) Offset {
	return NewOffset(time.Duration(o) + delta)
}

// TrackTime converts a playback position into the track's own time base.
func (o Offset) TrackTime(position time.Duration) time.Duration {
	return position - time.Duration(o)
}

// PlaybackTime converts an instant of the track's time base into playback time.
func (o Offset) PlaybackTime(trackTime time.Duration) time.Duration {
	return trackTime + time.Duration(o)
}
