package session_test

import (
	"errors"
	"math"
	"testing"
	"time"

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
	if volume, muted := controls.Volume(); volume != playback.FullVolume || muted {
		t.Fatalf("start = %v muted %v, want full and not muted", volume, muted)
	}
	// The limits themselves are the domain's; here, that the player's ceiling
	// applies: 100% when it declares none.
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
	if volume, _ := controls.Volume(); math.Abs(float64(volume)-0.99) > 1e-9 {
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

// A player that plays louder declares it, and the keys reach its ceiling.
func TestTheVolumeGoesUpToThePlayersCeiling(t *testing.T) {
	t.Parallel()
	capabilities := allCapabilities(playback.PrecisionExact)
	capabilities.Volume, capabilities.MaxVolume = true, 2
	player := &volumePlayer{fakePlayer: fakePlayer{capabilities: capabilities}}
	controls := session.NewControls(player, session.NewClock(playback.PrecisionExact, newFakeTime().Now), 0)
	if err := controls.SetVolume(1.5); err != nil || player.levels[len(player.levels)-1] != 1.5 {
		t.Fatalf("SetVolume(1.5) = %v, sent %v", err, player.levels)
	}
	if err := controls.SetVolume(3); err != nil || player.levels[len(player.levels)-1] != 2 {
		t.Fatalf("SetVolume(3) = %v, sent %v", err, player.levels)
	}
	// A key touch moves 1%.
	if err := controls.StepVolume(time.Now(), false, false); err != nil || player.levels[len(player.levels)-1] != 1.99 {
		t.Fatalf("a touch down = %v, sent %v", err, player.levels)
	}
}

// readerPlayer tells the level it keeps between its own sessions.
type readerPlayer struct {
	volumePlayer
	kept playback.Volume
}

func (p *readerPlayer) Volume() (playback.Volume, error) { return p.kept, nil }

// At the start the level is the player's own when it tells it (PotPlayer
// keeps the user's), or its 100% asked for explicitly: never a level the
// screens show and the player does not have.
func TestTheStartingVolumeIsThePlayers(t *testing.T) {
	t.Parallel()
	capabilities := allCapabilities(playback.PrecisionExact)
	capabilities.Volume = true
	reader := &readerPlayer{volumePlayer: volumePlayer{fakePlayer: fakePlayer{capabilities: capabilities}}, kept: 0.5}
	controls := session.NewControls(reader, session.NewClock(playback.PrecisionExact, newFakeTime().Now), 0)
	if err := controls.SyncVolume(); err != nil {
		t.Fatal(err)
	}
	if volume, _ := controls.Volume(); volume != 0.5 || len(reader.levels) != 0 {
		t.Fatalf("a player at 50%%: level %v, sent %v", volume, reader.levels)
	}
	if err := controls.StepVolume(time.Now(), false, false); err != nil || reader.levels[0] != 0.49 {
		t.Fatalf("the first touch down from 50%% sent %v (%v)", reader.levels, err)
	}
	controls, plain := newVolumeControls(t)
	if err := controls.SyncVolume(); err != nil || len(plain.levels) != 1 || plain.levels[0] != 1 {
		t.Fatalf("a player that does not tell: %v, sent %v", err, plain.levels)
	}
}
