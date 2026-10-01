package analysis_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"testing"
	"time"

	"lingoreel/internal/application/analysis"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/subtitle"
)

// dialogue returns n film-like lines (gaps of 0.8 to 6 s from 30 s on),
// reproducible from the seed, every one moved by shift.
func dialogue(seed uint64, n int, shift time.Duration, text string) media.SubtitleContent {
	random := rand.New(rand.NewPCG(seed, seed+1))
	at := 30 * time.Second
	events := make([]subtitle.Event, n)
	for i := range events {
		events[i] = subtitle.Event{ID: text, Start: at + shift, End: at + shift + 700*time.Millisecond, OriginalText: text}
		at += 800*time.Millisecond + time.Duration(random.Int64N(int64(5200*time.Millisecond)))
	}
	return media.SubtitleContent{Events: events}
}

const englishLine = "You know where she is? No, I do not know. That is very good, thank you. Let us go now."

// movieWithDownloads is the real case: Portuguese inside the video and two
// English files in subfolders, one for this cut and one for another.
func movieWithDownloads() (*fakeReader, *fakeFiles) {
	reader := &fakeReader{info: portugueseOnlyMovie(), content: map[uint64]media.SubtitleContent{
		4: dialogue(40, 24, 0, "Forçada"),
		5: dialogue(50, 600, 0, "Fala completa"),
	}}
	reader.info.Duration = 2 * time.Hour
	files := &fakeFiles{
		candidates: []string{`D:\Filmes\other\Movie DC.en.srt`, `D:\Filmes\theatrical\Movie.en.srt`},
		content: map[string]media.SubtitleContent{
			// Another cut: nothing lines up with the video.
			`D:\Filmes\other\Movie DC.en.srt`: dialogue(77, 700, 0, englishLine),
			// This cut, shown 500 ms early.
			`D:\Filmes\theatrical\Movie.en.srt`: dialogue(50, 600, -500*time.Millisecond, englishLine),
		},
	}
	return reader, files
}

func TestMatchingCandidateIsChosenAndCorrected(t *testing.T) {
	reader, files := movieWithDownloads()
	service := analysis.NewService(reader, files)
	result, err := service.Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, preferences)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := result.Info.Track(media.ExternalTrackBase)
	theatrical, _ := result.Info.Track(media.ExternalTrackBase + 1)
	if !other.Candidate || !theatrical.Candidate {
		t.Fatalf("files from subfolders must be candidates: %+v %+v", other, theatrical)
	}
	if other.Alignment.Verdict != subtitle.VerdictNoMatch || theatrical.Alignment.Verdict != subtitle.VerdictMatch {
		t.Fatalf("verdicts = %s, %s; want no-match, match", other.Alignment.Verdict, theatrical.Alignment.Verdict)
	}
	want := media.Selection{Audio: 1, Primary: media.ExternalTrackBase + 1, Translation: 5}
	if result.Suggested != want {
		t.Fatalf("Suggested = %+v, want %+v", result.Suggested, want)
	}

	loaded, err := service.LoadSubtitles(context.Background(), result, []uint64{want.Primary, want.Translation}, nil)
	if err != nil {
		t.Fatal(err)
	}
	primary := loaded[want.Primary]
	if !primary.Corrected || (primary.Correction.Offset-500*time.Millisecond).Abs() > 5*time.Millisecond {
		t.Fatalf("primary correction = %v %+v, want about +500 ms", primary.Corrected, primary.Correction)
	}
	// The first line was shown at 29.5 s; corrected it starts with the video's at 30 s.
	if first := primary.Timeline.Events()[0].Start; (first - 30*time.Second).Abs() > 5*time.Millisecond {
		t.Fatalf("first corrected line at %v, want 30s", first)
	}
	if loaded[want.Translation].Corrected {
		t.Fatal("a track of the video itself was corrected")
	}
}

func TestFilesForAnotherCutAreNeverChosen(t *testing.T) {
	reader, files := movieWithDownloads()
	// Named after the video, but timed for a cut that runs past its end.
	files.paths, files.candidates = []string{`D:\Filmes\Exemplo.en.srt`}, nil
	files.content[`D:\Filmes\Exemplo.en.srt`] = dialogue(50, 600, 0, englishLine)
	reader.info.Duration = 10 * time.Minute
	result, err := analysis.NewService(reader, files).Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, preferences)
	if err != nil {
		t.Fatal(err)
	}
	track, _ := result.Info.Track(media.ExternalTrackBase)
	if track.Alignment.Verdict != subtitle.VerdictOtherCut || result.Suggested.Primary != 0 {
		t.Fatalf("verdict %s, primary %d; want other-cut and no primary", track.Alignment.Verdict, result.Suggested.Primary)
	}
}

func TestWithoutAReferenceOnlyTheVideosOwnFilesAreChosen(t *testing.T) {
	// A video whose tracks are not read (.mp4): nothing to compare with.
	files := &fakeFiles{
		paths:      []string{`D:\Filmes\Movie.pt.srt`},
		candidates: []string{`D:\Filmes\subs\english.srt`},
		content: map[string]media.SubtitleContent{
			`D:\Filmes\Movie.pt.srt`:     dialogue(1, 300, 0, portugueseText),
			`D:\Filmes\subs\english.srt`: dialogue(1, 300, 0, englishLine),
		},
	}
	reader := &fakeReader{}
	result, err := analysis.NewService(reader, files).Analyze(context.Background(), `D:\Filmes\Movie.mp4`, preferences)
	if err != nil {
		t.Fatal(err)
	}
	if reader.requested != nil {
		t.Fatalf("read container tracks of a video without readable ones: %v", reader.requested)
	}
	candidate, _ := result.Info.Track(media.ExternalTrackBase + 1)
	if candidate.Alignment.Verdict != subtitle.VerdictUnknown {
		t.Fatalf("candidate verdict = %s, want unknown", candidate.Alignment.Verdict)
	}
	want := media.Selection{Translation: media.ExternalTrackBase}
	if result.Suggested != want {
		t.Fatalf("Suggested = %+v, want %+v (the unconfirmed candidate stays out)", result.Suggested, want)
	}
}

func TestAnUnreadableReferenceStillAnalyzes(t *testing.T) {
	reader, files := movieWithDownloads()
	reader.readErr = errors.New("damaged cluster")
	result, err := analysis.NewService(reader, files).Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, preferences)
	if err != nil {
		t.Fatalf("a reference that cannot be read failed the analysis: %v", err)
	}
	if track, _ := result.Info.Track(media.ExternalTrackBase + 1); track.Alignment.Verdict != subtitle.VerdictUnknown {
		t.Fatalf("verdict = %s, want unknown", track.Alignment.Verdict)
	}
	if result.Suggested.Primary != 0 {
		t.Fatalf("primary = %d, want none (candidates need a confirmed match)", result.Suggested.Primary)
	}
}

func TestCancellingWhileReadingTheReferenceStops(t *testing.T) {
	reader, files := movieWithDownloads()
	ctx, cancel := context.WithCancel(context.Background())
	reader.whileReading = cancel
	if _, err := analysis.NewService(reader, files).Analyze(ctx, `D:\Filmes\Exemplo.mkv`, preferences); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if reader.requested == nil {
		t.Fatal("the reference was never read: the test did not reach the cancellation it checks")
	}
}

func TestNoFilesMeansNoReferenceRead(t *testing.T) {
	reader := &fakeReader{info: portugueseOnlyMovie()}
	if _, err := analysis.NewService(reader, &fakeFiles{}).Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, preferences); err != nil {
		t.Fatal(err)
	}
	if reader.requested != nil {
		t.Fatalf("read %v without any file to compare", reader.requested)
	}
}
