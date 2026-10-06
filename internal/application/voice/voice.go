// Package voice brings the film's sound to the Modo Karaokê: it hears the
// player or decodes the film ahead, finds the voice, and keeps what it
// found by the film's clock. The system's own ways of hearing sound are
// adapters behind the ports here, so the rest is the same on every system.
package voice

import (
	"context"
	"time"
)

// Packet is a piece of the sound, at speech.SampleRate, interleaved when
// stereo.
type Packet struct {
	Samples  []float32
	Channels int
	// At is when the system took the first sample, on the wall clock: the
	// sound is heard a moment later, after the output's delay.
	At time.Time
}

// Tap hears what a player process plays, as it plays.
type Tap interface {
	// Listen hands every packet of the process's sound to hear, which
	// must return quickly, until ctx ends or the sound cannot be heard.
	Listen(ctx context.Context, pid int, hear func(Packet)) error
}
