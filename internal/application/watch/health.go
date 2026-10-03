package watch

import "lingoreel/internal/application/players"

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
