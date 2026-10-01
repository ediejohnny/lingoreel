//go:build windows

package vlc_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	appsession "lingoreel/internal/application/session"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/vlc"
)

var sampleVideo = filepath.Join("..", "..", "..", "testdata", "video", "study-sample.mkv")

// fakeArgsEnv turns the test binary into a fake VLC that records its command
// line in the named file and exits at once.
const fakeArgsEnv = "LINGOREEL_FAKE_VLC_ARGS"

func TestMain(m *testing.M) {
	if record := os.Getenv(fakeArgsEnv); record != "" {
		if err := os.WriteFile(record, []byte(strings.Join(os.Args[1:], "\n")), 0o600); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// A relative file name that starts with "-" would reach VLC as an option;
// Launch hands VLC the absolute path instead.
func TestLaunchGivesVLCAnAbsoluteVideoPath(t *testing.T) {
	dir := t.TempDir()
	const name = "-Ifake.mkv"
	video := filepath.Join(dir, name)
	if err := os.WriteFile(video, []byte("not a video"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(dir, "args.txt")
	t.Chdir(dir)
	t.Setenv(fakeArgsEnv, record)

	session, err := vlc.Launch(t.Context(), os.Args[0], player.LaunchRequest{File: name, AudioIndex: -1})
	if err == nil {
		session.Close()
		t.Fatal("the fake VLC exits at once; Launch must fail")
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the fake VLC recorded nothing: %v", err)
	}
	args := strings.Split(string(data), "\n")
	if last := args[len(args)-1]; !strings.EqualFold(last, video) || !filepath.IsAbs(last) {
		t.Fatalf("video argument = %q, want the absolute %q", last, video)
	}
	for _, arg := range args {
		if arg == name {
			t.Fatalf("the relative name reached VLC: %q", args)
		}
	}
}

func TestArgumentsKeepControlLocalAndPlayerQuiet(t *testing.T) {
	request := player.LaunchRequest{File: `D:\Filmes\Um filme; & "teste".mkv`, AudioIndex: -1}
	args := vlc.Arguments(request, 51234, "s3cr3t", `C:\Temp\endpoint`)

	for _, pair := range [][2]string{
		{"--intf", "dummy"}, {"--extraintf", "http"}, {"--http-host", "127.0.0.1"},
		{"--http-port", "51234"}, {"--http-password", "s3cr3t"}, {"--http-src", `C:\Temp\endpoint`},
	} {
		index := slices.Index(args, pair[0])
		if index < 0 || index+1 >= len(args) || args[index+1] != pair[1] {
			t.Fatalf("argument %s %s missing in %q", pair[0], pair[1], args)
		}
	}
	for _, flag := range []string{"--no-spu", "--no-one-instance", "--no-metadata-network-access", "--play-and-pause", "--dummy-quiet"} {
		if !slices.Contains(args, flag) {
			t.Fatalf("flag %s missing in %q", flag, args)
		}
	}
	for _, flag := range []string{"--start-paused", "--audio-track", "--start-time"} {
		if slices.Contains(args, flag) {
			t.Fatalf("flag %s present without being requested", flag)
		}
	}
	if args[len(args)-1] != request.File {
		t.Fatalf("the video must be the last argument, unchanged; got %q", args[len(args)-1])
	}
}

func TestArgumentsForOptionalRequests(t *testing.T) {
	request := player.LaunchRequest{File: `C:\v.mkv`, AudioIndex: 1, StartPaused: true, Start: 90*time.Second + 250*time.Millisecond}
	args := strings.Join(vlc.Arguments(request, 1, "p", `C:\e`), " ")

	for _, want := range []string{"--start-paused", "--audio-track 1", "--start-time 90.250"} {
		if !strings.Contains(args, want) {
			t.Fatalf("arguments %q lack %q", args, want)
		}
	}
}

func TestArgumentsPlaceTheWindowOnlyWhenAsked(t *testing.T) {
	// 1152 of a 1920-pixel video: VLC sizes its window with a 0.6 zoom.
	placed := player.LaunchRequest{File: `C:\v.mkv`, AudioIndex: -1, VideoWidth: 1920,
		Window: player.Placement{X: 144, Y: -118, Width: 1152, Height: 624}}
	args := vlc.Arguments(placed, 1, "p", `C:\e`)
	joined := strings.Join(args, " ")
	for _, want := range []string{"--zoom 0.6000", "--video-x 144", "--video-y -118"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("arguments %q lack %q", joined, want)
		}
	}
	if args[len(args)-1] != `C:\v.mkv` {
		t.Fatalf("the file must stay last: %q", args)
	}
	// A video that already fits keeps its size: no zoom, never enlarged.
	small := placed
	small.VideoWidth = 320
	if joined := strings.Join(vlc.Arguments(small, 1, "p", `C:\e`), " "); strings.Contains(joined, "--zoom") {
		t.Fatalf("a small video was zoomed: %q", joined)
	}
	for _, window := range []player.Placement{{}, {Width: 1152}, {Height: 624}, {Width: -1, Height: 624}} {
		unplaced := player.LaunchRequest{File: `C:\v.mkv`, AudioIndex: -1, VideoWidth: 1920, Window: window}
		if joined := strings.Join(vlc.Arguments(unplaced, 1, "p", `C:\e`), " "); strings.Contains(joined, "--zoom") || strings.Contains(joined, "--video-x") {
			t.Fatalf("window %+v: arguments %q place the window", window, joined)
		}
	}
}

func TestLaunchRejectsMissingOrRelativePaths(t *testing.T) {
	existing, err := filepath.Abs(sampleVideo)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct{ executable, video string }{
		"relative executable": {"vlc.exe", existing},
		"missing executable":  {filepath.Join(t.TempDir(), "vlc.exe"), existing},
		"missing video":       {os.Args[0], filepath.Join(t.TempDir(), "missing.mkv")},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			session, err := vlc.Launch(context.Background(), tc.executable, player.LaunchRequest{File: tc.video, AudioIndex: -1})
			if err == nil {
				session.Close()
				t.Fatal("Launch accepted an invalid path")
			}
		})
	}
}

// waitFor reads samples until one satisfies the condition.
func waitFor(t *testing.T, samples <-chan playback.Sample, what string, condition func(playback.Sample) bool) playback.Sample {
	t.Helper()
	deadline := time.After(10 * time.Second)
	var last playback.Sample
	for {
		select {
		case sample, open := <-samples:
			if !open {
				t.Fatalf("VLC stopped reporting while waiting for %s", what)
			}
			last = sample
			if condition(sample) {
				return sample
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s; last sample %+v", what, last)
		}
	}
}

// realVLC returns the installed VLC and the sample video, skipping unless
// LINGOREEL_VLC_TEST=1 (the tests open a VLC window).
func realVLC(t *testing.T) (executable, video string) {
	t.Helper()
	if os.Getenv("LINGOREEL_VLC_TEST") != "1" {
		t.Skip("set LINGOREEL_VLC_TEST=1 to run against the installed VLC")
	}
	for _, installation := range discovery.Detect(discovery.SystemEnvironment{}, nil) {
		if installation.ID == player.VLC {
			executable = installation.Path
		}
	}
	if executable == "" {
		t.Fatal("VLC is not installed")
	}
	video, err := filepath.Abs(sampleVideo)
	if err != nil {
		t.Fatal(err)
	}
	return executable, video
}

// The very first sample must already describe the open video at the requested
// start: VLC reports 0 s (or no video at all) until it applies --start-time.
func TestVLCFirstSampleIsAtTheRequestedStart(t *testing.T) {
	executable, video := realVLC(t)
	cases := []struct {
		name  string
		start time.Duration
		want  time.Duration
	}{
		{"do começo", 0, 0},
		{"no meio", 6 * time.Second, 6 * time.Second},
		// VLC ignores a start beyond the end; callers should not ask for one,
		// but Launch must still return an open video instead of timing out.
		{"além do fim", time.Hour, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			session, err := vlc.Launch(t.Context(), executable, player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true, Start: c.start})
			if err != nil {
				t.Fatalf("Launch: %v", err)
			}
			defer session.Close()
			select {
			case first, open := <-session.Samples():
				if !open {
					t.Fatal("samples closed before the first one")
				}
				if first.Status == playback.StatusStopped || first.Duration < 19*time.Second || first.Duration > 21*time.Second {
					t.Fatalf("first sample = %+v, want the open 20 s video", first)
				}
				if diff := (first.Position - c.want).Abs(); diff > 500*time.Millisecond {
					t.Fatalf("first position = %v, want %v ± 500ms", first.Position, c.want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no sample after Launch")
			}
		})
	}
}

// Past 2^31 microseconds (35 min 47 s) a 32-bit format overflows: a real
// 90-minute movie reported a length of -2147483648 µs. The 40-minute sample
// must report positions and length beyond that limit.
func TestVLCReportsPositionsBeyond36Minutes(t *testing.T) {
	executable, _ := realVLC(t)
	video, err := filepath.Abs(filepath.Join(filepath.Dir(sampleVideo), "long-sample.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	start := 38 * time.Minute
	session, err := vlc.Launch(t.Context(), executable, player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true, Start: start})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer session.Close()
	select {
	case first := <-session.Samples():
		if first.Duration < 39*time.Minute || first.Duration > 41*time.Minute {
			t.Fatalf("duration = %v, want about 40 min", first.Duration)
		}
		// Frames are 2.4 s apart in this file: the position lands on one.
		if diff := (first.Position - start).Abs(); diff > 3*time.Second {
			t.Fatalf("position = %v, want about %v", first.Position, start)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no sample after Launch")
	}
	if err := session.SeekTo(39 * time.Minute); err != nil {
		t.Fatalf("SeekTo: %v", err)
	}
	waitFor(t, session.Samples(), "a seek past 36 minutes", func(s playback.Sample) bool {
		return (s.Position - 39*time.Minute).Abs() <= 3*time.Second
	})
}

// TestVLCIntegration drives the real VLC through its commands.
func TestVLCIntegration(t *testing.T) {
	executable, video := realVLC(t)

	session, err := vlc.Launch(context.Background(), executable, player.LaunchRequest{File: video, AudioIndex: 0, StartPaused: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer session.Close()
	samples := session.Samples()

	first := waitFor(t, samples, "the paused start", func(s playback.Sample) bool {
		return s.Status == playback.StatusPaused && s.Duration > 19*time.Second
	})
	if first.Duration > 21*time.Second {
		t.Fatalf("duration = %v, want about 20 s", first.Duration)
	}

	if err := session.SeekTo(4500 * time.Millisecond); err != nil {
		t.Fatalf("SeekTo: %v", err)
	}
	waitFor(t, samples, "the exact seek", func(s playback.Sample) bool {
		return s.Position == 4500*time.Millisecond
	})

	if err := session.SetSpeed(1.25); err != nil {
		t.Fatalf("SetSpeed: %v", err)
	}
	waitFor(t, samples, "the new speed", func(s playback.Sample) bool { return s.Speed == 1.25 })

	if err := session.Play(); err != nil {
		t.Fatalf("Play: %v", err)
	}
	waitFor(t, samples, "playback moving forward", func(s playback.Sample) bool {
		return s.Status == playback.StatusPlaying && s.Position > 5*time.Second
	})

	if err := session.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	waitFor(t, samples, "the pause", func(s playback.Sample) bool { return s.Status == playback.StatusPaused })
	if err := session.StepFrame(false); err == nil {
		t.Fatal("VLC reported a backward frame step, which it cannot do")
	}
	// The session emulates the backward step with an exact seek one frame
	// back (1/24 s), which VLC must land on exactly.
	if !session.Capabilities().ExactSeek {
		t.Fatal("VLC must declare exact seeks")
	}
	oneFrameBack := 4500*time.Millisecond - 41_667*time.Microsecond
	if err := session.SeekTo(oneFrameBack); err != nil {
		t.Fatalf("SeekTo one frame back: %v", err)
	}
	waitFor(t, samples, "the seek one frame back", func(s playback.Sample) bool {
		return s.Position == oneFrameBack && s.Status == playback.StatusPaused
	})

	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case _, open := <-samples:
		for open {
			_, open = <-samples
		}
	case <-time.After(5 * time.Second):
		t.Fatal("samples kept flowing after Close")
	}
	if err := session.Err(); err != nil {
		t.Fatalf("Err after a normal Close = %v, want nil", err)
	}
}

// M10 (RNF-006): how far the session clock drifts from VLC between reports.
// VLC 3's own time is an estimate, so this measures our extrapolation against
// VLC's next report (at 1x, 2x and 0.75x), not the true error on screen; mpv,
// which reports exact positions, gets the absolute measurement (F11).
func TestVLCClockStaysCloseToVLCsReports(t *testing.T) {
	executable, video := realVLC(t)
	session, err := vlc.Launch(t.Context(), executable, player.LaunchRequest{File: video, AudioIndex: -1})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer session.Close()

	clock := appsession.NewClock(playback.PrecisionEstimated, time.Now)
	var errors []time.Duration
	measure := func(speed playback.Speed, length time.Duration) {
		if speed != 1 {
			if err := session.SetSpeed(speed); err != nil {
				t.Fatalf("SetSpeed(%v): %v", speed, err)
			}
			clock.SetSpeed(speed)
		}
		settleUntil := time.Now().Add(time.Second) // VLC takes a moment to apply the speed
		deadline := time.Now().Add(length)
		last := time.Duration(-1)
		for time.Now().Before(deadline) {
			sample, open := <-session.Samples()
			if !open {
				t.Fatal("VLC stopped reporting")
			}
			fresh := sample.Position != last
			last = sample.Position
			if fresh && sample.Status == playback.StatusPlaying && time.Now().After(settleUntil) {
				state := clock.State()
				predicted := state.Position - time.Duration(float64(time.Since(sample.ObservedAt))*float64(state.Speed))
				errors = append(errors, (predicted - sample.Position).Abs())
			}
			clock.Observe(sample)
		}
	}
	measure(1, 5*time.Second)
	measure(2, 3*time.Second)
	measure(0.75, 3*time.Second)

	if len(errors) < 10 {
		t.Fatalf("only %d comparisons; VLC reported too rarely", len(errors))
	}
	slices.Sort(errors)
	p50, p95, worst := errors[len(errors)/2], errors[len(errors)*95/100], errors[len(errors)-1]
	t.Logf("clock vs VLC over %d reports: p50 %v, p95 %v, worst %v", len(errors), p50, p95, worst)
	// Measured on 2026-10-01 (VLC 3.0.23, three runs): p50 33-36 ms, p95
	// 101-105 ms, worst 102-129 ms. The median meets RNF-006 (< 50 ms); the
	// tail does not, because VLC's own reports jitter. The ceilings catch a
	// regression with room for a loaded machine.
	if p50 > 60*time.Millisecond || p95 > 200*time.Millisecond {
		t.Fatalf("clock drifted from VLC: p50 %v (max 60ms), p95 %v (max 200ms)", p50, p95)
	}
}

// A VLC that dies must not look like one the user closed.
func TestVLCCrashIsReportedAsLost(t *testing.T) {
	executable, video := realVLC(t)
	session, err := vlc.Launch(t.Context(), executable, player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer session.Close()
	process, err := os.FindProcess(session.ProcessID())
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Kill(); err != nil {
		t.Fatalf("killing VLC: %v", err)
	}
	deadline := time.After(10 * time.Second)
	for open := true; open; {
		select {
		case _, open = <-session.Samples():
		case <-deadline:
			t.Fatal("samples still open after VLC died")
		}
	}
	if err := session.Err(); !errors.Is(err, playback.ErrPlayerLost) {
		t.Fatalf("Err after a crash = %v, want ErrPlayerLost", err)
	}
}

// The HTTP password sits on VLC's command line, which other programs can
// read. Reading it the same way, a request without the session token must be
// refused.
func TestVLCRefusesRequestsWithoutTheSessionToken(t *testing.T) {
	executable, video := realVLC(t)
	session, err := vlc.Launch(t.Context(), executable, player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer session.Close()

	query := fmt.Sprintf("(Get-CimInstance Win32_Process -Filter 'ProcessId=%d').CommandLine", session.ProcessID())
	out, err := exec.Command("powershell", "-NoProfile", "-Command", query).Output()
	if err != nil {
		t.Fatalf("reading VLC's command line: %v", err)
	}
	line := string(out)
	port := regexp.MustCompile(`--http-port (\d+)`).FindStringSubmatch(line)
	password := regexp.MustCompile(`--http-password ([0-9a-f]+)`).FindStringSubmatch(line)
	if port == nil || password == nil {
		t.Fatalf("port or password not found in %q", line)
	}
	for name, token := range map[string]string{"sem token": "", "token errado": "0123456789abcdef"} {
		t.Run(name, func(t *testing.T) {
			target := "http://127.0.0.1:" + port[1] + "/requests/lingoreel.json?command=pause"
			if token != "" {
				target += "&token=" + token
			}
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.SetBasicAuth("", password[1])
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
			if !strings.Contains(string(body), `"denied"`) {
				t.Fatalf("answer without the token = %s %q, want denied", response.Status, body)
			}
		})
	}
}

// A port that was free when picked may be taken before VLC binds it; the
// program there does not know the endpoint, so Launch tries another port.
func TestVLCLaunchMovesOffATakenPort(t *testing.T) {
	executable, video := realVLC(t)
	squatter := httptest.NewServer(http.NotFoundHandler())
	defer squatter.Close()
	taken := squatter.Listener.Addr().(*net.TCPAddr).Port

	picks := 0
	pick := func() (int, error) {
		picks++
		if picks == 1 {
			return taken, nil
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
		defer listener.Close()
		return listener.Addr().(*net.TCPAddr).Port, nil
	}
	session, err := vlc.LaunchWith(t.Context(), executable, player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true}, vlc.Options{PickPort: pick})
	if err != nil {
		t.Fatalf("LaunchWith: %v", err)
	}
	defer session.Close()
	if picks != 2 {
		t.Fatalf("ports picked = %d, want 2 (the taken one, then a free one)", picks)
	}
	waitFor(t, session.Samples(), "a sample from the second VLC", func(s playback.Sample) bool { return s.Duration > 19*time.Second })
}
