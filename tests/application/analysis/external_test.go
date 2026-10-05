package analysis_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// fakeFiles serves subtitle files from memory.
type fakeFiles struct {
	paths      []string
	candidates []string
	findErr    error
	content    map[string]media.SubtitleContent
	failing    map[string]error
	loaded     []string
}

func (f *fakeFiles) Find(string) ([]string, error) { return f.paths, f.findErr }

func (f *fakeFiles) FindCandidates(string) ([]string, error) { return f.candidates, f.findErr }

func (f *fakeFiles) Load(path string) (subtitle.Format, media.SubtitleContent, error) {
	f.loaded = append(f.loaded, path)
	if err := f.failing[path]; err != nil {
		return "", media.SubtitleContent{}, err
	}
	return subtitle.FormatSRT, f.content[path], nil
}

func lines(texts ...string) media.SubtitleContent {
	var events []subtitle.Event
	for i, text := range texts {
		events = append(events, event(string(rune('a'+i)), 1_000*(i+1), 1_000*(i+1)+900, text))
	}
	return media.SubtitleContent{Events: events}
}

const portugueseText = "Não, você está aqui. Eu vou fazer isso agora. Tudo bem, então vamos. Ele não tem nada. " +
	"Você sabe onde ela está? Não, eu não sei. Isso é muito bom, obrigado. Vamos embora agora, tudo aqui."

// A typical layout: only Portuguese inside, the user
// downloaded an English file and an unnamed Portuguese one.
func portugueseOnlyMovie() media.Info {
	return media.Info{Tracks: []media.Track{
		{Number: 1, Type: media.TrackAudio, Language: "en"},
		{Number: 4, Type: media.TrackSubtitle, Language: "pt", Name: "Forced SRT", Default: true, Readable: true},
		{Number: 5, Type: media.TrackSubtitle, Language: "pt", Name: "Complet SRT", Readable: true},
	}}
}

func TestExternalSubtitlesJoinTheAnalysisAndTheSelection(t *testing.T) {
	t.Parallel()
	files := &fakeFiles{
		paths: []string{`D:\Filmes\Exemplo.en.srt`, `D:\Filmes\Subs\legenda.srt`},
		content: map[string]media.SubtitleContent{
			`D:\Filmes\Exemplo.en.srt`:   lines("Captain Nova and Comet!", "Get them both!"),
			`D:\Filmes\Subs\legenda.srt`: lines(portugueseText),
		},
	}
	service := analysis.NewService(&fakeReader{info: portugueseOnlyMovie()}, files)
	result, err := service.Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, preferences)
	if err != nil {
		t.Fatal(err)
	}
	english, found := result.Info.Track(media.ExternalTrackBase)
	if !found || !english.External() || english.Language != "en" || !english.Readable || english.Name != "Exemplo.en.srt" {
		t.Fatalf("first file = %+v, %v", english, found)
	}
	unnamed, found := result.Info.Track(media.ExternalTrackBase + 1)
	if !found || unnamed.Language != "pt" {
		t.Fatalf("second file = %+v (its language must come from the text)", unnamed)
	}
	// English now exists: the downloaded file becomes the primary line and
	// the full internal Portuguese track stays the translation.
	want := media.Selection{Audio: 1, Primary: media.ExternalTrackBase, Translation: 5}
	if result.Suggested != want {
		t.Fatalf("Suggested = %+v, want %+v", result.Suggested, want)
	}
}

func TestLoadSubtitlesReadsFilesAndContainerTracksTogether(t *testing.T) {
	t.Parallel()
	files := &fakeFiles{
		paths:   []string{`D:\Filmes\Exemplo.en.srt`},
		content: map[string]media.SubtitleContent{`D:\Filmes\Exemplo.en.srt`: lines("Captain Nova and Comet!", "Get them both!")},
	}
	reader := &fakeReader{info: portugueseOnlyMovie(), content: map[uint64]media.SubtitleContent{5: lines("Capitão Nova e Cometa!")}}
	service := analysis.NewService(reader, files)
	result, err := service.Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, preferences)
	if err != nil {
		t.Fatal(err)
	}
	// The analysis read the container tracks as the timing reference.
	if !slices.Equal(reader.requested, []uint64{4, 5}) {
		t.Fatalf("analysis asked the container for %v, want its readable subtitles [4 5]", reader.requested)
	}
	files.loaded, reader.requested = nil, nil
	loaded, err := service.LoadSubtitles(context.Background(), result, []uint64{result.Suggested.Primary, result.Suggested.Translation}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reader.requested != nil {
		t.Fatalf("container read again when loading: %v", reader.requested)
	}
	// The analysis already read the file to learn its language; loading it
	// again would parse up to 32 MB twice.
	if len(files.loaded) != 0 {
		t.Fatalf("files read again when loading: %v", files.loaded)
	}
	if loaded[media.ExternalTrackBase].Timeline.Len() != 2 || loaded[5].Timeline.Len() != 1 {
		t.Fatalf("loaded = %+v", loaded)
	}
}

func TestUnreadableFilesAreListedButNeverSelected(t *testing.T) {
	t.Parallel()
	files := &fakeFiles{
		paths:   []string{`D:\Filmes\Exemplo.en.srt`},
		failing: map[string]error{`D:\Filmes\Exemplo.en.srt`: errors.New("garbage")},
	}
	result, err := analysis.NewService(&fakeReader{info: portugueseOnlyMovie()}, files).Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, preferences)
	if err != nil {
		t.Fatal(err)
	}
	track, found := result.Info.Track(media.ExternalTrackBase)
	if !found || track.Readable || track.Language != "en" {
		t.Fatalf("file = %+v, want listed, unreadable, language from its name", track)
	}
	if result.Suggested.Primary != 0 {
		t.Fatalf("primary = %d, want none (the only English is unreadable)", result.Suggested.Primary)
	}
}

func TestAFolderThatCannotBeListedStillAnalyzesTheVideo(t *testing.T) {
	t.Parallel()
	files := &fakeFiles{findErr: errors.New("access denied")}
	result, err := analysis.NewService(&fakeReader{info: studyInfo()}, files).Analyze(context.Background(), `D:\Filmes\x.mkv`, preferences)
	if err != nil || len(result.Info.Tracks) != 3 {
		t.Fatalf("Analyze = %+v, %v", result, err)
	}
}

func TestLoadSubtitlesRejectsUnknownTracksWithoutReading(t *testing.T) {
	t.Parallel()
	reader := &fakeReader{}
	files := &fakeFiles{}
	_, err := analysis.NewService(reader, files).LoadSubtitles(context.Background(), studyAnalysis(), []uint64{2, media.ExternalTrackBase + 9}, nil)
	if err == nil {
		t.Fatal("an unknown track number was accepted")
	}
	if reader.requested != nil || files.loaded != nil {
		t.Fatalf("read before validating: reader %v, files %v", reader.requested, files.loaded)
	}
}

// pickedFile is a file the user picks from another folder, in French.
const pickedFile = `E:\Downloads\Exemplo.FR.srt`

// pickRig is a video with one file next to it, analyzed, and files elsewhere
// to pick: a French one, one whose name tells no language, and a picture.
func pickRig(t *testing.T) (*analysis.Service, *fakeFiles, analysis.Analysis) {
	t.Helper()
	files := &fakeFiles{
		paths: []string{`D:\Filmes\Exemplo.en.srt`},
		content: map[string]media.SubtitleContent{
			`D:\Filmes\Exemplo.en.srt`:  lines("Hello."),
			pickedFile:                  lines("Bonjour."),
			`E:\Downloads\sem-nome.srt`: lines(portugueseText),
		},
		failing: map[string]error{`E:\Downloads\foto.jpg`: errors.New("not a subtitle")},
	}
	reader := &fakeReader{info: portugueseOnlyMovie(), content: map[uint64]media.SubtitleContent{5: lines("Olá.")}}
	service := analysis.NewService(reader, files)
	analyzed, err := service.Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, media.Preferences{Primary: "en"})
	if err != nil {
		t.Fatal(err)
	}
	return service, files, analyzed
}

// A file the user picks from anywhere joins an analyzed video as one more
// track: numbered after the others, named and in the language its name says,
// and measured against the video. The analysis passed in stays as it was.
func TestAPickedSubtitleFileJoinsTheVideo(t *testing.T) {
	t.Parallel()
	service, _, before := pickRig(t)
	tracksBefore := len(before.Info.Tracks)
	after, track, err := service.AddSubtitleFile(context.Background(), before, pickedFile)
	if err != nil {
		t.Fatal(err)
	}
	if track.Number != media.ExternalTrackBase+1 || track.Language != "fr" || !track.Added || !track.Readable || track.Name != "Exemplo.FR.srt" {
		t.Fatalf("picked track = %+v", track)
	}
	if track.Alignment.Verdict == "" {
		t.Fatal("the picked file was not measured against the video")
	}
	if len(before.Info.Tracks) != tracksBefore || len(after.Info.Tracks) != tracksBefore+1 {
		t.Fatalf("tracks before %d → %d, after %d: the analysis passed in changed", tracksBefore, len(before.Info.Tracks), len(after.Info.Tracks))
	}
}

// A picked file is read once: loading it later reads nothing again.
func TestAPickedFileIsReadOnce(t *testing.T) {
	t.Parallel()
	service, files, before := pickRig(t)
	after, track, err := service.AddSubtitleFile(context.Background(), before, pickedFile)
	if err != nil {
		t.Fatal(err)
	}
	files.loaded = nil
	loaded, err := service.LoadSubtitles(context.Background(), after, []uint64{track.Number}, nil)
	if err != nil || loaded[track.Number].Timeline.Len() != 1 || len(files.loaded) != 0 {
		t.Fatalf("loading the picked file: %v, read again %v", err, files.loaded)
	}
}

// The same file picked again, in another case, is the same track; a name
// with no language is read by its text, into the next number.
func TestPickedFilesAreKnownByTheirPath(t *testing.T) {
	t.Parallel()
	service, _, before := pickRig(t)
	after, track, err := service.AddSubtitleFile(context.Background(), before, pickedFile)
	if err != nil {
		t.Fatal(err)
	}
	again, same, err := service.AddSubtitleFile(context.Background(), after, `e:\downloads\exemplo.fr.srt`)
	if err != nil || same.Number != track.Number || len(again.Info.Tracks) != len(after.Info.Tracks) {
		t.Fatalf("the same file twice: %+v, %v, %d tracks", same, err, len(again.Info.Tracks))
	}
	if _, unnamed, err := service.AddSubtitleFile(context.Background(), after, `E:\Downloads\sem-nome.srt`); err != nil || unnamed.Language != "pt" || unnamed.Number != track.Number+1 {
		t.Fatalf("unnamed file: %+v, %v", unnamed, err)
	}
}

// A file that is no subtitle is refused, and the analysis does not change.
func TestAPickedFileThatIsNoSubtitleIsRefused(t *testing.T) {
	t.Parallel()
	service, _, before := pickRig(t)
	if kept, _, err := service.AddSubtitleFile(context.Background(), before, `E:\Downloads\foto.jpg`); err == nil || len(kept.Info.Tracks) != len(before.Info.Tracks) {
		t.Fatalf("a picture was taken as a subtitle: %v", err)
	}
}

// One video takes a bounded number of picked files.
func TestPickedSubtitleFilesHaveALimit(t *testing.T) {
	t.Parallel()
	files := &fakeFiles{content: map[string]media.SubtitleContent{}}
	service := analysis.NewService(&fakeReader{info: portugueseOnlyMovie()}, files)
	current, err := service.Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, media.Preferences{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 100 {
		next, _, err := service.AddSubtitleFile(context.Background(), current, fmt.Sprintf(`E:\Legendas\%d.en.srt`, i))
		if errors.Is(err, analysis.ErrTooManyFiles) {
			if i < 8 {
				t.Fatalf("refused after only %d files", i)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		current = next
	}
	t.Fatal("100 picked files were all taken")
}
