package app_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
)

// savedProject is a project already kept for the movie: two passages, a
// choice and where it was left.
func savedProject(t *testing.T, r *rig) {
	t.Helper()
	p := project.New()
	p.Choice = project.Choice{Audio: "track:2", Primary: "track:5", Translation: "track:4", Mode: "dual"}
	p.Passages = []project.Passage{
		{Name: "Loja", Start: 10_000, End: 20_000, Plan: &project.PassagePlan{Loop: true, Gap: 1000, Goal: 2}},
		{Name: "Rua", Start: 60_000, End: 64_000},
		// Of another cut: it starts after the end of this 90-minute video.
		{Name: "Créditos", Start: project.MillisOf(95 * time.Minute), End: project.MillisOf(96 * time.Minute)},
	}
	p.Last = project.Last{Position: 638_000}
	if err := r.projects.Save(movie, p); err != nil {
		t.Fatal(err)
	}
}

func openMovie(t *testing.T, r *rig) app.Video {
	t.Helper()
	video, err := r.api.OpenVideo(movie)
	if err != nil {
		t.Fatal(err)
	}
	return video
}

func names(passages []app.SavedPassage) []string {
	list := make([]string, len(passages))
	for i, passage := range passages {
		list[i] = passage.Name
	}
	return list
}

// The session screen shows the saved passages and where the video was left,
// before anything plays.
func TestTheSessionScreenShowsWhatTheVideoKept(t *testing.T) {
	r := newRig(t)
	savedProject(t, r)
	r.attach(t)
	video := openMovie(t, r)
	if got := names(video.SavedPassages); !slices.Equal(got, []string{"Loja", "Rua", "Créditos"}) {
		t.Fatalf("saved passages = %q", got)
	}
	loja := video.SavedPassages[0]
	if loja.Start != "0:10" || loja.End != "0:20" || loja.Plan != (app.PassagePlan{Loop: true, Gap: 1, Goal: 2}) ||
		loja.Summary != "Repete sem parar · intervalo de 1 s · meta de 2 voltas" || loja.Outside {
		t.Fatalf("Loja = %+v", loja)
	}
	if rua := video.SavedPassages[1]; rua.Summary != "Pausa no fim" || rua.Outside {
		t.Fatalf("Rua = %+v", rua)
	}
	if !video.SavedPassages[2].Outside {
		t.Fatal("a passage past the end of the video is not marked")
	}
	if video.Resume != (app.Resume{Offered: true, Position: 638, Text: "10:38"}) {
		t.Fatalf("resume = %+v", video.Resume)
	}
	if got := names(r.api.SavedPassages()); !slices.Equal(got, []string{"Loja", "Rua", "Créditos"}) {
		t.Fatalf("SavedPassages() = %q", got)
	}
}

// "Continuar de" is not offered in the first seconds nor in the credits.
func TestContinuarDeIsOfferedOnlyInTheMiddle(t *testing.T) {
	cases := []struct {
		name     string
		position time.Duration
		offered  bool
	}{
		{"never left", 0, false},
		{"first seconds", 29 * time.Second, false},
		{"just past them", 30 * time.Second, true},
		{"middle", 45 * time.Minute, true},
		{"before the credits", 89 * time.Minute, true},
		{"in the credits", 89*time.Minute + 1*time.Second, false},
	}
	for _, c := range cases {
		r := newRig(t)
		p := project.New()
		p.Last.Position = project.MillisOf(c.position)
		if err := r.projects.Save(movie, p); err != nil {
			t.Fatal(err)
		}
		r.attach(t)
		if got := openMovie(t, r).Resume.Offered; got != c.offered {
			t.Errorf("%s (%v): offered %v, want %v", c.name, c.position, got, c.offered)
		}
	}
}

// Passages can be renamed and deleted on the session screen, with no video
// playing; names stay unique whatever the case.
func TestPassagesAreManagedBeforeWatching(t *testing.T) {
	r := newRig(t)
	savedProject(t, r)
	r.attach(t)
	openMovie(t, r)

	if err := r.api.RenamePassage("loja", "Praça"); err != nil {
		t.Fatal(err)
	}
	wantMessage(t, "RenamePassage onto another name", r.api.RenamePassage("Praça", "rua"), "Já existe")
	wantMessage(t, "RenamePassage of an unknown name", r.api.RenamePassage("Feira", "Mercado"), "não encontrado")
	wantMessage(t, "RenamePassage to nothing", r.api.RenamePassage("Rua", "   "), "Dê um nome")
	// The same name in another case is the same passage: allowed.
	if err := r.api.RenamePassage("Rua", "RUA"); err != nil {
		t.Fatal(err)
	}
	if err := r.api.DeletePassage("créditos"); err != nil {
		t.Fatal(err)
	}
	stored, _ := r.projects.stored(t, movie)
	if got := []string{stored.Passages[0].Name, stored.Passages[1].Name}; len(stored.Passages) != 2 || !slices.Equal(got, []string{"Praça", "RUA"}) {
		t.Fatalf("stored passages = %q", got)
	}
	// The plan goes with the passage it belongs to.
	if stored.Passages[0].Plan == nil || stored.Passages[0].Plan.Goal != 2 {
		t.Fatalf("the plan was lost in the rename: %+v", stored.Passages[0])
	}
	if got := names(r.api.SavedPassages()); !slices.Equal(got, []string{"Praça", "RUA"}) {
		t.Fatalf("SavedPassages() = %q", got)
	}
	if err := r.api.SavePassage("X"); err == nil {
		t.Fatal("SavePassage worked with no video playing")
	}
}

// "Estudar este trecho" on the session screen opens the video studying it,
// with its plan; a passage of another cut is refused.
func TestWatchingFromASavedPassage(t *testing.T) {
	r := newRig(t)
	savedProject(t, r)
	r.attach(t)
	openMovie(t, r)
	wantMessage(t, "Watch from an unknown passage", r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5, Passage: "Feira"}), "não encontrado")
	wantMessage(t, "Watch from a passage past the end", r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5, Passage: "Créditos"}), "depois do fim")

	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5, Passage: "loja", Resume: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	if start := r.plugin.launched[len(r.plugin.launched)-1].Start; start != 10*time.Second {
		t.Fatalf("opened at %v, want the passage's start (the passage wins over Continuar de)", start)
	}
	passage := r.api.PlaybackState().Passage
	if !passage.Active || passage.Start != "0:10" || passage.End != "0:20" || passage.Goal != 2 {
		t.Fatalf("passage = %+v", passage)
	}

	// From the panel, a saved passage studies with its own plan.
	if err := r.api.StudySavedPassage("rua"); err != nil {
		t.Fatal(err)
	}
	if passage := r.api.PlaybackState().Passage; !passage.Active || passage.Start != "1:00" || passage.End != "1:04" || passage.Goal != 0 {
		t.Fatalf("passage = %+v", passage)
	}
	wantMessage(t, "StudySavedPassage past the end", r.api.StudySavedPassage("Créditos"), "depois do fim")
	wantMessage(t, "StudySavedPassage of an unknown name", r.api.StudySavedPassage("Feira"), "não encontrado")
}

// The video comes back as it was left: the position (asked with Continuar
// de), the speed and the Modo Karaokê used on it.
func TestTheVideoComesBackAsItWasLeft(t *testing.T) {
	r := startWatching(t)
	seekTo(t, r, 600)
	if err := r.api.SetSpeed(0.75); err != nil {
		t.Fatal(err)
	}
	if err := r.api.SetKaraoke(true); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the panel to show the speed", func() bool { return r.api.PlaybackState().Speed == 0.75 })
	r.api.StopWatching()
	eventually(t, "the end", func() bool { return !r.api.PlaybackState().Active })
	stored, _ := r.projects.stored(t, movie)
	if stored.Last.Position != 600_000 || stored.Last.Speed != 0.75 || stored.Last.Karaoke == nil || !stored.Last.Karaoke.Enabled {
		t.Fatalf("last = %+v (karaoke %+v)", stored.Last, stored.Last.Karaoke)
	}

	video := openMovie(t, r)
	if !video.Resume.Offered || video.Resume.Text != "10:00" {
		t.Fatalf("resume = %+v", video.Resume)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5, Resume: true}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	if start := r.plugin.launched[len(r.plugin.launched)-1].Start; start != 600*time.Second {
		t.Fatalf("Continuar de opened at %v, want 10:00", start)
	}
	running := r.plugin.current()
	eventually(t, "the video's speed", func() bool {
		running.mu.Lock()
		defer running.mu.Unlock()
		return slices.Contains(running.speeds, 0.75)
	})
	if state := r.api.PlaybackState(); !state.Karaoke.Enabled {
		t.Fatalf("karaoke = %+v, want the video's", state.Karaoke)
	}
	// The video's speed is not the profile's: a new video starts at normal.
	if speed := r.settings.profiles.Items[0].Speed; speed != 1 {
		t.Fatalf("the profile's speed became %v", speed)
	}
	r.api.StopWatching()
	eventually(t, "the end", func() bool { return !r.api.PlaybackState().Active })

	// Without Continuar de, it starts from the beginning.
	openMovie(t, r)
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	if start := r.plugin.launched[len(r.plugin.launched)-1].Start; start != 0 {
		t.Fatalf("without Continuar de it opened at %v", start)
	}
}

// A video watched without changing anything keeps where it was left, not a
// speed or a Modo Karaokê it never had: those stay the profile's.
func TestAnUntouchedVideoKeepsOnlyItsPosition(t *testing.T) {
	r := startWatching(t)
	seekTo(t, r, 120)
	r.api.StopWatching()
	eventually(t, "the end", func() bool { return !r.api.PlaybackState().Active })
	stored, _ := r.projects.stored(t, movie)
	if stored.Last.Position != 120_000 || stored.Last.Speed != 0 || stored.Last.Karaoke != nil {
		t.Fatalf("last = %+v", stored.Last)
	}
}

// Steps of a held sync key save once, a moment after the last step, and
// the study mode changed on the panel is kept for the video.
func TestChangesWhileWatchingAreSavedAMomentLater(t *testing.T) {
	// Longer than a state report (250 ms), which is when the mode is read.
	r := newRigWith(t, func(deps *app.Deps) { deps.SaveDelay = 600 * time.Millisecond })
	r.attach(t)
	openMovie(t, r)
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	_, before := r.projects.stored(t, movie)
	for range 3 {
		if err := r.api.Nudge("translation", true); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.api.SetStudyMode("always"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the waiting save", func() bool {
		stored, _ := r.projects.stored(t, movie)
		return stored.Tracks["track:5"].Offset != 0 && stored.Choice.Mode == "always"
	})
	time.Sleep(time.Second) // well past the delay: no more saves come
	stored, after := r.projects.stored(t, movie)
	if after-before != 1 {
		t.Fatalf("%d saves for three steps and a mode, want one", after-before)
	}
	if stored.Tracks["track:5"].Offset != project.MillisOf(150*time.Millisecond) {
		t.Fatalf("offset kept %d ms, want three steps of 50", stored.Tracks["track:5"].Offset)
	}
}

// "Esquecer tudo deste vídeo" removes what was kept and opens the video as
// new; never while it plays.
func TestResetForgetsEverythingAboutTheVideo(t *testing.T) {
	r := newRig(t)
	savedProject(t, r)
	r.attach(t)
	if video := openMovie(t, r); video.Mode != "dual" || video.Suggested.Primary != 5 {
		t.Fatalf("before the reset: mode %q, suggested %+v", video.Mode, video.Suggested)
	}
	video, err := r.api.ResetProject()
	if err != nil {
		t.Fatal(err)
	}
	if len(video.SavedPassages) != 0 || video.Resume.Offered || video.Suggested.Primary != 4 || video.Mode != "on-demand" {
		t.Fatalf("after the reset: %+v", video)
	}
	if !slices.Equal(r.projects.removed, []string{movie}) {
		t.Fatalf("removed %q", r.projects.removed)
	}
	if stored, _ := r.projects.stored(t, movie); len(stored.Passages) != 0 || stored.Last.Position != 0 {
		t.Fatalf("still stored: %+v", stored)
	}

	// Never while the video plays.
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	_, err = r.api.ResetProject()
	wantMessage(t, "ResetProject while watching", err, "Encerre o vídeo")
	r.api.StopWatching()
	eventually(t, "the end", func() bool { return !r.api.PlaybackState().Active })

	// A file that cannot be removed is told, and nothing else changes.
	r.projects.removeErr = errors.New("in use")
	_, err = r.api.ResetProject()
	wantMessage(t, "ResetProject of a file in use", err, "Não foi possível apagar")
}

// The slower laps of a progressive passage are the passage's, not the
// video's: they are not kept as the speed to come back to.
func TestAProgressivePassageLeavesTheVideosSpeed(t *testing.T) {
	r := startWatching(t)
	seekTo(t, r, 20)
	if err := r.api.StudyPassage(app.PassageRequest{Start: "0:20", End: "0:35", Progressive: true}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first lap's slower speed", func() bool { return r.api.PlaybackState().Speed < 1 })
	time.Sleep(600 * time.Millisecond) // a few state reports
	r.api.StopWatching()
	eventually(t, "the end", func() bool { return !r.api.PlaybackState().Active })
	if stored, _ := r.projects.stored(t, movie); stored.Last.Speed != 0 {
		t.Fatalf("kept speed %v, want none", stored.Last.Speed)
	}
}

// Steps that go on for longer than the delay are saved about once per
// delay, not once per step.
func TestALongRunOfStepsIsSavedOncePerDelay(t *testing.T) {
	const delay = 200 * time.Millisecond
	r := newRigWith(t, func(deps *app.Deps) { deps.SaveDelay = delay })
	r.attach(t)
	openMovie(t, r)
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	_, before := r.projects.stored(t, movie)
	const steps = 20
	for range steps {
		if err := r.api.Nudge("translation", true); err != nil {
			t.Fatal(err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	eventually(t, "the last step saved", func() bool {
		stored, _ := r.projects.stored(t, movie)
		return stored.Tracks["track:5"].Offset == project.MillisOf(steps*50*time.Millisecond)
	})
	time.Sleep(2 * delay)
	// 20 steps over half a second: a save at most every 200 ms.
	if _, after := r.projects.stored(t, movie); after-before > 5 {
		t.Fatalf("%d saves for %d steps over %v", after-before, steps, steps*25*time.Millisecond)
	}
}
