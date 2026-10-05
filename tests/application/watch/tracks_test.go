package watch_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
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

// switchedRig is a session paused on a line, with a correction on each line,
// whose line on top then takes another file (track 99).
func switchedRig(t *testing.T) (*rig, *watch.Session) {
	t.Helper()
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
	return r, s
}

// A line takes another file while the video is paused on it: the screen
// shows the new file, and the selection on that line goes.
func TestALineTakesAnotherSubtitle(t *testing.T) {
	t.Parallel()
	r, s := switchedRig(t)
	screenShows(t, r, "the new file on top", func(screen session.Screen) bool { return screen.Primary == "Another file." })
	if _, found := s.Selection(); found {
		t.Fatal("the selection on the switched line stayed")
	}
	if _, found := s.Loaded()[99]; !found {
		t.Fatal("the new track is not among the loaded ones")
	}
	if current := s.Tracks().Current; current.Primary != 99 || current.Translation != 5 {
		t.Fatalf("current tracks = %+v, want 99 on top and 5 below", current)
	}
}

// After a switch, undo no longer reaches the old file's changes, while the
// other line keeps its own.
func TestUndoForgetsTheSwitchedLine(t *testing.T) {
	t.Parallel()
	r, s := switchedRig(t)
	if undone, err := s.Undo(); !undone || err != nil {
		t.Fatalf("Undo = %v, %v; the other line's change should be there", undone, err)
	}
	screenShows(t, r, "the translation undone, the new file kept", func(screen session.Screen) bool {
		return screen.Translation == "Olá." && screen.Primary == "Another file."
	})
	if s.CanUndo() {
		t.Fatal("a change of the old file is still to undo")
	}
}

// What changes on the switched line is told with the new track, so the app
// keeps it on the new file and not on the old one.
func TestChangesOnASwitchedLineGoToItsNewTrack(t *testing.T) {
	t.Parallel()
	r, s := switchedRig(t)
	s.Nudge(session.LinePrimary, true)
	if _, tracks := r.listener.syncs(); len(tracks) != 1 || tracks[0] != 99 {
		t.Fatalf("sync told with tracks %v, want [99]", tracks)
	}
	if _, err := s.Select(session.LinePrimary); err != nil {
		t.Fatal(err)
	}
	if err := s.Edit(watch.EditChange{Text: new("Another one.")}); err != nil {
		t.Fatal(err)
	}
	if edits := r.listener.edits(); edits[len(edits)-1].track != 99 {
		t.Fatalf("edit told with track %d, want 99", edits[len(edits)-1].track)
	}
}

// Taking the translation away leaves no translation to show.
func TestNoneBelowTakesTheTranslationAway(t *testing.T) {
	t.Parallel()
	r, s := playingRig(t)
	if err := s.SetLine(session.LineTranslation, 0, analysis.LoadedTrack{}, project.Track{}); err != nil {
		t.Fatal(err)
	}
	if s.HasTranslation() || s.Tracks().Current.Translation != 0 {
		t.Fatalf("the session still has a translation (tracks %+v)", s.Tracks().Current)
	}
	screenShows(t, r, "no translation", func(screen session.Screen) bool { return screen.Translation == "" })
}

// A file that came without subtitles, or a line that does not exist, is
// refused.
func TestASwitchNeedsSubtitlesAndALine(t *testing.T) {
	t.Parallel()
	_, s := playingRig(t)
	if err := s.SetLine(session.LineTranslation, 7, analysis.LoadedTrack{}, project.Track{}); err == nil {
		t.Error("a track without subtitles was put on a line")
	}
	if err := s.SetLine(session.Line(5), 99, loadedFile(t, "x"), project.Track{}); err == nil {
		t.Error("an unknown line took a track")
	}
}

// What a line cannot show is refused before anything loads: a subtitle in
// pictures, an audio track, a track the video lacks, the track the other
// line shows, none on top, a line that does not exist. None below is fine.
func TestALineRefusesWhatItCannotShow(t *testing.T) {
	t.Parallel()
	s := newRig(t).start(t)
	s.AddTrack(media.Track{Number: 6, Type: media.TrackSubtitle, Language: "en", Codec: "S_HDMV/PGS"})
	for _, c := range []struct {
		line   session.Line
		number uint64
		want   error // nil: any error
	}{
		{session.LinePrimary, 6, watch.ErrUnknownTrack},
		{session.LinePrimary, 3, watch.ErrUnknownTrack},
		{session.LinePrimary, 99, watch.ErrUnknownTrack},
		{session.LinePrimary, 5, watch.ErrSameTrack},
		{session.LineTranslation, 4, watch.ErrSameTrack},
		{session.LinePrimary, 0, watch.ErrNeedsPrimary},
		{session.Line(7), 4, nil},
	} {
		err := s.CheckLine(c.line, c.number)
		if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
			t.Errorf("CheckLine(%d, %d) = %v, want %v", c.line, c.number, err, c.want)
		}
	}
	if err := s.CheckLine(session.LineTranslation, 0); err != nil {
		t.Errorf("none below refused: %v", err)
	}
}

// The switch itself checks the pair of lines again, as the other line may
// have changed while the file loaded, and a refused switch changes nothing.
func TestASwitchChecksThePairOfLinesAgain(t *testing.T) {
	t.Parallel()
	s := newRig(t).start(t)
	before := s.Tracks().Current
	if err := s.SetLine(session.LinePrimary, 5, loadedFile(t, "Same as below."), project.Track{}); !errors.Is(err, watch.ErrSameTrack) {
		t.Errorf("the other line's track on top = %v, want ErrSameTrack", err)
	}
	if err := s.SetLine(session.LinePrimary, 0, analysis.LoadedTrack{}, project.Track{}); !errors.Is(err, watch.ErrNeedsPrimary) {
		t.Errorf("none on top = %v, want ErrNeedsPrimary", err)
	}
	if after := s.Tracks().Current; after != before {
		t.Fatalf("a refused switch changed the tracks to %+v", after)
	}
}

// A sync step that crosses a switch of the line's subtitle is told with the
// track it moved: never one file's offset under the other file's number,
// which the app would keep on the wrong file.
func TestASyncStepIsToldWithTheTrackItMoved(t *testing.T) {
	t.Parallel()
	r, s := playingRig(t)
	const far = 10 * time.Second // where the second file was left
	files := []struct {
		number uint64
		loaded analysis.LoadedTrack
		saved  project.Track
	}{
		{4, loadedFile(t, "First file."), project.Track{}},
		{99, loadedFile(t, "Second file."), project.Track{Offset: project.MillisOf(far)}},
	}
	switched := make(chan error, 1)
	go func() {
		for i := range 2000 {
			file := files[i%2]
			if err := s.SetLine(session.LinePrimary, file.number, file.loaded, file.saved); err != nil {
				switched <- err
				return
			}
		}
		switched <- nil
	}()
	later := true
	for stepping := true; stepping; later = !later {
		select {
		case err := <-switched:
			if err != nil {
				t.Fatal(err)
			}
			stepping = false
		default:
			s.Nudge(session.LinePrimary, later)
		}
	}
	offsets, tracks := r.listener.syncs()
	for i, track := range tracks {
		base := time.Duration(0)
		if track == 99 {
			base = far
		}
		if moved := offsets[i].Duration() - base; moved < -time.Second || moved > time.Second {
			t.Fatalf("step %d was told on track %d with the offset %v, the other file's", i, track, offsets[i].Duration())
		}
	}
}

// The audio switches by the track's number: the player hears its place among
// the audio tracks, which is what players count by.
func TestTheAudioSwitchesWhileTheVideoPlays(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.running.audio = true
	s := r.start(t)
	audio := s.Tracks().Of(media.TrackAudio)
	if len(audio) < 2 {
		t.Fatalf("the sample video has %d audio tracks, want 2", len(audio))
	}
	r.running.took()
	if err := s.SelectAudio(audio[0].Number); err != nil {
		t.Fatal(err)
	}
	if calls := r.running.took(); !slices.Equal(calls, []string{"audio 0"}) {
		t.Fatalf("player calls = %v, want [audio 0]", calls)
	}
	if s.Tracks().Current.Audio != audio[0].Number {
		t.Fatalf("current audio = %d, want %d", s.Tracks().Current.Audio, audio[0].Number)
	}
	// A subtitle track is no audio track.
	if err := s.SelectAudio(4); err == nil || s.Tracks().Current.Audio != audio[0].Number {
		t.Fatalf("SelectAudio(subtitle) = %v, current %d", err, s.Tracks().Current.Audio)
	}
}

// A player that cannot switch the audio says so, and nothing changes.
func TestTheAudioStaysWithAPlayerThatCannotSwitchIt(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	s := r.start(t)
	before := s.Tracks().Current.Audio
	audio := s.Tracks().Of(media.TrackAudio)
	if err := s.SelectAudio(audio[0].Number); !errors.Is(err, session.ErrNotSupported) {
		t.Fatalf("SelectAudio = %v, want ErrNotSupported", err)
	}
	if s.Tracks().Current.Audio != before {
		t.Fatal("the current audio changed")
	}
}
