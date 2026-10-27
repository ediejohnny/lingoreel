//go:build windows

package ffmpeg_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
)

// standIn makes the test binary play an FFmpeg: "priority" answers
// "-version" only at a priority a busy computer still serves, "slow" never
// answers.
const standIn = "LINGOREEL_TEST_FFMPEG_STAND_IN"

func TestMain(m *testing.M) {
	switch os.Getenv(standIn) {
	case "priority":
		class, err := windows.GetPriorityClass(windows.CurrentProcess())
		if err == nil && class != windows.IDLE_PRIORITY_CLASS && class != windows.BELOW_NORMAL_PRIORITY_CLASS {
			fmt.Println("ffmpeg version stand-in")
		}
		os.Exit(0)
	case "slow":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// An FFmpeg that does not answer in time is told slow, not missing: on a
// busy computer it may well be there, and it is asked again later.
func TestAnFFmpegThatDoesNotAnswerInTimeIsSlowNotMissing(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(standIn, "slow")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := ffmpeg.Find(ctx, self); !errors.Is(err, ffmpeg.ErrSlow) {
		t.Fatalf("an FFmpeg that did not answer in time: %v, want ffmpeg.ErrSlow", err)
	}
}

// The check that an FFmpeg runs is not held to the idle priority its work
// runs at: with both processors of a school computer busy, a program at that
// priority waits tens of seconds for a turn, and an FFmpeg that is there
// would be told missing.
func TestTheCheckRunsAtAPriorityABusyComputerServes(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(standIn, "priority")
	if err := ffmpeg.Check(context.Background(), self); err != nil {
		t.Fatalf("an FFmpeg checked at a priority a busy computer starves: %v", err)
	}
}
