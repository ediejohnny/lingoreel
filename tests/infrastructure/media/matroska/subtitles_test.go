package matroska_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
)

type wantCue struct {
	id         string
	start, end time.Duration
	text       string
}

func readTracks(t *testing.T, data []byte, tracks ...uint64) map[uint64]media.SubtitleContent {
	t.Helper()
	container := analyze(t, data)
	content, err := matroska.ReadSubtitles(context.Background(), bytes.NewReader(data), int64(len(data)), container, tracks, nil)
	if err != nil {
		t.Fatalf("ReadSubtitles: %v", err)
	}
	return content
}

func assertCues(t *testing.T, got media.SubtitleContent, want []wantCue, wantSkipped int) {
	t.Helper()
	if len(got.Events) != len(want) || got.Skipped != wantSkipped {
		t.Fatalf("got %d events (%+v) and %d skipped, want %d and %d", len(got.Events), got.Events, got.Skipped, len(want), wantSkipped)
	}
	for i, w := range want {
		g := got.Events[i]
		if g.ID != w.id || g.Start != w.start || g.End != w.end || g.OriginalText != w.text {
			t.Fatalf("event %d = {%s %v %v %q}, want {%s %v %v %q}", i, g.ID, g.Start, g.End, g.OriginalText, w.id, w.start, w.end, w.text)
		}
	}
}

func ms(value int) time.Duration {
	return time.Duration(value) * time.Millisecond
}

func TestReadSubtitlesUsesBlockDurationsAndClusterTimestamps(t *testing.T) {
	data := subtitleFile(
		cluster(1_000,
			blockGroup(2, 0, 2_000, []byte("Hello")),
			blockGroup(2, 3_000, 1_500, []byte("<i>World</i>")),
		),
		cluster(10_000, blockGroup(2, -500, 1_000, []byte("Back"))), // negative relative timestamp
	)

	assertCues(t, readTracks(t, data, 2)[2], []wantCue{
		{"1", ms(1_000), ms(3_000), "Hello"},
		{"2", ms(4_000), ms(5_500), "World"},
		{"3", ms(9_500), ms(10_500), "Back"},
	}, 0)
}

func TestReadSubtitlesInfersMissingEnds(t *testing.T) {
	data := subtitleFile(cluster(0,
		simpleBlock(2, 1_000, "first"),
		simpleBlock(2, 4_000, "second"),
		simpleBlock(2, 20_000, "third"),
	))

	assertCues(t, readTracks(t, data, 2)[2], []wantCue{
		{"1", ms(1_000), ms(4_000), "first"},   // ends when the next cue starts
		{"2", ms(4_000), ms(14_000), "second"}, // capped at 10 s
		{"3", ms(20_000), ms(23_000), "third"}, // last cue lasts 3 s
	}, 0)
}

func TestReadSubtitlesHonorsTimestampScale(t *testing.T) {
	data := matroskaFile(
		segmentInfo(100_000, 10), // one tick is 0.1 ms
		tracksEl(trackEntry(2, typeSubtitle, "S_TEXT/UTF8")),
		cluster(10_000, blockGroup(2, 5_000, 20_000, []byte("scaled"))),
	)
	assertCues(t, readTracks(t, data, 2)[2], []wantCue{{"1", ms(1_500), ms(3_500), "scaled"}}, 0)
}

func TestReadSubtitlesSkipsOtherTracksAndOversizedFrames(t *testing.T) {
	bigVideoFrame := strings.Repeat("v", 3<<20)
	oversizedSubtitle := bytes.Repeat([]byte("s"), (1<<20)+1)
	data := matroskaFile(
		segmentInfo(1_000_000, 10_000),
		tracksEl(
			trackEntry(1, typeVideo, "V_MPEG4/ISO/AVC"),
			trackEntry(2, typeSubtitle, "S_TEXT/UTF8"),
		),
		cluster(0,
			simpleBlock(1, 0, bigVideoFrame),
			blockGroup(2, 100, 900, []byte("kept")),
			simpleBlock(1, 40, bigVideoFrame),
			blockGroup(2, 2_000, 500, oversizedSubtitle),
		),
	)

	assertCues(t, readTracks(t, data, 2)[2], []wantCue{{"1", ms(100), ms(1_000), "kept"}}, 1)
}

func TestReadSubtitlesDecodesASSBlocksAndDropsEmptyOnes(t *testing.T) {
	data := matroskaFile(
		segmentInfo(1_000_000, 10_000),
		tracksEl(trackEntry(3, typeSubtitle, "S_TEXT/ASS")),
		cluster(0,
			blockGroup(3, 0, 1_000, []byte(`1,0,Default,,0,0,0,,{\i1}Hi,{\i0} there\Nfriend`)),
			blockGroup(3, 2_000, 1_000, []byte(`2,0,Sign,,0,0,0,,{\p1}m 0 0 l 1 1`)), // drawing only
			blockGroup(3, 4_000, 1_000, []byte(`broken block`)),                      // too few fields
		),
	)
	assertCues(t, readTracks(t, data, 3)[3], []wantCue{{"1", 0, ms(1_000), "Hi, there\nfriend"}}, 2)
}

func zlibCompress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zlib.NewWriter(&buffer)
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestReadSubtitlesUndoesContentCompression(t *testing.T) {
	zipBomb := zlibCompress(t, bytes.Repeat([]byte("a"), 2<<20))
	data := matroskaFile(
		segmentInfo(1_000_000, 10_000),
		tracksEl(
			trackEntry(2, typeSubtitle, "S_TEXT/UTF8", compression(0, 1, 0, 0, nil)),           // zlib
			trackEntry(3, typeSubtitle, "S_TEXT/UTF8", compression(0, 1, 0, 3, []byte("Hel"))), // header stripping
		),
		cluster(0,
			blockGroup(2, 0, 1_000, zlibCompress(t, []byte("Compressed olá"))),
			blockGroup(3, 0, 1_000, []byte("lo there")),
			blockGroup(2, 2_000, 1_000, zipBomb),
			blockGroup(2, 4_000, 1_000, []byte("not zlib at all")),
		),
	)
	content := readTracks(t, data, 2, 3)

	assertCues(t, content[2], []wantCue{{"1", 0, ms(1_000), "Compressed olá"}}, 2)
	assertCues(t, content[3], []wantCue{{"1", 0, ms(1_000), "Hello there"}}, 0)
}

func TestTracksWithUnsupportedEncodingsAreUnreadable(t *testing.T) {
	data := matroskaFile(
		segmentInfo(1_000_000, 10),
		tracksEl(
			trackEntry(2, typeSubtitle, "S_TEXT/UTF8", compression(0, 1, 1, 0, nil)), // encrypted
			trackEntry(3, typeSubtitle, "S_TEXT/UTF8", compression(0, 1, 0, 2, nil)), // LZO
			trackEntry(4, typeSubtitle, "S_TEXT/UTF8", compression(0, 2, 1, 0, nil)), // only CodecPrivate encrypted
		),
	)
	container := analyze(t, data)
	info := container.MediaInfo()
	for number, want := range map[uint64]bool{2: false, 3: false, 4: true} {
		if track, _ := info.Track(number); track.Readable != want {
			t.Fatalf("track %d Readable = %v, want %v", number, track.Readable, want)
		}
	}
	for _, number := range []uint64{2, 3} {
		_, err := matroska.ReadSubtitles(context.Background(), bytes.NewReader(data), int64(len(data)), container, []uint64{number}, nil)
		if !errors.Is(err, matroska.ErrNotReadableTrack) {
			t.Fatalf("reading track %d: error = %v, want ErrNotReadableTrack", number, err)
		}
	}
}

func TestReadSubtitlesSplitsLacedBlocks(t *testing.T) {
	xiph := join([]byte{2}, []byte{2, 3}, []byte("AB"+"BCC"+"CCCC"))        // sizes 2, 3, the rest 4
	ebml := join([]byte{2}, encodeVint(2), []byte{0xC0}, []byte("ABBBCCC")) // 2, then +1 (0xC0 = 64 - 63), rest 3
	fixed := join([]byte{1}, []byte("AABB"))
	badXiph := join([]byte{3}, []byte{255, 255})

	cases := []struct {
		name    string
		flags   byte
		payload []byte
		want    []string
	}{
		{"xiph", 0x02, xiph, []string{"AB", "BCC", "CCCC"}},
		{"ebml", 0x06, ebml, []string{"AB", "BBC", "CC"}},
		{"fixed", 0x04, fixed, []string{"AA", "BB"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := subtitleFile(cluster(0, lacedBlockGroup(2, 0, 3_000, tc.flags, tc.payload)))
			events := readTracks(t, data, 2)[2].Events
			if len(events) != len(tc.want) {
				t.Fatalf("got %d frames %+v, want %v", len(events), events, tc.want)
			}
			share := ms(3_000) / time.Duration(len(tc.want))
			for i, text := range tc.want {
				if events[i].OriginalText != text || events[i].Start != share*time.Duration(i) || events[i].End != share*time.Duration(i+1) {
					t.Fatalf("frame %d = %+v, want %q from %v", i, events[i], text, share*time.Duration(i))
				}
			}
		})
	}

	broken := subtitleFile(cluster(0, lacedBlockGroup(2, 0, 3_000, 0x02, badXiph)))
	if content := readTracks(t, broken, 2)[2]; len(content.Events) != 0 || content.Skipped != 1 {
		t.Fatalf("badly laced block produced %+v", content)
	}
}

func TestReadSubtitlesReadsTwoTracksInOnePassAcrossUnknownSizeClusters(t *testing.T) {
	data := join(ebmlHeader("matroska"), unknownSizeEl(idSegment,
		segmentInfo(1_000_000, 10_000),
		tracksEl(
			trackEntry(2, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "eng")),
			trackEntry(3, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "por")),
		),
		unknownSizeEl(idCluster, uintEl(idClusterTimestamp, 0), blockGroup(2, 0, 1_000, []byte("one")), blockGroup(3, 100, 1_000, []byte("um"))),
		unknownSizeEl(idCluster, uintEl(idClusterTimestamp, 5_000), blockGroup(2, 0, 1_000, []byte("two")), blockGroup(3, 100, 1_000, []byte("dois"))),
	))
	content := readTracks(t, data, 2, 3)

	assertCues(t, content[2], []wantCue{{"1", 0, ms(1_000), "one"}, {"2", ms(5_000), ms(6_000), "two"}}, 0)
	assertCues(t, content[3], []wantCue{{"1", ms(100), ms(1_100), "um"}, {"2", ms(5_100), ms(6_100), "dois"}}, 0)
}

func TestReadSubtitlesRejectsTracksThatCannotBeRead(t *testing.T) {
	data := movieFile()
	container := analyze(t, data)
	for _, number := range []uint64{1, 2, 6, 99} { // video, audio, PGS, missing
		_, err := matroska.ReadSubtitles(context.Background(), bytes.NewReader(data), int64(len(data)), container, []uint64{number}, nil)
		if !errors.Is(err, matroska.ErrNotReadableTrack) {
			t.Fatalf("track %d: error = %v, want ErrNotReadableTrack", number, err)
		}
	}
}

func TestReadSubtitlesStopsWhenCancelled(t *testing.T) {
	data := subtitleFile(cluster(0, blockGroup(2, 0, 1_000, []byte("x"))))
	container := analyze(t, data)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := matroska.ReadSubtitles(ctx, bytes.NewReader(data), int64(len(data)), container, []uint64{2}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestReadSubtitlesReportsMonotonicProgressUpToTheFileSize(t *testing.T) {
	var clusters [][]byte
	for i := range 20 {
		clusters = append(clusters, cluster(uint64(i*1_000), blockGroup(2, 0, 500, []byte("line"))))
	}
	data := subtitleFile(clusters...)
	container := analyze(t, data)
	var reports []int64
	_, err := matroska.ReadSubtitles(context.Background(), bytes.NewReader(data), int64(len(data)), container, []uint64{2}, func(done, total int64) {
		if total != int64(len(data)) {
			t.Fatalf("progress total = %d, want %d", total, len(data))
		}
		reports = append(reports, done)
	})
	if err != nil {
		t.Fatalf("ReadSubtitles: %v", err)
	}
	if len(reports) < 20 || reports[len(reports)-1] != int64(len(data)) {
		t.Fatalf("progress reports = %v", reports)
	}
	for i := 1; i < len(reports); i++ {
		if reports[i] < reports[i-1] {
			t.Fatalf("progress went backwards: %v", reports)
		}
	}
}

func TestFileReaderReadsFromDiskAndReportsBadPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Filme.mkv")
	data := subtitleFile(cluster(0, blockGroup(2, 0, 1_000, []byte("do disco"))))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	reader := matroska.FileReader{}

	info, err := reader.Analyze(path)
	if err != nil || len(info.Tracks) != 1 {
		t.Fatalf("Analyze(%s) = %+v, %v", path, info, err)
	}
	content, err := reader.ReadSubtitles(context.Background(), path, []uint64{2}, nil)
	if err != nil || len(content[2].Events) != 1 || content[2].Events[0].OriginalText != "do disco" {
		t.Fatalf("ReadSubtitles = %+v, %v", content, err)
	}

	for _, bad := range []string{filepath.Join(t.TempDir(), "missing.mkv"), t.TempDir()} {
		if _, err := reader.Analyze(bad); err == nil {
			t.Fatalf("Analyze(%s) succeeded", bad)
		}
	}
}
