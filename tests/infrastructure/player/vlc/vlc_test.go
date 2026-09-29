//go:build windows

package vlc_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/vlc"
)

var sampleVideo = filepath.Join("..", "..", "..", "testdata", "video", "study-sample.mkv")

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

// TestVLCIntegration drives the real VLC. It opens a VLC window, so it only
// runs when LINGOREEL_VLC_TEST=1.
func TestVLCIntegration(t *testing.T) {
	if os.Getenv("LINGOREEL_VLC_TEST") != "1" {
		t.Skip("set LINGOREEL_VLC_TEST=1 to run against the installed VLC")
	}
	var executable string
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

	session.Close()
	select {
	case _, open := <-samples:
		for open {
			_, open = <-samples
		}
	case <-time.After(5 * time.Second):
		t.Fatal("samples kept flowing after Close")
	}
}
