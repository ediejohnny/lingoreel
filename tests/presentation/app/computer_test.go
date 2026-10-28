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

// screenAsks tells whether the last word the screens heard about a changed
// computer is a question still waiting.
func screenAsks(r *rig) bool {
	told, _ := r.ui.last(app.EventComputerChanged)
	change, _ := told.(app.ComputerChange)
	return change.Asked
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
	for _, r := range []*rig{old, moved} {
		if _, asked := r.ui.last(app.EventComputerChanged); asked || r.api.ComputerChange().Asked {
			t.Fatal("a computer of the same level was asked about")
		}
	}
}

// On a computer of another level the app asks before changing anything.
// Adjusting moves the level's settings and keeps the ones chosen by hand;
// declining keeps all of them and remembers the computer.
func TestAComputerOfAnotherLevelIsAskedAbout(t *testing.T) {
	t.Parallel()
	for _, adjust := range []bool{true, false} {
		r := computerRig(t, schoolComputer, adjustedTo(studioComputer), false)
		testkit.Eventually(t, "the question", func() bool { _, asked := r.ui.last(app.EventComputerChanged); return asked })
		change := r.api.ComputerChange()
		if !change.Asked || change.Computer.Level != string(capacity.LevelBasic) || change.Computer.Adjusted != string(capacity.LevelHigh) {
			t.Fatalf("the question: %+v", change)
		}
		if stored := storedConfig(r); !stored.Voice.Model || stored.Computer.Level != capacity.LevelHigh {
			t.Fatalf("a setting changed before the answer: voice AI %v, record %+v", stored.Voice.Model, stored.Computer)
		}
		if err := r.api.AnswerComputerChange(adjust); err != nil {
			t.Fatal(err)
		}
		stored := storedConfig(r)
		switch {
		case r.api.ComputerChange().Asked:
			t.Fatalf("adjust %v: the question still waits", adjust)
		case screenAsks(r):
			t.Fatalf("adjust %v: the screens were not told the question was answered", adjust)
		case adjust && (stored.Voice.Model || stored.Computer.Level != capacity.LevelBasic):
			t.Fatalf("adjusted: voice AI %v, record %+v", stored.Voice.Model, stored.Computer)
		case !adjust && (!stored.Voice.Model || stored.Computer.Level != capacity.LevelHigh ||
			stored.Computer.Declined != schoolComputer.Identity()):
			t.Fatalf("declined: voice AI %v, record %+v", stored.Voice.Model, stored.Computer)
		}
		// Answered, a second answer (a screen left open) changes nothing.
		if err := r.api.AnswerComputerChange(!adjust); err != nil {
			t.Fatal(err)
		}
		if again := storedConfig(r); again.Voice.Model != stored.Voice.Model || again.Computer != stored.Computer {
			t.Fatalf("adjust %v: a second answer changed the settings: %+v, was %+v", adjust, again.Computer, stored.Computer)
		}
	}

	// The settings' button answers the question too.
	button := computerRig(t, schoolComputer, adjustedTo(studioComputer), false)
	testkit.Eventually(t, "the question", func() bool { return button.api.ComputerChange().Asked })
	if err := button.api.AdjustToComputer(); err != nil || button.api.ComputerChange().Asked || screenAsks(button) {
		t.Fatalf("adjusted by the button (%v): the question still waits", err)
	}

	chosen := adjustedTo(studioComputer)
	chosen.Voice.Model = false
	r := computerRig(t, homeComputer, chosen, false)
	testkit.Eventually(t, "the question", func() bool { return r.api.ComputerChange().Asked })
	if err := r.api.AnswerComputerChange(true); err != nil {
		t.Fatal(err)
	}
	if stored := storedConfig(r); stored.Voice.Model || stored.Computer.Level != capacity.LevelMedium {
		t.Fatalf("the voice AI turned off by hand was turned on: %v, record %+v", stored.Voice.Model, stored.Computer)
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
