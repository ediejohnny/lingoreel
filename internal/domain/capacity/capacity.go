// Package capacity judges what a computer can take from its hardware alone,
// as a game sets itself up on its first run: a level whose heavy features
// start on, which the user may still change one by one. How busy the
// computer is at a moment says nothing of it (the user may be playing a game
// on another screen); the heavy features themselves step back while it is.
package capacity

import "strings"

// The thresholds of a weak computer: below them the player, the window and
// the voice AI would crowd each other while a film plays.
const (
	// MaxWeakProcessors: two logical processors or fewer leave the player
	// and the window no room.
	MaxWeakProcessors = 2
	// MinMemory is the memory below which Windows, the player and the app
	// already crowd each other: 4 GiB.
	MinMemory = 4 << 30
)

// The thresholds of a strong computer, from the voice alignment's spike:
// aligning a line's words in a language other than English (a 355 MB model)
// took 0.44 of real time on two processors, so it needs two processors to
// spare beside the film and room for the model.
const (
	// MinHighProcessors is how many logical processors a strong computer has.
	MinHighProcessors = 4
	// MinHighMemory is how much memory a strong computer has: 8 GiB.
	MinHighMemory = 8 << 30
)

// MaxProcessorName bounds a processor's name, in characters: Windows' own
// are under 64.
const MaxProcessorName = 128

// ProcessorName is a processor's name as the system tells it, without the
// spaces some processors pad it with, cut to MaxProcessorName characters.
func ProcessorName(told string) string {
	name := strings.TrimSpace(told)
	if runes := []rune(name); len(runes) > MaxProcessorName {
		name = strings.TrimSpace(string(runes[:MaxProcessorName]))
	}
	return name
}

// Machine is the computer's hardware. A zero field was not read and counts
// for nothing.
type Machine struct {
	// Processor is the processor's name, as Windows tells it.
	Processor string
	// Processors are the logical processors the app may use.
	Processors int
	// Memory is the physical memory, in bytes.
	Memory uint64
}

// Weak tells the computer would labour under the voice AI.
func (m Machine) Weak() bool {
	return (m.Processors > 0 && m.Processors <= MaxWeakProcessors) || (m.Memory > 0 && m.Memory < MinMemory)
}

// Level is the computer's level: basic when it is weak, high with the
// processors and the memory of a strong one, medium otherwise (also when
// they were not read).
func (m Machine) Level() Level {
	switch {
	case m.Weak():
		return LevelBasic
	case m.Processors >= MinHighProcessors && m.Memory >= MinHighMemory:
		return LevelHigh
	}
	return LevelMedium
}

// Identity is what tells the computer from another between runs.
func (m Machine) Identity() Identity {
	return Identity{Processor: m.Processor, Processors: m.Processors, MemoryGiB: int((m.Memory + 1<<29) >> 30)}
}

// Level is how much a computer can take.
type Level string

const (
	// LevelBasic leaves the voice AI, and everything heavier, off.
	LevelBasic Level = "basic"
	// LevelMedium turns the voice AI on.
	LevelMedium Level = "medium"
	// LevelHigh turns the heaviest features on too.
	LevelHigh Level = "high"
)

// Valid tells the level is one of the three.
func (l Level) Valid() bool { return l == LevelBasic || l == LevelMedium || l == LevelHigh }

// VoiceAI tells whether the level starts with the voice AI on.
func (l Level) VoiceAI() bool { return l == LevelMedium || l == LevelHigh }

// SuitsSpeechPack tells whether a language's speech pack suits the level:
// none at the basic one; English's small model at the medium one, since the
// other languages' large models take over twice its time; every one at the
// high level.
func (l Level) SuitsSpeechPack(language string) bool {
	switch l {
	case LevelHigh:
		return true
	case LevelMedium:
		return language == "en"
	}
	return false
}

// Identity is a computer as it is told from another: its processor's name,
// its logical processors and its memory in whole GiB, the nearest one
// (Windows reports a little under what is installed).
type Identity struct {
	Processor  string `json:"processor"`
	Processors int    `json:"processors"`
	MemoryGiB  int    `json:"memoryGiB"`
}

// Known tells something of the computer was read.
func (i Identity) Known() bool { return i.Processor != "" || i.Processors > 0 || i.MemoryGiB > 0 }
