package projectfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/project"
	"lingoreel/internal/infrastructure/persistence/projectfile"
)

func sample() project.Project {
	p := project.New()
	p.Tracks["uid:77"] = project.Track{Offset: 150, Edits: map[string]project.Edit{
		"12": {Original: project.HashOf("Isso está doente!"), Text: new("Que loucura!"), Start: new(project.Millis(61_120))},
	}}
	p.Tracks["file:Subs/Movie.pt-BR.srt"] = project.Track{Offset: -500}
	p.Choice = project.Choice{Primary: "uid:77", Translation: "file:Subs/Movie.pt-BR.srt", Mode: "dual"}
	p.Passages = []project.Passage{{Name: "Discussão na loja", Start: 4_223_000, End: 4_264_000}}
	return p
}

func TestBesideTheVideo(t *testing.T) {
	cases := map[string]string{
		`D:\Filmes\Movie.mkv`:       `D:\Filmes\Movie.dualsub.json`,
		`D:\Filmes\Movie.part1.mkv`: `D:\Filmes\Movie.part1.dualsub.json`,
		`D:\Filmes\Movie`:           `D:\Filmes\Movie.dualsub.json`,
	}
	for video, want := range cases {
		if got := projectfile.Beside(video); got != want {
			t.Errorf("Beside(%s) = %s, want %s", video, got, want)
		}
	}
}

func TestSaveAndLoadAgain(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Movie.mkv")
	store := projectfile.Open(filepath.Join(dir, "app"))

	empty, err := store.Load(video)
	if err != nil || empty.Tracks == nil || empty.Passages == nil || len(empty.Tracks) != 0 {
		t.Fatalf("a video without a file: %+v %v", empty, err)
	}
	if err := store.Save(video, sample()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Movie.dualsub.json"))
	if err != nil || !strings.Contains(string(data), `"version": 1`) {
		t.Fatalf("file beside the video: %v\n%s", err, data)
	}
	got, err := store.Load(video)
	if err != nil {
		t.Fatal(err)
	}
	edit := got.Tracks["uid:77"].Edits["12"]
	if got.Tracks["uid:77"].Offset != 150 || *edit.Text != "Que loucura!" || *edit.Start != 61_120 || edit.End != nil ||
		got.Tracks["file:Subs/Movie.pt-BR.srt"].Offset != -500 || got.Choice != sample().Choice ||
		len(got.Passages) != 1 || got.Passages[0] != sample().Passages[0] {
		t.Fatalf("loaded = %+v", got)
	}
	// A track kept without edits can take one at once.
	got.Tracks["file:Subs/Movie.pt-BR.srt"].Edits["1"] = project.Edit{Original: project.HashOf("x"), Text: new("y")}
	if _, err := os.Stat(filepath.Join(dir, "app")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the app's folder was written while the video's folder took the file")
	}
}

func TestAFolderThatTakesNoWritingUsesTheAppFolder(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Movie.mkv")
	// A folder where the file should be: writing beside the video fails.
	blocked := projectfile.Beside(video)
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	store := projectfile.Open(filepath.Join(dir, "app"))
	if err := store.Save(video, sample()); err != nil {
		t.Fatal(err)
	}
	fallback := store.Fallback(video)
	if !strings.HasPrefix(fallback, filepath.Join(dir, "app", "projects")) {
		t.Fatalf("fallback at %s", fallback)
	}
	if _, err := os.Stat(fallback); err != nil {
		t.Fatalf("no file in the app's folder: %v", err)
	}
	// The same video typed in another case has the same file.
	if store.Fallback(strings.ToUpper(video)) != fallback {
		t.Fatal("the fallback depends on the case of the path")
	}
	if store.Fallback(filepath.Join(dir, "Other.mkv")) == fallback {
		t.Fatal("two videos share a fallback")
	}
	// Loading finds it there, and nothing was left in the blocked folder.
	if got, err := store.Load(video); err != nil || got.Choice.Mode != "dual" {
		t.Fatalf("Load = %+v %v", got.Choice, err)
	}
	if entries, _ := os.ReadDir(blocked); len(entries) != 0 {
		t.Fatalf("left %d entries in the blocked folder", len(entries))
	}
	// Without an app folder the failure is reported.
	if err := projectfile.Open("").Save(video, sample()); err == nil {
		t.Fatal("saved nowhere without an error")
	}
}

func TestTheFileBesideTheVideoComesFirst(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Movie.mkv")
	store := projectfile.Open(filepath.Join(dir, "app"))
	beside, fallback := sample(), sample()
	fallback.Choice.Mode = "always"
	// One save through a blocked folder lands in the app's folder; the next,
	// with the folder free, beside the video: both files exist.
	blocked := projectfile.Beside(video)
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(video, fallback); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(video, beside); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(video); err != nil || got.Choice.Mode != "dual" {
		t.Fatalf("Load took %q (%v), want the file beside the video", got.Choice.Mode, err)
	}
}

func TestFilesThatCannotBeTrusted(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Movie.mkv")
	store := projectfile.Open(filepath.Join(dir, "app"))
	path := projectfile.Beside(video)
	cases := []struct {
		name string
		body string
		want error
	}{
		{"newer version", `{"version": 2, "tracks": {}}`, project.ErrNewerVersion},
		{"empty edit text", `{"version": 1, "tracks": {"uid:1": {"edits": {"1": {"original": "0123456789abcdef", "text": ""}}}}}`, project.ErrInvalid},
		{"end before start", `{"version": 1, "passages": [{"name": "P", "start": 10, "end": 5}]}`, project.ErrInvalid},
		{"not JSON", `{"version": 1, "tracks": `, nil},
		{"no version", `{"tracks": {}}`, nil},
		{"wrong types", `{"version": 1, "tracks": []}`, nil},
	}
	for _, c := range cases {
		if err := os.WriteFile(path, []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := store.Load(video)
		if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
			t.Errorf("%s: Load = %v, want %v", c.name, err, c.want)
		}
		if got.Tracks == nil || len(got.Tracks) != 0 || len(got.Passages) != 0 {
			t.Errorf("%s: a refused file left values: %+v", c.name, got)
		}
		if data, _ := os.ReadFile(path); string(data) != c.body {
			t.Errorf("%s: Load changed the file", c.name)
		}
	}
	// A file larger than the store reads is refused without reading it all.
	huge := `{"version": 1, "passages": [` + strings.Repeat(`{"name": "P", "start": 0, "end": 1},`, 300_000) + `]}`
	if err := os.WriteFile(path, []byte(huge), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(video); err == nil {
		t.Fatal("a huge file was read")
	}
	// An invalid project is never written.
	bad := sample()
	bad.Passages[0].End = 0
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(video, bad); !errors.Is(err, project.ErrInvalid) {
		t.Fatalf("Save of an invalid project = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("an invalid project reached the disk")
	}
}

func TestTrackKeysSurviveANewAnalysis(t *testing.T) {
	video := `D:\Filmes\Movie.mkv`
	cases := []struct {
		track media.Track
		want  string
	}{
		{media.Track{Number: media.ExternalTrackBase + 3, Path: `D:\Filmes\Subs\Movie.pt-BR.srt`}, "file:Subs/Movie.pt-BR.srt"},
		{media.Track{Number: media.ExternalTrackBase, Path: `D:\Filmes\Movie.en.srt`}, "file:Movie.en.srt"},
		// A file outside the video's folder is known by its name.
		{media.Track{Number: media.ExternalTrackBase, Path: `E:\Legendas\Movie.en.srt`}, "file:Movie.en.srt"},
		{media.Track{Number: media.ExternalTrackBase, Path: `D:\Legendas\Movie.en.srt`}, "file:Movie.en.srt"},
		{media.Track{Number: 4, UID: 1234567890123}, "uid:1234567890123"},
		{media.Track{Number: 4}, "track:4"},
	}
	for _, c := range cases {
		if got := projectfile.TrackKey(video, c.track); got != c.want {
			t.Errorf("TrackKey(%+v) = %s, want %s", c.track, got, c.want)
		}
	}
}
