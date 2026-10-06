package app_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// watchSwitchVideo watches the video with one more subtitle to switch to:
// audio 3, English 4 on top, Portuguese 5 below.
func watchSwitchVideo(t *testing.T, change func(*rig)) *rig {
	t.Helper()
	r := newRig(t)
	if change != nil {
		change(r)
	}
	r.attach(t)
	if _, err := r.api.OpenVideo(switchVideo); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	t.Cleanup(func() { r.api.StopWatching() })
	return r
}

// The session screen and the panel list each line's best subtitle first: the
// full English track before the one for the hearing impaired, then the
// Portuguese ones, the unreadable one last. Every track stays in the lists.
func TestTheListsShowTheBestSubtitleFirst(t *testing.T) {
	t.Parallel()
	numbers := func(tracks []app.Track) []uint64 {
		var list []uint64
		for _, track := range tracks {
			list = append(list, track.Number)
		}
		return list
	}
	want := []uint64{4, 7, 5, 6}
	r := newRig(t)
	r.attach(t)
	video, err := r.api.OpenVideo(switchVideo)
	if err != nil {
		t.Fatal(err)
	}
	if got := numbers(video.Subtitles); !slices.Equal(got, want) || video.Suggested.Primary != 4 {
		t.Fatalf("session screen: %v (primary %d), want %v", got, video.Suggested.Primary, want)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	testkit.Eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	if got := numbers(r.api.PlaybackState().Tracks.Subtitles); !slices.Equal(got, want) {
		t.Fatalf("panel: %v, want %v", got, want)
	}
}

// A line switched while the video plays is remembered as the choice, and
// what the user adjusts afterwards is kept on the new track, not the old one.
func TestASwitchedSubtitleKeepsItsOwnSync(t *testing.T) {
	t.Parallel()
	r := watchSwitchVideo(t, nil)
	if err := r.api.SwitchSubtitle("primary", 7); err != nil {
		t.Fatal(err)
	}
	if current := r.api.PlaybackState().Tracks.Current; current != (app.Selection{Audio: 3, Primary: 7, Translation: 5}) {
		t.Fatalf("current tracks = %+v, want 7 on top", current)
	}
	if stored, _ := r.projects.stored(t, switchVideo); stored.Choice.Primary != "track:7" {
		t.Fatalf("remembered choice = %+v, want track 7 on top", stored.Choice)
	}
	if err := r.api.Nudge("primary", true); err != nil {
		t.Fatal(err)
	}
	if err := r.api.StopWatching(); err != nil {
		t.Fatal(err)
	}
	stored, _ := r.projects.stored(t, switchVideo)
	if stored.Tracks["track:7"].Offset != 50 || stored.Tracks["track:4"].Offset != 0 {
		t.Fatalf("offsets: track 7 = %v, track 4 = %v; want the 50 ms step on 7 only", stored.Tracks["track:7"].Offset, stored.Tracks["track:4"].Offset)
	}
}

// A switch the line cannot take (here the track the other line shows; the
// rules are the session's) is refused before anything loads: nothing
// changes, nothing is saved, no loading notice shows on the video.
func TestARefusedSubtitleSwitchChangesNothing(t *testing.T) {
	t.Parallel()
	r := watchSwitchVideo(t, nil)
	_, saves := r.projects.stored(t, switchVideo)
	notices := r.display.noticeCount()
	if err := r.api.SwitchSubtitle("primary", 5); err == nil {
		t.Fatal("the line on top took the track the other line shows")
	}
	if err := r.api.SwitchSubtitle("middle", 7); err == nil {
		t.Fatal("a line that does not exist took a track")
	}
	if current := r.api.PlaybackState().Tracks.Current; current != (app.Selection{Audio: 3, Primary: 4, Translation: 5}) {
		t.Fatalf("current tracks changed to %+v", current)
	}
	if _, after := r.projects.stored(t, switchVideo); after != saves {
		t.Fatalf("a refused switch saved the project (%d saves, was %d)", after, saves)
	}
	if r.display.noticeCount() != notices {
		t.Fatal("a refused switch started loading its track")
	}
}

// "Nenhuma" below takes the translation away, and is remembered as the choice.
func TestNoneBelowTakesTheTranslationAway(t *testing.T) {
	t.Parallel()
	r := watchSwitchVideo(t, nil)
	if err := r.api.SwitchSubtitle("translation", 0); err != nil {
		t.Fatal(err)
	}
	if current := r.api.PlaybackState().Tracks.Current; current.Translation != 0 {
		t.Fatalf("translation = %d, want none", current.Translation)
	}
	if stored, _ := r.projects.stored(t, switchVideo); stored.Choice.Translation != "" {
		t.Fatalf("remembered translation = %q, want none", stored.Choice.Translation)
	}
}

// The audio switches by the track's number, the player hears its place
// among the audio tracks, and the choice is remembered; a player that cannot
// switch says so.
func TestTheAudioSwitchesWhileWatching(t *testing.T) {
	t.Parallel()
	r := watchSwitchVideo(t, func(r *rig) { r.plugin.audioSwitch = true })
	if err := r.api.SwitchAudio(2); err != nil {
		t.Fatal(err)
	}
	if asked := r.plugin.current().audioIndexes(); !slices.Equal(asked, []int{0}) {
		t.Fatalf("player asked for audio %v, want [0] (track 2 is the first audio track)", asked)
	}
	if stored, _ := r.projects.stored(t, switchVideo); stored.Choice.Audio != "track:2" {
		t.Fatalf("remembered audio = %q, want track 2", stored.Choice.Audio)
	}
	if err := r.api.SwitchAudio(4); err == nil {
		t.Fatal("a subtitle track was taken as audio")
	}

	fixed := watchSwitchVideo(t, nil)
	if err := fixed.api.SwitchAudio(2); err == nil {
		t.Fatalf("SwitchAudio on a player that cannot = %v, want it refused", err)
	}
	if stored, _ := fixed.projects.stored(t, switchVideo); stored.Choice.Audio != "track:3" {
		t.Fatalf("remembered audio = %q, want track 3 kept", stored.Choice.Audio)
	}
}

// A subtitle loads one at a time, and a session that ends while it loads
// never takes it, nor saves it as the choice.
func TestASubtitleLoadingWhenTheSessionEndsIsDropped(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	if _, err := r.api.OpenVideo(slowVideo); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	readGates.Store(slowVideo, gate)
	t.Cleanup(func() { readGates.Delete(slowVideo) })
	switched := make(chan error, 1)
	go func() { switched <- r.api.SwitchSubtitle("primary", 7) }()
	testkit.Eventually(t, "the subtitle loading", func() bool { return r.api.PlaybackState().Tracks.Loading })
	// A switch the line could take on its own is refused at once, not queued.
	second := make(chan error, 1)
	go func() { second <- r.api.SwitchSubtitle("translation", 7) }()
	select {
	case err := <-second:
		if err == nil {
			t.Fatal("a second switch went through while the first one loads")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a second switch waited for the first instead of being refused")
	}
	if err := r.api.StopWatching(); err != nil {
		t.Fatal(err)
	}
	close(gate)
	if err := <-switched; err == nil {
		t.Fatal("the switch went through after the session ended")
	}
	if stored, _ := r.projects.stored(t, slowVideo); stored.Choice.Primary != "track:4" {
		t.Fatalf("remembered choice = %+v, want track 4 kept", stored.Choice)
	}
}

// writeSubtitle writes a subtitle file in a folder of its own, away from the
// video's.
func writeSubtitle(t *testing.T, name, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("1\r\n00:00:01,000 --> 00:00:02,000\r\n"+text+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// fileTrack finds the track of a subtitle file in a list; zero for none.
func fileTrack(tracks []app.Track) uint64 {
	for _, track := range tracks {
		if track.Number >= media.ExternalTrackBase {
			return track.Number
		}
	}
	return 0
}

// addedVideo is a video with no subtitle file next to it.
const addedVideo = `D:\Filmes\Movie.mkv`

// addFile opens addedVideo and adds a subtitle file from another folder on
// its session screen; it returns the file's path and track.
func addFile(t *testing.T, r *rig) (string, uint64) {
	t.Helper()
	if _, err := r.api.OpenVideo(addedVideo); err != nil {
		t.Fatal(err)
	}
	path := writeSubtitle(t, "Outra.Legenda.en.srt", "From the file.")
	r.ui.subtitle = path
	added, err := r.api.AddSubtitleFile("primary")
	if err != nil {
		t.Fatal(err)
	}
	return path, fileTrack(added.Subtitles)
}

// A subtitle file from any folder joins the video on the session screen as
// the line's suggestion, once however often it is picked; the log names the
// file and never its folder.
func TestASubtitleFileJoinsTheVideoOnce(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	before, err := r.api.OpenVideo(addedVideo)
	if err != nil {
		t.Fatal(err)
	}
	path, number := addFile(t, r)
	again, err := r.api.AddSubtitleFile("primary")
	if err != nil || number == 0 || fileTrack(again.Subtitles) != number || len(again.Subtitles) != len(before.Subtitles)+1 {
		t.Fatalf("added twice: %d subtitles (were %d), track %d then %d, %v", len(again.Subtitles), len(before.Subtitles), number, fileTrack(again.Subtitles), err)
	}
	if again.Suggested.Primary != number {
		t.Fatalf("suggested %+v, want the file %d on top", again.Suggested, number)
	}
	if stored, _ := r.projects.stored(t, addedVideo); len(stored.AddedFiles) != 1 {
		t.Fatalf("remembered files = %q, want the one file", stored.AddedFiles)
	}
	if log := r.log.String(); !strings.Contains(log, filepath.Base(path)) || strings.Contains(log, filepath.Dir(path)) {
		t.Fatalf("the log should name the file and never its folder:\n%s", log)
	}
}

// A cancelled dialog changes nothing, and a file that is no subtitle is
// refused.
func TestNoSubtitleFileAddsNothing(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	if _, err := r.api.OpenVideo(addedVideo); err != nil {
		t.Fatal(err)
	}
	r.ui.subtitle = ""
	if cancelled, err := r.api.AddSubtitleFile("primary"); err != nil || cancelled.Path != "" {
		t.Fatalf("a cancelled dialog = %+v, %v", cancelled.Path, err)
	}
	r.ui.subtitle = writeSubtitle(t, "nota.txt", "")
	if err := os.WriteFile(r.ui.subtitle, []byte("just some notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.api.AddSubtitleFile("primary"); err == nil {
		t.Fatal("a file that is no subtitle was added")
	}
	if stored, _ := r.projects.stored(t, addedVideo); len(stored.AddedFiles) != 0 {
		t.Fatalf("remembered files = %q, want none", stored.AddedFiles)
	}
}

// A file watched with comes back, still chosen, the next time the video
// opens; once it is gone, it is left out without failing the video.
func TestAnAddedFileComesBackUntilItIsGone(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	path, number := addFile(t, r)
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: number, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	if err := r.api.StopWatching(); err != nil {
		t.Fatal(err)
	}
	reopened, err := r.api.OpenVideo(addedVideo)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileTrack(reopened.Subtitles); got == 0 || reopened.Suggested.Primary != got {
		t.Fatalf("reopened: file track %d, suggested %+v", got, reopened.Suggested)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	gone, err := r.api.OpenVideo(addedVideo)
	if err != nil {
		t.Fatal(err)
	}
	if fileTrack(gone.Subtitles) != 0 {
		t.Fatal("a file that is gone still shows")
	}
}

// A file added while the video plays describes the video from its project
// under the lock the session saves it under: saving passages meanwhile is
// no data race (run with -race).
func TestAddingAFileWhilePassagesAreSavedIsSafe(t *testing.T) {
	t.Parallel()
	r := watchSwitchVideo(t, nil)
	if err := r.api.StudyPassage(app.PassageRequest{Start: "0:10", End: "0:20"}); err != nil {
		t.Fatal(err)
	}
	r.ui.subtitle = writeSubtitle(t, "Legenda.pt-BR.srt", "Do arquivo.")
	saved := make(chan error, 1)
	go func() {
		for range 50 {
			if err := r.api.SavePassage("Trecho"); err != nil {
				saved <- err
				return
			}
		}
		saved <- nil
	}()
	for range 20 {
		if _, err := r.api.AddSubtitleFile("translation"); err != nil {
			t.Fatal(err)
		}
	}
	if err := <-saved; err != nil {
		t.Fatal(err)
	}
}

// Added while the video plays, the file goes straight to its line.
func TestASubtitleFileAddedWhileWatchingGoesOnItsLine(t *testing.T) {
	t.Parallel()
	r := watchSwitchVideo(t, nil)
	r.ui.subtitle = writeSubtitle(t, "Legenda.pt-BR.srt", "Do arquivo.")
	if _, err := r.api.AddSubtitleFile("translation"); err != nil {
		t.Fatal(err)
	}
	state := r.api.PlaybackState()
	number := fileTrack(state.Tracks.Subtitles)
	if number == 0 || state.Tracks.Current.Translation != number {
		t.Fatalf("translation = %d, file track = %d; want the file below", state.Tracks.Current.Translation, number)
	}
}
