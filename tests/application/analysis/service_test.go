package analysis_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"lingoreel/internal/application/analysis"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/subtitle"
)

// fakeReader records calls and returns canned results.
type fakeReader struct {
	info          media.Info
	content       map[uint64]media.SubtitleContent
	err           error
	analyzeCalls  []string
	checkCalls    []string
	checkErr      error
	requested     []uint64
	progressCalls int
}

func (f *fakeReader) Analyze(path string) (media.Info, error) {
	f.analyzeCalls = append(f.analyzeCalls, path)
	return f.info, f.err
}

func (f *fakeReader) Check(path string) error {
	f.checkCalls = append(f.checkCalls, path)
	return f.checkErr
}

func (f *fakeReader) ReadSubtitles(ctx context.Context, path string, tracks []uint64, progress func(done, total int64)) (map[uint64]media.SubtitleContent, error) {
	f.requested = append([]uint64{}, tracks...)
	if progress != nil {
		progress(50, 100)
		progress(100, 100)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.content, f.err
}

var preferences = media.Preferences{Audio: "en", Primary: "en", Translation: "pt-BR"}

func studyInfo() media.Info {
	return media.Info{Tracks: []media.Track{
		{Number: 1, Type: media.TrackAudio, Language: "en", Default: true},
		{Number: 2, Type: media.TrackSubtitle, Language: "en", Readable: true},
		{Number: 3, Type: media.TrackSubtitle, Language: "pt-BR", Readable: true},
	}}
}

// studyAnalysis is an analyzed video with a fourth, Spanish subtitle track.
func studyAnalysis() analysis.Analysis {
	info := studyInfo()
	info.Tracks = append(info.Tracks, media.Track{Number: 4, Type: media.TrackSubtitle, Language: "es", Readable: true})
	return analysis.Analysis{Path: `C:\x.mkv`, Info: info}
}

func TestAnalyzeRejectsUnsupportedFilesWithoutReadingThem(t *testing.T) {
	reader := &fakeReader{info: studyInfo()}
	service := analysis.NewService(reader, nil)

	for _, path := range []string{`C:\Filmes\Filme.txt`, `C:\Filmes\Filme`, `C:\Filmes\Filme.mkv.txt`, ``} {
		if _, err := service.Analyze(context.Background(), path, preferences); !errors.Is(err, analysis.ErrUnsupportedFile) {
			t.Fatalf("Analyze(%q) error = %v, want ErrUnsupportedFile", path, err)
		}
	}
	if len(reader.analyzeCalls) != 0 {
		t.Fatalf("unsupported files reached the reader: %v", reader.analyzeCalls)
	}
}

func TestAnalyzeSuggestsTracks(t *testing.T) {
	reader := &fakeReader{info: studyInfo()}
	service := analysis.NewService(reader, nil)

	for _, path := range []string{`D:\Filmes\Matrix.MKV`, `D:\Séries\Episódio 1.webm`} {
		result, err := service.Analyze(context.Background(), path, preferences)
		if err != nil {
			t.Fatalf("Analyze(%q): %v", path, err)
		}
		if result.Path != path || len(result.Info.Tracks) != 3 {
			t.Fatalf("Analyze(%q) = %+v", path, result)
		}
		if result.Suggested != (media.Selection{Audio: 1, Primary: 2, Translation: 3}) {
			t.Fatalf("Suggested = %+v", result.Suggested)
		}
	}
}

// Other videos open too (PRD §1: "especially" .mkv): their own tracks are not
// read, so their subtitles come only from the files next to them.
func TestVideosWithoutReadableTracksUseOnlySubtitleFiles(t *testing.T) {
	reader := &fakeReader{info: studyInfo()}
	files := &fakeFiles{
		paths:   []string{`D:\Filmes\Filme.en.srt`},
		content: map[string]media.SubtitleContent{`D:\Filmes\Filme.en.srt`: lines("Hello there.")},
	}
	service := analysis.NewService(reader, files)
	for _, path := range []string{`D:\Filmes\Filme.mp4`, `D:\Filmes\Filme.AVI`, `D:\Filmes\Filme.m4v`, `D:\Filmes\Filme.mov`, `D:\Filmes\Filme.wmv`} {
		result, err := service.Analyze(context.Background(), path, preferences)
		if err != nil {
			t.Fatalf("Analyze(%q): %v", path, err)
		}
		if len(result.Info.Tracks) != 1 || result.Suggested.Primary != media.ExternalTrackBase || result.Suggested.Audio != 0 {
			t.Fatalf("Analyze(%q) = %+v, want only the subtitle file, chosen, and the player's own audio", path, result)
		}
	}
	if len(reader.analyzeCalls) != 0 {
		t.Fatalf("the Matroska reader opened %v", reader.analyzeCalls)
	}

	// Without the container reader, the file must still be checked.
	missing := errors.New("file not found")
	reader.checkErr = missing
	if _, err := service.Analyze(context.Background(), `D:\Filmes\Sumiu.mp4`, preferences); !errors.Is(err, missing) {
		t.Fatalf("Analyze of a missing mp4 = %v, want the check error", err)
	}
}

func TestAnalyzeStopsWhenCancelled(t *testing.T) {
	files := &fakeFiles{paths: []string{`D:\a.srt`, `D:\b.srt`}, content: map[string]media.SubtitleContent{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := analysis.NewService(&fakeReader{info: studyInfo()}, files).Analyze(ctx, `D:\x.mkv`, preferences); !errors.Is(err, context.Canceled) {
		t.Fatalf("Analyze = %v, want context.Canceled", err)
	}
	if len(files.loaded) != 0 {
		t.Fatalf("files read after cancellation: %v", files.loaded)
	}
}

func TestAnalyzePropagatesReaderErrors(t *testing.T) {
	failure := errors.New("disk on fire")
	service := analysis.NewService(&fakeReader{err: failure}, nil)

	if _, err := service.Analyze(context.Background(), `C:\x.mkv`, preferences); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want the reader's error", err)
	}
}

func event(id string, start, end int, text string) subtitle.Event {
	return subtitle.Event{ID: id, Start: time.Duration(start) * time.Millisecond, End: time.Duration(end) * time.Millisecond, OriginalText: text}
}

func TestLoadSubtitlesBuildsTimelinesForUniqueTracks(t *testing.T) {
	reader := &fakeReader{content: map[uint64]media.SubtitleContent{
		2: {Events: []subtitle.Event{event("2", 5_000, 6_000, "second"), event("1", 1_000, 2_000, "first")}, Skipped: 1},
		3: {Events: []subtitle.Event{event("1", 1_000, 2_000, "primeira")}},
	}}
	service := analysis.NewService(reader, nil)
	var reports []int64

	loaded, err := service.LoadSubtitles(context.Background(), studyAnalysis(), []uint64{2, 0, 3, 2}, func(done, total int64) {
		reports = append(reports, done)
	})
	if err != nil {
		t.Fatalf("LoadSubtitles: %v", err)
	}
	if !slices.Equal(reader.requested, []uint64{2, 3}) {
		t.Fatalf("reader was asked for %v, want [2 3]", reader.requested)
	}
	if !slices.Equal(reports, []int64{50, 100}) {
		t.Fatalf("progress = %v, want it forwarded", reports)
	}
	primary := loaded[2]
	if primary.Skipped != 1 || primary.Timeline.Len() != 2 {
		t.Fatalf("track 2 = %+v", primary)
	}
	if active := primary.Timeline.ActiveAt(1_500 * time.Millisecond); len(active) != 1 || active[0].OriginalText != "first" {
		t.Fatalf("timeline lookup = %+v", active)
	}
	if loaded[3].Timeline.Len() != 1 {
		t.Fatalf("track 3 = %+v", loaded[3])
	}
}

func TestLoadSubtitlesWithoutTracksDoesNotReadTheFile(t *testing.T) {
	reader := &fakeReader{}
	loaded, err := analysis.NewService(reader, nil).LoadSubtitles(context.Background(), studyAnalysis(), []uint64{0, 0}, nil)
	if err != nil || len(loaded) != 0 || reader.requested != nil {
		t.Fatalf("LoadSubtitles(no tracks) = %v, %v; reader asked for %v", loaded, err, reader.requested)
	}
}

func TestLoadSubtitlesReportsMissingTracksAndInvalidEvents(t *testing.T) {
	missing := &fakeReader{content: map[uint64]media.SubtitleContent{}}
	if _, err := analysis.NewService(missing, nil).LoadSubtitles(context.Background(), studyAnalysis(), []uint64{4}, nil); err == nil {
		t.Fatal("a track missing from the reader result was accepted")
	}

	invalid := &fakeReader{content: map[uint64]media.SubtitleContent{4: {Events: []subtitle.Event{event("1", 2_000, 1_000, "backwards")}}}}
	if _, err := analysis.NewService(invalid, nil).LoadSubtitles(context.Background(), studyAnalysis(), []uint64{4}, nil); !errors.Is(err, subtitle.ErrInvalidEvent) {
		t.Fatalf("invalid events error = %v, want ErrInvalidEvent", err)
	}
}

func TestLoadSubtitlesHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := analysis.NewService(&fakeReader{}, nil).LoadSubtitles(ctx, studyAnalysis(), []uint64{2}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
