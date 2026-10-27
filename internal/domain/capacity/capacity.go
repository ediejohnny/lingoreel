// Package capacity judges whether a computer can afford the voice AI: a
// weak one starts with it off, and the user may still turn it on.
package capacity

// The starting thresholds of a weak computer, to be calibrated on real
// ones: below them the voice AI would take more than its share of one core
// (about 5%) while a film plays.
const (
	// MaxWeakProcessors: two logical processors or fewer leave the player
	// and the window no room.
	MaxWeakProcessors = 2
	// MinMemory is the memory below which Windows, the player and the app
	// already crowd each other: 4 GiB.
	MinMemory = 4 << 30
	// MinModelSpeed is how many seconds of sound the model must judge in
	// one second of work.
	MinModelSpeed = 25
)

// Machine is what a computer offers the voice AI. A zero field was not
// measured and counts for nothing.
type Machine struct {
	// Processors are the logical processors the app may use.
	Processors int
	// Memory is the physical memory, in bytes.
	Memory uint64
	// ModelSpeed is how many seconds of sound the voice model judged in one
	// second of work.
	ModelSpeed float64
}

// Weak tells the computer would labour under the voice AI.
func (m Machine) Weak() bool {
	return (m.Processors > 0 && m.Processors <= MaxWeakProcessors) ||
		(m.Memory > 0 && m.Memory < MinMemory) ||
		(m.ModelSpeed > 0 && m.ModelSpeed < MinModelSpeed)
}
