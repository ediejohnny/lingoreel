package app_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// keptStills is the stills kept on the disk: how much they take, clearing
// them, which fails while fail is set, and whether their disk is crowded.
type keptStills struct {
	mu      sync.Mutex
	size    int64
	fail    error
	crowded bool
}

func (s *keptStills) Crowded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.crowded
}

func (s *keptStills) Size() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.size, nil
}

func (s *keptStills) Clear() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return 0, s.fail
	}
	freed := s.size
	s.size = 0
	return freed, nil
}

// The stills' settings are the computer's: on and kept on the disk at first,
// kept at once, telling how much the kept stills take; clearing them frees
// it, or says in words why it could not.
func TestTheThumbnailSettings(t *testing.T) {
	t.Parallel()
	stills := &keptStills{size: 12_345_678}
	r := newRigWith(t, func(deps *app.Deps) { deps.Stills = stills })
	r.attach(t)
	view := settingsOf(t, r).Thumbnails
	if !view.On || view.OnBattery || !view.OnDisk || view.KeptBytes != 12_345_678 || view.Crowded {
		t.Fatalf("first thumbnail settings = %+v", view)
	}
	stills.mu.Lock()
	stills.crowded = true
	stills.mu.Unlock()
	if !settingsOf(t, r).Thumbnails.Crowded {
		t.Fatal("a crowded disk was not told")
	}
	view.OnBattery, view.OnDisk = true, false
	if err := r.api.SetThumbnails(view); err != nil {
		t.Fatal(err)
	}
	if kept := r.settings.config.Thumbnails; !kept.On || !kept.OnBattery || kept.OnDisk {
		t.Fatalf("kept thumbnail settings = %+v", kept)
	}
	if err := r.api.ClearThumbnails(); err != nil || settingsOf(t, r).Thumbnails.KeptBytes != 0 {
		t.Fatalf("ClearThumbnails = %v, %d bytes left", err, settingsOf(t, r).Thumbnails.KeptBytes)
	}
	stills.size, stills.fail = 1000, errors.New("Access is denied.")
	if err := r.api.ClearThumbnails(); err == nil || !strings.Contains(err.Error(), "miniaturas") {
		t.Fatalf("a clearing that failed = %v", err)
	}
}

// Turned off, a video's stills are never taken; turned on while it plays,
// they start at once.
func TestTheThumbnailSettingsReachTheVideo(t *testing.T) {
	t.Parallel()
	window := &hostWindow{recordingDisplay: &recordingDisplay{}}
	var opens atomic.Int32
	r := newRigWith(t, func(d *app.Deps) {
		d.Ports.OpenWindow = func(context.Context, watch.WindowRequest) (watch.Window, error) { return window, nil }
		d.Ports.OpenStills = func(context.Context, string, string) (thumbnail.Film, error) {
			opens.Add(1)
			return nil, thumbnail.ErrNoStills
		}
	})
	r.plugin.drawsInHost = true
	r.attach(t)
	off := settingsOf(t, r).Thumbnails
	off.On = false
	if err := r.api.SetThumbnails(off); err != nil {
		t.Fatal(err)
	}
	if _, err := r.api.OpenVideo(movie); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.api.StopWatching() })
	time.Sleep(300 * time.Millisecond)
	if opens.Load() != 0 {
		t.Fatal("stills were taken with the thumbnails off")
	}
	off.On = true
	if err := r.api.SetThumbnails(off); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the stills to start", func() bool { return opens.Load() > 0 })
}
