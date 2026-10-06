// Package voice brings the film's sound to the karaoke mode: it hears the
// player or decodes the film ahead, finds the voice, and keeps what it
// found by the film's clock. The system's own ways of hearing sound are
// adapters behind the ports here, so the rest is the same on every system.
package voice

import (
	"context"
	"time"
)

// Packet is a piece of the sound, at speech.SampleRate, interleaved when
// stereo. Its samples are only good during the call that hands it over.
type Packet struct {
	Samples  []float32
	Channels int
	// System says the sound is everything the speakers play, not only the
	// player's: Windows could not hear the player alone.
	System bool
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

// Stretch is part of a film's voice to decode ahead of the video.
type Stretch struct {
	File string
	// Track is the voice's position among the film's audio tracks
	// (media.VoiceTrack); Channels and Codec are the track's, so a decoder
	// can take the centre of a surround mix, where the dialogue is.
	Track    int
	Channels int
	Codec    string
	From     time.Duration
	Length   time.Duration
}

// Decoder decodes a film's sound ahead of the video: the user's FFmpeg.
type Decoder interface {
	// Decode hands the stretch's sound to hear in pieces, at
	// speech.SampleRate from the stretch's start, until it ends, ctx ends or
	// the decoding fails. The pieces are only good during the call.
	Decode(ctx context.Context, stretch Stretch, hear func(samples []float32, channels int)) error
}

// Power tells whether the computer runs on its battery, where the voice
// model rests unless the user wants it there too.
type Power interface {
	// HasBattery tells whether the computer has a battery at all.
	HasBattery() bool
	// OnBattery tells whether the computer runs on its battery now.
	OnBattery() bool
	// Watch calls changed each time the computer goes on or off its
	// battery, until ctx ends.
	Watch(ctx context.Context, changed func(onBattery bool))
}
