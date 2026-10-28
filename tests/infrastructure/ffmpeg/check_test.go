//go:build windows

package ffmpeg_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
)

// An FFmpeg that does not answer in time is told slow, not missing: on a
// busy computer it may well be there, and it is asked again later. Only
// places that hold no FFmpeg at all are told missing. (The search is given
// its places, all in the test's own folder: no test looks around the
// computer for an FFmpeg.)
func TestAnFFmpegThatDoesNotAnswerInTimeIsSlowNotMissing(t *testing.T) {
	self := standInPath(t)
	nothing := filepath.Join(t.TempDir(), "ffmpeg.exe")
	t.Setenv(standIn, "slow")
	if _, err := ffmpeg.FindAmong(context.Background(), []string{"", nothing}); !errors.Is(err, ffmpeg.ErrNotFound) {
		t.Fatalf("no FFmpeg anywhere: %v, want ffmpeg.ErrNotFound", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := ffmpeg.FindAmong(ctx, []string{nothing, self}); !errors.Is(err, ffmpeg.ErrSlow) {
		t.Fatalf("an FFmpeg that did not answer in time: %v, want ffmpeg.ErrSlow", err)
	}
}

// A search waits only a few seconds at each place, with nothing else bounding
// it: an FFmpeg that never answers is told slow long before it would end.
// (Check alone waits as long as it is let: a file the person chose may take
// its time on its first opening.)
func TestASearchWaitsBrieflyAtEachPlace(t *testing.T) {
	self := standInPath(t)
	t.Setenv(standIn, "slow")
	started := time.Now()
	if _, err := ffmpeg.FindAmong(context.Background(), []string{self}); !errors.Is(err, ffmpeg.ErrSlow) {
		t.Fatalf("an FFmpeg that never answers: %v, want ffmpeg.ErrSlow", err)
	}
	if waited := time.Since(started); waited > 30*time.Second {
		t.Fatalf("the search waited %v at one place", waited)
	}
}

// The voice's hearing is told the same: a decoder the computer was too busy
// to start is busy, and looked for again; none at all is not.
func TestADecoderThatDoesNotAnswerInTimeIsBusyNotMissing(t *testing.T) {
	self := standInPath(t)
	nothing := filepath.Join(t.TempDir(), "ffmpeg.exe")
	t.Setenv(standIn, "slow")
	if _, err := ffmpeg.DecoderAmong(context.Background(), []string{nothing}); err == nil || errors.Is(err, voice.ErrDecoderBusy) {
		t.Fatalf("no FFmpeg anywhere: %v, want an error that is not voice.ErrDecoderBusy", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if decoder, err := ffmpeg.DecoderAmong(ctx, []string{nothing, self}); decoder != nil || !errors.Is(err, voice.ErrDecoderBusy) {
		t.Fatalf("an FFmpeg that did not answer in time: %v, %v; want no decoder and voice.ErrDecoderBusy", decoder, err)
	}
}

// The check that an FFmpeg runs is not held to the idle priority its work
// runs at: with both processors of a school computer busy, a program at that
// priority waits tens of seconds for a turn, and an FFmpeg that is there
// would be told missing.
func TestTheCheckRunsAtAPriorityABusyComputerServes(t *testing.T) {
	self := standInPath(t)
	t.Setenv(standIn, "priority")
	if err := ffmpeg.Check(context.Background(), self); err != nil {
		t.Fatalf("an FFmpeg checked at a priority a busy computer starves: %v", err)
	}
}
