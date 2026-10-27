//go:build windows

package ffmpeg_test

import (
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// standIn makes the test binary play an FFmpeg:
//   - "priority" answers "-version" only at a priority a busy computer
//     still serves;
//   - "slow" never answers;
//   - "describe" describes a film only at a priority a busy computer still
//     serves, and never answers at a lower one;
//   - "still-never" describes a film at once, but a still never comes;
//   - "hold" leaves the file standInMark names, then never answers.
const (
	standIn     = "LINGOREEL_TEST_FFMPEG_STAND_IN"
	standInMark = "LINGOREEL_TEST_FFMPEG_MARK"
)

// standInFilm is what FFmpeg tells of a film it was given no output for.
const standInFilm = "Input #0, matroska,webm, from 'film.mkv':\n" +
	"  Stream #0:0: Video: h264 (High), yuv420p(tv, bt709, progressive), 1920x1080 [SAR 1:1 DAR 16:9], 23.98 fps\n" +
	"At least one output file must be specified\n"

func TestMain(m *testing.M) {
	switch os.Getenv(standIn) {
	case "":
		os.Exit(m.Run())
	case "priority":
		if served() {
			fmt.Println("ffmpeg version stand-in")
		}
	case "describe":
		if !served() {
			time.Sleep(time.Minute)
		}
		describeFilm()
	case "still-never":
		if slices.Contains(os.Args, "-frames:v") {
			time.Sleep(time.Minute)
		}
		describeFilm()
	case "hold":
		_ = os.WriteFile(os.Getenv(standInMark), nil, 0o600)
		time.Sleep(time.Minute)
	default: // "slow"
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}

// served tells a process at a priority a computer whose processors are busy
// with ordinary programs still gives turns to.
func served() bool {
	class, err := windows.GetPriorityClass(windows.CurrentProcess())
	return err == nil && class != windows.IDLE_PRIORITY_CLASS && class != windows.BELOW_NORMAL_PRIORITY_CLASS
}

// describeFilm tells the film on the error output and ends as FFmpeg does
// without an output.
func describeFilm() {
	fmt.Fprint(os.Stderr, standInFilm)
	os.Exit(1)
}

// standInPath is the test binary, to run as a stand-in FFmpeg.
func standInPath(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return self
}
