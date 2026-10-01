package session_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/playback"
)

const frameRate24 = 24.0

func newControls(t *testing.T, player *fakePlayer, status playback.Status, position time.Duration, frameRate float64) (*session.Controls, *session.Clock, *fakeTime) {
	t.Helper()
	now := newFakeTime()
	clock := session.NewClock(player.capabilities.Precision, now.Now)
	clock.Observe(playback.Sample{Position: position, Duration: ms(60_000), Speed: 1, Status: status, ObservedAt: now.Now()})
	return session.NewControls(player, clock, frameRate), clock, now
}

func TestTogglePauseFollowsTheCurrentState(t *testing.T) {
	player := &fakePlayer{capabilities: allCapabilities(playback.PrecisionEstimated)}
	controls, clock, _ := newControls(t, player, playback.StatusPlaying, ms(5_000), frameRate24)

	if err := controls.TogglePause(); err != nil || clock.State().Status != playback.StatusPaused {
		t.Fatalf("toggle from playing: %v, status %s", err, clock.State().Status)
	}
	if err := controls.TogglePause(); err != nil || clock.State().Status != playback.StatusPlaying {
		t.Fatalf("toggle from paused: %v, status %s", err, clock.State().Status)
	}
	if !slices.Equal(player.calls, []string{"pause", "play"}) {
		t.Fatalf("player calls = %v", player.calls)
	}
}

func TestSeekIsClampedAndMovesTheClockAtOnce(t *testing.T) {
	cases := []struct {
		name string
		seek func(*session.Controls) error
		want time.Duration
	}{
		{"forward 5 s", func(c *session.Controls) error { return c.SeekBy(5 * time.Second) }, ms(15_000)},
		{"back 10 s", func(c *session.Controls) error { return c.SeekBy(-10 * time.Second) }, 0},
		{"back past the start", func(c *session.Controls) error { return c.SeekBy(-30 * time.Second) }, 0},
		{"past the end", func(c *session.Controls) error { return c.SeekTo(2 * time.Minute) }, ms(60_000)},
		{"precision step", func(c *session.Controls) error { return c.SeekBy(100 * time.Millisecond) }, ms(10_100)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			player := &fakePlayer{capabilities: allCapabilities(playback.PrecisionEstimated)}
			controls, clock, _ := newControls(t, player, playback.StatusPaused, ms(10_000), frameRate24)
			if err := tc.seek(controls); err != nil {
				t.Fatalf("seek: %v", err)
			}
			if player.seekTarget != tc.want || clock.Position() != tc.want {
				t.Fatalf("player sought %v, clock at %v; want %v", player.seekTarget, clock.Position(), tc.want)
			}
		})
	}
}

func TestSpeedChangesAreValidatedAndStepped(t *testing.T) {
	player := &fakePlayer{capabilities: allCapabilities(playback.PrecisionExact)}
	controls, clock, _ := newControls(t, player, playback.StatusPlaying, 0, frameRate24)

	for _, step := range []struct {
		action func() error
		want   playback.Speed
	}{
		{controls.Faster, 1.1},
		{controls.Faster, 1.25},
		{controls.Slower, 1.1},
		{controls.NormalSpeed, 1},
		{controls.Slower, 0.9},
	} {
		if err := step.action(); err != nil || player.speed != step.want || clock.State().Speed != step.want {
			t.Fatalf("speed = player %v / clock %v, err %v; want %v", player.speed, clock.State().Speed, err, step.want)
		}
	}

	calls := len(player.calls)
	if err := controls.SetSpeed(3); !errors.Is(err, session.ErrInvalidSpeed) || len(player.calls) != calls {
		t.Fatalf("SetSpeed(3) = %v and reached the player", err)
	}
}

func TestFrameStepOnAnEstimatedPlayerPausesAndAdvancesOneFrame(t *testing.T) {
	player := &fakePlayer{capabilities: allCapabilities(playback.PrecisionEstimated)}
	controls, clock, _ := newControls(t, player, playback.StatusPlaying, ms(10_000), frameRate24)

	if err := controls.StepFrame(true); err != nil {
		t.Fatalf("StepFrame: %v", err)
	}
	if !slices.Equal(player.calls, []string{"pause", "frame+"}) {
		t.Fatalf("player calls = %v, want pause then frame+", player.calls)
	}
	// 1/24 s rounded to the nanosecond.
	if want := ms(10_000) + 41_666_667*time.Nanosecond; clock.Position() != want {
		t.Fatalf("clock after one frame = %v, want %v", clock.Position(), want)
	}
	if err := controls.StepFrame(false); err != nil || clock.Position() != ms(10_000) {
		t.Fatalf("frame back: %v, clock %v", err, clock.Position())
	}
}

func TestFrameStepOnAnExactPlayerLetsThePlayerReportThePosition(t *testing.T) {
	player := &fakePlayer{capabilities: allCapabilities(playback.PrecisionExact)}
	controls, clock, _ := newControls(t, player, playback.StatusPaused, ms(10_000), 0)

	if err := controls.StepFrame(true); err != nil || clock.Position() != ms(10_000) {
		t.Fatalf("StepFrame = %v, clock %v; the exact player reports the new position itself", err, clock.Position())
	}
}

func TestFrameStepNeedsTheCapabilityAndAKnownRate(t *testing.T) {
	forwardOnly := allCapabilities(playback.PrecisionEstimated)
	forwardOnly.FrameStepBackward = false
	player := &fakePlayer{capabilities: forwardOnly}
	controls, _, _ := newControls(t, player, playback.StatusPaused, ms(10_000), frameRate24)
	if err := controls.StepFrame(false); !errors.Is(err, session.ErrNotSupported) {
		t.Fatalf("backward step on a forward-only player = %v", err)
	}

	unknownRate := &fakePlayer{capabilities: allCapabilities(playback.PrecisionEstimated)}
	controls, _, _ = newControls(t, unknownRate, playback.StatusPaused, ms(10_000), 0)
	if controls.Capabilities().FrameStepForward || !errors.Is(controls.StepFrame(true), session.ErrNotSupported) {
		t.Fatal("an estimated player without a known frame rate must not step frames")
	}
	if len(unknownRate.calls) != 0 {
		t.Fatalf("unsupported step reached the player: %v", unknownRate.calls)
	}
}

// VLC has no backward frame command but seeks exactly: the controls emulate
// the step with a seek one frame back and mark it as emulated for the UI.
func TestBackwardFrameIsEmulatedWithAnExactSeek(t *testing.T) {
	vlcLike := allCapabilities(playback.PrecisionEstimated)
	vlcLike.FrameStepBackward, vlcLike.ExactSeek = false, true
	player := &fakePlayer{capabilities: vlcLike}
	controls, clock, _ := newControls(t, player, playback.StatusPlaying, ms(10_000), frameRate24)

	capabilities := controls.Capabilities()
	if !capabilities.FrameStepBackward || !capabilities.FrameStepBackwardEmulated {
		t.Fatalf("capabilities = %+v, want an emulated backward step", capabilities)
	}
	if err := controls.StepFrame(false); err != nil {
		t.Fatalf("StepFrame(false): %v", err)
	}
	if !slices.Equal(player.calls, []string{"pause", "seek"}) {
		t.Fatalf("player calls = %v, want pause then seek (never frame-)", player.calls)
	}
	want := ms(10_000) - 41_666_667*time.Nanosecond
	if player.seekTarget != want || clock.Position() != want {
		t.Fatalf("seek target %v, clock %v, want %v", player.seekTarget, clock.Position(), want)
	}

	// Without exact seeks, or without a frame rate, there is nothing to emulate.
	roughSeek := vlcLike
	roughSeek.ExactSeek = false
	rough := &fakePlayer{capabilities: roughSeek}
	controls, _, _ = newControls(t, rough, playback.StatusPaused, ms(10_000), frameRate24)
	if controls.Capabilities().FrameStepBackward || !errors.Is(controls.StepFrame(false), session.ErrNotSupported) {
		t.Fatal("a player with keyframe seeks got an emulated backward step")
	}
	noRate := &fakePlayer{capabilities: vlcLike}
	controls, _, _ = newControls(t, noRate, playback.StatusPaused, ms(10_000), 0)
	if controls.Capabilities().FrameStepBackward || len(noRate.calls) != 0 {
		t.Fatal("an unknown frame rate got an emulated backward step")
	}
	// At the very start the step stops at zero.
	atStart := &fakePlayer{capabilities: vlcLike}
	controls, _, _ = newControls(t, atStart, playback.StatusPaused, ms(10), frameRate24)
	if err := controls.StepFrame(false); err != nil || atStart.seekTarget != 0 {
		t.Fatalf("step back at 10 ms = %v, target %v, want 0", err, atStart.seekTarget)
	}
}

func TestMissingCapabilitiesAreReportedNotIgnored(t *testing.T) {
	player := &fakePlayer{capabilities: playback.Capabilities{Precision: playback.PrecisionEstimated}}
	controls, _, _ := newControls(t, player, playback.StatusPlaying, 0, frameRate24)

	for name, action := range map[string]func() error{
		"pause": controls.Pause, "play": controls.Play, "seek": func() error { return controls.SeekBy(time.Second) },
		"speed": controls.Faster, "audio": func() error { return controls.SelectAudioTrack(1) },
	} {
		if err := action(); !errors.Is(err, session.ErrNotSupported) {
			t.Fatalf("%s without capability = %v, want ErrNotSupported", name, err)
		}
	}
	if len(player.calls) != 0 {
		t.Fatalf("unsupported commands reached the player: %v", player.calls)
	}
}

func TestPlayerErrorsLeaveTheClockUntouched(t *testing.T) {
	failure := errors.New("pipe broken")
	player := &fakePlayer{capabilities: allCapabilities(playback.PrecisionEstimated), err: failure}
	controls, clock, _ := newControls(t, player, playback.StatusPlaying, ms(10_000), frameRate24)

	if err := controls.SeekTo(ms(30_000)); !errors.Is(err, failure) || clock.Position() != ms(10_000) {
		t.Fatalf("failed seek = %v, clock %v", err, clock.Position())
	}
	if err := controls.Pause(); !errors.Is(err, failure) || clock.State().Status != playback.StatusPlaying {
		t.Fatalf("failed pause = %v, status %s", err, clock.State().Status)
	}
}

func TestSelectAudioTrackValidatesTheIndex(t *testing.T) {
	player := &fakePlayer{capabilities: allCapabilities(playback.PrecisionExact)}
	controls, _, _ := newControls(t, player, playback.StatusPaused, 0, frameRate24)

	if err := controls.SelectAudioTrack(-1); err == nil {
		t.Fatal("negative audio index accepted")
	}
	if err := controls.SelectAudioTrack(2); err != nil || player.audioIndex != 2 {
		t.Fatalf("SelectAudioTrack(2) = %v, player index %d", err, player.audioIndex)
	}
}
