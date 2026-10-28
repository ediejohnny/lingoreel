package app_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// A basic, a medium and a high computer.
var (
	schoolComputer = capacity.Machine{Processor: "Core 2 Duo", Processors: 2, Memory: 4 << 30}
	homeComputer   = capacity.Machine{Processor: "Core i3", Processors: 4, Memory: 4 << 30}
	studioComputer = capacity.Machine{Processor: "Core i7", Processors: 8, Memory: 24 << 30}
)

// computerRig is the app on a computer, with the configuration stored
// before it opens; firstRun opens it for the first time.
func computerRig(t *testing.T, machine capacity.Machine, stored settings.Config, firstRun bool) *rig {
	t.Helper()
	r := newRigWith(t, func(deps *app.Deps) {
		deps.FirstRun = firstRun
		deps.SystemLanguage = func() string { return "pt-BR" }
		deps.Machine = func() capacity.Machine { return machine }
	})
	r.settings.config = stored
	r.attach(t)
	return r
}

// storedConfig is the configuration as last saved.
func storedConfig(r *rig) settings.Config {
	r.settings.mu.Lock()
	defer r.settings.mu.Unlock()
	return r.settings.config
}

// adjustedTo is a configuration adjusted to a computer.
func adjustedTo(machine capacity.Machine) settings.Config {
	config := settings.DefaultConfig()
	config.AdjustTo(machine)
	return config
}

// The first run's answers record the computer: skipping takes its level's
// settings, and a choice apart from them counts as the person's.
func TestTheFirstRunRecordsTheComputer(t *testing.T) {
	t.Parallel()
	skipped := computerRig(t, schoolComputer, settings.DefaultConfig(), true)
	if err := skipped.api.SkipWelcome(); err != nil {
		t.Fatal(err)
	}
	if stored := storedConfig(skipped); stored.Voice.Model || stored.Computer.Level != capacity.LevelBasic ||
		stored.Computer.Identity != schoolComputer.Identity() {
		t.Fatalf("skipped on a school computer: voice AI %v, record %+v", stored.Voice.Model, stored.Computer)
	}

	risked := computerRig(t, schoolComputer, settings.DefaultConfig(), true)
	if view := risked.api.Welcome(); view.Computer.Level != string(capacity.LevelBasic) || view.VoiceModel {
		t.Fatalf("the welcome on a school computer: %+v, voice AI %v", view.Computer, view.VoiceModel)
	}
	choices := app.WelcomeChoices{Spoken: "pt-BR", Studied: "en", Mode: "on-demand", VoiceModel: true}
	if err := risked.api.FinishWelcome(choices); err != nil {
		t.Fatal(err)
	}
	if stored := storedConfig(risked); !stored.Voice.Model || stored.Computer.Level != capacity.LevelBasic {
		t.Fatalf("answered on a school computer: voice AI %v, record %+v", stored.Voice.Model, stored.Computer)
	}
}

// Someone who used the app before the levels, or who moved to another
// computer of the same level, is recorded without a question and keeps
// every setting.
func TestAComputerOfTheSameLevelIsRecordedQuietly(t *testing.T) {
	t.Parallel()
	before := settings.DefaultConfig()
	before.Voice.Model = false
	old := computerRig(t, studioComputer, before, false)
	testkit.Eventually(t, "the computer recorded", func() bool { return storedConfig(old).Computer.Adjusted() })
	if stored := storedConfig(old); stored.Voice.Model || stored.Computer.Level != capacity.LevelHigh {
		t.Fatalf("someone from before the levels: voice AI %v, record %+v", stored.Voice.Model, stored.Computer)
	}

	medium := adjustedTo(homeComputer)
	other := homeComputer
	other.Processor = "Ryzen 3"
	moved := computerRig(t, other, medium, false)
	testkit.Eventually(t, "the other computer recorded", func() bool {
		return storedConfig(moved).Computer.Identity == other.Identity()
	})
	if stored := storedConfig(moved); !stored.Voice.Model || stored.Computer.Level != capacity.LevelMedium {
		t.Fatalf("a computer of the same level changed a setting: voice AI %v, record %+v", stored.Voice.Model, stored.Computer)
	}
}

// As a game does, the app reads the computer on every start and, on one of
// another level, adjusts by itself, asking nothing: settings taken from a
// strong computer go down on a school one, and back up at home; what was
// chosen by hand stays.
func TestAComputerOfAnotherLevelIsAdjustedTo(t *testing.T) {
	t.Parallel()
	school := computerRig(t, schoolComputer, adjustedTo(studioComputer), false)
	testkit.Eventually(t, "the school computer recorded", func() bool {
		return storedConfig(school).Computer.Identity == schoolComputer.Identity()
	})
	stored := storedConfig(school)
	if stored.Voice.Model || stored.Computer.Level != capacity.LevelBasic {
		t.Fatalf("on a school computer: voice AI %v, record %+v", stored.Voice.Model, stored.Computer)
	}
	if got := settingsOf(t, school).General.Computer; got.Adjusted != string(capacity.LevelBasic) {
		t.Fatalf("the settings show the computer adjusted to %q, want the school computer's level", got.Adjusted)
	}

	home := computerRig(t, studioComputer, stored, false)
	testkit.Eventually(t, "back home", func() bool { return storedConfig(home).Computer.Identity == studioComputer.Identity() })
	if back := storedConfig(home); !back.Voice.Model || back.Computer.Level != capacity.LevelHigh {
		t.Fatalf("back on the strong computer: voice AI %v, record %+v", back.Voice.Model, back.Computer)
	}

	chosen := adjustedTo(studioComputer)
	chosen.Voice.Model = false
	r := computerRig(t, homeComputer, chosen, false)
	testkit.Eventually(t, "the home computer recorded", func() bool { return storedConfig(r).Computer.Level == capacity.LevelMedium })
	if storedConfig(r).Voice.Model {
		t.Fatal("the voice AI turned off by hand was turned on")
	}
}

// The settings show the computer as read, and their button puts every
// setting the level decides to the level's own, a choice made by hand too.
func TestTheSettingsAdjustToTheComputer(t *testing.T) {
	t.Parallel()
	chosen := adjustedTo(schoolComputer)
	chosen.Voice.Model = true
	r := computerRig(t, schoolComputer, chosen, false)
	want := app.ComputerView{Processor: "Core 2 Duo", Processors: 2, MemoryGiB: 4, Level: "basic", Adjusted: "basic"}
	if got := settingsOf(t, r).General.Computer; got != want {
		t.Fatalf("the settings show the computer as %+v, want %+v", got, want)
	}
	if err := r.api.AdjustToComputer(); err != nil {
		t.Fatal(err)
	}
	if stored := storedConfig(r); stored.Voice.Model || settingsOf(t, r).Voice.Model {
		t.Fatal("adjusting to the computer kept the voice AI turned on by hand")
	}
}
