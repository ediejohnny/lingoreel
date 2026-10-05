package app_test

import (
	"context"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// seekTo moves the paused video and waits for the panel to show it there.
// It first waits for the player's first report, as a user always does:
// a seek before it would be undone by it.
func seekTo(t *testing.T, r *rig, seconds float64) {
	t.Helper()
	testkit.Eventually(t, "the player's first report", func() bool { return r.api.PlaybackState().Duration > 0 })
	if err := r.api.SeekTo(seconds); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the position", func() bool { return math.Abs(r.api.PlaybackState().Position-seconds) < 0.01 })
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
	t.Parallel()
	r := startWatching(t)
	seekTo(t, r, 20)
	if err := r.api.RunShortcut("passage-mark-start", false); err != nil {
		t.Fatal(err)
	}
	seekTo(t, r, 35)
	if err := r.api.RunShortcut("passage-mark-end", false); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the passage saved", func() bool { stored, _ := r.projects.stored(t, movie); return len(stored.Passages) == 1 })
	stored, _ := r.projects.stored(t, movie)
	saved := stored.Passages[0]
	// Fitted to the lines: half a second of lead-in before the start, which
	// the name, in whole seconds, does not show.
	if saved.Name != ptbr.PassageName(20*time.Second, 35*time.Second) || saved.Start != 19_500 || saved.End != 35_000 {
		t.Fatalf("saved %+v", saved)
	}
	if saved.Plan == nil || saved.Plan.Loop || saved.Plan.Goal != 0 {
		t.Fatalf("saved plan %+v, want pausing at the end", saved.Plan)
	}
	state := r.api.PlaybackState()
	if !state.Passage.Active || len(state.SavedPassages) != 1 || state.SavedPassages[0].Name != ptbr.PassageName(20*time.Second, 35*time.Second) {
		t.Fatalf("panel: passage %+v, saved %+v", state.Passage, state.SavedPassages)
	}
	if notice := r.display.lastNotice(); notice != "Salvo: "+ptbr.PassageName(20*time.Second, 35*time.Second) {
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
	testkit.Eventually(t, "the second passage", func() bool { stored, _ := r.projects.stored(t, movie); return len(stored.Passages) == 2 })
	if stored, _ := r.projects.stored(t, movie); stored.Passages[1].Name != ptbr.PassageName(20*time.Second, 35*time.Second)+" (2)" {
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
	t.Parallel()
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
	testkit.Eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	_, before := r.projects.stored(t, movie)
	seekTo(t, r, 20)
	if err := r.api.RunShortcut("passage-mark-start", false); err != nil {
		t.Fatal(err)
	}
	seekTo(t, r, 35)
	if err := r.api.RunShortcut("passage-mark-end", false); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the refusal on the video", func() bool { return r.display.lastNotice() != "" })
	stored, after := r.projects.stored(t, movie)
	if len(stored.Passages) != project.MaxPassages || after != before {
		t.Fatalf("a full project took a passage: %d passages, %d saves (was %d)", len(stored.Passages), after, before)
	}
}

// hostWindow is the app's window for a player that draws into it; it keeps
// the session its controls drive.
type hostWindow struct {
	*recordingDisplay
	mu     sync.Mutex
	remote watch.Remote
}

func (w *hostWindow) Handle() uintptr            { return 1 }
func (w *hostWindow) Closed() <-chan struct{}    { return nil }
func (w *hostWindow) SetShortcuts(shortcut.Map)  {}
func (w *hostWindow) Attach(remote watch.Remote) { w.mu.Lock(); w.remote = remote; w.mu.Unlock() }

func (w *hostWindow) names() []string {
	w.mu.Lock()
	remote := w.remote
	w.mu.Unlock()
	if remote == nil {
		return nil
	}
	var names []string
	for _, saved := range remote.SavedPassages() {
		names = append(names, saved.Name)
	}
	return names
}

// The bar's menu lists what the app lists: the saved passages reach the
// session when the video opens, and follow every save, rename and delete.
func TestTheBarMenuListsTheSavedPassages(t *testing.T) {
	t.Parallel()
	window := &hostWindow{recordingDisplay: &recordingDisplay{}}
	r := newRigWith(t, func(d *app.Deps) {
		d.Ports.OpenWindow = func(context.Context, watch.WindowRequest) (watch.Window, error) { return window, nil }
	})
	r.plugin.drawsInHost = true
	kept := project.New()
	kept.Passages = []project.Passage{{Name: "Loja", Start: 10_000, End: 20_000}}
	if err := r.projects.Save(movie, kept); err != nil {
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
	same := func(want ...string) func() bool {
		return func() bool { return slices.Equal(window.names(), want) }
	}
	testkit.Eventually(t, "the kept passage in the menu", same("Loja"))

	seekTo(t, r, 30)
	if err := r.api.RunShortcut("passage-mark-start", false); err != nil {
		t.Fatal(err)
	}
	seekTo(t, r, 36)
	if err := r.api.RunShortcut("passage-mark-end", false); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the marked passage in the menu", same("Loja", ptbr.PassageName(30*time.Second, 36*time.Second)))
	if err := r.api.RenamePassage("Loja", "Na loja"); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the renamed passage in the menu", same("Na loja", ptbr.PassageName(30*time.Second, 36*time.Second)))
	if err := r.api.DeletePassage("Na loja"); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the deleted passage gone from the menu", same(ptbr.PassageName(30*time.Second, 36*time.Second)))
	if err := r.api.SavePassage("Rua"); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the saved passage in the menu", same(ptbr.PassageName(30*time.Second, 36*time.Second), "Rua"))
}
