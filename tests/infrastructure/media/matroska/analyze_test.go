package matroska_test

import (
	"bytes"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
)

func analyze(t *testing.T, data []byte) matroska.Container {
	t.Helper()
	container, err := matroska.Analyze(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return container
}

func movieFile() []byte {
	return matroskaFile(
		segmentInfo(1_000_000, 5_400_000), // 90 minutes in milliseconds
		tracksEl(
			trackEntry(1, typeVideo, "V_MPEG4/ISO/AVC", uintEl(idDefaultDuration, 41_708_333), el(idVideo, uintEl(idPixelWidth, 1920), uintEl(idPixelHeight, 1080))),
			trackEntry(2, typeAudio, "A_AAC", stringEl(idLanguage, "eng"), uintEl(idFlagDefault, 1)),
			trackEntry(3, typeAudio, "A_AC3", stringEl(idLanguage, "por"), stringEl(idName, "Português"), uintEl(idFlagDefault, 0)),
			trackEntry(4, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "eng"), stringEl(idName, "English")),
			trackEntry(5, typeSubtitle, "S_TEXT/ASS", stringEl(idLanguage, "por"), stringEl(idName, "Brazilian Portuguese"), uintEl(idFlagDefault, 0)),
			trackEntry(6, typeSubtitle, "S_HDMV/PGS", stringEl(idLanguage, "eng"), uintEl(idFlagForced, 1)),
			trackEntry(7, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "eng"), uintEl(idFlagHearingImpaired, 1)),
		),
		cluster(0, simpleBlock(1, 0, "video frame")),
	)
}

func TestAnalyzeDescribesAMovie(t *testing.T) {
	t.Parallel()
	info := analyze(t, movieFile()).MediaInfo()

	if info.Duration != 90*time.Minute {
		t.Fatalf("Duration = %v, want 90m", info.Duration)
	}
	if len(info.Tracks) != 7 {
		t.Fatalf("got %d tracks, want 7", len(info.Tracks))
	}
	video, _ := info.Track(1)
	if video.Type != media.TrackVideo || video.Width != 1920 || video.Height != 1080 || math.Abs(video.FrameRate-23.976) > 0.001 {
		t.Fatalf("video track = %+v", video)
	}
	expectations := []struct {
		number                    uint64
		kind                      media.TrackType
		language                  media.LanguageTag
		format                    subtitle.Format
		readable                  bool
		isDefault, isForced, isHI bool
	}{
		{2, media.TrackAudio, "en", "", false, true, false, false},
		{3, media.TrackAudio, "pt", "", false, false, false, false},
		{4, media.TrackSubtitle, "en", subtitle.FormatSRT, true, true, false, false},
		{5, media.TrackSubtitle, "pt-BR", subtitle.FormatASS, true, false, false, false},
		{6, media.TrackSubtitle, "en", "", false, true, true, false},
		{7, media.TrackSubtitle, "en", subtitle.FormatSRT, true, true, false, true},
	}
	for _, want := range expectations {
		got, found := info.Track(want.number)
		if !found || got.Type != want.kind || got.Language != want.language || got.SubtitleFormat != want.format ||
			got.Readable != want.readable || got.Default != want.isDefault || got.Forced != want.isForced || got.HearingImpaired != want.isHI {
			t.Fatalf("track %d = %+v, want %+v", want.number, got, want)
		}
		if got.UID != 1000+want.number {
			t.Fatalf("track %d UID = %d", want.number, got.UID)
		}
	}

	selection := media.SelectTracks(info, media.Preferences{Audio: "en", Primary: "en", Translation: "pt-BR"})
	if selection != (media.Selection{Audio: 2, Primary: 4, Translation: 5}) {
		t.Fatalf("SelectTracks = %+v, want audio 2, primary 4, translation 5", selection)
	}
}

func TestAnalyzeAppliesSpecDefaults(t *testing.T) {
	t.Parallel()
	data := matroskaFile(
		segmentInfo(1_000_000, 1000),
		tracksEl(
			trackEntry(1, typeSubtitle, "S_TEXT/UTF8"), // no language, no default flag
			trackEntry(2, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "por"), stringEl(idLanguageBCP47, "pt-BR")), // BCP 47 wins
			trackEntry(3, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "und")),
		),
	)
	info := analyze(t, data).MediaInfo()

	first, _ := info.Track(1)
	if first.Language != "en" || !first.Default || !first.LanguageGuessed {
		t.Fatalf("track without flags = %+v, want English (guessed) and default", first)
	}
	if second, _ := info.Track(2); second.LanguageGuessed {
		t.Fatal("a declared language was marked as guessed")
	}
	if second, _ := info.Track(2); second.Language != "pt-BR" {
		t.Fatalf("BCP 47 track language = %q, want pt-BR", second.Language)
	}
	if third, _ := info.Track(3); third.Language != "" {
		t.Fatalf("undetermined language = %q, want empty", third.Language)
	}
}

// Without a Language element the specification says "eng", but that is a
// default, not a statement: the track name decides when it names a language
// itself, so a track called "Português" is never taken for English.
func TestTrackNameDecidesWhenTheLanguageIsOnlyTheDefault(t *testing.T) {
	t.Parallel()
	data := matroskaFile(
		segmentInfo(1_000_000, 1000),
		tracksEl(
			trackEntry(1, typeSubtitle, "S_TEXT/UTF8", stringEl(idName, "Português (Brasil)")),
			trackEntry(2, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "und"), stringEl(idName, "Spanish")),
			trackEntry(3, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "eng"), stringEl(idName, "Português")), // declared wins
			trackEntry(4, typeSubtitle, "S_TEXT/UTF8", stringEl(idName, "Commentary")),
		),
	)
	info := analyze(t, data).MediaInfo()
	want := map[uint64]struct {
		language media.LanguageTag
		guessed  bool
	}{
		1: {"pt-BR", false},
		2: {"es", false},
		3: {"en", false},
		4: {"en", true},
	}
	for number, expected := range want {
		track, _ := info.Track(number)
		if track.Language != expected.language || track.LanguageGuessed != expected.guessed {
			t.Errorf("track %d = %q guessed=%v, want %q guessed=%v", number, track.Language, track.LanguageGuessed, expected.language, expected.guessed)
		}
	}
}

func TestAnalyzeRejectsFilesThatAreNotMatroska(t *testing.T) {
	t.Parallel()
	cases := map[string][]byte{
		"empty":           nil,
		"random bytes":    []byte("this is not a video at all"),
		"other doc type":  join(ebmlHeader("notmkv"), el(idSegment)),
		"no segment":      ebmlHeader("matroska"),
		"only zero bytes": make([]byte, 64),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := matroska.Analyze(bytes.NewReader(data), int64(len(data))); !errors.Is(err, matroska.ErrNotMatroska) {
				t.Fatalf("Analyze error = %v, want ErrNotMatroska", err)
			}
		})
	}
}

func TestAnalyzeAcceptsWebM(t *testing.T) {
	t.Parallel()
	data := join(ebmlHeader("webm"), el(idSegment, tracksEl(trackEntry(1, typeSubtitle, "D_WEBVTT/SUBTITLES"))))
	container := analyze(t, data)
	track, _ := container.MediaInfo().Track(1)
	if container.DocType != "webm" || !track.Readable || track.SubtitleFormat != subtitle.FormatWebVTT {
		t.Fatalf("WebM container = %q, track %+v", container.DocType, track)
	}
}

func TestAnalyzeFailsWithoutTracksAndOnTruncation(t *testing.T) {
	t.Parallel()
	noTracks := matroskaFile(segmentInfo(1_000_000, 10))
	if _, err := matroska.Analyze(bytes.NewReader(noTracks), int64(len(noTracks))); !errors.Is(err, matroska.ErrCorrupt) {
		t.Fatalf("file without tracks error = %v, want ErrCorrupt", err)
	}

	full := movieFile()
	for _, cut := range []int{len(full) / 3, len(full) / 2, len(full) - 30} {
		truncated := full[:cut]
		if _, err := matroska.Analyze(bytes.NewReader(truncated), int64(len(truncated))); err == nil {
			t.Fatalf("Analyze accepted a file cut at %d of %d bytes", cut, len(full))
		}
	}
}

// Tracks written after an unknown-size cluster cannot be reached by walking
// the file, so they must be found through the SeekHead index.
func TestAnalyzeFindsTracksThroughSeekHead(t *testing.T) {
	t.Parallel()
	seekHead := func(tracksOffset uint64) []byte {
		return el(idSeekHead, el(idSeek, el(idSeekID, encodeID(idTracks)), fixedUintEl(idSeekPosition, tracksOffset)))
	}
	body := join(segmentInfo(1_000_000, 1000), unknownSizeEl(idCluster, uintEl(idClusterTimestamp, 0), simpleBlock(1, 0, "x")))
	tracks := tracksEl(trackEntry(9, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "fre")))
	// Seek positions count from the start of the segment data.
	tracksOffset := uint64(len(seekHead(0)) + len(body))
	data := join(ebmlHeader("matroska"), unknownSizeEl(idSegment, seekHead(tracksOffset), body, tracks))

	info := analyze(t, data).MediaInfo()
	if track, found := info.Track(9); !found || track.Language != "fr" {
		t.Fatalf("tracks behind the clusters were not found: %+v", info.Tracks)
	}

	wrongTarget := join(ebmlHeader("matroska"), unknownSizeEl(idSegment, seekHead(0), body, tracks))
	if _, err := matroska.Analyze(bytes.NewReader(wrongTarget), int64(len(wrongTarget))); !errors.Is(err, matroska.ErrCorrupt) {
		t.Fatalf("SeekHead pointing at the wrong element: error = %v, want ErrCorrupt", err)
	}
}

func TestAnalyzeFindsTracksAfterKnownSizeClusters(t *testing.T) {
	t.Parallel()
	data := matroskaFile(
		segmentInfo(1_000_000, 1000),
		cluster(0, simpleBlock(1, 0, "frame")),
		cluster(1000, simpleBlock(1, 0, "frame")),
		tracksEl(trackEntry(3, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "ita"))),
	)
	if track, found := analyze(t, data).MediaInfo().Track(3); !found || track.Language != "it" {
		t.Fatal("tracks after known-size clusters were not found")
	}
}

func TestAnalyzeIgnoresInvalidScaleAndDuration(t *testing.T) {
	t.Parallel()
	data := matroskaFile(
		el(idInfo, uintEl(idTimestampScale, 0), floatEl(idDuration, math.NaN())),
		tracksEl(trackEntry(1, typeSubtitle, "S_TEXT/UTF8")),
	)
	container := analyze(t, data)
	if container.TimestampScale != 1_000_000 || container.Duration != 0 {
		t.Fatalf("scale = %d, duration = %v; want the default scale and zero duration", container.TimestampScale, container.Duration)
	}

	negative := matroskaFile(segmentInfo(1_000_000, -5), tracksEl(trackEntry(1, typeSubtitle, "S_TEXT/UTF8")))
	if got := analyze(t, negative).Duration; got != 0 {
		t.Fatalf("negative duration became %v", got)
	}
}
