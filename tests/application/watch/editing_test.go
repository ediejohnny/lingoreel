package watch_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The rig's lines: on top "Hello there." (e1), below "Olá." (p1), both
// from 1 s to 3 s.

// playbackAt is the player telling a paused position.
func playbackAt(position time.Duration) playback.Sample {
	return playback.Sample{Position: position, Duration: time.Minute, Status: playback.StatusPaused, Speed: 1, ObservedAt: time.Now()}
}

// screenShows waits until the display's latest screen passes check.
func screenShows(t *testing.T, r *rig, what string, check func(session.Screen) bool) {
	t.Helper()
	testkit.Eventually(t, what, func() bool {
		screen, shown, _, _ := r.display.last()
		return shown > 0 && check(screen)
	})
}

// shownLine is the event a line shows now, as a selection finds it.
func shownLine(t *testing.T, s *watch.Session) subtitle.Event {
	t.Helper()
	selection, found := s.Selection()
	if !found {
		t.Fatal("nothing selected")
	}
	return selection.Event
}

func TestEditingAPausedLine(t *testing.T) {
	t.Parallel()
	r, s := playingRig(t)
	at(t, r, s, 1500*time.Millisecond)
	screenShows(t, r, "the line", func(screen session.Screen) bool { return screen.Primary == "Hello there." })

	selection, err := s.Select(session.LinePrimary)
	if err != nil || selection.Event.ID != "e1" || selection.Edited() {
		t.Fatalf("Select = %+v, %v", selection, err)
	}
	screenShows(t, r, "the outline", func(screen session.Screen) bool {
		return screen.Highlighted && screen.Highlight == session.LinePrimary
	})

	// Writing what the line already says changes nothing, not even the history.
	if err := s.Edit(watch.EditChange{Text: new("Hello there.")}); err != nil {
		t.Fatal(err)
	}
	if s.CanUndo() || len(r.listener.edits()) != 0 {
		t.Fatalf("a same-text edit was recorded: can undo %v, listener %+v", s.CanUndo(), r.listener.edits())
	}

	// The text changes on screen at once; the times and the file's text stay.
	if err := s.Edit(watch.EditChange{Text: new("Hi there.")}); err != nil {
		t.Fatal(err)
	}
	screenShows(t, r, "the new text", func(screen session.Screen) bool { return screen.Primary == "Hi there." })
	event := shownLine(t, s)
	if event.OriginalText != "Hello there." || event.Start != time.Second || event.End != 3*time.Second {
		t.Fatalf("after the text edit the line is %+v", event)
	}
	edit, kept := s.Edits(session.LinePrimary)["e1"]
	if !kept || edit.Original != project.HashOf("Hello there.") || edit.Text == nil || *edit.Text != "Hi there." ||
		edit.Start != nil || edit.End != nil {
		t.Fatalf("kept edit = %+v (%v)", edit, kept)
	}
	if last := r.listener.edits(); len(last) != 1 || last[0].line != session.LinePrimary || len(last[0].edits) != 1 {
		t.Fatalf("the listener heard %+v", last)
	}

	// Times alone keep the new text.
	if err := s.Edit(watch.EditChange{Start: new(1200 * time.Millisecond), End: new(2500 * time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	if event := shownLine(t, s); event.Text() != "Hi there." || event.Start != 1200*time.Millisecond || event.End != 2500*time.Millisecond {
		t.Fatalf("after the times edit the line is %+v", event)
	}

	// Undo takes back one change at a time, then redo makes it again.
	if undone, err := s.Undo(); !undone || err != nil {
		t.Fatalf("Undo = %v, %v", undone, err)
	}
	if event := shownLine(t, s); event.Text() != "Hi there." || event.Start != time.Second {
		t.Fatalf("the first undo left %+v", event)
	}
	if undone, err := s.Undo(); !undone || err != nil {
		t.Fatalf("Undo = %v, %v", undone, err)
	}
	if event := shownLine(t, s); event.Text() != "Hello there." || len(s.Edits(session.LinePrimary)) != 0 || s.CanUndo() {
		t.Fatalf("after undoing everything: %+v, edits %v, can undo %v", event, s.Edits(session.LinePrimary), s.CanUndo())
	}
	if undone, err := s.Undo(); undone || err != nil {
		t.Fatalf("Undo with nothing left = %v, %v", undone, err)
	}
	if redone, err := s.Redo(); !redone || err != nil || !s.CanRedo() {
		t.Fatalf("Redo = %v, %v, can redo %v", redone, err, s.CanRedo())
	}
	screenShows(t, r, "the redone text", func(screen session.Screen) bool { return screen.Primary == "Hi there." })
	if last := r.listener.edits(); len(last[len(last)-1].edits) != 1 {
		t.Fatalf("the listener's last edits = %+v", last[len(last)-1])
	}

	// Writing the file's text again is no edit; restoring drops the times too.
	if err := s.Edit(watch.EditChange{Text: new("Hello there.")}); err != nil {
		t.Fatal(err)
	}
	if edit := s.Edits(session.LinePrimary)["e1"]; edit.Text != nil {
		t.Fatalf("the file's own text was kept as an edit: %+v", edit)
	}
	if err := s.Edit(watch.EditChange{End: new(2 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreOriginal(); err != nil {
		t.Fatal(err)
	}
	if event := shownLine(t, s); event != selection.Original || len(s.Edits(session.LinePrimary)) != 0 {
		t.Fatalf("restored line %+v, edits %v", event, s.Edits(session.LinePrimary))
	}

	s.ClearSelection()
	if _, found := s.Selection(); found {
		t.Fatal("the selection stayed")
	}
	screenShows(t, r, "no outline", func(screen session.Screen) bool { return !screen.Highlighted })
}

func TestEditsThatAreRefused(t *testing.T) {
	t.Parallel()
	r, s := playingRig(t)
	at(t, r, s, 1500*time.Millisecond)
	if err := s.Edit(watch.EditChange{Text: new("x")}); !errors.Is(err, watch.ErrNothingSelected) {
		t.Fatalf("edit without a selection: %v", err)
	}
	if err := s.RestoreOriginal(); !errors.Is(err, watch.ErrNothingSelected) {
		t.Fatalf("restore without a selection: %v", err)
	}
	if _, err := s.Select(session.LinePrimary); err != nil {
		t.Fatal(err)
	}
	before := shownLine(t, s)
	refused := []struct {
		name   string
		change watch.EditChange
		want   error
	}{
		{"blank text", watch.EditChange{Text: new(" \n\t")}, watch.ErrEmptyText},
		{"empty text", watch.EditChange{Text: new("")}, watch.ErrEmptyText},
		{"end at the start", watch.EditChange{Start: new(2 * time.Second), End: new(2 * time.Second)}, watch.ErrBadTimes},
		{"end before the start", watch.EditChange{End: new(500 * time.Millisecond)}, watch.ErrBadTimes},
		{"start before the video", watch.EditChange{Start: new(-time.Millisecond)}, watch.ErrBadTimes},
	}
	for _, c := range refused {
		if err := s.Edit(c.change); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	if after := shownLine(t, s); after != before || s.CanUndo() || len(r.listener.edits()) != 0 {
		t.Fatalf("a refused edit changed the line: %+v, can undo %v", after, s.CanUndo())
	}

	// Windows line breaks are kept as plain ones.
	if err := s.Edit(watch.EditChange{Text: new("Hi\r\nthere.")}); err != nil {
		t.Fatal(err)
	}
	if got := shownLine(t, s).Text(); got != "Hi\nthere." {
		t.Fatalf("text = %q", got)
	}

	// Playing, nothing is edited.
	if err := s.Controls().Play(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Select(session.LineTranslation); !errors.Is(err, watch.ErrPlaying) {
		t.Errorf("select while playing: %v", err)
	}
	if err := s.Edit(watch.EditChange{Text: new("Other.")}); !errors.Is(err, watch.ErrPlaying) {
		t.Errorf("edit while playing: %v", err)
	}
	if _, err := s.Undo(); !errors.Is(err, watch.ErrPlaying) {
		t.Errorf("undo while playing: %v", err)
	}
	if _, err := s.Redo(); !errors.Is(err, watch.ErrPlaying) {
		t.Errorf("redo while playing: %v", err)
	}
	if err := s.RestoreOriginal(); !errors.Is(err, watch.ErrPlaying) {
		t.Errorf("restore while playing: %v", err)
	}
	// Playing lets the selection go on the runner's loop, so the line is
	// read on the screen.
	screenShows(t, r, "the line unchanged while playing", func(screen session.Screen) bool { return screen.Primary == "Hi\nthere." })

	// Where no line shows, there is nothing to pick.
	at(t, r, s, 5*time.Second)
	if _, err := s.Select(session.LinePrimary); !errors.Is(err, watch.ErrNoLineHere) {
		t.Fatalf("select between lines: %v", err)
	}
}

func TestSavedEditsComeBack(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.request.Saved = map[session.Line]project.Track{
		session.LinePrimary: {Edits: map[string]project.Edit{
			"e1": {Original: project.HashOf("Hello there."), Text: new("Fixed."), End: new(project.Millis(2500))},
		}},
		// One edit was made on another text, one on a line that is gone.
		session.LineTranslation: {Edits: map[string]project.Edit{
			"p1":   {Original: project.HashOf("Oi."), Text: new("Errado.")},
			"gone": {Original: project.HashOf("Tchau."), Text: new("Até.")},
		}},
	}
	s := r.start(t)
	r.running.samples <- playbackAt(1500 * time.Millisecond)
	screenShows(t, r, "the saved edit", func(screen session.Screen) bool {
		return screen.Primary == "Fixed." && screen.Translation == "Olá."
	})
	if s.StaleEdits() != 2 {
		t.Fatalf("stale edits = %d, want 2", s.StaleEdits())
	}
	if len(s.Edits(session.LinePrimary)) != 1 || len(s.Edits(session.LineTranslation)) != 0 {
		t.Fatalf("edits: top %v, bottom %v", s.Edits(session.LinePrimary), s.Edits(session.LineTranslation))
	}

	// The saved edit is a line like any other: the original is still there.
	selection, err := s.Select(session.LinePrimary)
	if err != nil || selection.Original.Text() != "Hello there." || selection.Event.End != 2500*time.Millisecond || !selection.Edited() {
		t.Fatalf("Select = %+v, %v", selection, err)
	}
	if err := s.RestoreOriginal(); err != nil {
		t.Fatal(err)
	}
	screenShows(t, r, "the file's text", func(screen session.Screen) bool { return screen.Primary == "Hello there." })
	last := r.listener.edits()
	if len(last) != 1 || last[0].line != session.LinePrimary || len(last[0].edits) != 0 {
		t.Fatalf("the listener heard %+v, want the top line without edits", last)
	}
}

func TestEditTimesAndExportFollowTheOffset(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	// The top line plays 0.5 s late; the bottom one 1.5 s early.
	r.request.Saved = map[session.Line]project.Track{
		session.LinePrimary:     {Offset: 500},
		session.LineTranslation: {Offset: -1500},
	}
	s := r.start(t)
	r.running.samples <- playbackAt(2 * time.Second)
	testkit.Eventually(t, "the position", func() bool { return s.State().Position >= 2*time.Second })

	selection, err := s.Select(session.LinePrimary)
	if err != nil || selection.Offset.PlaybackTime(selection.Event.Start) != 1500*time.Millisecond {
		t.Fatalf("Select = %+v, %v", selection, err)
	}
	// Times are typed as the user sees them, and kept on the file's clock.
	if err := s.Edit(watch.EditChange{Start: new(2 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	if edit := s.Edits(session.LinePrimary)["e1"]; edit.Start == nil || *edit.Start != 1500 {
		t.Fatalf("kept edit = %+v, want the start at 1500 ms of the file", edit)
	}

	top := s.Export(session.LinePrimary)
	if len(top) != 1 || top[0].Start != 2*time.Second || top[0].End != 3500*time.Millisecond {
		t.Fatalf("exported top = %+v, want 2 s to 3.5 s", top)
	}
	// What would start before the video starts at zero.
	bottom := s.Export(session.LineTranslation)
	if len(bottom) != 1 || bottom[0].Start != 0 || bottom[0].End != 1500*time.Millisecond || bottom[0].Text() != "Olá." {
		t.Fatalf("exported bottom = %+v, want 0 s to 1.5 s", bottom)
	}

	// A line that would end before the video is left out.
	early := newRig(t)
	early.request.Saved = map[session.Line]project.Track{session.LinePrimary: {Offset: -3000}}
	if got := early.start(t).Export(session.LinePrimary); len(got) != 0 {
		t.Fatalf("exported %+v from before the video", got)
	}
}

func TestTheOutlineFollowsTheSelectedLine(t *testing.T) {
	t.Parallel()
	r, s := playingRig(t)
	at(t, r, s, 1500*time.Millisecond)
	if _, err := s.Select(session.LineTranslation); err != nil {
		t.Fatal(err)
	}
	screenShows(t, r, "the outline below", func(screen session.Screen) bool {
		return screen.Highlighted && screen.Highlight == session.LineTranslation
	})
	// A hidden translation shows no outline, and neither does a line gone by.
	s.SetTranslationVisible(false)
	screenShows(t, r, "no outline on the hidden line", func(screen session.Screen) bool { return !screen.Highlighted })
	s.SetTranslationVisible(true)
	screenShows(t, r, "the outline again", func(screen session.Screen) bool { return screen.Highlighted })
	at(t, r, s, 5*time.Second)
	screenShows(t, r, "no outline between lines", func(screen session.Screen) bool { return !screen.Highlighted })
	if _, found := s.Selection(); !found {
		t.Fatal("moving on lost the selection, and the draft with it")
	}
}

// Every edit reaches the screen: the redraw that follows a change never
// paints the screen from before it over the new one, however the runner's
// own drawing of the change interleaves with it.
func TestEveryEditReachesTheScreen(t *testing.T) {
	t.Parallel()
	r, s := playingRig(t)
	at(t, r, s, 1500*time.Millisecond)
	screenShows(t, r, "the line", func(screen session.Screen) bool { return screen.Primary == "Hello there." })
	if _, err := s.Select(session.LinePrimary); err != nil {
		t.Fatal(err)
	}
	// The race it guards against shows only now and then: the full check
	// tries many times, the quick one a few.
	tries := 300
	if testing.Short() {
		tries = 30
	}
	for i := range tries {
		text := fmt.Sprintf("Edit number %d.", i)
		if err := s.Edit(watch.EditChange{Text: &text}); err != nil {
			t.Fatal(err)
		}
		screenShows(t, r, text, func(screen session.Screen) bool { return screen.Primary == text })
	}
}

// Playing again means the line was not going to be corrected: the outline
// goes at once, and does not come back when the video stops on that line
// again.
func TestPlayingLeavesTheLineBeingEdited(t *testing.T) {
	t.Parallel()
	r, s := playingRig(t)
	at(t, r, s, 1500*time.Millisecond)
	screenShows(t, r, "the line", func(screen session.Screen) bool { return screen.Primary == "Hello there." })
	if _, err := s.Select(session.LinePrimary); err != nil {
		t.Fatal(err)
	}
	screenShows(t, r, "the outline", func(screen session.Screen) bool { return screen.Highlighted })

	if err := s.Controls().Play(); err != nil {
		t.Fatal(err)
	}
	screenShows(t, r, "the line without its outline", func(screen session.Screen) bool {
		return screen.Primary == "Hello there." && !screen.Highlighted
	})
	if _, active := s.Selection(); active {
		t.Fatal("the line is still being edited while the video plays")
	}
	at(t, r, s, 1500*time.Millisecond)
	time.Sleep(50 * time.Millisecond) // a selection kept would outline the line again
	screenShows(t, r, "the line, paused, without its outline", func(screen session.Screen) bool {
		return screen.Primary == "Hello there." && !screen.Highlighted
	})
}
