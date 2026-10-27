package voice

import (
	"math/rand/v2"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// speedSample is how much sound the model judges to tell its speed: long
// enough to even out the first chunks, short enough to cost a weak computer
// well under a second.
const speedSample = 3 * time.Second

// ModelSpeed measures how many seconds of sound the model judges in one
// second of work, on noise made up here: no film is needed.
func ModelSpeed(model speech.Model) (float64, error) {
	stream, err := model.NewStream()
	if err != nil {
		return 0, err
	}
	noise := rand.New(rand.NewPCG(1, 2)) // the same sound every time
	chunk := make([]float32, speech.ChunkSamples)
	chunks := int(speedSample.Seconds() * speech.SampleRate / speech.ChunkSamples)
	started := time.Now()
	for range chunks {
		for i := range chunk {
			chunk[i] = float32(noise.Float64()*2-1) * 0.1
		}
		stream.Speech(chunk)
	}
	worked := max(time.Since(started).Seconds(), 1e-9) // a clock too coarse to see the work
	return float64(chunks*speech.ChunkSamples) / speech.SampleRate / worked, nil
}
