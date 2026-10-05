package watch_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/tests/testkit"
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

	// Playing, the same position for more than 3 s: frozen. The samples'
	// own times tell it (the limit itself is the clock's, tested there).
	at := time.Now()
	playing := func(position, after time.Duration) {
		at = at.Add(after)
		r.running.samples <- playback.Sample{Position: position, Duration: time.Minute, Status: playback.StatusPlaying, Speed: 1, ObservedAt: at}
	}
	playing(20*time.Second, 0)
	playing(20*time.Second, 3300*time.Millisecond)
	testkit.Eventually(t, "the freeze", func() bool { return s.Health() == watch.HealthStalled })
	playing(21*time.Second, 100*time.Millisecond)
	testkit.Eventually(t, "the player moving again", func() bool { return s.Health() == watch.HealthGood })

	s.Notify("Aviso")
	if _, _, notices, _ := r.display.last(); len(notices) == 0 || notices[len(notices)-1] != "Aviso" {
		t.Fatalf("notices = %v", notices)
	}
}
