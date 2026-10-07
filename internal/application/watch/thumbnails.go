package watch

import (
	"context"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

// A video in the app's own window gets stills for its seek bar: taken in the
// background while it plays, the moment under the pointer first, until the
// session ends (which never waits for them).

// ThumbnailSetup is how the seek bar's stills are taken, from the settings;
// the zero value takes them, keeps them on the disk, and on the battery
// takes only the pointer's.
type ThumbnailSetup struct {
	// Off takes none: the bar shows the time alone.
	Off bool
	// OnBattery takes them in the background on the battery too.
	OnBattery bool
	// InMemory keeps them for the session only, never reading or writing
	// the disk.
	InMemory bool
}

// startStills begins the video's stills when the app's window shows it and
// stills can be taken; the film's FFmpeg is the one chosen for the voice.
func (s *Session) startStills(ports Ports, request Request, info media.Info) {
	if ports.OpenStills == nil {
		return
	}
	chosen := request.Voice.FFmpegPath
	open := func(ctx context.Context, file string) (thumbnail.Film, error) {
		return ports.OpenStills(ctx, chosen, file)
	}
	duration := func() time.Duration {
		if info.Duration > 0 {
			return info.Duration
		}
		return s.State().Duration // MP4, AVI: the player tells
	}
	s.onBattery.Store(ports.Power != nil && ports.Power.OnBattery())
	s.newStills = func(setup ThumbnailSetup) *thumbnail.Service {
		if setup.Off {
			return nil
		}
		options := thumbnail.Options{OnStop: ports.StillsStopped, Warn: ports.StillsUnkept}
		if !setup.InMemory {
			options.Store = ports.KeepStills
		}
		stills := thumbnail.Start(request.Analysis.Path, open, duration, options)
		stills.Rest(s.onBattery.Load() && !setup.OnBattery)
		return stills
	}
	s.stillsSetup = request.Thumbnails
	s.stills = s.newStills(request.Thumbnails)
}

// SetThumbnailSetup changes how the stills are taken, at once: turned off
// they end, turned on (or moved between the disk and memory) they start
// again, and on the battery they rest or not.
func (s *Session) SetThumbnailSetup(setup ThumbnailSetup) {
	s.stillsMu.Lock()
	if s.newStills == nil {
		s.stillsMu.Unlock()
		return
	}
	before, stills := s.stillsSetup, s.stills
	s.stillsSetup = setup
	if setup.Off != before.Off || setup.InMemory != before.InMemory {
		s.stills = s.newStills(setup)
		s.stillsMu.Unlock()
		stills.Close()
		return
	}
	s.stillsMu.Unlock()
	stills.Rest(s.onBattery.Load() && !setup.OnBattery)
}

// restStills follows the computer going on and off its battery.
func (s *Session) restStills(onBattery bool) {
	s.onBattery.Store(onBattery)
	s.stillsMu.Lock()
	stills, setup := s.stills, s.stillsSetup
	s.stillsMu.Unlock()
	stills.Rest(onBattery && !setup.OnBattery)
}

// endStills ends the stills with the session: none start after.
func (s *Session) endStills() {
	s.stillsMu.Lock()
	stills := s.stills
	s.stills, s.newStills = nil, nil
	s.stillsMu.Unlock()
	stills.Close()
}

func (s *Session) currentStills() *thumbnail.Service {
	s.stillsMu.Lock()
	defer s.stillsMu.Unlock()
	return s.stills
}

// Thumbnail is what the seek bar shows for a moment under the pointer.
func (s *Session) Thumbnail(at time.Duration) thumbnail.View { return s.currentStills().Thumbnail(at) }

// WantThumbnail tells the moment the pointer is on, whose still goes first.
func (s *Session) WantThumbnail(at time.Duration) { s.currentStills().Want(at) }
