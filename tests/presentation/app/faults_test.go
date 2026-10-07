package app_test

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/diskfault"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// failed is a write the system refused with code.
func failed(code error) error {
	return fmt.Errorf("saving: %w", &fs.PathError{Op: "write", Path: `C:\Users\x\AppData\Roaming\LingoReel\config.json`, Err: code})
}

// A write the system refused says why and what to do, after what failed: a
// full disk for the settings, a file in use for the kept stills. One the
// system told nothing useful about keeps its message alone.
func TestFailedWritesSayWhyAndWhatToDo(t *testing.T) {
	t.Parallel()
	stills := &keptStills{size: 1000}
	r := newRigWith(t, func(d *app.Deps) {
		d.Faults = diskfault.Of
		d.Stills = stills
	})
	r.attach(t)

	r.settings.mu.Lock()
	r.settings.saveError = failed(windows.ERROR_DISK_FULL)
	r.settings.mu.Unlock()
	err := r.api.SetSyncNotice(false)
	if want := ptbr.T("error.settings.notSaved") + " " + ptbr.T("error.disk.full"); err == nil || err.Error() != want {
		t.Fatalf("settings on a full disk = %v, want %q", err, want)
	}
	r.settings.mu.Lock()
	r.settings.saveError = errors.New("something else")
	r.settings.mu.Unlock()
	if err := r.api.SetSyncNotice(false); err == nil || err.Error() != ptbr.T("error.settings.notSaved") {
		t.Fatalf("settings refused for no known reason = %v", err)
	}

	stills.mu.Lock()
	stills.fail = failed(windows.ERROR_SHARING_VIOLATION)
	stills.mu.Unlock()
	err = r.api.ClearThumbnails()
	if want := ptbr.T("error.thumbnails.notCleared") + " " + ptbr.T("error.disk.inUse"); err == nil || err.Error() != want {
		t.Fatalf("clearing stills in use = %v, want %q", err, want)
	}
}

// A session that cannot start because the disk is full says so, not that
// the player was slow; a player that does not start says it is the player.
// A karaoke mode choice that cannot be kept says why, through the profile's
// own failure.
func TestASessionAndItsChoicesSayWhyTheyFailed(t *testing.T) {
	t.Parallel()
	r := newRigWith(t, func(d *app.Deps) { d.Faults = diskfault.Of })
	r.attach(t)
	openMovie(t, r)
	watch := func() error {
		return r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5})
	}
	r.plugin.mu.Lock()
	r.plugin.launchErr = failed(windows.ERROR_DISK_FULL)
	r.plugin.mu.Unlock()
	if err, want := watch(), ptbr.T("error.start.failed")+" "+ptbr.T("error.disk.full"); err == nil || err.Error() != want {
		t.Fatalf("a start on a full disk = %v, want %q", err, want)
	}
	r.plugin.mu.Lock()
	r.plugin.launchErr = errors.New("no answer")
	r.plugin.mu.Unlock()
	if err := watch(); err == nil || strings.Contains(err.Error(), ptbr.T("error.disk.full")) {
		t.Fatalf("a player that did not start = %v", err)
	}

	r.plugin.mu.Lock()
	r.plugin.launchErr = nil
	r.plugin.mu.Unlock()
	if err := watch(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	testkit.Eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	r.settings.mu.Lock()
	r.settings.profilesError = failed(windows.ERROR_DISK_FULL)
	r.settings.mu.Unlock()
	err := r.api.SetKaraokeIndicator("countdown")
	if want := ptbr.T("error.choiceNotSaved") + " " + ptbr.T("error.disk.full"); err == nil || err.Error() != want {
		t.Fatalf("a karaoke choice on a full disk = %v, want %q", err, want)
	}
}
