package playback

import (
	"errors"
	"time"
)

// ErrPlayerLost reports a player that crashed or stopped answering, as
// opposed to one the user closed.
var ErrPlayerLost = errors.New("the player stopped responding")

// Status is what a player is doing.
type Status string

const (
	// StatusUnknown is the state before the player has reported anything.
	StatusUnknown Status = ""
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
	Pause bool
	Seek  bool
	// ExactSeek means a seek lands on the requested time, not on the nearest
	// keyframe, so a backward frame step can be emulated by seeking.
	ExactSeek         bool
	Speed             bool
	FrameStepForward  bool
	FrameStepBackward bool
	// FrameStepBackwardEmulated marks a backward step made by an exact seek
	// one frame back instead of a player command; the UI calls it approximate.
	FrameStepBackwardEmulated bool
	AudioTrackSelection       bool
	SubtitleSuppression       bool
	// Volume means the player takes volume changes (session.VolumePlayer).
	Volume bool
	// MaxVolume is the loudest the player plays: FullVolume (100%) when
	// unset, at most MaxVolume (200%).
	MaxVolume Volume
	Precision Precision
}

// VolumeCeiling is the loudest level the player takes.
func (c Capabilities) VolumeCeiling() Volume {
	if !(c.MaxVolume > 0) {
		return FullVolume
	}
	return min(c.MaxVolume, MaxVolume)
}
