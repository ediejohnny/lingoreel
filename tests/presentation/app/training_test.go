package app_test

import (
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

func startWatching(t *testing.T) *rig {
	t.Helper()
	r := newRig(t)
	r.attach(t)
	if _, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	t.Cleanup(func() { r.api.StopWatching() })
	return r
}

func TestKaraokeChoicesAreKeptInTheProfile(t *testing.T) {
	t.Parallel()
	r := startWatching(t)
	state := r.api.PlaybackState()
	if state.Karaoke.Enabled || state.Karaoke.Indicator != "bar" || state.Karaoke.Repeat != 0 {
		t.Fatalf("a new profile's karaoke = %+v, want off with the bar", state.Karaoke)
	}
	if err := r.api.SetKaraoke(true); err != nil {
		t.Fatal(err)
	}
	if err := r.api.SetKaraokeIndicator("countdown"); err != nil {
		t.Fatal(err)
	}
	if err := r.api.SetLineRepeats(3); err != nil {
		t.Fatal(err)
	}
	state = r.api.PlaybackState()
	saved := r.settings.profiles.Items[0].Study.Karaoke
	if !state.Karaoke.Enabled || state.Karaoke.Indicator != "countdown" || state.Karaoke.Repeat != 3 ||
		!saved.Enabled || saved.Indicator != "countdown" || saved.Repeat != 3 {
		t.Fatalf("state %+v, saved %+v", state.Karaoke, saved)
	}
	for _, bad := range []func() error{
		func() error { return r.api.SetKaraokeIndicator("dots") },
		func() error { return r.api.SetLineRepeats(11) },
		func() error { return r.api.SetLineRepeats(-1) },
	} {
		if err := bad(); err == nil {
			t.Fatal("a bad choice was taken")
		}
	}
	if saved := r.settings.profiles.Items[0].Study.Karaoke; saved.Repeat != 3 || saved.Indicator != "countdown" {
		t.Fatalf("a refused choice changed the profile: %+v", saved)
	}
}

func TestStudyPassageChecksWhatTheUserTyped(t *testing.T) {
	t.Parallel()
	r := startWatching(t)
	cases := []struct {
		request app.PassageRequest
		message string
	}{
		{app.PassageRequest{Start: "1:99:00", End: "2:00"}, "1:10:23"},
		{app.PassageRequest{Start: "", End: "2:00"}, "1:10:23"},
		{app.PassageRequest{Start: "0:50", End: "0:50,5"}, "pelo menos 1 segundo"},
		{app.PassageRequest{Start: "0:50", End: "0:40"}, "pelo menos 1 segundo"},
		{app.PassageRequest{Start: "0:10", End: "0:20", Loop: true, Gap: 6}, "intervalo"},
		{app.PassageRequest{Start: "0:10", End: "0:20", Goal: 100}, "meta"},
	}
	for _, c := range cases {
		err := r.api.StudyPassage(c.request)
		if err == nil || !strings.Contains(err.Error(), c.message) {
			t.Errorf("StudyPassage(%+v) = %v, want a message with %q", c.request, err, c.message)
		}
	}
	if err := r.api.StudyPassage(app.PassageRequest{Start: "0:10", End: "0:20,5", Goal: 3}); err != nil {
		t.Fatal(err)
	}
	passage := r.api.PlaybackState().Passage
	if !passage.Active || passage.Start != "0:10" || passage.End != "0:20,5" || passage.Lap != 1 || passage.Goal != 3 {
		t.Fatalf("passage = %+v", passage)
	}
	if err := r.api.StopPassage(); err != nil || r.api.PlaybackState().Passage.Active {
		t.Fatalf("StopPassage = %v", err)
	}
	if err := r.api.ResumePassage(); err == nil || !strings.Contains(err.Error(), "Nenhum trecho") {
		t.Fatalf("ResumePassage without a passage = %v", err)
	}
}

func TestTrainingNeedsASession(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	if err := r.api.ReplayLine(); err == nil {
		t.Fatal("ReplayLine without a session")
	}
	if _, err := r.api.PassageOfLine(); err == nil {
		t.Fatal("PassageOfLine without a session")
	}
	if err := r.api.StudyPassage(app.PassageRequest{Start: "0:10", End: "0:20"}); err == nil {
		t.Fatal("StudyPassage without a session")
	}
}
