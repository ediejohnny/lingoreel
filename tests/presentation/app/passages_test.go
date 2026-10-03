package app_test

import (
	"math"
	"testing"

	"lingoreel/internal/domain/project"
	"lingoreel/internal/presentation/app"
)

// seekTo moves the paused video and waits for the panel to show it there.
func seekTo(t *testing.T, r *rig, seconds float64) {
	t.Helper()
	if err := r.api.SeekTo(seconds); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the position", func() bool { return math.Abs(r.api.PlaybackState().Position-seconds) < 0.01 })
}

func (d *recordingDisplay) lastNotice() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.notices) == 0 {
		return ""
	}
	return d.notices[len(d.notices)-1]
}

// Ctrl+F9 then Ctrl+F10 saves the passage at once, named by its times, and
// it is studied straight away; the same span again keeps both.
func TestTheEndMarkSavesThePassageNamedByItsTimes(t *testing.T) {
	r := startWatching(t)
	seekTo(t, r, 20)
	if err := r.api.RunShortcut("passage-mark-start", false); err != nil {
		t.Fatal(err)
	}
	seekTo(t, r, 35)
	if err := r.api.RunShortcut("passage-mark-end", false); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the passage saved", func() bool { stored, _ := r.projects.stored(t, movie); return len(stored.Passages) == 1 })
	stored, _ := r.projects.stored(t, movie)
	saved := stored.Passages[0]
	// Fitted to the lines: half a second of lead-in before the start, which
	// the name, in whole seconds, does not show.
	if saved.Name != "Trecho 0:20–0:35" || saved.Start != 19_500 || saved.End != 35_000 {
		t.Fatalf("saved %+v", saved)
	}
	if saved.Plan == nil || saved.Plan.Loop || saved.Plan.Goal != 0 {
		t.Fatalf("saved plan %+v, want pausing at the end", saved.Plan)
	}
	state := r.api.PlaybackState()
	if !state.Passage.Active || len(state.SavedPassages) != 1 || state.SavedPassages[0].Name != "Trecho 0:20–0:35" {
		t.Fatalf("panel: passage %+v, saved %+v", state.Passage, state.SavedPassages)
	}
	if notice := r.display.lastNotice(); notice != "Salvo: Trecho 0:20–0:35" {
		t.Fatalf("notice %q", notice)
	}

	// The same span again: a second passage, not a replaced one.
	seekTo(t, r, 20)
	if err := r.api.RunShortcut("passage-mark-start", false); err != nil {
		t.Fatal(err)
	}
	seekTo(t, r, 35)
	if err := r.api.RunShortcut("passage-mark-end", false); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the second passage", func() bool { stored, _ := r.projects.stored(t, movie); return len(stored.Passages) == 2 })
	if stored, _ := r.projects.stored(t, movie); stored.Passages[1].Name != "Trecho 0:20–0:35 (2)" {
		t.Fatalf("second name %q", stored.Passages[1].Name)
	}

	// An end with no start is told on the video and saves nothing.
	if err := r.api.RunShortcut("passage-mark-end", false); err != nil {
		t.Fatalf("an end without a start = %v", err)
	}
	if stored, _ := r.projects.stored(t, movie); len(stored.Passages) != 2 {
		t.Fatalf("an end without a start saved %+v", stored.Passages)
	}
}

// A project that already holds the most passages it may refuses the next,
// saying so on the video, and keeps what it had.
func TestTheEndMarkRespectsThePassageLimit(t *testing.T) {
	r := newRig(t)
	full := project.New()
	for i := range project.MaxPassages {
		full.Passages = append(full.Passages, project.Passage{Name: "P" + string(rune('a'+i%26)) + string(rune('a'+i/26%26)), Start: 0, End: 1000})
	}
	if err := r.projects.Save(movie, full); err != nil {
		t.Fatal(err)
	}
	r.attach(t)
	if _, err := r.api.OpenVideo(movie); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	_, before := r.projects.stored(t, movie)
	seekTo(t, r, 20)
	if err := r.api.RunShortcut("passage-mark-start", false); err != nil {
		t.Fatal(err)
	}
	seekTo(t, r, 35)
	if err := r.api.RunShortcut("passage-mark-end", false); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the refusal on the video", func() bool { return r.display.lastNotice() != "" })
	stored, after := r.projects.stored(t, movie)
	if len(stored.Passages) != project.MaxPassages || after != before {
		t.Fatalf("a full project took a passage: %d passages, %d saves (was %d)", len(stored.Passages), after, before)
	}
}
