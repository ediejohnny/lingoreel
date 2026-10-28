package app

import "github.com/ediejohnny/lingoreel/internal/domain/settings"

// EventComputerChanged tells the screens the app runs on another computer,
// of another level than the one the settings were adjusted to: they ask
// whether to adjust again (ComputerChange, AnswerComputerChange).
const EventComputerChanged = "computer:changed"

// meetComputer compares the computer with the one the settings were adjusted
// to, once the settings are loaded. The first run's welcome records it. One
// already known, or one never adjusted to (someone from before the levels),
// or another of the same level, is recorded and nothing changes; another of
// another level waits for the person's answer. Called without a.mu.
func (a *API) meetComputer() {
	machine := a.machine()
	id := machine.Identity()
	a.mu.Lock()
	welcoming, record := a.welcoming, a.config.Computer
	known := welcoming || record.Knows(id)
	changed := !known && record.Adjusted() && record.Level != machine.Level()
	if changed {
		a.computerChanged = true
	}
	a.mu.Unlock()
	switch {
	case known:
	case changed:
		a.logger.Info("another computer", "level", machine.Level(), "adjustedTo", record.Level)
		a.emit(EventComputerChanged, a.ComputerChange())
	default:
		if err := a.changeConfig(func(next *settings.Config) error {
			if !next.Computer.Knows(id) {
				next.RecordComputer(machine)
			}
			return nil
		}); err != nil {
			a.logger.Warn("the computer was not recorded", "error", err)
		}
	}
}

// ComputerChange is the question asked when the app runs on a computer of
// another level, with the computer; Asked is false when none waits.
func (a *API) ComputerChange() ComputerChange {
	view := a.computerView()
	a.mu.Lock()
	defer a.mu.Unlock()
	return ComputerChange{Asked: a.computerChanged, Computer: view}
}

// AnswerComputerChange answers the question of the computer that changed:
// adjusting moves to its level what still is as the old level had it,
// keeping what was chosen by hand; declining keeps every setting and the
// computer is not asked about again. With no question waiting it does
// nothing.
func (a *API) AnswerComputerChange(adjust bool) error {
	machine := a.machine()
	a.mu.Lock()
	asked := a.computerChanged
	a.mu.Unlock()
	if !asked {
		return nil
	}
	if err := a.changeConfig(func(next *settings.Config) error {
		if adjust {
			next.AdjustKeepingChoices(machine)
		} else {
			next.DeclineComputer(machine.Identity())
		}
		return nil
	}); err != nil {
		return err
	}
	a.mu.Lock()
	a.computerChanged = false
	a.mu.Unlock()
	a.logger.Info("computer change answered", "adjusted", adjust, "level", machine.Level())
	a.applyVoice()
	return nil
}

// AdjustToComputer puts every setting the computer's level decides to the
// level's own, also the ones chosen by hand, as the settings' button asks;
// a question of a changed computer is answered by it.
func (a *API) AdjustToComputer() error {
	machine := a.machine()
	if err := a.changeConfig(func(next *settings.Config) error {
		next.AdjustTo(machine)
		return nil
	}); err != nil {
		return err
	}
	a.mu.Lock()
	a.computerChanged = false
	a.mu.Unlock()
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
