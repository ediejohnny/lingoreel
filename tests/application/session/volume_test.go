package session_test

import (
	"errors"
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

// volumePlayer is a player whose volume can change; it records the levels
// it is asked for.
type volumePlayer struct {
	fakePlayer
	levels []playback.Volume
	fail   error
}

func (p *volumePlayer) SetVolume(volume playback.Volume) error {
	if p.fail != nil {
		return p.fail
	}
	p.levels = append(p.levels, volume)
	return nil
}

func newVolumeControls(t *testing.T) (*session.Controls, *volumePlayer) {
	t.Helper()
	capabilities := allCapabilities(playback.PrecisionExact)
	capabilities.Volume = true
	player := &volumePlayer{fakePlayer: fakePlayer{capabilities: capabilities}}
	now := newFakeTime()
	clock := session.NewClock(capabilities.Precision, now.Now)
	return session.NewControls(player, clock, frameRate24), player
}

func TestVolumeStartsFullAndClamps(t *testing.T) {
	t.Parallel()
	controls, player := newVolumeControls(t)
	if volume, muted := controls.Volume(); volume != playback.MaxVolume || muted {
		t.Fatalf("start = %v muted %v, want full and not muted", volume, muted)
	}
	// The limits themselves are the domain's; here, that they apply.
	for _, c := range []struct{ asked, sent playback.Volume }{{0.4, 0.4}, {1.5, 1}} {
		if err := controls.SetVolume(c.asked); err != nil {
			t.Fatal(err)
		}
		if got := player.levels[len(player.levels)-1]; got != c.sent {
			t.Errorf("SetVolume(%v) sent %v, want %v", c.asked, got, c.sent)
		}
	}
}

func TestChangeVolumeSteps(t *testing.T) {
	t.Parallel()
	controls, player := newVolumeControls(t)
	if err := controls.ChangeVolume(-playback.VolumeStep); err != nil {
		t.Fatal(err)
	}
	if volume, _ := controls.Volume(); math.Abs(float64(volume)-0.95) > 1e-9 {
		t.Fatalf("one step down from full = %v", volume)
	}
	if err := controls.ChangeVolume(10); err != nil || player.levels[len(player.levels)-1] != 1 {
		t.Fatalf("a big step up must stop at full: %v %v", err, player.levels)
	}
}

func TestMuteKeepsTheLevel(t *testing.T) {
	t.Parallel()
	controls, player := newVolumeControls(t)
	if err := controls.SetVolume(0.6); err != nil {
		t.Fatal(err)
	}
	if err := controls.ToggleMute(); err != nil {
		t.Fatal(err)
	}
	if volume, muted := controls.Volume(); volume != 0.6 || !muted || player.levels[len(player.levels)-1] != 0 {
		t.Fatalf("muted: level %v muted %v sent %v", volume, muted, player.levels)
	}
	if err := controls.ToggleMute(); err != nil {
		t.Fatal(err)
	}
	if volume, muted := controls.Volume(); volume != 0.6 || muted || player.levels[len(player.levels)-1] != 0.6 {
		t.Fatalf("unmuted: level %v muted %v sent %v", volume, muted, player.levels)
	}
	// Changing the level while muted brings the sound back at the new level.
	_ = controls.ToggleMute()
	if err := controls.ChangeVolume(playback.VolumeStep); err != nil {
		t.Fatal(err)
	}
	if _, muted := controls.Volume(); muted || player.levels[len(player.levels)-1] == 0 {
		t.Fatalf("a volume change kept the sound off: %v", player.levels)
	}
}

func TestVolumeNeedsThePlayer(t *testing.T) {
	t.Parallel()
	// A player that declares the capability without the method, and one
	// with the method but without the capability: both refuse.
	capabilities := allCapabilities(playback.PrecisionExact)
	capabilities.Volume = true
	plain := &fakePlayer{capabilities: capabilities}
	controls := session.NewControls(plain, session.NewClock(playback.PrecisionExact, newFakeTime().Now), 0)
	if err := controls.SetVolume(0.5); !errors.Is(err, session.ErrNotSupported) || controls.Capabilities().Volume {
		t.Fatalf("player without SetVolume: err %v, capability %v", err, controls.Capabilities().Volume)
	}
	silent := &volumePlayer{fakePlayer: fakePlayer{capabilities: allCapabilities(playback.PrecisionExact)}}
	controls = session.NewControls(silent, session.NewClock(playback.PrecisionExact, newFakeTime().Now), 0)
	if err := controls.ToggleMute(); !errors.Is(err, session.ErrNotSupported) || len(silent.levels) != 0 {
		t.Fatalf("player without the capability: err %v, sent %v", err, silent.levels)
	}
}

func TestAFailedVolumeChangeKeepsTheOldLevel(t *testing.T) {
	t.Parallel()
	controls, player := newVolumeControls(t)
	player.fail = errors.New("player gone")
	if err := controls.SetVolume(0.2); err == nil {
		t.Fatal("the failure was not reported")
	}
	if volume, muted := controls.Volume(); volume != 1 || muted {
		t.Fatalf("after a failure: %v muted %v, want the old level", volume, muted)
	}
}
