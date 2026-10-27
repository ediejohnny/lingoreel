//go:build windows

package ffmpeg_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The film's description, which every still waits for, runs at a priority a
// busy computer still serves: at the idle one, with both processors busy, it
// waited past its time and the film had no stills for the whole session.
func TestTheDescriptionRunsAtAPriorityABusyComputerServes(t *testing.T) {
	self := standInPath(t)
	t.Setenv(standIn, "describe")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := (ffmpeg.Stills{Path: self}).Open(ctx, "film.mkv"); err != nil {
		t.Fatalf("a film described at a priority a busy computer starves: %v", err)
	}
}

// What the computer, not the film, kept from being done in time is told
// busy, so it is asked again later: a description that never came, a still
// that never came, and a description that waited past its time for its turn
// behind the voice's FFmpeg.
func TestWhatABusyComputerHeldBackIsBusy(t *testing.T) {
	self := standInPath(t)
	second := func() context.Context {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)
		return ctx
	}

	t.Setenv(standIn, "slow")
	if _, err := (ffmpeg.Stills{Path: self}).Open(second(), "film.mkv"); !errors.Is(err, thumbnail.ErrBusy) {
		t.Fatalf("a description that never came: %v, want thumbnail.ErrBusy", err)
	}

	t.Setenv(standIn, "still-never")
	film, err := (ffmpeg.Stills{Path: self}).Open(context.Background(), "film.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := film.Grab(second(), thumbnail.Shot{At: time.Minute}); !errors.Is(err, thumbnail.ErrBusy) {
		t.Fatalf("a still that never came: %v, want thumbnail.ErrBusy", err)
	}

	mark := filepath.Join(t.TempDir(), "holding")
	t.Setenv(standIn, "hold")
	t.Setenv(standInMark, mark)
	hold, release := context.WithCancel(context.Background())
	held := make(chan error, 1)
	go func() {
		held <- ffmpeg.Decoder{Path: self}.Decode(hold,
			voice.Stretch{File: "film.mkv", Channels: 2, Codec: "A_AAC", Length: time.Minute}, func([]float32, int) {})
	}()
	testkit.Eventually(t, "the voice's FFmpeg to hold the turn", func() bool {
		_, err := os.Stat(mark)
		return err == nil
	})
	_, err = (ffmpeg.Stills{Path: self}).Open(second(), "film.mkv")
	release()
	<-held
	if !errors.Is(err, thumbnail.ErrBusy) {
		t.Fatalf("a description that waited past its time for its turn: %v, want thumbnail.ErrBusy", err)
	}
}

// A run that reached its deadline was held back by the computer when it had
// less than half the processor it could take; one that worked the whole time
// was not, so a film too heavy for its stills is never tried for ever.
func TestStarvedTellsTheComputerFromTheWork(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		cpu, wall time.Duration
		share     float64
		want      bool
	}{
		{0, 15 * time.Second, 1, true},
		{7 * time.Second, 15 * time.Second, 1, true},
		{8 * time.Second, 15 * time.Second, 1, false},
		{15 * time.Second, 15 * time.Second, 1, false},
		// A still held to a quarter of a processor could take 3.75 s of 15.
		{time.Second, 15 * time.Second, 0.25, true},
		{2 * time.Second, 15 * time.Second, 0.25, false},
	} {
		if got := ffmpeg.Starved(c.cpu, c.wall, c.share); got != c.want {
			t.Errorf("Starved(%v of processor in %v, share %v) = %v, want %v", c.cpu, c.wall, c.share, got, c.want)
		}
	}
}
