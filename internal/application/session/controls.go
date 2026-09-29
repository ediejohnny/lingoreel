package session

import (
	"errors"
	"fmt"
	"time"

	"lingoreel/internal/domain/playback"
)

var (
	// ErrNotSupported reports a feature the current player lacks.
	ErrNotSupported = errors.New("not supported by this player")
	// ErrInvalidSpeed reports a speed outside the supported range.
	ErrInvalidSpeed = errors.New("invalid playback speed")
)

// Player is what a running player offers to the session.
type Player interface {
	Capabilities() playback.Capabilities
	Play() error
	Pause() error
	SeekTo(position time.Duration) error
	SetSpeed(speed playback.Speed) error
	StepFrame(forward bool) error
	// SelectAudioTrack picks the audio track by its position among the
	// file's audio tracks, starting at zero.
	SelectAudioTrack(index int) error
}

// Controls is the single owner of playback time arithmetic: seeking, speed
// and frame stepping all go through it, and it keeps the clock in step.
type Controls struct {
	player       Player
	clock        *Clock
	capabilities playback.Capabilities
	frame        time.Duration
	hasFrame     bool
}

// NewControls creates the controls; frameRate is the video's real rate, or 0
// when unknown.
func NewControls(player Player, clock *Clock, frameRate float64) *Controls {
	frame, hasFrame := playback.FrameDuration(frameRate)
	return &Controls{player: player, clock: clock, capabilities: player.Capabilities(), frame: frame, hasFrame: hasFrame}
}

// Capabilities reports what the controls can do with this player, so the UI
// can disable the rest.
func (c *Controls) Capabilities() playback.Capabilities {
	capabilities := c.capabilities
	// Estimated players give no position feedback after a frame step, so the
	// session must know the frame length to follow it.
	if capabilities.Precision != playback.PrecisionExact && !c.hasFrame {
		capabilities.FrameStepForward, capabilities.FrameStepBackward = false, false
	}
	return capabilities
}

// Position returns the current playback position.
func (c *Controls) Position() time.Duration {
	return c.clock.Position()
}

// TogglePause pauses a playing video and resumes a paused one.
func (c *Controls) TogglePause() error {
	if c.clock.State().Status == playback.StatusPlaying {
		return c.Pause()
	}
	return c.Play()
}

// Play resumes playback.
func (c *Controls) Play() error {
	if !c.capabilities.Pause {
		return ErrNotSupported
	}
	if err := c.player.Play(); err != nil {
		return fmt.Errorf("resuming playback: %w", err)
	}
	c.clock.SetStatus(playback.StatusPlaying)
	return nil
}

// Pause pauses playback.
func (c *Controls) Pause() error {
	if !c.capabilities.Pause {
		return ErrNotSupported
	}
	if err := c.player.Pause(); err != nil {
		return fmt.Errorf("pausing playback: %w", err)
	}
	c.clock.SetStatus(playback.StatusPaused)
	return nil
}

// SeekBy moves the position by delta, forward or backward.
func (c *Controls) SeekBy(delta time.Duration) error {
	return c.SeekTo(c.clock.Position() + delta)
}

// SeekTo moves to an absolute position, kept inside the video.
func (c *Controls) SeekTo(position time.Duration) error {
	if !c.capabilities.Seek {
		return ErrNotSupported
	}
	position = max(position, 0)
	if duration := c.clock.State().Duration; duration > 0 {
		position = min(position, duration)
	}
	if err := c.player.SeekTo(position); err != nil {
		return fmt.Errorf("seeking to %v: %w", position, err)
	}
	c.clock.SetPosition(position)
	return nil
}

// SetSpeed changes the playback speed.
func (c *Controls) SetSpeed(speed playback.Speed) error {
	if !c.capabilities.Speed {
		return ErrNotSupported
	}
	if !speed.Valid() {
		return fmt.Errorf("%w: %v", ErrInvalidSpeed, speed)
	}
	if err := c.player.SetSpeed(speed); err != nil {
		return fmt.Errorf("changing speed to %v: %w", speed, err)
	}
	c.clock.SetSpeed(speed)
	return nil
}

// Faster moves to the next preset speed.
func (c *Controls) Faster() error {
	return c.SetSpeed(c.clock.State().Speed.Faster())
}

// Slower moves to the previous preset speed.
func (c *Controls) Slower() error {
	return c.SetSpeed(c.clock.State().Speed.Slower())
}

// NormalSpeed restores 1x.
func (c *Controls) NormalSpeed() error {
	return c.SetSpeed(playback.NormalSpeed)
}

// StepFrame moves exactly one frame, pausing first when needed.
func (c *Controls) StepFrame(forward bool) error {
	capabilities := c.Capabilities()
	if (forward && !capabilities.FrameStepForward) || (!forward && !capabilities.FrameStepBackward) {
		return ErrNotSupported
	}
	if c.clock.State().Status == playback.StatusPlaying {
		if err := c.Pause(); err != nil {
			return err
		}
	}
	if err := c.player.StepFrame(forward); err != nil {
		return fmt.Errorf("stepping one frame: %w", err)
	}
	if capabilities.Precision != playback.PrecisionExact {
		step := c.frame
		if !forward {
			step = -step
		}
		c.clock.SetPosition(max(c.clock.Position()+step, 0))
	}
	return nil
}

// SelectAudioTrack switches the audio track by its position among audio tracks.
func (c *Controls) SelectAudioTrack(index int) error {
	if !c.capabilities.AudioTrackSelection {
		return ErrNotSupported
	}
	if index < 0 {
		return fmt.Errorf("invalid audio track index %d", index)
	}
	if err := c.player.SelectAudioTrack(index); err != nil {
		return fmt.Errorf("selecting audio track %d: %w", index, err)
	}
	return nil
}
