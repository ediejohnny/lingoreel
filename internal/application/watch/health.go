package watch

import (
	"lingoreel/internal/application/players"
	"lingoreel/internal/application/session"
)

// Health is how the player is doing during a session.
type Health int

const (
	// HealthGood is a player that answers.
	HealthGood Health = iota
	// HealthReconnecting is a player that stopped answering and is being
	// tried again.
	HealthReconnecting
	// HealthStalled is a player that says it plays but whose position does
	// not move; the subtitles wait for it.
	HealthStalled
)

// Notify shows a short notice on the video, such as the player's health;
// a lost notice is cosmetic.
func (s *Session) Notify(text string) {
	if s.display != nil && text != "" {
		_ = s.display.Notify(text)
	}
}

// Shown is what each line shows at the current position, for diagnostics.
func (s *Session) Shown() session.Visible {
	return s.subtitles.At(s.clock.Position())
}

// HasDisplay tells the subtitles have somewhere to be drawn.
func (s *Session) HasDisplay() bool {
	return s.display != nil
}

// Health reports the player's state for the panel.
func (s *Session) Health() Health {
	if r, ok := s.running.(players.Reconnector); ok && r.Reconnecting() {
		return HealthReconnecting
	}
	if s.clock.Stalled() {
		return HealthStalled
	}
	return HealthGood
}
