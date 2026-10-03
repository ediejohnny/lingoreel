//go:build windows

package potplayer_test

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/potplayer"
	"lingoreel/tests/infrastructure/player/plugintest"
)

var sampleVideo = filepath.Join("..", "..", "..", "testdata", "video", "study-sample.mkv")

// fakeArgsEnv turns the test binary into a fake PotPlayer that records its
// command line in the named file and exits at once.
const fakeArgsEnv = "LINGOREEL_FAKE_POTPLAYER_ARGS"

func TestMain(m *testing.M) {
	if record := os.Getenv(fakeArgsEnv); record != "" {
		if err := os.WriteFile(record, []byte(strings.Join(os.Args[1:], "\n")), 0o600); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestPotPlayerPluginDescriptor(t *testing.T) {
	plugin := potplayer.Plugin{}
	plugintest.Descriptor(t, plugin)
	descriptor := plugin.Descriptor()
	if descriptor.ID != player.PotPlayer || descriptor.DrawsInHost {
		t.Fatalf("descriptor %+v: PotPlayer draws in its own window", descriptor)
	}
	if !descriptor.FixedAudioTrack {
		t.Fatal("PotPlayer has no message for the audio track; the descriptor must say so")
	}
	hints := plugin.Hints()
	if hints.ID != descriptor.ID || len(hints.Executables) == 0 {
		t.Fatalf("hints %+v do not match the descriptor", hints)
	}
	for _, executable := range hints.Executables {
		if !strings.HasPrefix(strings.ToLower(executable), "potplayer") {
			t.Fatalf("hint %q points at another program", executable)
		}
	}
}

// PotPlayer must no longer be listed among the players without a plugin, or
// the app would offer it twice and call it "coming soon".
func TestPotPlayerIsNoLongerAPlayerWithoutAPlugin(t *testing.T) {
	for _, hints := range discovery.Known() {
		if hints.ID == player.PotPlayer {
			t.Fatal("PotPlayer has a plugin now; its hints belong to the plugin")
		}
	}
}

func TestArgumentsOpenTheirOwnWindowAndHidePotPlayersSubtitles(t *testing.T) {
	blank := `C:\Temp\sp\blank.srt`
	args := potplayer.Arguments(player.LaunchRequest{File: `D:\Filmes\Um filme; & "teste".mkv`, AudioIndex: -1}, blank)

	if args[0] != `D:\Filmes\Um filme; & "teste".mkv` {
		t.Fatalf("the video must come first, got %q", args)
	}
	if !slices.Contains(args, "/new") {
		t.Fatalf("without /new PotPlayer reuses another window: %q", args)
	}
	if !slices.Contains(args, "/sub="+blank) {
		t.Fatalf("without the blank subtitle PotPlayer draws its own: %q", args)
	}
	// Without a blank subtitle there is no empty option left behind.
	for _, arg := range potplayer.Arguments(player.LaunchRequest{File: `C:\v.mkv`}, "") {
		if strings.HasPrefix(arg, "/sub=") {
			t.Fatalf("a launch without a blank subtitle passed %q", arg)
		}
	}
}

// A relative file name could reach PotPlayer as an option; Launch hands it
// the absolute path instead.
func TestLaunchGivesPotPlayerAnAbsoluteVideoPath(t *testing.T) {
	dir := t.TempDir()
	const name = "video.mkv"
	video := filepath.Join(dir, name)
	if err := os.WriteFile(video, []byte("not a video"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(dir, "args.txt")
	t.Chdir(dir)
	t.Setenv(fakeArgsEnv, record)

	session, err := potplayer.Launch(t.Context(), os.Args[0], player.LaunchRequest{File: name, AudioIndex: -1})
	if err == nil {
		session.Close()
		t.Fatal("the fake PotPlayer exits at once; Launch must fail")
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the fake PotPlayer recorded nothing: %v", err)
	}
	args := strings.Split(string(data), "\n")
	if !strings.EqualFold(args[0], video) || !filepath.IsAbs(args[0]) {
		t.Fatalf("video argument = %q, want the absolute %q", args[0], video)
	}
}

// A launch that fails must leave no blank subtitle behind.
func TestAFailedLaunchRemovesItsTemporaryFolder(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mkv")
	if err := os.WriteFile(video, []byte("not a video"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeArgsEnv, filepath.Join(dir, "args.txt"))
	before := temporaryFolders(t)
	session, err := potplayer.Launch(t.Context(), os.Args[0], player.LaunchRequest{File: video, AudioIndex: -1})
	if err == nil {
		session.Close()
		t.Fatal("the fake PotPlayer exits at once; Launch must fail")
	}
	if after := temporaryFolders(t); after > before {
		t.Fatalf("%d temporary folder(s) left behind", after-before)
	}
}

func temporaryFolders(t *testing.T) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "lingoreel-potplayer-*"))
	if err != nil {
		t.Fatal(err)
	}
	return len(matches)
}

// realPotPlayer finds the installed PotPlayer. These tests show its window
// on the screen: it refuses to be placed outside it (measured).
func realPotPlayer(t *testing.T) (executable, video string) {
	t.Helper()
	if os.Getenv("LINGOREEL_POTPLAYER_TEST") != "1" {
		t.Skip("set LINGOREEL_POTPLAYER_TEST=1 to run against the installed PotPlayer (its window appears on the screen)")
	}
	hints := []discovery.Hints{potplayer.Plugin{}.Hints()}
	for _, installation := range discovery.Detect(discovery.SystemEnvironment{}, hints, nil) {
		if installation.ID == player.PotPlayer {
			executable = installation.Path
		}
	}
	if executable == "" {
		t.Fatal("PotPlayer is not installed")
	}
	video, err := filepath.Abs(sampleVideo)
	if err != nil {
		t.Fatal(err)
	}
	return executable, video
}

// The PotPlayer plugin passes the conformance kit every plugin must pass.
func TestPotPlayerPluginPassesTheKit(t *testing.T) {
	executable, video := realPotPlayer(t)
	plugintest.Launch(t, potplayer.Plugin{}, executable, video)
}

// The first sample must already be at the requested start and paused:
// publishing the opening moment would show the subtitles of the wrong one.
func TestPotPlayerFirstSampleIsAtTheRequestedStartAndPaused(t *testing.T) {
	executable, video := realPotPlayer(t)
	cases := []struct {
		name  string
		start time.Duration
		want  time.Duration
	}{
		{"do começo", 0, 0},
		{"no meio", 6 * time.Second, 6 * time.Second},
		{"depois do fim", time.Hour, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			session, err := potplayer.Launch(t.Context(), executable,
				player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true, Start: c.start})
			if err != nil {
				t.Fatalf("Launch: %v", err)
			}
			defer session.Close()
			sample, open := <-session.Samples()
			if !open {
				t.Fatal("PotPlayer reported nothing")
			}
			if (sample.Position - c.want).Abs() > time.Second {
				t.Fatalf("first position %v, want about %v", sample.Position, c.want)
			}
			if sample.Status != playback.StatusPaused {
				t.Fatalf("first status %q, want paused", sample.Status)
			}
			if sample.Duration <= 0 {
				t.Fatalf("first duration %v, want the video's length", sample.Duration)
			}
		})
	}
}

// Every command must reach PotPlayer and move it.
func TestPotPlayerTakesEveryDeclaredCommand(t *testing.T) {
	executable, video := realPotPlayer(t)
	session, err := potplayer.Launch(t.Context(), executable,
		player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer session.Close()

	if err := session.SeekTo(4 * time.Second); err != nil {
		t.Fatalf("SeekTo: %v", err)
	}
	at := waitForPosition(t, session, 4*time.Second, time.Second)
	if err := session.StepFrame(true); err != nil {
		t.Fatalf("StepFrame: %v", err)
	}
	after := waitForChange(t, session, at)
	if after <= at {
		t.Fatalf("one frame forward left the position at %v", after)
	}
	if err := session.StepFrame(false); err == nil {
		t.Fatal("PotPlayer has no backward frame step; StepFrame(false) must fail")
	}
	if err := session.SelectAudioTrack(0); err == nil {
		t.Fatal("PotPlayer cannot pick the audio track; SelectAudioTrack must fail")
	}
	// The speed goes to the nearest step of 0.1 and the samples say which.
	if err := session.SetSpeed(1.5); err != nil {
		t.Fatalf("SetSpeed: %v", err)
	}
	if sample := nextSample(t, session); math.Abs(float64(sample.Speed)-1.5) > 0.051 {
		t.Fatalf("speed reported %v, want about 1.5", sample.Speed)
	}
	if err := session.SetSpeed(1); err != nil {
		t.Fatalf("SetSpeed(1): %v", err)
	}
	if sample := nextSample(t, session); sample.Speed != playback.NormalSpeed {
		t.Fatalf("speed reported %v, want 1", sample.Speed)
	}
	if err := session.SetVolume(playback.MaxVolume / 2); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if err := session.Play(); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if status := waitForStatus(t, session, playback.StatusPlaying); status != playback.StatusPlaying {
		t.Fatalf("status after Play = %q", status)
	}
	if err := session.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if status := waitForStatus(t, session, playback.StatusPaused); status != playback.StatusPaused {
		t.Fatalf("status after Pause = %q", status)
	}
}

// The subtitles go over the picture, not over PotPlayer's playlist and bar.
func TestPotPlayerNamesItsWindowAndThePictureInsideIt(t *testing.T) {
	executable, video := realPotPlayer(t)
	session, err := potplayer.Launch(t.Context(), executable,
		player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true,
			Window: player.Placement{X: 80, Y: 80, Width: 960, Height: 600}})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer session.Close()

	main := windows.HWND(session.Window())
	if main == 0 || !windows.IsWindow(main) {
		t.Fatalf("Window = %#x, want PotPlayer's main window", session.Window())
	}
	area := windows.HWND(session.VideoArea())
	if area == 0 {
		t.Fatal("VideoArea = 0, want the window holding the picture")
	}
	if area == main {
		t.Fatal("VideoArea is the main window: the playlist and the bar would be covered")
	}
	if parent := rootOf(area); parent != main {
		t.Fatalf("VideoArea %#x is not inside the main window %#x", area, parent)
	}
	if !windows.IsWindowVisible(area) {
		t.Fatal("VideoArea is hidden")
	}
	// Closing clears both, so nothing is followed afterwards.
	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if session.Window() != 0 || session.VideoArea() != 0 {
		t.Fatalf("after Close: window %#x, area %#x; want none", session.Window(), session.VideoArea())
	}
}

var procGetAncestor = windows.NewLazySystemDLL("user32.dll").NewProc("GetAncestor")

// rootOf returns the top-level window a child belongs to.
func rootOf(hwnd windows.HWND) windows.HWND {
	const gaRoot = 2
	root, _, _ := procGetAncestor.Call(uintptr(hwnd), gaRoot)
	return windows.HWND(root)
}

func nextSample(t *testing.T, session *potplayer.Session) playback.Sample {
	t.Helper()
	select {
	case sample, open := <-session.Samples():
		if !open {
			t.Fatal("PotPlayer stopped reporting")
		}
		return sample
	case <-time.After(5 * time.Second):
		t.Fatal("PotPlayer reported nothing in 5 s")
	}
	return playback.Sample{}
}

func waitForPosition(t *testing.T, session *potplayer.Session, want, tolerance time.Duration) time.Duration {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	last := time.Duration(0)
	for time.Now().Before(deadline) {
		last = nextSample(t, session).Position
		if (last - want).Abs() <= tolerance {
			return last
		}
	}
	t.Fatalf("PotPlayer stayed at %v, want about %v", last, want)
	return last
}

func waitForChange(t *testing.T, session *potplayer.Session, from time.Duration) time.Duration {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if position := nextSample(t, session).Position; position != from {
			return position
		}
	}
	t.Fatalf("the position stayed at %v", from)
	return from
}

func waitForStatus(t *testing.T, session *potplayer.Session, want playback.Status) playback.Status {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	status := playback.StatusUnknown
	for time.Now().Before(deadline) {
		if status = nextSample(t, session).Status; status == want {
			return status
		}
	}
	return status
}
