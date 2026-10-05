package subtitlefile_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
)

const srt = "1\n00:00:01,000 --> 00:00:02,500\nHello there.\n\n2\n00:00:03,000 --> 00:00:04,000\nGeneral Kenobi.\n"

// layout creates files (and their folders) under a temporary root; names
// ending in "/" are folders.
func layout(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		if name[len(name)-1] == '/' {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(srt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func find(t *testing.T, root, video string) []string {
	t.Helper()
	found, err := subtitlefile.Files{}.Find(filepath.Join(root, video))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	relative := make([]string, len(found))
	for i, path := range found {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		relative[i] = filepath.ToSlash(rel)
	}
	return relative
}

func assertFound(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("Find = %q, want %q", got, want)
	}
}

// A YTS-style release: the only video in its folder, subtitles in Subs/.
func TestFindTakesEverySubtitleOfTheFoldersOnlyVideo(t *testing.T) {
	t.Parallel()
	root := layout(t, "Movie (2020).mkv", "Movie.en.srt", "extra.ass", "notes.txt", "cover.jpg",
		"Subs/2_English.srt", "Subs/3_Portuguese.srt", "Subs/readme.nfo")
	assertFound(t, find(t, root, "Movie (2020).mkv"),
		"Movie.en.srt", "extra.ass", "Subs/2_English.srt", "Subs/3_Portuguese.srt")
}

// With several videos, only files named after this one belong to it.
func TestFindTakesOnlyFilesNamedAfterTheVideo(t *testing.T) {
	t.Parallel()
	root := layout(t, "A.mkv", "B.mp4", "A.en.srt", "A.pt-BR.forced.srt", "B.en.srt", "A 2.srt", "AB.srt", "random.srt",
		"Subs/A.es.srt", "Subs/B.es.srt", "Subs/English.srt")
	assertFound(t, find(t, root, "A.mkv"), "A.en.srt", "A.pt-BR.forced.srt", "Subs/A.es.srt")
}

func TestFindIgnoresLetterCase(t *testing.T) {
	t.Parallel()
	root := layout(t, "Movie.mkv", "Other.mkv", "MOVIE.EN.SRT", "movie.Pt.VTT", "SUBTITLES/movie.es.Ass")
	assertFound(t, find(t, root, "Movie.mkv"), "MOVIE.EN.SRT", "movie.Pt.VTT", "SUBTITLES/movie.es.Ass")
}

// Series releases keep one folder per episode inside Subs.
func TestFindUsesTheEpisodeFolderInsideSubs(t *testing.T) {
	t.Parallel()
	root := layout(t, "S01E01.mkv", "S01E02.mkv", "Subs/S01E01/2_English.srt", "Subs/S01E01/3_Spanish.srt",
		"Subs/S01E02/2_English.srt", "Legendas/s01e01/Português.srt")
	assertFound(t, find(t, root, "S01E01.mkv"),
		"Legendas/s01e01/Português.srt", "Subs/S01E01/2_English.srt", "Subs/S01E01/3_Spanish.srt")
}

func TestFindSkipsFoldersNamedLikeSubtitlesAndOtherFolders(t *testing.T) {
	t.Parallel()
	root := layout(t, "Movie.mkv", "Movie.en.srt/", "Extras/Movie.en.srt", "Subs/Deep/Movie.en.srt")
	if got := find(t, root, "Movie.mkv"); len(got) != 0 {
		t.Fatalf("Find = %q, want nothing", got)
	}
}

func TestFindTakesAFolderNamedAfterTheVideo(t *testing.T) {
	t.Parallel()
	root := layout(t, "Movie.mkv", "Other.mkv", "movie/English.srt", "Movie/Deep/x.srt", "Other/English.srt")
	assertFound(t, find(t, root, "Movie.mkv"), "movie/English.srt")
}

func relativeTo(t *testing.T, root string, paths []string) []string {
	t.Helper()
	relative := make([]string, len(paths))
	for i, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		relative[i] = filepath.ToSlash(rel)
	}
	return relative
}

func candidates(t *testing.T, root, video string) []string {
	t.Helper()
	found, err := subtitlefile.Files{}.FindCandidates(filepath.Join(root, video))
	if err != nil {
		t.Fatalf("FindCandidates: %v", err)
	}
	return relativeTo(t, root, found)
}

// The real case: subtitles downloaded into folders named after the zip, in a
// folder with several films.
func TestFindCandidatesLooksOneLevelIntoAnySubfolder(t *testing.T) {
	t.Parallel()
	root := layout(t, "Exemplo.mkv", "End.mkv", "Exemplo.pt.srt", "End.en.srt", "notes.txt",
		"exemplo.de.filme.(1000001)/Example Movie.eng.srt", "exemplo.de.filme.(1000001)/info.nfo",
		"exemplo.de.filme.(1000002)/Example Movie DC.en.srt", "Subs/English.srt", "deep/er/x.srt", "empty/")
	assertFound(t, candidates(t, root, "Exemplo.mkv"),
		"End.en.srt", "Subs/English.srt",
		"exemplo.de.filme.(1000001)/Example Movie.eng.srt", "exemplo.de.filme.(1000002)/Example Movie DC.en.srt")
}

func TestFindCandidatesLeavesOutTheVideosOwnFiles(t *testing.T) {
	t.Parallel()
	root := layout(t, "Movie.mkv", "Movie.en.srt", "Subs/Movie.pt.srt", "Subs/other.srt", "MOVIE/x.srt")
	assertFound(t, candidates(t, root, "Movie.mkv"))
	root = layout(t, "Movie.mkv", "Other.mkv", "Movie.en.srt", "SUBS/movie.pt.srt", "Subs2/Movie.en.srt")
	assertFound(t, candidates(t, root, "Movie.mkv"), "Subs2/Movie.en.srt")
}

func TestFindCandidatesIsBounded(t *testing.T) {
	t.Parallel()
	var names []string
	for i := range 60 {
		names = append(names, "A.mkv", "B.mkv", filepath.ToSlash(filepath.Join("f"+string(rune('a'+i%26))+string(rune('a'+i/26)), "x.srt")))
	}
	for i := range 70 {
		names = append(names, "loose"+string(rune('a'+i%26))+string(rune('a'+i/26))+".srt")
	}
	root := layout(t, names...)
	// 70 loose files already pass the limit of 50.
	if got := candidates(t, root, "A.mkv"); len(got) != 50 {
		t.Fatalf("%d candidates, want the limit of 50", len(got))
	}
}

func TestFindReportsAMissingFolder(t *testing.T) {
	t.Parallel()
	if _, err := (subtitlefile.Files{}).Find(filepath.Join(t.TempDir(), "gone", "Movie.mkv")); err == nil {
		t.Fatal("Find in a missing folder succeeded")
	}
}

func write(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadReadsEachFormat(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"a.srt": srt,
		"a.vtt": "WEBVTT\n\n00:00:01.000 --> 00:00:02.500\nHello there.\n",
		"a.ass": "[Script Info]\nScriptType: v4.00+\n\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:01.00,0:00:02.50,Default,,0,0,0,,Hello there.\n",
	}
	for name, content := range cases {
		format, got, err := subtitlefile.Files{}.Load(write(t, name, []byte(content)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got.Events) == 0 || got.Events[0].Text() != "Hello there." {
			t.Fatalf("%s (%s): events = %+v", name, format, got.Events)
		}
	}
}

// Subtitles downloaded from the internet come in legacy encodings.
func TestLoadDecodesUTF16AndWindows1252(t *testing.T) {
	t.Parallel()
	text := "1\r\n00:00:01,000 --> 00:00:02,000\r\nNão, você está aqui.\r\n"
	units := utf16.Encode([]rune(text))
	utf16LE := []byte{0xFF, 0xFE}
	for _, u := range units {
		utf16LE = append(utf16LE, byte(u), byte(u>>8))
	}
	windows1252 := []byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nN\xe3o, voc\xea est\xe1 aqui.\r\n")
	for name, data := range map[string][]byte{"utf16.srt": utf16LE, "ansi.srt": windows1252} {
		_, got, err := subtitlefile.Files{}.Load(write(t, name, data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got.Events) != 1 || got.Events[0].Text() != "Não, você está aqui." {
			t.Fatalf("%s: events = %+v", name, got.Events)
		}
	}
}

func TestLoadRejectsWhatIsNotASubtitle(t *testing.T) {
	t.Parallel()
	folder := t.TempDir()
	big := filepath.Join(t.TempDir(), "huge.srt")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	// A sparse 40 MiB file: refused by size, without being read.
	if err := f.Truncate(40 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()

	cases := map[string]struct {
		path string
		want error
	}{
		"pasta":         {folder, subtitlefile.ErrUnsupported},
		"grande demais": {big, subtitlefile.ErrTooLarge},
		"inexistente":   {filepath.Join(folder, "gone.srt"), os.ErrNotExist},
		"extensão":      {write(t, "notes.txt", []byte("hello")), subtitlefile.ErrUnsupported},
	}
	for name, c := range cases {
		if _, _, err := (subtitlefile.Files{}).Load(c.path); !errors.Is(err, c.want) {
			t.Errorf("%s: Load = %v, want %v", name, err, c.want)
		}
	}
}

func TestLoadSurvivesGarbage(t *testing.T) {
	t.Parallel()
	garbage := make([]byte, 64<<10)
	for i := range garbage {
		garbage[i] = byte(i * 7919)
	}
	for _, name := range []string{"garbage.srt", "garbage.ass", "garbage.vtt", "empty.srt"} {
		data := garbage
		if name == "empty.srt" {
			data = nil
		}
		_, got, err := subtitlefile.Files{}.Load(write(t, name, data))
		if err == nil {
			for _, event := range got.Events {
				if !event.Valid() {
					t.Fatalf("%s: invalid event %+v", name, event)
				}
			}
			if _, err := subtitle.NewTimeline(got.Events); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
}
