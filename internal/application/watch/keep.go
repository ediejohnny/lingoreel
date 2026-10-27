package watch

import (
	"context"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
)

// keepOnVoice reads the study line against the voice heard every
// voice.KeepEvery while the session runs, and shows the line where the voice
// puts it. Turned off, the line goes back to its own offset.
func (s *Session) keepOnVoice(ctx context.Context) {
	ticker := time.NewTicker(voice.KeepEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.keepTurn()
		}
	}
}

// keepTurn is one reading of the study line against the voice.
func (s *Session) keepTurn() {
	v := s.voice
	v.mu.Lock()
	wanted, follower := s.keepWanted(), v.follower
	v.mu.Unlock()
	if !wanted {
		if v.keeper.Forget() {
			s.lineMu.Lock()
			s.subtitles.SetVoiceShift(session.LinePrimary, 0)
			s.lineMu.Unlock()
		}
		return
	}
	var heard voice.HeardAround
	if follower != nil {
		heard = follower.Heard
	}
	now := s.now()
	// The track read and the shift set go together: a switch of the line
	// never takes the shift found for the file before it.
	s.lineMu.Lock()
	defer s.lineMu.Unlock()
	line := voice.KeptLine{Number: s.Tracks().Current.Primary}
	if track := s.subtitles.Track(session.LinePrimary); track != nil {
		line.Timeline, line.Offset = track.Timeline, track.Offset
	}
	if shift, changed := v.keeper.Turn(now, s.clock.PositionAt(now), s.clock.Seeks(), line, heard); changed {
		s.subtitles.SetVoiceShift(session.LinePrimary, shift)
	}
}
