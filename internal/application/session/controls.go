package session

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
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

// VolumePlayer is a player whose volume can be changed; its capabilities say
// Volume and its ceiling. Players start at their 100% (playback.FullVolume).
type VolumePlayer interface {
	SetVolume(volume playback.Volume) error
}

// VolumeReader is a volume player that tells its level, such as one that
// keeps the user's own level between sessions.
type VolumeReader interface {
	Volume() (playback.Volume, error)
}

// Controls is the single owner of playback time arithmetic: seeking, speed
// and frame stepping all go through it, and it keeps the clock in step.
type Controls struct {
	player       Player
	clock        *Clock
	capabilities playback.Capabilities
	frame        time.Duration
	hasFrame     bool

	// beforePlay and afterSeek let a Practice see the user's commands:
	// Play at the end of a passage, a seek out of it.
	beforePlay func()
	afterSeek  func(position time.Duration)

	// volume is the level last asked for; muted keeps it to restore.
	volumeMu sync.Mutex
	volume   playback.Volume
	muted    bool
	ramp     playback.VolumeRamp
}

// NewControls creates the controls; frameRate is the video's real rate, or 0
// when unknown.
func NewControls(player Player, clock *Clock, frameRate float64) *Controls {
	frame, hasFrame := playback.FrameDuration(frameRate)
	capabilities := player.Capabilities()
	if _, ok := player.(VolumePlayer); !ok {
		capabilities.Volume = false
	}
	return &Controls{
		player: player, clock: clock, capabilities: capabilities, frame: frame, hasFrame: hasFrame,
		volume: playback.FullVolume,
	}
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
	// A player without a backward step but with exact seeks gets one: an
	// exact seek one frame back.
	if !capabilities.FrameStepBackward && capabilities.Seek && capabilities.ExactSeek && c.hasFrame {
		capabilities.FrameStepBackward, capabilities.FrameStepBackwardEmulated = true, true
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
	if c.beforePlay != nil {
		c.beforePlay()
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
	if c.afterSeek != nil {
		c.afterSeek(position)
	}
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
	if !forward && capabilities.FrameStepBackwardEmulated {
		return c.SeekTo(c.clock.Position() - c.frame)
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

// Volume is the level last asked for and whether the sound is muted.
func (c *Controls) Volume() (playback.Volume, bool) {
	c.volumeMu.Lock()
	defer c.volumeMu.Unlock()
	return c.volume, c.muted
}

// SetVolume changes the level, kept inside the player's ceiling; it also
// unmutes.
func (c *Controls) SetVolume(volume playback.Volume) error {
	c.volumeMu.Lock()
	defer c.volumeMu.Unlock()
	return c.applyVolume(volume.Within(c.capabilities.VolumeCeiling()), false)
}

// ChangeVolume moves the level by delta, inside the player's ceiling.
func (c *Controls) ChangeVolume(delta playback.Volume) error {
	c.volumeMu.Lock()
	defer c.volumeMu.Unlock()
	return c.applyVolume((c.volume + delta).Within(c.capabilities.VolumeCeiling()), false)
}

// StepVolume is a volume key going down (repeat false) or held: a touch is
// one step, a held key moves on at the ramp's pace and stops at 100% going
// up (playback.VolumeRamp). It unmutes.
func (c *Controls) StepVolume(now time.Time, up, repeat bool) error {
	c.volumeMu.Lock()
	defer c.volumeMu.Unlock()
	if !c.capabilities.Volume {
		return ErrNotSupported
	}
	next := c.ramp.Next(now, up, repeat, c.volume, c.capabilities.VolumeCeiling())
	if next == c.volume && !c.muted {
		return nil
	}
	return c.applyVolume(next, false)
}

// ToggleMute silences the sound or brings back the level it had.
func (c *Controls) ToggleMute() error {
	c.volumeMu.Lock()
	defer c.volumeMu.Unlock()
	return c.applyVolume(c.volume, !c.muted)
}

// applyVolume runs with volumeMu held, so two changes never cross on their
// way to the player.
func (c *Controls) applyVolume(volume playback.Volume, muted bool) error {
	if !c.capabilities.Volume {
		return ErrNotSupported
	}
	heard := volume
	if muted {
		heard = playback.MinVolume
	}
	if err := c.player.(VolumePlayer).SetVolume(heard); err != nil {
		return fmt.Errorf("changing the volume: %w", err)
	}
	c.volume, c.muted = volume, muted
	return nil
}

// SyncVolume makes the level known at the start: the player's own when it
// tells it, otherwise its 100% asked for explicitly, so the screens never
// show a level the player does not have.
func (c *Controls) SyncVolume() error {
	c.volumeMu.Lock()
	defer c.volumeMu.Unlock()
	if !c.capabilities.Volume {
		return nil
	}
	if reader, ok := c.player.(VolumeReader); ok {
		volume, err := reader.Volume()
		if err != nil {
			return fmt.Errorf("reading the volume: %w", err)
		}
		c.volume = volume.Within(c.capabilities.VolumeCeiling())
		return nil
	}
	return c.applyVolume(playback.FullVolume, false)
}
