package settings

import (
	"fmt"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
)

// ComputerRecord is the computer the settings were last adjusted to, with
// its level, and one the person declined to adjust to. A setting the level
// decides that differs from what the recorded level puts there was chosen
// by hand.
type ComputerRecord struct {
	Level    capacity.Level    `json:"level"`
	Identity capacity.Identity `json:"identity"`
	Declined capacity.Identity `json:"declined"`
}

// Adjusted tells the settings were adjusted to a computer.
func (r ComputerRecord) Adjusted() bool { return r.Level != "" }

// Knows tells there is nothing to ask about a computer: it is the one
// recorded, the one declined, or one nothing was read of.
func (r ComputerRecord) Knows(id capacity.Identity) bool {
	return !id.Known() || id == r.Identity || id == r.Declined
}

func (r ComputerRecord) validate() error {
	if r.Level != "" && !r.Level.Valid() {
		return fmt.Errorf("%w: computer level %q", ErrInvalidSettings, r.Level)
	}
	for _, id := range []capacity.Identity{r.Identity, r.Declined} {
		if id.Processors < 0 || id.MemoryGiB < 0 || utf8.RuneCountInString(id.Processor) > capacity.MaxProcessorName {
			return fmt.Errorf("%w: computer %+v", ErrInvalidSettings, id)
		}
	}
	return nil
}

// levelled are the settings a computer's level decides: where each one is,
// and what a level puts there.
var levelled = []struct {
	setting func(c *Config) *bool
	of      func(l capacity.Level) bool
}{
	{func(c *Config) *bool { return &c.Voice.Model }, capacity.Level.VoiceAI},
	{func(c *Config) *bool { return &c.Voice.Sync }, capacity.Level.VoiceAI},
}

// AdjustTo puts every setting the computer's level decides to the level's
// own, also one chosen by hand, and records the computer.
func (c *Config) AdjustTo(m capacity.Machine) {
	level := m.Level()
	for _, s := range levelled {
		*s.setting(c) = s.of(level)
	}
	c.RecordComputer(m)
}

// AdjustKeepingChoices moves to the computer's level the settings still as
// the recorded level has them; the ones chosen by hand stay. With no level
// recorded, every setting counts as chosen.
func (c *Config) AdjustKeepingChoices(m capacity.Machine) {
	if old := c.Computer.Level; old.Valid() {
		level := m.Level()
		for _, s := range levelled {
			if setting := s.setting(c); *setting == s.of(old) {
				*setting = s.of(level)
			}
		}
	}
	c.RecordComputer(m)
}

// RecordComputer records the computer and its level and changes no setting:
// the ones apart from the level count as chosen by hand.
func (c *Config) RecordComputer(m capacity.Machine) {
	c.Computer.Level, c.Computer.Identity = m.Level(), m.Identity()
}

// DeclineComputer remembers the person declined to adjust to a computer, so
// it is not asked about again.
func (c *Config) DeclineComputer(id capacity.Identity) { c.Computer.Declined = id }
