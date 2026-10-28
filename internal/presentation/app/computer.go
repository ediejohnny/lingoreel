package app

import "github.com/ediejohnny/lingoreel/internal/domain/settings"

// meetComputer reads the computer on every start, as a game does, once the
// settings are loaded. The first run's welcome records it; the computer
// recorded, or one nothing was read of, changes nothing. Any other is
// recorded, and when its level is another, every setting the level decides
// that is still as the old level had it moves to the new one; what was
// chosen by hand stays. Someone from before the levels is only recorded.
// Nothing is asked. Called without a.mu.
func (a *API) meetComputer() {
	machine := a.machine()
	id := machine.Identity()
	a.mu.Lock()
	welcoming, record := a.welcoming, a.config.Computer
	a.mu.Unlock()
	if welcoming || record.Knows(id) {
		return
	}
	if err := a.changeConfig(func(next *settings.Config) error {
		if !next.Computer.Knows(id) {
			next.AdjustKeepingChoices(machine)
		}
		return nil
	}); err != nil {
		a.logger.Warn("the computer was not recorded", "error", err)
		return
	}
	if record.Adjusted() && record.Level != machine.Level() {
		a.logger.Info("adjusted to another computer", "level", machine.Level(), "was", record.Level)
		a.applyVoice()
	}
}

// AdjustToComputer puts every setting the computer's level decides to the
// level's own, also the ones chosen by hand, as the settings' button asks.
func (a *API) AdjustToComputer() error {
	machine := a.machine()
	if err := a.changeConfig(func(next *settings.Config) error {
		next.AdjustTo(machine)
		return nil
	}); err != nil {
		return err
	}
	a.logger.Info("adjusted to the computer", "level", machine.Level())
	a.applyVoice()
	return nil
}

// computerView is the computer as read, with the level the settings were
// adjusted to.
func (a *API) computerView() ComputerView {
	machine := a.machine()
	id := machine.Identity()
	a.mu.Lock()
	adjusted := a.config.Computer.Level
	a.mu.Unlock()
	return ComputerView{
		Processor: id.Processor, Processors: id.Processors, MemoryGiB: id.MemoryGiB,
		Level: string(machine.Level()), Adjusted: string(adjusted),
	}
}
