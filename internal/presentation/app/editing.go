package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/project"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/infrastructure/subtitleformat"
	"lingoreel/internal/presentation/ptbr"
)

// Messages of the editor.
var (
	errEditPlaying  = errors.New("Pause o vídeo para corrigir uma legenda.")
	errNoLineNow    = errors.New("Não há fala nesta legenda agora.")
	errNotSelected  = errors.New("Escolha uma fala antes: clique nela no vídeo pausado.")
	errEmptyLine    = errors.New("A fala não pode ficar vazia.")
	errLongLine     = errors.New("A fala pode ter até 2000 letras.")
	errLineTimes    = errors.New("O fim precisa vir depois do início, e o início não pode ser antes do começo do vídeo.")
	errEditClock    = errors.New("Escreva os tempos como 1:02:03,456 ou 2:03,456.")
	errNothingToSay = errors.New("Esta legenda não tem falas para exportar.")
	errExport       = errors.New("Não foi possível gravar a legenda exportada.")
)

// edit runs an editing command and reports the new state, wording the
// session's refusals for the user.
func (a *API) edit(command func(*watching) error) error {
	current := a.current()
	if current == nil || current.session == nil {
		return errNotWatching
	}
	err := command(current)
	switch {
	case err == nil:
		a.emit(EventPlayback, a.describe(current))
		return nil
	case errors.Is(err, watch.ErrPlaying):
		return errEditPlaying
	case errors.Is(err, watch.ErrNoLineHere):
		return errNoLineNow
	case errors.Is(err, watch.ErrNothingSelected):
		return errNotSelected
	case errors.Is(err, watch.ErrEmptyText):
		return errEmptyLine
	case errors.Is(err, watch.ErrBadTimes):
		return errLineTimes
	}
	var refused settingsError
	if errors.As(err, &refused) {
		return err
	}
	a.logger.Warn("editing failed", "error", err)
	return errors.New("Não foi possível corrigir a fala.")
}

// SelectLine picks the line on screen ("primary" or "translation") for
// editing, as a click on it in the paused video does.
func (a *API) SelectLine(line string) error {
	which, err := lineOf(line)
	if err != nil {
		return err
	}
	return a.edit(func(w *watching) error {
		_, err := w.session.Select(which)
		return err
	})
}

// ClearSelection leaves the editing.
func (a *API) ClearSelection() error {
	return a.edit(func(w *watching) error {
		w.session.ClearSelection()
		return nil
	})
}

// EditLine saves the editor's text and times on the selected line. Times
// are typed on the playback clock ("1:02:03,456").
func (a *API) EditLine(request EditRequest) error {
	if utf8.RuneCountInString(request.Text) > project.MaxTextRunes {
		return errLongLine
	}
	start, err := study.ParseClock(request.Start)
	if err != nil {
		return errEditClock
	}
	end, err := study.ParseClock(request.End)
	if err != nil {
		return errEditClock
	}
	text := request.Text
	return a.edit(func(w *watching) error {
		return w.session.Edit(watch.EditChange{Text: &text, Start: &start, End: &end})
	})
}

// RestoreLine puts the selected line back as its file has it.
func (a *API) RestoreLine() error {
	return a.edit(func(w *watching) error { return w.session.RestoreOriginal() })
}

// UndoEdit takes back the latest correction.
func (a *API) UndoEdit() error {
	return a.edit(func(w *watching) error {
		_, err := w.session.Undo()
		return err
	})
}

// RedoEdit makes the latest undone correction again.
func (a *API) RedoEdit() error {
	return a.edit(func(w *watching) error {
		_, err := w.session.Redo()
		return err
	})
}

// ExportSubtitle writes a line's subtitle ("primary" or "translation") as
// the user sees it, corrections and offset in, to a file the user picks:
// .ass for Advanced SubStation, otherwise SubRip. The video is never
// written: any other extension gets ".srt" added.
func (a *API) ExportSubtitle(line string) error {
	which, err := lineOf(line)
	if err != nil {
		return err
	}
	current := a.current()
	if current == nil || current.session == nil {
		return errNotWatching
	}
	events := current.session.Export(which)
	if len(events) == 0 {
		return errNothingToSay
	}
	a.mu.Lock()
	ui := a.ui
	a.mu.Unlock()
	if ui == nil {
		return ErrNotReady
	}
	suggested := strings.TrimSuffix(filepath.Base(current.video), filepath.Ext(current.video)) + " (corrigida).srt"
	path, err := ui.SaveSubtitleDialog(suggested)
	if err != nil {
		a.logger.Warn("save dialog failed", "error", err)
		return errors.New("Não foi possível abrir a janela para salvar o arquivo.")
	}
	if path == "" {
		return nil // cancelled
	}
	var body bytes.Buffer
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ass":
		err = subtitleformat.WriteASS(&body, events)
	case ".srt":
		err = subtitleformat.WriteSRT(&body, events)
	default:
		path += ".srt"
		err = subtitleformat.WriteSRT(&body, events)
	}
	if err == nil {
		err = os.WriteFile(path, body.Bytes(), 0o644)
	}
	if err != nil {
		a.logger.Warn("export failed", "path", path, "error", err)
		return errExport
	}
	return nil
}

// describeEditing is the editor's part of the panel's state.
func describeEditing(current *watching) Editing {
	s := current.session
	editing := Editing{CanUndo: s.CanUndo(), CanRedo: s.CanRedo(), StaleEdits: s.StaleEdits()}
	selection, active := s.Selection()
	if !active {
		return editing
	}
	editing.Active = true
	editing.Line = lineName(selection.Line)
	editing.LineName = current.names[selection.Line]
	editing.Original = selection.Original.OriginalText
	editing.Text = selection.Event.Text()
	editing.Start = ptbr.EditClock(selection.Offset.PlaybackTime(selection.Event.Start))
	editing.End = ptbr.EditClock(selection.Offset.PlaybackTime(selection.Event.End))
	editing.OriginalStart = ptbr.EditClock(selection.Offset.PlaybackTime(selection.Original.Start))
	editing.OriginalEnd = ptbr.EditClock(selection.Offset.PlaybackTime(selection.Original.End))
	editing.Edited = selection.Edited()
	return editing
}
