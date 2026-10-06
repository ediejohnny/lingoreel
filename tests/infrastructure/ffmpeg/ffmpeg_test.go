package ffmpeg_test

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// value returns the argument after a flag, "" when the flag is missing.
func value(args []string, flag string) string {
	if i := slices.Index(args, flag); i >= 0 && i+1 < len(args) {
		return args[i+1]
	}
	return ""
}

// FFmpeg takes the dialogue's channel from a surround mix and both sides of
// a stereo one; a DTS track is decoded by its core. The film is always read
// as a local file, never as another of FFmpeg's protocols.
func TestTheArgumentsTakeTheVoiceOfTheMix(t *testing.T) {
	t.Parallel()
	surround, channels := ffmpeg.Arguments(voice.Stretch{File: `D:\film.mkv`, Track: 1, Channels: 6, Codec: "A_DTS", From: 90 * time.Second, Length: time.Minute})
	core, input := slices.Index(surround, "-core_only"), slices.Index(surround, "-i")
	switch {
	case value(surround, "-i") != `file:D:\film.mkv`:
		t.Fatalf("input = %q, want the local file", value(surround, "-i"))
	case channels != 1 || value(surround, "-ac") != "1":
		t.Fatalf("surround gives %d channels: %q", channels, surround)
	case core < 0 || core > input || value(surround, "-core_only") != "1":
		t.Fatalf("DTS core missing before the input: %q", surround)
	case value(surround, "-ss") != "90.000" || value(surround, "-t") != "60.000" || value(surround, "-map") != "0:a:1":
		t.Fatalf("stretch or track wrong: %q", surround)
	case value(surround, "-af") != "pan=mono|c0=FC,aresample=16000:async=1:first_pts=0":
		t.Fatalf("filter = %q", value(surround, "-af"))
	case surround[len(surround)-1] != "pipe:1" || value(surround, "-f") != "f32le":
		t.Fatalf("output not floats on the standard output: %q", surround)
	}

	stereo, channels := ffmpeg.Arguments(voice.Stretch{File: `D:\film.mkv`, Track: 0, Channels: 2, Codec: "A_AAC", Length: time.Minute})
	if channels != 2 || value(stereo, "-ac") != "2" || slices.Contains(stereo, "-core_only") || value(stereo, "-af") != "aresample=16000:async=1:first_pts=0" {
		t.Fatalf("stereo: %d channels, %q", channels, stereo)
	}
}

// testFFmpeg is the FFmpeg the tests may use, never shipped with the app:
// LINGOREEL_TEST_FFMPEG, or one installed on the machine.
func testFFmpeg(t *testing.T) string {
	t.Helper()
	path, err := ffmpeg.Find(context.Background(), os.Getenv("LINGOREEL_TEST_FFMPEG"))
	if err != nil {
		t.Skip("no FFmpeg for the tests (LINGOREEL_TEST_FFMPEG)")
	}
	return path
}

// A real FFmpeg decodes a stretch of a 5.1 film whose centre speaks from
// 1 s to 2 s (a tone), while music plays at the sides: the sound handed over
// starts at the stretch's start and holds the centre alone.
func TestARealFFmpegDecodesTheCentreOfAStretch(t *testing.T) {
	testkit.Slow(t)
	path := testFFmpeg(t)
	if err := ffmpeg.Check(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if err := ffmpeg.Check(context.Background(), filepath.Join(os.Getenv("SystemRoot"), `System32\whoami.exe`)); err == nil {
		t.Fatal("whoami taken for an FFmpeg")
	}

	film := filepath.Join(t.TempDir(), "film.mkv")
	make := exec.Command(path, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=1000:duration=1,adelay=1000,apad=whole_dur=3",
		"-f", "lavfi", "-i", "sine=frequency=300:duration=3",
		"-filter_complex", "[0][1][1]join=inputs=3:channel_layout=3.0:map=0.0-FC|1.0-FL|2.0-FR,pan=5.1|FL=FL|FR=FR|FC=FC[out]",
		"-map", "[out]", "-c:a", "ac3", film)
	if out, err := make.CombinedOutput(); err != nil {
		t.Fatalf("making the film: %v: %s", err, out)
	}

	var sound []float32
	got := 0
	err := ffmpeg.Decoder{Path: path}.Decode(context.Background(),
		voice.Stretch{File: film, Track: 0, Channels: 6, Codec: "A_AC3", From: 500 * time.Millisecond, Length: 2 * time.Second},
		func(samples []float32, channels int) {
			got = channels
			sound = append(sound, samples...)
		})
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 || len(sound) < 31_000 || len(sound) > 33_000 {
		t.Fatalf("%d samples in %d channels, want 2 s of mono at 16 kHz", len(sound), got)
	}
	// Loudness of each 10 ms: the tone is heard from 0.5 s into the stretch.
	loud := func(from time.Duration) bool {
		start, sum := int(from.Seconds()*16000), 0.0
		for _, s := range sound[start : start+160] {
			sum += float64(s) * float64(s)
		}
		return 10*math.Log10(sum/160+1e-12) > -30
	}
	for _, check := range []struct {
		at   time.Duration
		loud bool
	}{{200 * time.Millisecond, false}, {480 * time.Millisecond, false}, {520 * time.Millisecond, true}, {time.Second, true}, {1550 * time.Millisecond, false}} {
		if loud(check.at) != check.loud {
			t.Errorf("at %v into the stretch: loud = %v, want %v (the sides leak into the centre, or the start is off)", check.at, !check.loud, check.loud)
		}
	}
}
