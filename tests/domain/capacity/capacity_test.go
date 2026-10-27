package capacity_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
)

const gib = 1 << 30

// A computer is weak when its processors or its memory fall under their
// threshold; a part that could not be read (zero) counts for nothing.
func TestAWeakComputerIsTold(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		machine capacity.Machine
		weak    bool
	}{
		{"strong", capacity.Machine{Processors: 8, Memory: 24 * gib}, false},
		{"nothing read", capacity.Machine{}, false},
		{"two processors", capacity.Machine{Processors: 2, Memory: 24 * gib}, true},
		{"three processors", capacity.Machine{Processors: 3, Memory: 24 * gib}, false},
		{"just under 4 GiB", capacity.Machine{Processors: 8, Memory: 4*gib - 1}, true},
		{"4 GiB", capacity.Machine{Processors: 8, Memory: 4 * gib}, false},
		{"only the memory read, little", capacity.Machine{Memory: 2 * gib}, true},
	} {
		if got := c.machine.Weak(); got != c.weak {
			t.Errorf("%s: Weak() = %v, want %v", c.name, got, c.weak)
		}
	}
}

// A computer's level comes from its hardware alone: basic when it is weak,
// high with four logical processors and 8 GiB, medium otherwise; only the
// medium and high levels turn the voice AI on.
func TestAComputersLevelIsTold(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		machine capacity.Machine
		level   capacity.Level
	}{
		{"weak", capacity.Machine{Processors: 2, Memory: 24 * gib}, capacity.LevelBasic},
		{"nothing read", capacity.Machine{}, capacity.LevelMedium},
		{"three processors", capacity.Machine{Processors: 3, Memory: 24 * gib}, capacity.LevelMedium},
		{"four processors, 8 GiB", capacity.Machine{Processors: 4, Memory: 8 * gib}, capacity.LevelHigh},
		{"four processors, just under 8 GiB", capacity.Machine{Processors: 4, Memory: 8*gib - 1}, capacity.LevelMedium},
		{"many processors, little memory", capacity.Machine{Processors: 16, Memory: 3 * gib}, capacity.LevelBasic},
		{"many processors, the memory unread", capacity.Machine{Processors: 16}, capacity.LevelMedium},
	} {
		if got := c.machine.Level(); got != c.level {
			t.Errorf("%s: Level() = %q, want %q", c.name, got, c.level)
		}
	}
	for level, on := range map[capacity.Level]bool{capacity.LevelBasic: false, capacity.LevelMedium: true, capacity.LevelHigh: true} {
		if level.VoiceAI() != on || !level.Valid() {
			t.Errorf("%q: VoiceAI() = %v, Valid() = %v; want %v, true", level, level.VoiceAI(), level.Valid(), on)
		}
	}
	if capacity.Level("ultra").Valid() || capacity.Level("").Valid() {
		t.Error("a level that is none of the three passed for valid")
	}
}

// A computer is told from another by its processor's name, its logical
// processors and its memory in whole GiB, the nearest one (Windows reports a
// little under what is installed).
func TestAComputerIsToldByItsHardware(t *testing.T) {
	t.Parallel()
	here := capacity.Machine{Processor: "Example(R) Quad Core CPU @ 3.20GHz", Processors: 8, Memory: 24*gib - 150<<20}
	if got := here.Identity(); got.MemoryGiB != 24 || got.Processors != 8 || got.Processor != here.Processor || !got.Known() {
		t.Fatalf("Identity() = %+v, want 24 GiB, 8 processors and the processor's name", got)
	}
	more := here
	more.Memory = 32 * gib
	if here.Identity() == more.Identity() {
		t.Fatal("more memory went unnoticed")
	}
	if (capacity.Machine{}).Identity().Known() {
		t.Fatal("a computer nothing was read of passed for known")
	}
}

// A processor's name as the system tells it loses its padding, and one past
// the limit is cut, by characters, to fit.
func TestAProcessorsNameIsKeptWithinItsLimit(t *testing.T) {
	t.Parallel()
	if got := capacity.ProcessorName("  Core 2 Duo   "); got != "Core 2 Duo" {
		t.Fatalf("ProcessorName() = %q, want the name without its padding", got)
	}
	long := capacity.ProcessorName(strings.Repeat("é", capacity.MaxProcessorName+40))
	if !utf8.ValidString(long) || utf8.RuneCountInString(long) != capacity.MaxProcessorName {
		t.Fatalf("a long name kept %d characters (valid UTF-8: %v), want %d",
			utf8.RuneCountInString(long), utf8.ValidString(long), capacity.MaxProcessorName)
	}
}
