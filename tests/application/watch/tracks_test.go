package watch_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// loadedFile is a subtitle file of one line, from 1 s to 3 s, numbered as
// every file is: from the start.
func loadedFile(t *testing.T, text string) analysis.LoadedTrack {
	t.Helper()
	timeline, err := subtitle.NewTimeline([]subtitle.Event{{ID: "e1", Start: time.Second, End: 3 * time.Second, OriginalText: text}})
	if err != nil {
		t.Fatal(err)
	}
	return analysis.LoadedTrack{Timeline: timeline}
}

// A line takes another file while the video is paused on it: the screen
// shows the new file, the selection on that line goes, and undo no longer
// reaches the old file's changes, while the other line keeps its own.
func TestALineTakesAnotherSubtitle(t *testing.T) {
	t.Parallel()
	r, s := playingRig(t)
	at(t, r, s, 1500*time.Millisecond)
	screenShows(t, r, "both lines", func(screen session.Screen) bool { return screen.Primary == "Hello there." })
	for _, edit := range []struct {
		line session.Line
		text string
	}{{session.LineTranslation, "Oi."}, {session.LinePrimary, "Hi there."}} {
		if _, err := s.Select(edit.line); err != nil {
			t.Fatal(err)
		}
		if err := s.Edit(watch.EditChange{Text: new(edit.text)}); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.SetLine(session.LinePrimary, 99, loadedFile(t, "Another file."), project.Track{}); err != nil {
		t.Fatal(err)
	}
	screenShows(t, r, "the new file on top", func(screen session.Screen) bool { return screen.Primary == "Another file." })
	if _, found := s.Selection(); found {
		t.Fatal("the selection on the switched line stayed")
	}
	if undone, err := s.Undo(); !undone || err != nil {
		t.Fatalf("Undo = %v, %v; the other line's change should be there", undone, err)
	}
	screenShows(t, r, "the translation undone, the new file kept", func(screen session.Screen) bool {
		return screen.Translation == "Olá." && screen.Primary == "Another file."
	})
	if s.CanUndo() {
		t.Fatal("a change of the old file is still to undo")
	}
	if _, found := s.Loaded()[99]; !found {
		t.Fatal("the new track is not among the loaded ones")
	}

	// Taking the translation away leaves no translation to show.
	if err := s.SetLine(session.LineTranslation, 0, analysis.LoadedTrack{}, project.Track{}); err != nil {
		t.Fatal(err)
	}
	if s.HasTranslation() {
		t.Fatal("the session still has a translation")
	}
	screenShows(t, r, "no translation", func(screen session.Screen) bool { return screen.Translation == "" })

	// A file that came without subtitles, or a line that does not exist, is refused.
	if err := s.SetLine(session.LineTranslation, 7, analysis.LoadedTrack{}, project.Track{}); err == nil {
		t.Error("a track without subtitles was put on a line")
	}
	if err := s.SetLine(session.Line(5), 99, loadedFile(t, "x"), project.Track{}); err == nil {
		t.Error("an unknown line took a track")
	}
}