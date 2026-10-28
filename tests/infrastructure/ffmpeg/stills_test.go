package ffmpeg_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// A still is the keyframe at or before the moment, read from the local file
// as the container the app opened: nothing after the keyframe is decoded,
// and the frame before the moment is kept, never dropped.
func TestTheStillArgumentsTakeTheKeyframeOfTheLocalFile(t *testing.T) {
	t.Parallel()
	args := ffmpeg.StillArguments(`D:\film.mkv`, "matroska", 90*time.Second, ffmpeg.Look{}, true)
	input := slices.Index(args, "-i")
	before := func(flag string) bool { i := slices.Index(args, flag); return i >= 0 && i < input }
	switch {
	case value(args, "-i") != `file:D:\film.mkv` || value(args, "-f") != "matroska" || value(args, "-protocol_whitelist") != "file":
		t.Fatalf("not the local file as the container opened: %q", args)
	case !before("-noaccurate_seek") || !before("-ss") || value(args, "-ss") != "90.000" || !before("-threads"):
		t.Fatalf("the seek does not land on the keyframe before the moment: %q", args)
	case value(args, "-fps_mode") != "passthrough" || value(args, "-frames:v") != "1" || value(args, "-map") != "0:V:0":
		t.Fatalf("not the one frame of the first picture (never a cover): %q", args)
	case slices.Contains(args, "-skip_frame") || slices.Contains(args, "-discard"):
		// Both read several times more of the file for the same frame.
		t.Fatalf("keyframe-only decoding asked for: %q", args)
	case value(args, "-c:v") != "mjpeg" || args[len(args)-1] != "pipe:1" || value(args, "-stats_enc_pre") != "pipe:2":
		t.Fatalf("not a JPEG on the standard output with its time on the error output: %q", args)
	}
	if untimed := ffmpeg.StillArguments(`D:\film.mkv`, "matroska", 0, ffmpeg.Look{}, false); slices.Contains(untimed, "-stats_enc_pre") {
		t.Fatalf("an FFmpeg that cannot tell the time is asked for it: %q", untimed)
	}

	for _, c := range []struct {
		name      string
		look      ffmpeg.Look
		want, not []string
	}{
		{"tagged", ffmpeg.Look{}, []string{"out_color_matrix=bt601:out_range=pc", "format=yuvj444p"}, []string{"in_color_matrix", "zscale", "bwdif"}},
		{"untagged HD", ffmpeg.Look{UntaggedHD: true}, []string{"in_color_matrix=bt709:in_range=tv"}, nil},
		{"untagged HD at every level", ffmpeg.Look{UntaggedHD: true, FullRange: true}, []string{"in_color_matrix=bt709"}, []string{"in_range"}},
		{"interlaced", ffmpeg.Look{Interlaced: true}, []string{"bwdif=mode=send_frame:deint=interlaced,scale="}, nil},
		{"PQ", ffmpeg.Look{Transfer: ffmpeg.TransferPQ}, []string{"zscale=tin=smpte2084", "tonemap=mobius:desat=0"}, []string{"hable"}},
		{"HLG", ffmpeg.Look{Transfer: ffmpeg.TransferHLG}, []string{"zscale=tin=arib-std-b67"}, nil},
	} {
		filter := ffmpeg.StillFilter(c.look)
		for _, part := range c.want {
			if !strings.Contains(filter, part) {
				t.Errorf("%s: %q lacks %q", c.name, filter, part)
			}
		}
		// Tone mapping inside "scale" costs seconds of every core per still.
		for _, part := range append(c.not, "out_transfer", "intent=") {
			if strings.Contains(filter, part) {
				t.Errorf("%s: %q has %q", c.name, filter, part)
			}
		}
		if c.look.Transfer != "" && strings.Index(filter, "scale=") > strings.Index(filter, "zscale") {
			t.Errorf("%s: tone-mapped before it is made small: %q", c.name, filter)
		}
	}
}

// How a still is taken comes from FFmpeg's description of the file: the
// first picture that is not a cover, its colour tags, its fields and Dolby
// Vision's profile.
func TestTheLookIsReadFromFFmpegsDescription(t *testing.T) {
	t.Parallel()
	const header = "Input #0, matroska,webm, from 'file:D:\\film.mkv':\n  Metadata:\n    title           : A film\n  Duration: 01:42:10.05, start: 0.000000, bitrate: 9000 kb/s\n"
	for _, c := range []struct {
		name, streams string
		want          ffmpeg.Look
	}{
		{"tagged HD", "  Stream #0:0(eng): Video: h264 (High), yuv420p(tv, bt709, progressive), 1920x1080 [SAR 1:1 DAR 16:9], 23.98 fps, 23.98 tbr, 1k tbn (default)\n",
			ffmpeg.Look{}},
		{"untagged HD", "  Stream #0:0[0x1](und): Video: h264 (High) (avc1 / 0x31637661), yuv420p(progressive), 1280x720 [SAR 1:1 DAR 16:9], 2500 kb/s, 25 fps\n",
			ffmpeg.Look{UntaggedHD: true}},
		{"untagged SD", "  Stream #0:0: Video: mpeg4 (Simple Profile) (XVID / 0x44495658), yuv420p, 640x352 [SAR 1:1 DAR 20:11], 1100 kb/s, 23.98 fps\n",
			ffmpeg.Look{}},
		{"DVD", "  Stream #0:0: Video: mpeg2video (Main), yuv420p(tv, top first), 720x480 [SAR 32:27 DAR 16:9], 29.97 fps\n",
			ffmpeg.Look{Interlaced: true}},
		{"swapped fields", "  Stream #0:0: Video: mpeg2video (Main), yuv420p(tv, bt470bg, top coded first (swapped)), 720x576 [SAR 64:45 DAR 16:9], 25 fps\n",
			ffmpeg.Look{Interlaced: true}},
		{"PQ", "  Stream #0:0: Video: hevc (Main 10), yuv420p10le(tv, bt2020nc/bt2020/smpte2084), 3840x2160 [SAR 1:1 DAR 16:9], 23.98 fps\n      Side data:\n        Mastering Display Metadata\n",
			ffmpeg.Look{Transfer: ffmpeg.TransferPQ}},
		{"HLG", "  Stream #0:0: Video: hevc (Main 10), yuv420p10le(tv, bt2020nc/bt2020/arib-std-b67), 1920x1080, 50 fps\n",
			ffmpeg.Look{Transfer: ffmpeg.TransferHLG}},
		{"cover first", "  Stream #0:0: Video: mjpeg (Baseline), yuvj420p(pc, bt470bg/unknown/unknown), 600x800 [SAR 1:1 DAR 3:4], 90k tbr, 90k tbn (attached pic)\n  Stream #0:1: Video: h264 (High), yuv420p(tv, bt709, progressive), 1920x800, 24 fps\n",
			ffmpeg.Look{}},
		{"Dolby Vision", "  Stream #0:0: Video: hevc (Main 10), yuv420p10le(tv, bt2020nc/bt2020/smpte2084), 3840x2160, 23.98 fps\n      Side data:\n        DOVI configuration record: version: 1.0, profile: 5, level: 6, rpu flag: 1, el flag: 0, bl flag: 1, compatibility id: 0\n  Stream #0:1(eng): Audio: eac3, 48000 Hz, 5.1(side), fltp, 640 kb/s\n",
			ffmpeg.Look{Transfer: ffmpeg.TransferPQ, DolbyVision: 5}},
		{"the next stream's record", "  Stream #0:0: Video: h264 (High), yuv420p(tv, bt709), 1920x1080, 24 fps\n  Stream #0:1: Video: hevc, yuv420p10le, 1920x1080\n      Side data:\n        DOVI configuration record: version: 1.0, profile: 8\n",
			ffmpeg.Look{}},
	} {
		got, err := ffmpeg.ReadLook(header + c.streams + "At least one output file must be specified\n")
		if err != nil || got != c.want {
			t.Errorf("%s: look %+v (%v), want %+v", c.name, got, err, c.want)
		}
	}
	if _, err := ffmpeg.ReadLook(header + "  Stream #0:0: Audio: flac, 48000 Hz, stereo, s16\n"); !errors.Is(err, thumbnail.ErrNoStills) {
		t.Fatalf("a file with no picture gave %v, want ErrNoStills", err)
	}
}

// decodeStill decodes a still and checks its size.
func decodeStill(t *testing.T, picture thumbnail.Picture, width, height int) image.Image {
	t.Helper()
	still, err := jpeg.Decode(bytes.NewReader(picture.JPEG))
	if err != nil {
		t.Fatal(err)
	}
	if bounds := still.Bounds(); bounds.Dx() != width || bounds.Dy() != height || picture.Width != width || picture.Height != height {
		t.Fatalf("still %v, told %dx%d, want %dx%d", bounds, picture.Width, picture.Height, width, height)
	}
	return still
}

// rgbAt is a still's colour at a point, in 8 bits.
func rgbAt(still image.Image, x, y int) [3]int {
	r, g, b, _ := still.At(x, y).RGBA()
	return [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
}

func near(got, want [3]int, by int) bool {
	for i := range got {
		if got[i] < want[i]-by || got[i] > want[i]+by {
			return false
		}
	}
	return true
}

// A real FFmpeg takes the still at its moment: a film red for 10 s and blue
// for the next 10 (a keyframe each second) gives red at 3 s and blue at 15 s,
// each from the keyframe at or just before the moment, in the still's box.
func TestARealFFmpegTakesTheStillAtItsMoment(t *testing.T) {
	testkit.Slow(t)
	path := testFFmpeg(t)
	file := filepath.Join(t.TempDir(), "colours.mkv")
	makeFilm(t, path, "-f", "lavfi", "-i", "color=c=red:s=640x360:r=25:d=10", "-f", "lavfi", "-i", "color=c=blue:s=640x360:r=25:d=10",
		"-filter_complex", "[0][1]concat=n=2:v=1:a=0", "-c:v", "mpeg4", "-g", "25", "-q:v", "2", file)
	film, err := ffmpeg.Stills{Path: path}.Open(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		at   time.Duration
		want [3]int
	}{{3 * time.Second, [3]int{255, 0, 0}}, {15 * time.Second, [3]int{0, 0, 255}}} {
		picture, err := film.Grab(context.Background(), thumbnail.Shot{At: c.at})
		if err != nil {
			t.Fatal(err)
		}
		if got := rgbAt(decodeStill(t, picture, 384, 216), 192, 108); !near(got, c.want, 40) {
			t.Errorf("at %v: colour %v, want about %v", c.at, got, c.want)
		}
		if picture.At > c.at || picture.At < c.at-time.Second {
			t.Errorf("at %v: the still is from %v, want the keyframe at or just before", c.at, picture.At)
		}
	}
}

// A still keeps the picture's shape whatever its pixels: a DVD's 720x480
// with wide pixels is 16:9, and a square-pixel 4:3 film is narrower.
func TestARealFFmpegKeepsThePicturesShape(t *testing.T) {
	testkit.Slow(t)
	path := testFFmpeg(t)
	for _, c := range []struct {
		name, source  string
		width, height int
	}{
		{"wide pixels", "color=c=green:s=720x480:r=25:d=2,setsar=32/27", 384, 216},
		{"4:3", "color=c=green:s=640x480:r=25:d=2", 288, 216},
	} {
		file := filepath.Join(t.TempDir(), "shape.mkv")
		makeFilm(t, path, "-f", "lavfi", "-i", c.source, "-c:v", "mpeg4", "-q:v", "2", file)
		film, err := ffmpeg.Stills{Path: path}.Open(context.Background(), file)
		if err != nil {
			t.Fatal(err)
		}
		picture, err := film.Grab(context.Background(), thumbnail.Shot{At: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		decodeStill(t, picture, c.width, c.height)
	}
}

// describe has the test FFmpeg describe a file, as the app does.
func describe(t *testing.T, path, file string) string {
	t.Helper()
	out, _ := exec.Command(path, "-hide_banner", "-nostdin", "-i", "file:"+file).CombinedOutput()
	return string(out)
}

// An HD film with no colour tags keeps its colours: players take its
// picture as BT.709, and a still read as SD's BT.601 would be off by about
// 11 levels of red and green.
func TestARealFFmpegKeepsTheColoursOfAnUntaggedHDFilm(t *testing.T) {
	testkit.Slow(t)
	path := testFFmpeg(t)
	file := filepath.Join(t.TempDir(), "untagged.mkv")
	makeFilm(t, path, "-f", "lavfi", "-i",
		"color=c=0xC04020:s=1280x720:r=25:d=2,format=rgb24,scale=out_color_matrix=bt709:out_range=tv,format=yuv420p,setparams=colorspace=unknown:color_primaries=unknown:color_trc=unknown:range=unknown",
		"-c:v", "mpeg4", "-q:v", "2", file)
	if look, err := ffmpeg.ReadLook(describe(t, path, file)); err != nil || !look.UntaggedHD {
		t.Fatalf("the test film is not untagged HD: %+v (%v)", look, err)
	}
	film, err := ffmpeg.Stills{Path: path}.Open(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	picture, err := film.Grab(context.Background(), thumbnail.Shot{At: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if got := rgbAt(decodeStill(t, picture, 384, 216), 192, 108); !near(got, [3]int{0xC0, 0x40, 0x20}, 5) {
		t.Fatalf("colour %v, want about [192 64 32]", got)
	}
}

// An HDR film is tone-mapped, not washed out: a highlight far above
// ordinary white comes out white, and 100 nits sits in the upper middle,
// where read as an ordinary picture they would be about 205 and 130.
func TestARealFFmpegToneMapsAnHDRFilm(t *testing.T) {
	testkit.Slow(t)
	path := testFFmpeg(t)
	file := filepath.Join(t.TempDir(), "pq.mkv")
	// The frames carry the tags (the encoder's own options are not kept).
	makeFilm(t, path, "-f", "lavfi", "-i",
		"color=c=black:s=640x360:r=25:d=2,format=yuv420p10le,geq=lum='if(lt(X,W/2),512,768)':cb=512:cr=512,"+
			"setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc:range=tv",
		"-c:v", "ffv1", file)
	if look, err := ffmpeg.ReadLook(describe(t, path, file)); err != nil || look.Transfer != ffmpeg.TransferPQ {
		t.Fatalf("the test film is not PQ: %+v (%v)", look, err)
	}
	film, err := ffmpeg.Stills{Path: path}.Open(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	picture, err := film.Grab(context.Background(), thumbnail.Shot{At: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	// The mobius curve leaves the mid tones as they are: 100 nits stays as
	// bright as the film shows it (hable dimmed it to about 160); only the
	// highlight is bent, and stays brighter.
	still := decodeStill(t, picture, 384, 216)
	if mid, high := rgbAt(still, 96, 108)[1], rgbAt(still, 288, 108)[1]; mid < 200 || mid > 235 || high < 245 || high <= mid {
		t.Fatalf("100 nits at %d, the highlight at %d: want 200–235 and at least 245, above it", mid, high)
	}
}

// A still asked for after its context ended is not taken, and a file with
// no picture gives no stills.
func TestARealFFmpegStillStopsWithItsContextAndNeedsAPicture(t *testing.T) {
	testkit.Slow(t)
	path := testFFmpeg(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "short.mkv")
	makeFilm(t, path, "-f", "lavfi", "-i", "color=c=gray:s=320x180:r=25:d=2", "-c:v", "mpeg4", file)
	film, err := ffmpeg.Stills{Path: path}.Open(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if _, err := film.Grab(ended, thumbnail.Shot{At: time.Second}); !errors.Is(err, context.Canceled) || time.Since(start) > 2*time.Second {
		t.Fatalf("an ended context gave %v after %v", err, time.Since(start))
	}

	sound := filepath.Join(dir, "sound.mkv")
	makeFilm(t, path, "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-c:a", "flac", sound)
	if _, err := (ffmpeg.Stills{Path: path}).Open(context.Background(), sound); !errors.Is(err, thumbnail.ErrNoStills) {
		t.Fatalf("a film with no picture gave %v, want ErrNoStills", err)
	}
}
