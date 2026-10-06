package speech_test

import (
	"slices"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// numbering says, for each chunk it judges, its number in tenths, and keeps
// the chunks.
type numbering struct{ chunks [][]float32 }

func (n *numbering) Speech(chunk []float32) float32 {
	n.chunks = append(n.chunks, slices.Clone(chunk))
	return float32(len(n.chunks)-1) / 10
}

func analyzer(t *testing.T, channels int, stream speech.Stream) *speech.Analyzer {
	t.Helper()
	a, err := speech.NewAnalyzer(channels, stream)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func speeches(frames []speech.Frame) []float32 {
	out := make([]float32, len(frames))
	for i, f := range frames {
		out[i] = f.Speech
	}
	return out
}

// The model judges 32 ms at a time, the frames are 10 ms: each frame takes
// the speech of the chunk holding its centre, as soon as that chunk is
// judged. At the end, what is left is judged with silence after it, for the
// frames whose centre it holds.
func TestEachFrameTakesTheChunkAtItsCentre(t *testing.T) {
	t.Parallel()
	stream := &numbering{}
	a := analyzer(t, 1, stream)
	sound := make([]float32, 1700) // 3 chunks and 164 samples
	for i := range sound {
		sound[i] = 1
	}

	frames := a.Write(sound, nil)
	// Centres at 80, 240, 400 | 560, 720, 880 | 1040, 1200, 1360, 1520.
	if want := []float32{0, 0, 0, 0.1, 0.1, 0.1, 0.2, 0.2, 0.2, 0.2}; !slices.Equal(speeches(frames), want) {
		t.Fatalf("frames = %v, want %v", speeches(frames), want)
	}
	frames = a.Flush(frames[:0])
	if want := []float32{0.3}; !slices.Equal(speeches(frames), want) { // the centre at 1680
		t.Fatalf("flushed frames = %v, want %v", speeches(frames), want)
	}
	last := stream.chunks[3]
	if last[163] != 1 || slices.ContainsFunc(last[164:], func(s float32) bool { return s != 0 }) {
		t.Fatal("the last chunk is not the sound left followed by silence")
	}
}

// The player's sound comes in packets of any size, even one ending between
// the left and the right sample: the model hears the same middle of the
// sound, and the frames are the same.
func TestTheAnalyzerTakesTheSoundInAnyPieces(t *testing.T) {
	t.Parallel()
	sound := make([]float32, 2*3000)
	for i := range 3000 {
		sound[2*i], sound[2*i+1] = float32(i%97)/97, float32(i%13)/13
	}
	whole := &numbering{}
	frames := analyzer(t, 2, whole).Write(sound, nil)

	pieces := &numbering{}
	a := analyzer(t, 2, pieces)
	var inPieces []speech.Frame
	for at, i := 0, 0; at < len(sound); i++ {
		size := min([]int{1, 7, 160, 333, 2}[i%5], len(sound)-at)
		inPieces = a.Write(sound[at:at+size], inPieces)
		at += size
	}
	if !slices.Equal(inPieces, frames) || !slices.EqualFunc(pieces.chunks, whole.chunks, slices.Equal) {
		t.Fatalf("%d frames in pieces, %d whole, or they differ", len(inPieces), len(frames))
	}
	if middle := (sound[2*600] + sound[2*600+1]) / 2; whole.chunks[1][600-512] != middle {
		t.Fatalf("the model heard %v, want the middle %v", whole.chunks[1][600-512], middle)
	}
}
