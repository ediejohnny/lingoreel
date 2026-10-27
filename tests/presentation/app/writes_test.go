package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// A slow disk never holds the app: the video's project and the settings are
// written outside the app's lock, so the screens and the keys go on; the
// writes run one at a time and the newest is the one kept. Forgetting the video waits for a
// write on its way, and nothing writes the project back afterwards.
func TestASlowDiskHoldsNothing(t *testing.T) {
	t.Parallel()
	open := func(t *testing.T) *rig {
		t.Helper()
		r := newRig(t)
		kept := project.New()
		kept.Passages = []project.Passage{{Name: "Loja", Start: 10_000, End: 20_000}}
		if err := r.projects.Save(movie, kept); err != nil {
			t.Fatal(err)
		}
		r.attach(t)
		if _, err := r.api.OpenVideo(movie); err != nil {
			t.Fatal(err)
		}
		return r
	}
	within := func(t *testing.T, what string, run func()) {
		t.Helper()
		done := make(chan struct{})
		go func() {
			run()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s waited for the disk", what)
		}
	}

	t.Run("the newest is kept", func(t *testing.T) {
		t.Parallel()
		r := open(t)
		entered, release := r.projects.hold()
		first, second := make(chan error, 1), make(chan error, 1)
		go func() { first <- r.api.RenamePassage("Loja", "Rua") }()
		<-entered
		within(t, "the passages list", func() {
			if list := r.api.SavedPassages(); len(list) != 1 || list[0].Name != "Rua" {
				t.Errorf("passages while writing = %+v", list)
			}
		})
		go func() { second <- r.api.RenamePassage("Rua", "Praça") }()
		release()
		if err := <-first; err != nil {
			t.Fatal(err)
		}
		if err := <-second; err != nil {
			t.Fatal(err)
		}
		if kept, _ := r.projects.stored(t, movie); len(kept.Passages) != 1 || kept.Passages[0].Name != "Praça" {
			t.Fatalf("kept %+v, want the newest", kept.Passages)
		}
	})

	t.Run("a setting being written", func(t *testing.T) {
		t.Parallel()
		r := open(t)
		entered, release := r.settings.hold()
		changed := make(chan error, 1)
		go func() { changed <- r.api.SetTheme("dark") }()
		<-entered
		// The screens go on, with the theme kept as it was until it is written.
		within(t, "the preferences", func() {
			if theme := r.api.Preferences().Theme; theme != "system" {
				t.Errorf("theme while writing = %q", theme)
			}
		})
		release()
		if err := <-changed; err != nil || r.api.Preferences().Theme != "dark" {
			t.Fatalf("SetTheme = %v, theme %q", err, r.api.Preferences().Theme)
		}
	})

	t.Run("forgetting waits for the write", func(t *testing.T) {
		t.Parallel()
		r := open(t)
		entered, release := r.projects.hold()
		renamed := make(chan error, 1)
		go func() { renamed <- r.api.RenamePassage("Loja", "Rua") }()
		<-entered
		forgot := make(chan error, 1)
		go func() {
			_, err := r.api.ResetProject()
			forgot <- err
		}()
		release()
		if err := <-renamed; err != nil {
			t.Fatal(err)
		}
		if err := <-forgot; err != nil {
			t.Fatal(err)
		}
		if kept, _ := r.projects.stored(t, movie); len(kept.Passages) != 0 {
			t.Fatalf("the forgotten project came back with %+v", kept.Passages)
		}
	})
}

// A save that still fails as the app closes asks, with Windows' own box,
// whether to try again: trying again after making room keeps the changes,
// and cancelling closes without them.
func TestAFailedSaveAtClosingAsksToTryAgain(t *testing.T) {
	t.Parallel()
	for _, retry := range []bool{true, false} {
		t.Run(map[bool]string{true: "try again", false: "close anyway"}[retry], func(t *testing.T) {
			t.Parallel()
			var r *rig
			asked := 0
			r = newRigWith(t, func(d *app.Deps) {
				d.AskRetry = func(title, text string) bool {
					asked++
					if !strings.Contains(text, "Tentar de novo?") {
						t.Errorf("asked %q", text)
					}
					if retry {
						r.projects.mu.Lock()
						r.projects.saveErr = nil // room was made
						r.projects.mu.Unlock()
					}
					return retry
				}
			})
			r.attach(t)
			if _, err := r.api.OpenVideo(movie); err != nil {
				t.Fatal(err)
			}
			if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
				t.Fatal(err)
			}
			testkit.Eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
			r.projects.mu.Lock()
			r.projects.saveErr = errors.New("disk full")
			r.projects.mu.Unlock()
			onTheLine(t, r)
			if err := errors.Join(r.api.SelectLine("primary"), r.api.EditLine(app.EditRequest{Text: "Hello.", Start: "0:01", End: "0:02"})); err != nil {
				t.Fatal(err)
			}
			r.lifecycle.Shutdown()
			kept, _ := r.projects.stored(t, movie)
			if asked != 1 || (len(kept.Tracks) > 0) != retry {
				t.Fatalf("asked %d times, kept %+v", asked, kept.Tracks)
			}
		})
	}
}
