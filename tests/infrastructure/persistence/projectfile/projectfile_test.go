package projectfile_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/projectfile"
)

// currentVersion is how a file written now names its version.
var currentVersion = fmt.Sprintf(`"version": %d`, project.Version)

func sample() project.Project {
	p := project.New()
	p.Tracks["uid:77"] = project.Track{Offset: 150, Edits: map[string]project.Edit{
		"12": {Original: project.HashOf("Isso está doente!"), Text: new("Que loucura!"), Start: new(project.Millis(61_120))},
	}}
	p.Tracks["file:Subs/Movie.pt-BR.srt"] = project.Track{Offset: -500}
	p.Tracks["uid:5"] = project.Track{Hearing: &project.Hearing{Language: "pt", Scores: []project.HeardScore{
		{Language: "en", Score: 0.2, Lines: 8}, {Language: "pt", Score: 0.9, Lines: 8},
	}}}
	p.Choice = project.Choice{Primary: "uid:77", Translation: "file:Subs/Movie.pt-BR.srt", Mode: "dual"}
	p.Passages = []project.Passage{
		{Name: "Discussão na loja", Start: 4_223_000, End: 4_264_000,
			Plan: &project.PassagePlan{Loop: true, Gap: 2000, Progressive: true, Goal: 5}},
		{Name: "Sem plano", Start: 1000, End: 9000},
	}
	p.Last = project.Last{Position: 638_000, Speed: 0.9, Karaoke: &project.Karaoke{
		Enabled: true, MinGap: 8000, Lead: 4000, Indicator: "countdown", Fill: true, Repeat: 2,
	}}
	return p
}

// samePassages compares passages field by field: the plan is a pointer.
func samePassages(a, b []project.Passage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Start != b[i].Start || a[i].End != b[i].End || (a[i].Plan == nil) != (b[i].Plan == nil) {
			return false
		}
		if a[i].Plan != nil && *a[i].Plan != *b[i].Plan {
			return false
		}
	}
	return true
}

// film writes a video of its own content in dir.
func film(t *testing.T, dir, name, content string) string {
	t.Helper()
	video := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(video), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(video, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return video
}

// legacy is where an older version kept a video's project in the app's
// folder when the video's folder took no writing: by its path.
func legacy(app, video string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(video))))
	return filepath.Join(app, "projects", hex.EncodeToString(sum[:8])+projectfile.Extension)
}

// only fails unless the folder holds just the names given.
func only(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	if strings.Join(got, "|") != strings.Join(names, "|") {
		t.Fatalf("%s holds %q, want %q", dir, got, names)
	}
}

// changeInPlace writes the film over with content of the same size, as a
// download that ended does, an hour later.
func changeInPlace(t *testing.T, video, content string) {
	t.Helper()
	info, err := os.Stat(video)
	if err != nil || info.Size() != int64(len(content)) {
		t.Fatalf("changeInPlace with %d bytes over %v (%v)", len(content), info, err)
	}
	if err := os.WriteFile(video, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	later := info.ModTime().Add(time.Hour)
	if err := os.Chtimes(video, later, later); err != nil {
		t.Fatal(err)
	}
}

func gone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s is still there (%v)", filepath.Base(path), err)
	}
}

func TestBesideTheVideo(t *testing.T) {
	t.Parallel()
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

// The project is kept in the app's folder, never beside the film, and comes
// back whole.
func TestSaveAndLoadAgain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := film(t, filepath.Join(dir, "Filmes"), "Movie.mkv", "the film")
	store := projectfile.Open(filepath.Join(dir, "app"))

	empty, err := store.Load(video)
	if err != nil || empty.Tracks == nil || empty.Passages == nil || len(empty.Tracks) != 0 {
		t.Fatalf("a video without a file: %+v %v", empty, err)
	}
	if err := store.Save(video, sample()); err != nil {
		t.Fatal(err)
	}
	only(t, filepath.Join(dir, "Filmes"), "Movie.mkv")
	path, err := store.File(video)
	if err != nil || filepath.Dir(path) != filepath.Join(dir, "app", "projects") {
		t.Fatalf("File = %s, %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), currentVersion) {
		t.Fatalf("file in the app's folder: %v\n%s", err, data)
	}
	got, err := store.Load(video)
	if err != nil {
		t.Fatal(err)
	}
	edit := got.Tracks["uid:77"].Edits["12"]
	if got.Tracks["uid:77"].Offset != 150 || *edit.Text != "Que loucura!" || *edit.Start != 61_120 || edit.End != nil ||
		got.Tracks["file:Subs/Movie.pt-BR.srt"].Offset != -500 || got.Choice != sample().Choice ||
		!samePassages(got.Passages, sample().Passages) {
		t.Fatalf("loaded = %+v", got)
	}
	want := sample().Last
	if got.Last.Position != want.Position || got.Last.Speed != want.Speed || got.Last.Karaoke == nil || *got.Last.Karaoke != *want.Karaoke {
		t.Fatalf("last = %+v, want %+v", got.Last, want)
	}
	// A track kept without edits can take one at once.
	got.Tracks["file:Subs/Movie.pt-BR.srt"].Edits["1"] = project.Edit{Original: project.HashOf("x"), Text: new("y")}
}

// A film renamed or moved keeps its project; another film has its own.
func TestTheProjectFollowsTheFilm(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := film(t, dir, "Movie.mkv", strings.Repeat("the film ", 40_000))
	store := projectfile.Open(filepath.Join(dir, "app"))
	if err := store.Save(video, sample()); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(dir, "Filme da escola.mkv")
	moved := filepath.Join(dir, "Pendrive", "Outro nome.mkv")
	if err := os.Rename(video, renamed); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(renamed); err != nil || got.Choice.Mode != "dual" {
		t.Fatalf("renamed: %q, %v", got.Choice.Mode, err)
	}
	if err := os.MkdirAll(filepath.Dir(moved), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(renamed, moved); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(moved); err != nil || got.Choice.Mode != "dual" || len(got.Passages) != 2 {
		t.Fatalf("moved: %+v, %v", got.Choice, err)
	}
	other := film(t, dir, "Outro filme.mkv", strings.Repeat("another film ", 30_000))
	if got, err := store.Load(other); err != nil || got.Choice.Mode != "" || len(got.Passages) != 0 {
		t.Fatalf("another film: %+v, %v", got, err)
	}
}

// A film written over in place (a download that ended, a copy put in its
// place) keeps the project its path last kept, and the next save keeps it
// by the new content.
func TestAFilmChangedInPlaceKeepsItsProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := film(t, dir, "Movie.mkv", "half of the film")
	store := projectfile.Open(filepath.Join(dir, "app"))
	if err := store.Save(video, sample()); err != nil {
		t.Fatal(err)
	}
	before, err := store.File(video)
	if err != nil {
		t.Fatal(err)
	}
	changeInPlace(t, video, "whole film, done")
	got, err := store.Load(video)
	if err != nil || got.Choice.Mode != "dual" {
		t.Fatalf("changed in place: %q, %v", got.Choice.Mode, err)
	}
	got.Choice.Mode = "always"
	if err := store.Save(video, got); err != nil {
		t.Fatal(err)
	}
	after, err := store.File(video)
	if err != nil || after == before {
		t.Fatalf("the new content is kept as %s (%v), the old as %s", after, err, before)
	}
	if again, err := store.Load(video); err != nil || again.Choice.Mode != "always" {
		t.Fatalf("again: %q, %v", again.Choice.Mode, err)
	}
}

// A project an older version left beside the film moves into the app's
// folder on the first opening: the old file goes, and so does the older
// copy kept by the film's path.
func TestAProjectBesideTheFilmMovesIn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	video := film(t, filepath.Join(dir, "Filmes"), "Movie.mkv", "the film")
	if err := os.WriteFile(projectfile.Beside(video), []byte(`{"version": 2, "choice": {"mode": "dual"}, "passages": [{"name": "Rua", "start": 60000, "end": 64000}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	older := legacy(app, video)
	if err := os.MkdirAll(filepath.Dir(older), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(older, []byte(`{"version": 2, "choice": {"mode": "always"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := projectfile.Open(app)
	got, err := store.Load(video)
	if err != nil || got.Choice.Mode != "dual" || len(got.Passages) != 1 {
		t.Fatalf("Load = %+v, %v", got, err)
	}
	only(t, filepath.Join(dir, "Filmes"), "Movie.mkv")
	gone(t, older)
	path, _ := store.File(video)
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), currentVersion) || !strings.Contains(string(data), `"Rua"`) {
		t.Fatalf("moved in: %v\n%s", err, data)
	}
	// A fresh store (the app opened again) finds it there.
	if again, err := projectfile.Open(app).Load(video); err != nil || len(again.Passages) != 1 || again.Choice.Mode != "dual" {
		t.Fatalf("again = %+v, %v", again, err)
	}
}

// A project an older version kept in the app's folder by the film's path
// (its folder took no writing) moves in too, whatever the path's case.
func TestAProjectKeptByThePathMovesIn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	video := film(t, dir, "Movie.mkv", "the film")
	older := legacy(app, strings.ToUpper(video))
	if err := os.MkdirAll(filepath.Dir(older), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(older, []byte(`{"version": 3, "choice": {"mode": "always"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := projectfile.Open(app)
	if got, err := store.Load(video); err != nil || got.Choice.Mode != "always" {
		t.Fatalf("Load = %q, %v", got.Choice.Mode, err)
	}
	gone(t, older)
	if got, err := projectfile.Open(app).Load(video); err != nil || got.Choice.Mode != "always" {
		t.Fatalf("again = %q, %v", got.Choice.Mode, err)
	}
}

// An old project that cannot be removed (in use by another program, in a
// folder without rights) stays where it is but is no longer read: what is
// saved after wins, and forgetting the video leaves it empty.
func TestAnOldProjectThatCannotGoStaysUnread(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("an open file can be removed here")
	}
	dir := t.TempDir()
	video := film(t, dir, "Movie.mkv", "the film")
	beside := projectfile.Beside(video)
	if err := os.WriteFile(beside, []byte(`{"version": 3, "choice": {"mode": "dual"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// Go opens files without letting them be removed while open.
	holder, err := os.Open(beside)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { holder.Close() })
	store := projectfile.Open(filepath.Join(dir, "app"))
	got, err := store.Load(video)
	if err != nil || got.Choice.Mode != "dual" {
		t.Fatalf("Load = %q, %v", got.Choice.Mode, err)
	}
	if _, err := os.Stat(beside); err != nil {
		t.Fatalf("the file in use went: %v", err)
	}
	// Changed in place before any save, the film keeps the project moved in,
	// and the old file is not read (nor moved in) again.
	changeInPlace(t, video, "the mlif")
	if got, err = store.Load(video); err != nil || got.Choice.Mode != "dual" {
		t.Fatalf("changed in place: %q, %v", got.Choice.Mode, err)
	}
	if path, _ := store.File(video); path != "" {
		gone(t, path)
	}
	got.Choice.Mode = "always"
	if err := store.Save(video, got); err != nil {
		t.Fatal(err)
	}
	if again, err := projectfile.Open(filepath.Join(dir, "app")).Load(video); err != nil || again.Choice.Mode != "always" {
		t.Fatalf("after a save: %q, %v", again.Choice.Mode, err)
	}
	if err := store.Remove(video); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if again, err := projectfile.Open(filepath.Join(dir, "app")).Load(video); err != nil || again.Choice.Mode != "" {
		t.Fatalf("forgotten, the old file came back: %q, %v", again.Choice.Mode, err)
	}
}

func TestFilesThatCannotBeTrusted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := film(t, dir, "Movie.mkv", "the film")
	store := projectfile.Open(filepath.Join(dir, "app"))
	kept, err := store.File(video)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		body string
		want error
	}{
		{"newer version", fmt.Sprintf(`{"version": %d, "tracks": {}}`, project.Version+1), project.ErrNewerVersion},
		{"language heard but not tried", `{"version": 4, "tracks": {"uid:1": {"edits": {}, "hearing": {"scores": [{"language": "en", "score": 0.5, "lines": 8}], "language": "pt"}}}}`, project.ErrInvalid},
		{"language heard not a language", `{"version": 4, "tracks": {"uid:1": {"edits": {}, "hearing": {"scores": [{"language": "../en", "score": 0.5, "lines": 8}]}}}}`, project.ErrInvalid},
		{"score heard past 1", `{"version": 4, "tracks": {"uid:1": {"edits": {}, "hearing": {"scores": [{"language": "en", "score": 1.5, "lines": 8}]}}}}`, project.ErrInvalid},
		{"too many added files", `{"version": 3, "addedFiles": ["1","2","3","4","5","6","7","8","9","10","11","12","13","14","15","16","17"]}`, project.ErrInvalid},
		{"empty added file", `{"version": 3, "addedFiles": ["  "]}`, project.ErrInvalid},
		{"empty edit text", `{"version": 1, "tracks": {"uid:1": {"edits": {"1": {"original": "0123456789abcdef", "text": ""}}}}}`, project.ErrInvalid},
		{"end before start", `{"version": 1, "passages": [{"name": "P", "start": 10, "end": 5}]}`, project.ErrInvalid},
		{"passage not an object", `{"version": 1, "passages": [1]}`, nil},
		{"goal too high", `{"version": 2, "passages": [{"name": "P", "start": 0, "end": 5000, "plan": {"goal": 100}}]}`, project.ErrInvalid},
		{"gap too long", `{"version": 2, "passages": [{"name": "P", "start": 0, "end": 5000, "plan": {"loop": true, "gap": 5001}}]}`, project.ErrInvalid},
		{"speed out of range", `{"version": 2, "last": {"position": 0, "speed": 9}}`, project.ErrInvalid},
		{"position past the limit", `{"version": 2, "last": {"position": 360000001}}`, project.ErrInvalid},
		{"unknown indicator", `{"version": 2, "last": {"position": 0, "karaoke": {"minGap": 10000, "lead": 5000, "indicator": "dots"}}}`, project.ErrInvalid},
		{"not JSON", `{"version": 1, "tracks": `, nil},
		{"no version", `{"tracks": {}}`, nil},
		{"wrong types", `{"version": 1, "tracks": []}`, nil},
	}
	// Left beside the film by an older version, or kept by the app: refused
	// alike, never changed, and an old one never moved in.
	for _, path := range []string{projectfile.Beside(video), kept} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, c := range cases {
			if err := os.WriteFile(path, []byte(c.body), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := store.Load(video)
			if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
				t.Errorf("%s, %s: Load = %v, want %v", filepath.Base(path), c.name, err, c.want)
			}
			if got.Tracks == nil || len(got.Tracks) != 0 || len(got.Passages) != 0 {
				t.Errorf("%s, %s: a refused file left values: %+v", filepath.Base(path), c.name, got)
			}
			if data, _ := os.ReadFile(path); string(data) != c.body {
				t.Errorf("%s, %s: Load changed the file", filepath.Base(path), c.name)
			}
			if path != kept {
				if _, err := os.Stat(kept); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("%s: a refused old file was moved in", c.name)
				}
			}
		}
		// A file larger than the store reads is refused without reading it all.
		huge := `{"version": 1, "passages": [` + strings.Repeat(`{"name": "P", "start": 0, "end": 1},`, 300_000) + `]}`
		if err := os.WriteFile(path, []byte(huge), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(video); err == nil {
			t.Fatalf("%s: a huge file was read", filepath.Base(path))
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	// An invalid project is never written.
	bad := sample()
	bad.Passages[0].End = 0
	if err := store.Save(video, bad); !errors.Is(err, project.ErrInvalid) {
		t.Fatalf("Save of an invalid project = %v", err)
	}
	gone(t, kept)
	// The mark of the film's path names a key, never a file elsewhere.
	mark := strings.TrimSuffix(legacy(filepath.Join(dir, "app"), video), projectfile.Extension) + ".film"
	if err := os.WriteFile(mark, []byte(`..\..\escape`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "escape"+projectfile.Extension), []byte(`{"version": 3, "choice": {"mode": "dual"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(video); err != nil || got.Choice.Mode != "" {
		t.Fatalf("a mark led out of the app's folder: %q, %v", got.Choice.Mode, err)
	}
}

// A file of version 1 (before the plans and "last") still opens, its
// passages pausing at the end with no helps, and the next save writes
// the current version without losing anything.
func TestAVersion1FileOpensAndIsKept(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := film(t, dir, "Movie.mkv", "the film")
	store := projectfile.Open(filepath.Join(dir, "app"))
	old := `{"version": 1, "tracks": {"uid:7": {"offset": 250, "edits": {}}},
		"choice": {"audio": "", "primary": "uid:7", "translation": "", "mode": "on-demand"},
		"passages": [{"name": "Loja", "start": 10000, "end": 25000}, {"name": "Rua", "start": 60000, "end": 64000}]}`
	if err := os.WriteFile(projectfile.Beside(video), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(video)
	if err != nil {
		t.Fatalf("Load of a version 1 file: %v", err)
	}
	if len(got.Passages) != 2 || got.Passages[0].Name != "Loja" || got.Passages[1].End != 64_000 || got.Tracks["uid:7"].Offset != 250 {
		t.Fatalf("loaded = %+v", got)
	}
	for _, passage := range got.Passages {
		if passage.Plan != nil || passage.Practice().Mode != "pause" || passage.Practice().Goal != 0 {
			t.Fatalf("passage %q from version 1 = plan %+v, practice %+v", passage.Name, passage.Plan, passage.Practice())
		}
	}
	if got.Last.Position != 0 || got.Last.Speed != 0 || got.Last.Karaoke != nil {
		t.Fatalf("a version 1 file came with a last state: %+v", got.Last)
	}
	if err := store.Save(video, got); err != nil {
		t.Fatal(err)
	}
	path, _ := store.File(video)
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), currentVersion) || !strings.Contains(string(data), `"Rua"`) {
		t.Fatalf("saved again: %v\n%s", err, data)
	}
}

// Forgetting a video removes its project, is quiet when there is nothing to
// remove, and leaves a folder of that name alone.
func TestRemoveForgetsTheProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := film(t, dir, "Movie.mkv", "the film")
	store := projectfile.Open(filepath.Join(dir, "app"))
	if err := store.Save(video, sample()); err != nil {
		t.Fatal(err)
	}
	path, _ := store.File(video)
	if err := store.Remove(video); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	gone(t, path)
	if got, err := store.Load(video); err != nil || len(got.Passages) != 0 || got.Choice.Mode != "" {
		t.Fatalf("after Remove, Load = %+v %v; want an empty project", got, err)
	}
	if err := store.Remove(video); err != nil {
		t.Fatalf("Remove with nothing left: %v", err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(video); err != nil || got.Choice.Mode != "" {
		t.Fatalf("a folder in the way was read: %q, %v", got.Choice.Mode, err)
	}
	if err := store.Remove(video); err != nil {
		t.Fatalf("Remove with a folder in the way: %v", err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatal("Remove deleted a folder")
	}
	// Saved again, the video has a project again.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(video, sample()); err != nil {
		t.Fatal(err)
	}
	if got, err := projectfile.Open(filepath.Join(dir, "app")).Load(video); err != nil || got.Choice.Mode != "dual" {
		t.Fatalf("saved after Remove: %q, %v", got.Choice.Mode, err)
	}
	// A film changed in place, forgotten before any save, has nothing either.
	changeInPlace(t, video, "the mlif")
	if err := store.Remove(video); err != nil {
		t.Fatalf("Remove of a film changed in place: %v", err)
	}
	if got, err := projectfile.Open(filepath.Join(dir, "app")).Load(video); err != nil || got.Choice.Mode != "" {
		t.Fatalf("forgotten after a change in place: %q, %v", got.Choice.Mode, err)
	}
}

// A film that cannot be read has its project kept nowhere, and without the
// app's folder nothing is kept: never beside the film.
func TestNothingIsKeptWithoutAFilmOrAFolder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := filepath.Join(dir, "Movie.mkv")
	if err := projectfile.Open(filepath.Join(dir, "app")).Save(missing, sample()); err == nil {
		t.Fatal("saved the project of a film that is not there")
	}
	video := film(t, dir, "Movie.mkv", "the film")
	if err := projectfile.Open("").Save(video, sample()); !errors.Is(err, projectfile.ErrNoFolder) {
		t.Fatalf("Save without the app's folder = %v", err)
	}
	only(t, dir, "Movie.mkv")
}

func TestTrackKeysSurviveANewAnalysis(t *testing.T) {
	t.Parallel()
	video := `D:\Filmes\Movie.mkv`
	cases := []struct {
		track media.Track
		want  string
	}{
		{media.Track{Number: media.ExternalTrackBase + 3, Path: `D:\Filmes\Subs\Movie.pt-BR.srt`}, "file:Subs/Movie.pt-BR.srt"},
		{media.Track{Number: media.ExternalTrackBase, Path: `D:\Filmes\Movie.en.srt`}, "file:Movie.en.srt"},
		// A file outside the video's folder is known by its path from it, or by
		// its whole path on another drive: two files of the same name stay two.
		{media.Track{Number: media.ExternalTrackBase, Path: `D:\Legendas\Movie.en.srt`}, "file:../Legendas/Movie.en.srt"},
		{media.Track{Number: media.ExternalTrackBase, Path: `E:\Legendas\Movie.en.srt`}, "file:E:/Legendas/Movie.en.srt"},
		{media.Track{Number: 4, UID: 1234567890123}, "uid:1234567890123"},
		{media.Track{Number: 4}, "track:4"},
	}
	for _, c := range cases {
		if got := projectfile.TrackKey(video, c.track); got != c.want {
			t.Errorf("TrackKey(%+v) = %s, want %s", c.track, got, c.want)
		}
	}
}

// A version 2 file opens with no added files, and the files picked for a
// video come back by their paths, wherever they are.
func TestAddedFilesComeBack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := film(t, filepath.Join(dir, "Filmes"), "Movie.mkv", "the film")
	store := projectfile.Open(filepath.Join(dir, "app"))
	if err := os.WriteFile(projectfile.Beside(video), []byte(`{"version": 2, "passages": [{"name": "Rua", "start": 60000, "end": 64000}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(video)
	if err != nil || len(got.AddedFiles) != 0 || len(got.Passages) != 1 {
		t.Fatalf("a version 2 file = %+v, %v", got, err)
	}
	picked := []string{filepath.Join(dir, "Legendas", "Movie.en.srt"), filepath.Join(dir, "Filmes", "Subs", "Movie.fr.srt"), `Z:\Outro\Movie.de.srt`}
	for _, path := range picked {
		got.AddedFiles = append(got.AddedFiles, projectfile.AddedPath(video, path))
	}
	if got.AddedFiles[0] != "../Legendas/Movie.en.srt" || got.AddedFiles[1] != "Subs/Movie.fr.srt" || got.AddedFiles[2] != "Z:/Outro/Movie.de.srt" {
		t.Fatalf("added paths = %q", got.AddedFiles)
	}
	if err := store.Save(video, got); err != nil {
		t.Fatal(err)
	}
	again, err := store.Load(video)
	if err != nil || len(again.AddedFiles) != 3 {
		t.Fatalf("again = %+v, %v", again.AddedFiles, err)
	}
	for i, added := range again.AddedFiles {
		if got := projectfile.ResolveAdded(video, added); got != filepath.Clean(picked[i]) {
			t.Errorf("ResolveAdded(%q) = %s, want %s", added, got, picked[i])
		}
	}
}
