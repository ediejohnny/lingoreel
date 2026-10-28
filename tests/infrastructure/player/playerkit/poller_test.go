package playerkit_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/playerkit"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// quick reads every few milliseconds; a player silent for a second is lost.
var quick = playerkit.Timing{
	Interval:   5 * time.Millisecond,
	FirstRetry: 5 * time.Millisecond,
	MaxRetry:   20 * time.Millisecond,
	Limit:      time.Second,
}

// fakePlayer answers with a position one second further at each reading,
// which takes it a while, unless it is silent.
type fakePlayer struct {
	takes    time.Duration
	readings atomic.Int64
	silent   atomic.Bool

	mu              sync.Mutex
	started, finish time.Time
}

func (f *fakePlayer) read(context.Context) (playback.Sample, error) {
	started := time.Now()
	if f.silent.Load() {
		return playback.Sample{}, errors.New("no answer")
	}
	time.Sleep(f.takes)
	n := f.readings.Add(1)
	f.mu.Lock()
	f.started, f.finish = started, time.Now()
	f.mu.Unlock()
	return playback.Sample{Position: time.Duration(n) * time.Second, Status: playback.StatusPlaying}, nil
}

func startPoller(t *testing.T, player *fakePlayer) (*playerkit.Poller, context.CancelFunc, chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	exited := make(chan struct{})
	poller := playerkit.NewPoller("the player", quick, slog.New(slog.DiscardHandler))
	poller.Start(ctx, exited, player.read)
	return poller, cancel, exited
}

// A consumer that reads late gets the newest reading, not the first one,
// stamped at the middle of the reading that made it.
func TestThePollerKeepsOnlyTheLatestSample(t *testing.T) {
	t.Parallel()
	player := &fakePlayer{takes: 100 * time.Millisecond}
	poller, _, _ := startPoller(t, player)
	testkit.Eventually(t, "a few readings", func() bool { return player.readings.Load() >= 3 })
	// Once a silent reading is heard, the last answer was already published.
	player.silent.Store(true)
	testkit.Eventually(t, "the silence heard", poller.Reconnecting)
	sample := <-poller.Samples()
	last := player.readings.Load()
	if sample.Position != time.Duration(last)*time.Second {
		t.Fatalf("the consumer got the reading at %v, the newest is at %vs", sample.Position, last)
	}
	player.mu.Lock()
	middle := player.started.Add(player.finish.Sub(player.started) / 2)
	player.mu.Unlock()
	// The start or the end of the reading would be half a reading away.
	if gap := sample.ObservedAt.Sub(middle).Abs(); gap > player.takes/4 {
		t.Fatalf("observed %v away from the middle of its reading", gap)
	}
}

// A player silent for a while is reported as reconnecting and the samples go
// on once it answers; one silent past the limit is lost and the samples end.
func TestASilentPlayerIsWaitedForThenLost(t *testing.T) {
	t.Parallel()
	player := &fakePlayer{}
	poller, _, _ := startPoller(t, player)
	testkit.Eventually(t, "a first reading", func() bool { return player.readings.Load() > 0 })

	player.silent.Store(true)
	testkit.Eventually(t, "reconnecting", poller.Reconnecting)
	player.silent.Store(false)
	answered := player.readings.Load()
	testkit.Eventually(t, "the player answering again", func() bool {
		return !poller.Reconnecting() && player.readings.Load() > answered
	})
	if poller.Lost() {
		t.Fatal("a short silence lost the player")
	}

	player.silent.Store(true)
	deadline := time.After(10 * quick.Limit)
	for {
		select {
		case _, open := <-poller.Samples():
			if open {
				continue
			}
			if !poller.Lost() || poller.Reconnecting() {
				t.Fatalf("the samples ended: lost %v, reconnecting %v", poller.Lost(), poller.Reconnecting())
			}
			return
		case <-deadline:
			t.Fatal("the samples still run long past the limit of silence")
		}
	}
}

// A player that exits, or a session that stops, ends the samples without the
// player counting as lost; a poller never started has nothing to wait for.
func TestAnEndedSessionEndsThePollingWithoutLoss(t *testing.T) {
	t.Parallel()
	for name, end := range map[string]func(context.CancelFunc, chan struct{}){
		"the player exited":    func(_ context.CancelFunc, exited chan struct{}) { close(exited) },
		"the session stopping": func(cancel context.CancelFunc, _ chan struct{}) { cancel() },
	} {
		t.Run(name, func(t *testing.T) {
			player := &fakePlayer{}
			poller, cancel, exited := startPoller(t, player)
			testkit.Eventually(t, "a first reading", func() bool { return player.readings.Load() > 0 })
			end(cancel, exited)
			if !poller.Wait(5 * time.Second) {
				t.Fatal("the polling did not end")
			}
			for range poller.Samples() {
			}
			if poller.Lost() {
				t.Fatal("an ended session counted as a lost player")
			}
		})
	}

	idle := playerkit.NewPoller("the player", quick, slog.New(slog.DiscardHandler))
	if !idle.Wait(0) {
		t.Fatal("a poller never started was waited for")
	}
}
