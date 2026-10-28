package app

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// Messages of the editor.
var (
	errEditPlaying  = worded(func() string { return ptbr.T("error.edit.playing") })
	errNoLineNow    = worded(func() string { return ptbr.T("error.edit.noLine") })
	errNotSelected  = worded(func() string { return ptbr.T("error.edit.notSelected") })
	errEmptyLine    = worded(func() string { return ptbr.T("error.edit.empty") })
	errLongLine     = worded(func() string { return ptbr.T("error.edit.tooLong") })
	errLineTimes    = worded(func() string { return ptbr.T("error.edit.times") })
	errEditClock    = worded(func() string { return ptbr.T("error.edit.clock") })
	errNothingToSay = worded(func() string { return ptbr.T("error.export.empty") })
	errExport       = worded(func() string { return ptbr.T("error.export.failed") })
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
	return errors.New(ptbr.T("error.edit.failed"))
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
// the user sees it, corrections and offset in, to a file the user picks, in
// the format its name asks for (Deps.WriteSubtitle).
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
	ui := a.window()
	if ui == nil {
		return ErrNotReady
	}
	suggested := ptbr.T("export.fileName", "video", strings.TrimSuffix(filepath.Base(current.video), filepath.Ext(current.video)))
	path, err := ui.SaveSubtitleDialog(suggested)
	if err != nil {
		a.logger.Warn("save dialog failed", "error", err)
		return errors.New(ptbr.T("error.export.dialog"))
	}
	if path == "" {
		return nil // cancelled
	}
	if a.deps.WriteSubtitle == nil {
		return errExport
	}
	if written, err := a.deps.WriteSubtitle(path, events); err != nil {
		a.logger.Warn("export failed", "path", written, "error", err)
		return a.because(errExport, err)
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
	editing.LineName = current.lineNames()[selection.Line]
	editing.Original = selection.Original.OriginalText
	editing.Text = selection.Event.Text()
	editing.Start = ptbr.EditClock(selection.Offset.PlaybackTime(selection.Event.Start))
	editing.End = ptbr.EditClock(selection.Offset.PlaybackTime(selection.Event.End))
	editing.OriginalStart = ptbr.EditClock(selection.Offset.PlaybackTime(selection.Original.Start))
	editing.OriginalEnd = ptbr.EditClock(selection.Offset.PlaybackTime(selection.Original.End))
	editing.Edited = selection.Edited()
	return editing
}
