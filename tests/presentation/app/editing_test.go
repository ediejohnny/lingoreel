package app_test

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lingoreel/internal/domain/project"
	"lingoreel/internal/presentation/app"
)

const movie = `D:\Filmes\Movie.mkv`

// The rig's video has "Hi." from 1 s to 2 s on both subtitles (tracks 4
// and 5); the project names tracks "track:<number>".

// onTheLine puts the paused video inside the line.
func onTheLine(t *testing.T, r *rig) {
	t.Helper()
	if err := r.api.SeekTo(1.5); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the position", func() bool { return math.Abs(r.api.PlaybackState().Position-1.5) < 0.01 })
}

// wantMessage fails unless err is the user's message holding part.
func wantMessage(t *testing.T, what string, err error, part string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), part) {
		t.Errorf("%s = %v, want a message with %q", what, err, part)
	}
}

func TestCorrectingALineFromThePanel(t *testing.T) {
	r := startWatching(t)
	onTheLine(t, r)
	wantMessage(t, "EditLine without a selection", r.api.EditLine(app.EditRequest{Text: "x", Start: "0:01", End: "0:02"}), "Escolha uma fala")
	wantMessage(t, "SelectLine sideways", r.api.SelectLine("sideways"), "")

	if err := r.api.SelectLine("primary"); err != nil {
		t.Fatal(err)
	}
	state := r.api.PlaybackState()
	editing := state.Editing
	if !editing.Active || editing.Line != "primary" || editing.LineName != state.Lines.Primary || editing.Original != "Hi." ||
		editing.Text != "Hi." || editing.Start != "0:01,000" || editing.End != "0:02,000" || editing.Edited || editing.CanUndo {
		t.Fatalf("editing = %+v", editing)
	}

	refused := []struct {
		request app.EditRequest
		message string
	}{
		{app.EditRequest{Text: "Hello.", Start: "1:99", End: "0:02"}, "1:02:03,456"},
		{app.EditRequest{Text: "Hello.", Start: "0:01", End: ""}, "1:02:03,456"},
		{app.EditRequest{Text: "Hello.", Start: "0:02", End: "0:01,999"}, "O fim precisa vir depois do início"},
		{app.EditRequest{Text: "  ", Start: "0:01", End: "0:02"}, "não pode ficar vazia"},
		{app.EditRequest{Text: strings.Repeat("é", project.MaxTextRunes+1), Start: "0:01", End: "0:02"}, "até 2000 letras"},
	}
	for _, c := range refused {
		wantMessage(t, "EditLine", r.api.EditLine(c.request), c.message)
	}
	if _, saves := r.projects.stored(t, movie); saves != 1 { // the choice, at Watch
		t.Fatalf("refused edits saved the project: %d saves", saves)
	}

	if err := r.api.EditLine(app.EditRequest{Text: "Hello.", Start: "0:01,000", End: "0:02,500"}); err != nil {
		t.Fatal(err)
	}
	editing = r.api.PlaybackState().Editing
	if editing.Text != "Hello." || editing.End != "0:02,500" || editing.Original != "Hi." || !editing.Edited || !editing.CanUndo {
		t.Fatalf("after the edit: %+v", editing)
	}
	// Saved at once, by the track and the line's ID; the start did not change.
	stored, _ := r.projects.stored(t, movie)
	edit := stored.Tracks["track:4"].Edits["1"]
	if edit.Text == nil || *edit.Text != "Hello." || edit.End == nil || *edit.End != 2500 || edit.Start != nil ||
		edit.Original != project.HashOf("Hi.") || len(stored.Tracks["track:5"].Edits) != 0 {
		t.Fatalf("stored = %+v", stored.Tracks)
	}

	// The longest line allowed is taken.
	if err := r.api.EditLine(app.EditRequest{Text: strings.Repeat("é", project.MaxTextRunes), Start: "0:01", End: "0:02,5"}); err != nil {
		t.Fatalf("a line of %d letters: %v", project.MaxTextRunes, err)
	}

	// Undo and redo are saved too.
	if err := r.api.UndoEdit(); err != nil {
		t.Fatal(err)
	}
	if err := r.api.UndoEdit(); err != nil {
		t.Fatal(err)
	}
	if stored, _ := r.projects.stored(t, movie); len(stored.Tracks["track:4"].Edits) != 0 || r.api.PlaybackState().Editing.Edited {
		t.Fatalf("after undoing everything the project keeps %+v", stored.Tracks["track:4"].Edits)
	}
	if err := r.api.RedoEdit(); err != nil {
		t.Fatal(err)
	}
	if editing := r.api.PlaybackState().Editing; editing.Text != "Hello." || !editing.CanRedo {
		t.Fatalf("after redo: %+v", editing)
	}
	if err := r.api.RestoreLine(); err != nil {
		t.Fatal(err)
	}
	if editing := r.api.PlaybackState().Editing; editing.Text != "Hi." || editing.End != "0:02,000" || editing.Edited {
		t.Fatalf("after restore: %+v", editing)
	}

	// Playing, the panel says to pause.
	if err := r.api.TogglePause(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "playing", func() bool { return r.api.PlaybackState().Status == "playing" })
	wantMessage(t, "EditLine while playing", r.api.EditLine(app.EditRequest{Text: "Oi.", Start: "0:01", End: "0:02"}), "Pause o vídeo")
	wantMessage(t, "UndoEdit while playing", r.api.UndoEdit(), "Pause o vídeo")

	if err := r.api.ClearSelection(); err != nil || r.api.PlaybackState().Editing.Active {
		t.Fatalf("ClearSelection = %v, editing %+v", err, r.api.PlaybackState().Editing)
	}
}

func TestExportWritesWhatTheUserSees(t *testing.T) {
	r := startWatching(t)
	onTheLine(t, r)
	if err := errors.Join(r.api.SelectLine("primary"), r.api.EditLine(app.EditRequest{Text: "Hello.", Start: "0:01", End: "0:02"})); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()

	// A name that is not a subtitle never writes over it: the video stays.
	r.ui.savePath = filepath.Join(dir, "Movie.mkv")
	if err := r.api.ExportSubtitle("primary"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Movie.mkv")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the export wrote the .mkv name: %v", err)
	}
	srt, err := os.ReadFile(filepath.Join(dir, "Movie.mkv.srt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(srt, []byte("\uFEFF1\r\n00:00:01,000 --> 00:00:02,000\r\nHello.\r\n")) {
		t.Fatalf("exported SubRip = %q", srt)
	}
	if got := r.ui.suggested; len(got) != 1 || got[0] != "Movie (corrigida).srt" {
		t.Fatalf("the dialog suggested %q", got)
	}

	// .ass in any case writes Advanced SubStation; the bottom line is its own.
	r.ui.savePath = filepath.Join(dir, "Filme.ASS")
	if err := r.api.ExportSubtitle("translation"); err != nil {
		t.Fatal(err)
	}
	ass, err := os.ReadFile(r.ui.savePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(ass, []byte("Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hi.")) {
		t.Fatalf("exported ASS = %q", ass)
	}

	// Cancelled, nothing is written; a folder that is not there is reported.
	r.ui.savePath = ""
	if err := r.api.ExportSubtitle("primary"); err != nil {
		t.Fatalf("cancelled export = %v", err)
	}
	r.ui.savePath = filepath.Join(dir, "missing", "x.srt")
	wantMessage(t, "export into a missing folder", r.api.ExportSubtitle("primary"), "Não foi possível gravar")
	wantMessage(t, "export of a line sideways", r.api.ExportSubtitle("sideways"), "")
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Fatalf("the folder has %d files, want the two exports", len(entries))
	}
}

func TestTheProjectComesBackWhenTheVideoReopens(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if _, err := r.api.OpenVideo(movie); err != nil {
		t.Fatal(err)
	}
	// Not what the app suggests: Portuguese audio, the subtitles swapped, and
	// the translation always shown (the profile's mode is on demand).
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 2, Primary: 5, Translation: 4, Mode: "always"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	onTheLine(t, r)
	if err := errors.Join(r.api.SelectLine("primary"), r.api.EditLine(app.EditRequest{Text: "Olá.", Start: "0:01", End: "0:02"})); err != nil {
		t.Fatal(err)
	}
	wantMessage(t, "SavePassage without a passage", r.api.SavePassage("Loja"), "Comece a estudar")
	if err := r.api.StudyPassage(app.PassageRequest{Start: "0:10", End: "0:20"}); err != nil {
		t.Fatal(err)
	}
	// Without a name, the passage is named by its times, as the end key does.
	if err := r.api.SavePassage("   "); err != nil {
		t.Fatal(err)
	}
	if saved := r.api.PlaybackState().SavedPassages; len(saved) != 1 || saved[0].Name != "Trecho 0:10–0:20" || saved[0].Summary != "Pausa no fim" {
		t.Fatalf("saved without a name = %+v", saved)
	}
	wantMessage(t, "SavePassage with a long name", r.api.SavePassage(strings.Repeat("a", project.MaxPassageNameLen+1)), "Dê um nome")
	if err := errors.Join(r.api.DeletePassage("Trecho 0:10–0:20"), r.api.SavePassage("Rua")); err != nil {
		t.Fatal(err)
	}
	// The same name, in any case, replaces the passage, with its plan.
	if err := errors.Join(r.api.StudyPassage(app.PassageRequest{Start: "0:30", End: "0:40", Loop: true, Gap: 1.5, Goal: 3}), r.api.SavePassage("RUA ")); err != nil {
		t.Fatal(err)
	}
	saved := r.api.PlaybackState().SavedPassages
	if len(saved) != 1 || saved[0].Name != "RUA" || saved[0].Start != "0:30" || saved[0].End != "0:40" ||
		saved[0].Plan != (app.PassagePlan{Loop: true, Gap: 1.5, Goal: 3}) || saved[0].Summary != "Repete sem parar · intervalo de 1,5 s · meta de 3 voltas" {
		t.Fatalf("saved passages = %+v, want Rua replaced", saved)
	}
	if err := errors.Join(r.api.DeletePassage("rua"), r.api.StudyPassage(app.PassageRequest{Start: "0:10", End: "0:20"}), r.api.SavePassage("Loja")); err != nil {
		t.Fatal(err)
	}

	// Offsets wait a moment after the last step (or the end of the session),
	// not every step of a held key.
	if err := r.api.Nudge("translation", true); err != nil {
		t.Fatal(err)
	}
	offset := r.api.PlaybackState().TranslationOffset
	if stored, _ := r.projects.stored(t, movie); stored.Tracks["track:4"].Offset != 0 {
		t.Fatal("the offset was saved at every step")
	}
	r.api.StopWatching()
	eventually(t, "the end", func() bool { return !r.api.PlaybackState().Active })

	stored, _ := r.projects.stored(t, movie)
	if stored.Tracks["track:4"].Offset <= 0 || stored.Choice != (project.Choice{Audio: "track:2", Primary: "track:5", Translation: "track:4", Mode: "always"}) {
		t.Fatalf("stored = %+v", stored)
	}

	video, err := r.api.OpenVideo(movie)
	if err != nil {
		t.Fatal(err)
	}
	if video.Suggested != (app.Selection{Audio: 2, Primary: 5, Translation: 4}) || video.Mode != "always" || video.ProjectNote != "" {
		t.Fatalf("reopened: suggested %+v, mode %q, note %q", video.Suggested, video.Mode, video.ProjectNote)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 2, Primary: 5, Translation: 4, Mode: video.Mode}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	state := r.api.PlaybackState()
	if state.TranslationOffset != offset || len(state.SavedPassages) != 1 || state.SavedPassages[0].Name != "Loja" ||
		state.SavedPassages[0].Start != "0:10" || state.SavedPassages[0].End != "0:20" {
		t.Fatalf("reopened: offset %q (want %q), passages %+v", state.TranslationOffset, offset, state.SavedPassages)
	}
	onTheLine(t, r)
	if err := r.api.SelectLine("primary"); err != nil {
		t.Fatal(err)
	}
	if editing := r.api.PlaybackState().Editing; editing.Text != "Olá." || editing.Original != "Hi." || !editing.Edited {
		t.Fatalf("the saved correction did not come back: %+v", editing)
	}

	// Names are matched without case; an unknown one is reported.
	wantMessage(t, "DeletePassage of an unknown name", r.api.DeletePassage("Rua"), "não encontrado")
	if err := r.api.DeletePassage("loja"); err != nil || len(r.api.PlaybackState().SavedPassages) != 0 {
		t.Fatalf("DeletePassage = %v, left %+v", err, r.api.PlaybackState().SavedPassages)
	}
	if stored, _ := r.projects.stored(t, movie); len(stored.Passages) != 0 {
		t.Fatalf("the deletion was not saved: %+v", stored.Passages)
	}
}

func TestARememberedChoiceMustStillFit(t *testing.T) {
	r := newRig(t)
	// Track 6 is a picture subtitle the app cannot read; 9 is not in the video.
	remembered := project.New()
	remembered.Choice = project.Choice{Audio: "track:9", Primary: "track:6", Translation: "track:3", Mode: "passive"}
	if err := r.projects.Save(movie, remembered); err != nil {
		t.Fatal(err)
	}
	r.attach(t)
	fresh := newRig(t)
	fresh.attach(t)
	want, err := fresh.api.OpenVideo(movie)
	if err != nil {
		t.Fatal(err)
	}
	video, err := r.api.OpenVideo(movie)
	if err != nil {
		t.Fatal(err)
	}
	if video.Suggested != want.Suggested || video.Mode != want.Mode {
		t.Fatalf("suggested %+v in %q, want the app's own %+v in %q", video.Suggested, video.Mode, want.Suggested, want.Mode)
	}
}

func TestAProjectTheAppCannotReadIsNeverWritten(t *testing.T) {
	for _, c := range []struct {
		name    string
		loadErr error
		note    string
	}{
		{"newer version", project.ErrNewerVersion, "versão mais nova"},
		{"damaged", errors.New("broken JSON"), "não pôde ser lido"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			r.projects.loadErr = c.loadErr
			r.attach(t)
			video, err := r.api.OpenVideo(movie)
			if err != nil || !strings.Contains(video.ProjectNote, c.note) {
				t.Fatalf("OpenVideo = %q, %v", video.ProjectNote, err)
			}
			if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { r.api.StopWatching() })
			eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
			onTheLine(t, r)
			// The correction still works for this session, in memory only.
			if err := errors.Join(r.api.SelectLine("primary"), r.api.EditLine(app.EditRequest{Text: "Hello.", Start: "0:01", End: "0:02"})); err != nil {
				t.Fatal(err)
			}
			state := r.api.PlaybackState()
			if !state.Editing.Edited || !strings.Contains(state.ProjectNote, c.note) {
				t.Fatalf("editing %+v, note %q", state.Editing, state.ProjectNote)
			}
			if _, saves := r.projects.stored(t, movie); saves != 0 {
				t.Fatalf("the unreadable project was written %d times", saves)
			}
		})
	}
}

func TestAFailedSaveIsShown(t *testing.T) {
	r := newRig(t)
	r.projects.saveErr = errors.New("disk full")
	r.attach(t)
	if _, err := r.api.OpenVideo(movie); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	if note := r.api.PlaybackState().ProjectNote; !strings.Contains(note, "Não foi possível gravar") {
		t.Fatalf("note = %q", note)
	}
}

func TestEditingNeedsASession(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	for name, call := range map[string]func() error{
		"SelectLine":     func() error { return r.api.SelectLine("primary") },
		"EditLine":       func() error { return r.api.EditLine(app.EditRequest{Text: "x", Start: "0:01", End: "0:02"}) },
		"UndoEdit":       r.api.UndoEdit,
		"ExportSubtitle": func() error { return r.api.ExportSubtitle("primary") },
		"SavePassage":    func() error { return r.api.SavePassage("Loja") },
		"DeletePassage":  func() error { return r.api.DeletePassage("Loja") },
	} {
		if err := call(); err == nil {
			t.Errorf("%s without a session", name)
		}
	}
	if _, saves := r.projects.stored(t, movie); saves != 0 {
		t.Fatalf("%d saves without a session", saves)
	}
}
