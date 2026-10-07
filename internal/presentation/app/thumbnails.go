package app

import (
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// StillsCache is the seek bar's stills kept in the app's folder.
type StillsCache interface {
	// Size is how much they take on the disk.
	Size() (int64, error)
	// Clear takes them all off the disk and tells how much it freed.
	Clear() (int64, error)
	// Crowded tells their disk is nearly full: new stills stay in memory.
	Crowded() bool
}

var errStillsNotCleared = worded(func() string { return ptbr.T("error.thumbnails.notCleared") })

// thumbnailSetup is how a session takes the seek bar's stills, from the
// settings.
func thumbnailSetup(t settings.ThumbnailPreferences) watch.ThumbnailSetup {
	return watch.ThumbnailSetup{Off: !t.On, OnBattery: t.OnBattery, InMemory: !t.OnDisk}
}

// thumbnailsView is the settings screen's part on the seek bar's stills: the
// choices and how much the kept ones take now.
func (a *API) thumbnailsView(t settings.ThumbnailPreferences) ThumbnailSettings {
	view := ThumbnailSettings{
		On: t.On, OnBattery: t.OnBattery, OnDisk: t.OnDisk,
		Battery: a.deps.Ports.Power != nil && a.deps.Ports.Power.HasBattery(),
	}
	if a.deps.Stills != nil {
		if size, err := a.deps.Stills.Size(); err == nil {
			view.KeptBytes = size
		}
		view.Crowded = a.deps.Stills.Crowded()
	}
	return view
}

// SetThumbnails changes how the seek bar's stills are taken, at once and
// from then on.
func (a *API) SetThumbnails(view ThumbnailSettings) error {
	a.mu.Lock()
	next := a.config
	next.Thumbnails = settings.ThumbnailPreferences{On: view.On, OnBattery: view.OnBattery, OnDisk: view.OnDisk}
	err := a.commitConfig(next)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	a.applyThumbnails()
	return nil
}

// ClearThumbnails takes the kept stills off the disk; a video playing keeps
// its own in memory and may keep them anew.
func (a *API) ClearThumbnails() error {
	if a.deps.Stills == nil {
		return nil
	}
	freed, err := a.deps.Stills.Clear()
	if err != nil {
		a.logger.Warn("the seek bar's stills were not cleared", "error", err)
		return a.because(errStillsNotCleared, err)
	}
	a.logger.Info("the seek bar's stills were cleared", "bytes", freed)
	return nil
}

// applyThumbnails gives the video being watched the stills' settings.
func (a *API) applyThumbnails() {
	current := a.current()
	if current == nil || current.session == nil {
		return
	}
	a.mu.Lock()
	setup := thumbnailSetup(a.config.Thumbnails)
	a.mu.Unlock()
	current.session.SetThumbnailSetup(setup)
}
