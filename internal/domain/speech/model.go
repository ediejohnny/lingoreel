package speech

// ChunkSamples is how much 16 kHz mono sound a model judges at a time:
// 32 ms.
const ChunkSamples = 512

// Model tells speech from every other sound (music, effects, a crowd), as
// a neural network trained on voices does; the energy of the sound alone
// cannot.
type Model interface {
	// NewStream starts judging a stretch of sound. Streams are independent
	// of each other and may run at the same time.
	NewStream() (Stream, error)
}

// Stream judges one stretch of sound, chunk after chunk, carrying what the
// model remembers from one chunk to the next. It is not safe for concurrent
// use.
type Stream interface {
	// Speech is how likely the next ChunkSamples samples are speech, from 0
	// to 1. A chunk of another length is taken as no speech.
	Speech(chunk []float32) float32
}
