package session

import (
	"time"

	"lingoreel/internal/domain/subtitle"
)

// Sync carries out the sync shortcuts: each line moves earlier or later by
// steps, faster while a shortcut is held. Windows repeats only the key
// pressed last, so one hold is tracked for every shortcut.
type Sync struct {
	subtitles *Subtitles
	now       func() time.Time
	nudger    *subtitle.Nudger
}

// NewSync creates the sync shortcuts of a session.
func NewSync(subtitles *Subtitles, steps subtitle.NudgeSteps, now func() time.Time) *Sync {
	return &Sync{subtitles: subtitles, now: now, nudger: subtitle.NewNudger(steps)}
}

// Nudge moves a line one step earlier or later. repeat tells that the
// shortcut is being held. It returns the line's new offset, or false when the
// line has no subtitles.
func (s *Sync) Nudge(line Line, later, repeat bool) (subtitle.Offset, bool) {
	if !knownLine(line) {
		return 0, false
	}
	move := s.nudger.Move(s.now(), repeat)
	if !later {
		move = -move
	}
	return s.subtitles.ShiftOffset(line, move)
}

// Reset puts a line back at the timing of its file.
func (s *Sync) Reset(line Line) {
	s.subtitles.SetOffset(line, 0)
}
