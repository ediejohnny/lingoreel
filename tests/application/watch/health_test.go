package watch_test

import (
	"testing"
	"time"

	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/playback"
)

func TestThePlayersHealth(t *testing.T) {
	r, s := playingRig(t)
	if s.Health() != watch.HealthGood {
		t.Fatalf("health at the start = %v", s.Health())
	}
	r.running.reconnecting.Store(true)
	if s.Health() != watch.HealthReconnecting {
		t.Fatalf("health while reconnecting = %v", s.Health())
	}
	r.running.reconnecting.Store(false)

	// Playing, the same position for more than 3 s: frozen.
	start := time.Now()
	for time.Since(start) < 3300*time.Millisecond {
		r.running.samples <- playback.Sample{Position: 20 * time.Second, Duration: time.Minute, Status: playback.StatusPlaying, Speed: 1, ObservedAt: time.Now()}
		time.Sleep(100 * time.Millisecond)
	}
	eventually(t, "the freeze", func() bool { return s.Health() == watch.HealthStalled })
	r.running.samples <- playback.Sample{Position: 21 * time.Second, Duration: time.Minute, Status: playback.StatusPlaying, Speed: 1, ObservedAt: time.Now()}
	eventually(t, "the player moving again", func() bool { return s.Health() == watch.HealthGood })

	s.Notify("Aviso")
	if _, _, notices, _ := r.display.last(); len(notices) == 0 || notices[len(notices)-1] != "Aviso" {
		t.Fatalf("notices = %v", notices)
	}
}
