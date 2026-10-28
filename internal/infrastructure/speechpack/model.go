package speechpack

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/onnx"
)

// frame is how much sound one output of a wav2vec2 model covers: its
// convolutions step 320 samples at 16 kHz.
const frame = 20 * time.Millisecond

// smallFileLimit bounds what is read of vocab.json and meta.json.
const smallFileLimit = 1 << 20

// ErrAnswer reports a model whose output is not one row of its alphabet per
// frame.
var ErrAnswer = errors.New("speech pack: the model did not answer frames of its alphabet")

// meta is how a model reads its sound and names its special tokens.
type meta struct {
	Pad         string `json:"pad_token"`
	Delimiter   string `json:"word_delimiter_token"`
	DoNormalize bool   `json:"do_normalize"`
}

// Model is a language's speech pack opened: ONNX Runtime running its CTC
// model, with the model's alphabet. It is not safe for concurrent use.
type Model struct {
	runtime   *onnx.Runtime
	session   *onnx.Session
	alphabet  speech.Alphabet
	normalize bool
}

// Open loads ONNX Runtime from runtimePath and the model at modelPath on
// threads processors; its vocab.json and meta.json are beside it.
func Open(runtimePath, modelPath string, threads int) (*Model, error) {
	folder := filepath.Dir(modelPath)
	var info meta
	var vocabulary map[string]int
	for name, into := range map[string]any{"meta.json": &info, "vocab.json": &vocabulary} {
		data, err := readBounded(filepath.Join(folder, name))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, into); err != nil {
			return nil, fmt.Errorf("speech pack %s: %w", name, err)
		}
	}
	alphabet, err := speech.NewAlphabet(vocabulary, info.Pad, info.Delimiter)
	if err != nil {
		return nil, err
	}
	runtime, err := onnx.Load(runtimePath)
	if err != nil {
		return nil, err
	}
	session, err := runtime.Open(modelPath, "input_values", "logits", threads)
	if err != nil {
		runtime.Close()
		return nil, err
	}
	return &Model{runtime: runtime, session: session, alphabet: alphabet, normalize: info.DoNormalize}, nil
}

// Version is ONNX Runtime's version.
func (m *Model) Version() string { return m.runtime.Version() }

// Align places words in 16 kHz mono sound (left as it is); the times count
// from the sound's start. A line with no letter of the model's alphabet
// returns speech.ErrNothingToAlign.
func (m *Model) Align(sound []float32, words []string) ([]speech.Word, error) {
	input := sound
	if m.normalize {
		input = normalized(sound)
	}
	logits, shape, err := m.session.Run(input, []int64{1, int64(len(input))})
	if err != nil {
		return nil, err
	}
	size := m.alphabet.Size()
	if len(shape) != 3 || shape[2] != int64(size) || int64(len(logits)) != shape[1]*shape[2] {
		return nil, fmt.Errorf("%w: %v", ErrAnswer, shape)
	}
	rows := make([][]float32, shape[1])
	for i := range rows {
		rows[i] = logits[i*size : (i+1)*size]
	}
	return m.alphabet.Align(rows, frame, words)
}

// Close releases the model and the runtime.
func (m *Model) Close() {
	m.session.Close()
	m.runtime.Close()
}

// normalized is the sound with zero mean and unit variance, as wav2vec2's
// feature extractor makes it.
func normalized(sound []float32) []float32 {
	mean := 0.0
	for _, s := range sound {
		mean += float64(s)
	}
	mean /= float64(max(len(sound), 1))
	variance := 0.0
	for _, s := range sound {
		variance += (float64(s) - mean) * (float64(s) - mean)
	}
	scale := 1 / math.Sqrt(variance/float64(max(len(sound), 1))+1e-7)
	out := make([]float32, len(sound))
	for i, s := range sound {
		out[i] = float32((float64(s) - mean) * scale)
	}
	return out
}

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, smallFileLimit+1))
	if err == nil && len(data) > smallFileLimit {
		err = fmt.Errorf("%s is larger than %d bytes", path, smallFileLimit)
	}
	return data, err
}
