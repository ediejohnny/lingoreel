package lrdev_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/lrdev"
)

var studySample = filepath.Join("..", "testdata", "video", "study-sample.mkv")

// Options are checked before the player starts, so a missing file proves an
// option was accepted and ErrInvalidOption proves it was refused.
func TestPlayValidatesOptionsBeforeStartingAPlayer(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.mkv")
	cases := []struct {
		name string
		args []string
		want error
	}{
		{"no video", []string{"play"}, lrdev.ErrMissingArgument},
		{"options only", []string{"play", "--paused"}, lrdev.ErrMissingArgument},
		{"unknown option", []string{"play", missing, "--fast"}, lrdev.ErrInvalidOption},
		{"two videos", []string{"play", missing, missing}, lrdev.ErrInvalidOption},
		{"player without a value", []string{"play", missing, "--player"}, lrdev.ErrInvalidOption},
		{"start without a value", []string{"play", missing, "--start"}, lrdev.ErrInvalidOption},
		{"negative start", []string{"play", missing, "--start", "-0.001"}, lrdev.ErrInvalidOption},
		{"start past the limit", []string{"play", missing, "--start", "86400.001"}, lrdev.ErrInvalidOption},
		{"start NaN", []string{"play", missing, "--start", "NaN"}, lrdev.ErrInvalidOption},
		{"start infinite", []string{"play", missing, "--start", "Inf"}, lrdev.ErrInvalidOption},
		{"start overflowing", []string{"play", missing, "--start", "1e400"}, lrdev.ErrInvalidOption},
		{"start as text", []string{"play", missing, "--start", "abc"}, lrdev.ErrInvalidOption},
		{"empty start", []string{"play", missing, "--start", ""}, lrdev.ErrInvalidOption},
		{"start with a unit", []string{"play", missing, "--start", "5s"}, lrdev.ErrInvalidOption},
		{"start zero taken", []string{"play", missing, "--start", "0"}, os.ErrNotExist},
		{"start at the limit taken", []string{"play", missing, "--start", "86400"}, os.ErrNotExist},
		{"start with a comma taken", []string{"play", missing, "--start", "2,5"}, os.ErrNotExist},
		{"options before the video", []string{"play", "--paused", "--player", "VLC", missing}, os.ErrNotExist},
		{"an unknown player does not stop the analysis", []string{"play", missing, "--player", "xyz"}, os.ErrNotExist},
		{"step without a value", []string{"play", missing, "--step"}, lrdev.ErrInvalidOption},
		{"step below 10 ms", []string{"play", missing, "--step", "9"}, lrdev.ErrInvalidOption},
		{"step above 1 s", []string{"play", missing, "--step", "1001"}, lrdev.ErrInvalidOption},
		{"negative step", []string{"play", missing, "--step", "-50"}, lrdev.ErrInvalidOption},
		{"step with a unit", []string{"play", missing, "--step", "50ms"}, lrdev.ErrInvalidOption},
		{"decimal step", []string{"play", missing, "--step", "50,5"}, lrdev.ErrInvalidOption},
		{"least step taken", []string{"play", missing, "--step", "10"}, os.ErrNotExist},
		{"largest step taken", []string{"play", missing, "--step", "1000"}, os.ErrNotExist},
		{"unsupported format", []string{"play", "movie.txt"}, analysis.ErrUnsupportedFile},
		// The sample lasts 20.052 s: starting at or past its end is refused
		// after the analysis and before any player is looked for.
		{"start at the end of the video", []string{"play", studySample, "--start", "20.052"}, lrdev.ErrInvalidOption},
		{"start past the end of the video", []string{"play", studySample, "--start", "25"}, lrdev.ErrInvalidOption},
		// overlay shares play's validation.
		{"overlay without a video", []string{"overlay"}, lrdev.ErrMissingArgument},
		{"overlay with an unknown option", []string{"overlay", missing, "--fast"}, lrdev.ErrInvalidOption},
		{"overlay with a NaN start", []string{"overlay", missing, "--start", "NaN"}, lrdev.ErrInvalidOption},
		{"overlay takes valid options", []string{"overlay", missing, "--paused", "--start", "2"}, os.ErrNotExist},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A cancelled context stops play at its first blocking step, so a
			// validation that wrongly lets a case through fails at once instead
			// of opening a real player.
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			var out bytes.Buffer
			err := lrdev.Run(ctx, c.args, &out)
			if !errors.Is(err, c.want) {
				t.Fatalf("Run(%q) = %v, want %v", c.args, err, c.want)
			}
		})
	}
}

func TestHelpListsPlay(t *testing.T) {
	var out bytes.Buffer
	if err := lrdev.Run(t.Context(), []string{"help"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"play <video>", "overlay <video>", "--player", "--start", "--paused", "--step", "Shift+, / Shift+.", "Ctrl+, / Ctrl+."} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help does not mention %q:\n%s", want, out.String())
		}
	}
}

// changeLine matches one printed change: "[h:mm:ss.mmm] text".
var changeLine = regexp.MustCompile(`^\[(\d+):(\d{2}):(\d{2})(?:\.(\d{3}))?\] (.*)$`)

type change struct {
	at   time.Duration
	text string
	// wall is when the line was written, an oracle independent of the
	// session clock.
	wall time.Time
}

// stampedWriter records the wall-clock time of every write; lrdev writes one
// line per write.
type stampedWriter struct {
	mu    sync.Mutex
	all   bytes.Buffer
	lines []stampedLine
}

type stampedLine struct {
	text string
	at   time.Time
}

func (w *stampedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	for _, line := range strings.Split(strings.TrimRight(string(p), "\r\n"), "\n") {
		w.lines = append(w.lines, stampedLine{text: strings.TrimRight(line, "\r"), at: now})
	}
	return w.all.Write(p)
}

func (w *stampedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.all.String()
}

func parseChanges(t *testing.T, lines []stampedLine) []change {
	t.Helper()
	var changes []change
	for _, line := range lines {
		m := changeLine.FindStringSubmatch(line.text)
		if m == nil {
			continue
		}
		hours, _ := strconv.Atoi(m[1])
		minutes, _ := strconv.Atoi(m[2])
		seconds, _ := strconv.Atoi(m[3])
		millis := 0
		if m[4] != "" {
			millis, _ = strconv.Atoi(m[4])
		}
		at := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute +
			time.Duration(seconds)*time.Second + time.Duration(millis)*time.Millisecond
		changes = append(changes, change{at: at, text: m[5], wall: line.at})
	}
	return changes
}

// TestPlayFollowsSubtitlesWithRealVLC opens the study sample in the real VLC
// and checks that each subtitle appears near its start time, in order.
func TestPlayFollowsSubtitlesWithRealVLC(t *testing.T) {
	if os.Getenv("LINGOREEL_VLC_TEST") != "1" {
		t.Skip("set LINGOREEL_VLC_TEST=1 to run against the installed VLC")
	}
	// Starting at 3 s covers the end of one line, a gap, two more lines and
	// the gap after them (the lines of the sample end at 9 s).
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	out := &stampedWriter{}
	if err := lrdev.Run(ctx, []string{"play", studySample, "--player", "vlc", "--start", "3"}, out); err != nil {
		t.Fatalf("play: %v\n%s", err, out.String())
	}
	output := out.String()
	for _, want := range []string{"Player:       VLC", "Top line:     #3 (en)", "Bottom line:  #4 (pt-BR)", "Precision:    estimated", "Session ended."} {
		if !strings.Contains(output, want) {
			t.Errorf("output lacks %q:\n%s", want, output)
		}
	}

	// The first report must already be at the start position: nothing may be
	// printed for 0 s before VLC answers.
	expected := []change{
		{at: 3 * time.Second, text: "en: I'm gonna have to think about it.  │  pt-BR: Vou ter que pensar sobre isso."},
		{at: 3500 * time.Millisecond, text: "—"},
		{at: 4 * time.Second, text: "en: That's sick!  │  pt-BR: Que loucura!"},
		{at: 6 * time.Second, text: "—"},
		{at: 7250 * time.Millisecond, text: "en: I'm not going there.  │  pt-BR: Eu não vou para lá. / Não mesmo."},
		{at: 9 * time.Second, text: "—"},
	}
	out.mu.Lock()
	changes := parseChanges(t, out.lines)
	out.mu.Unlock()
	if len(changes) != len(expected) {
		t.Fatalf("got %d changes, want %d:\n%s", len(changes), len(expected), output)
	}

	// The printed position is the session clock's own reading when the change
	// fires, so it only proves the runner wakes at the boundaries of that
	// clock; how far the clock is from VLC's picture is measured in the
	// real-VLC tests. The wall-clock gaps are the independent check: at 1x,
	// the real time between two changes must match the video time between
	// them, which fails if the clock drifts from VLC or jumps on re-anchoring.
	const clockTolerance = 150 * time.Millisecond
	const wallTolerance = 300 * time.Millisecond
	for i, want := range expected {
		got := changes[i]
		if got.text != want.text {
			t.Errorf("change %d text = %q, want %q", i, got.text, want.text)
		}
		if diff := (got.at - want.at).Abs(); diff > clockTolerance {
			t.Errorf("change %d at %v, want %v ± %v", i, got.at, want.at, clockTolerance)
		}
		if i == 0 {
			continue
		}
		videoGap := want.at - expected[i-1].at
		wallGap := got.wall.Sub(changes[i-1].wall)
		if diff := (wallGap - videoGap).Abs(); diff > wallTolerance {
			t.Errorf("real time between changes %d and %d = %v, video time = %v (± %v)", i-1, i, wallGap, videoGap, wallTolerance)
		}
	}
}
