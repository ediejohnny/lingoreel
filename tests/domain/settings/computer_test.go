package settings_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
)

const gib = 1 << 30

// A basic, a medium and two high computers.
var (
	schoolPC  = capacity.Machine{Processor: "Core 2 Duo", Processors: 2, Memory: 4 * gib}
	homePC    = capacity.Machine{Processor: "Core i3", Processors: 4, Memory: 4 * gib}
	studioPC  = capacity.Machine{Processor: "Core i7", Processors: 8, Memory: 24 * gib}
	studioPC2 = capacity.Machine{Processor: "Ryzen 7", Processors: 16, Memory: 32 * gib}
)

// Adjusting to a computer, as the settings' button does, puts every setting
// its level decides to the level's own, also one changed by hand, and
// remembers the computer.
func TestAdjustingToAComputerTakesItsLevel(t *testing.T) {
	t.Parallel()
	config := settings.DefaultConfig()
	config.AdjustTo(schoolPC)
	if config.Voice.Model || config.Computer.Level != capacity.LevelBasic || config.Computer.Identity != schoolPC.Identity() {
		t.Fatalf("adjusted to a school computer: voice AI %v, record %+v", config.Voice.Model, config.Computer)
	}
	config.Voice.Model = true // turned on by hand, at one's own risk
	config.AdjustTo(schoolPC)
	if config.Voice.Model {
		t.Fatal("adjusting again kept a choice made by hand")
	}
	config.AdjustTo(studioPC)
	if !config.Voice.Model || config.Computer.Level != capacity.LevelHigh {
		t.Fatalf("adjusted to a strong computer: voice AI %v, level %q", config.Voice.Model, config.Computer.Level)
	}
}

// On another computer the person agreed to adjust to, what they set apart
// from the old level's own stays theirs, and the rest follows the new level.
func TestAnotherComputerKeepsWhatWasChosenByHand(t *testing.T) {
	t.Parallel()
	followed := settings.DefaultConfig()
	followed.AdjustTo(studioPC)
	followed.AdjustKeepingChoices(schoolPC)
	if followed.Voice.Model || followed.Computer.Level != capacity.LevelBasic {
		t.Fatalf("a setting left as the level had it did not follow the weaker computer: voice AI %v, level %q",
			followed.Voice.Model, followed.Computer.Level)
	}

	// Off by hand on a strong computer, where the level turns it on: a
	// medium one, which turns it on too, leaves it off.
	chosen := settings.DefaultConfig()
	chosen.AdjustTo(studioPC)
	chosen.Voice.Model = false
	chosen.AdjustKeepingChoices(homePC)
	if chosen.Voice.Model || chosen.Computer.Identity != homePC.Identity() || chosen.Computer.Level != capacity.LevelMedium {
		t.Fatalf("a choice made by hand was undone by a new computer: voice AI %v, record %+v", chosen.Voice.Model, chosen.Computer)
	}

	// Never adjusted, nothing tells the level's settings from the person's.
	unknown := settings.DefaultConfig()
	unknown.Voice.Model = false
	unknown.AdjustKeepingChoices(studioPC)
	if unknown.Voice.Model || unknown.Computer.Level != capacity.LevelHigh {
		t.Fatalf("settings never adjusted were taken for the level's: voice AI %v, level %q", unknown.Voice.Model, unknown.Computer.Level)
	}
}

// Recording a computer changes no setting: someone who already uses the app
// keeps every choice, and the ones apart from the level count as theirs.
func TestRecordingAComputerChangesNothing(t *testing.T) {
	t.Parallel()
	config := settings.DefaultConfig()
	config.Voice.Model = false
	config.RecordComputer(studioPC)
	if config.Voice.Model || config.Computer.Level != capacity.LevelHigh || config.Computer.Identity != studioPC.Identity() {
		t.Fatalf("recording changed a setting or missed the computer: voice AI %v, record %+v", config.Voice.Model, config.Computer)
	}
	config.AdjustKeepingChoices(homePC)
	if config.Voice.Model {
		t.Fatal("the choice kept when recording was not counted as the person's")
	}
}

// The record knows the computer it was made on, one the person declined to
// adjust to, and one nothing was read of; any other is news.
func TestTheRecordKnowsItsComputers(t *testing.T) {
	t.Parallel()
	config := settings.DefaultConfig()
	if config.Computer.Adjusted() || config.Computer.Knows(studioPC.Identity()) {
		t.Fatal("a configuration never adjusted to a computer passed for adjusted to one")
	}
	config.AdjustTo(studioPC)
	if !config.Computer.Adjusted() || !config.Computer.Knows(studioPC.Identity()) || !config.Computer.Knows(capacity.Identity{}) {
		t.Fatalf("the record does not know its own computer: %+v", config.Computer)
	}
	if config.Computer.Knows(schoolPC.Identity()) {
		t.Fatal("another computer passed for the one recorded")
	}
	config.DeclineComputer(schoolPC.Identity())
	if !config.Computer.Knows(schoolPC.Identity()) || config.Computer.Knows(studioPC2.Identity()) {
		t.Fatalf("the declined computer is asked about again, or a third one is not: %+v", config.Computer)
	}
	if !config.Computer.Knows(capacity.Identity{}) {
		t.Fatal("with a computer declined, one nothing was read of became news")
	}
}

// A record with a level that is none of the three, or a computer with fewer
// than no processors or memory, or a processor's name past the limit, is
// refused.
func TestABrokenComputerRecordIsRefused(t *testing.T) {
	t.Parallel()
	for _, record := range []settings.ComputerRecord{
		{Level: "ultra"},
		{Level: capacity.LevelHigh, Identity: capacity.Identity{Processors: -1}},
		{Level: capacity.LevelHigh, Declined: capacity.Identity{MemoryGiB: -4}},
		{Level: capacity.LevelHigh, Identity: capacity.Identity{Processor: strings.Repeat("é", capacity.MaxProcessorName+1)}},
	} {
		config := settings.DefaultConfig()
		config.Computer = record
		if err := config.Validate(); !errors.Is(err, settings.ErrInvalidSettings) {
			t.Errorf("record %+v: Validate() = %v, want ErrInvalidSettings", record, err)
		}
	}
	config := settings.DefaultConfig()
	config.AdjustTo(homePC)
	if err := config.Validate(); err != nil {
		t.Fatalf("an adjusted configuration is refused: %v", err)
	}
}
