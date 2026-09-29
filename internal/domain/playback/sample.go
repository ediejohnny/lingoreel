package playback

import "time"

// Status is what a player is doing.
type Status string

const (
	StatusPlaying Status = "playing"
	StatusPaused  Status = "paused"
	StatusStopped Status = "stopped"
)

// Sample is one observation of a player: where it was, how fast it went and
// when the observation was taken on the local monotonic clock.
type Sample struct {
	Position   time.Duration
	Duration   time.Duration
	Speed      Speed
	Status     Status
	ObservedAt time.Time
}

// Precision tells how far a player's reported position can be trusted.
type Precision string

const (
	// PrecisionExact positions follow the displayed frame.
	PrecisionExact Precision = "exact"
	// PrecisionEstimated positions are approximations updated a few times per
	// second, so the clock extrapolates between them.
	PrecisionEstimated Precision = "estimated"
)

// Capabilities declares what a player can do. Features a player lacks are
// shown as unavailable instead of failing silently.
type Capabilities struct {
	Pause               bool
	Seek                bool
	Speed               bool
	FrameStepForward    bool
	FrameStepBackward   bool
	AudioTrackSelection bool
	SubtitleSuppression bool
	Precision           Precision
}
