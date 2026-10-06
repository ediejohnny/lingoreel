// Package silero tells speech from other sounds with the Silero VAD
// network (v6.2.1, MIT, model/LICENSE), computed here in Go from the
// model's own weights: no runtime, no library, no file beside the app.
package silero

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// The model as published (silero_vad_16k_op15.onnx, commit 7e30209 of
// github.com/snakers4/silero-vad), unmodified.
//
//go:embed model/silero_vad_16k_op15.onnx
var embedded []byte

// pinned is the model's sha256: no other file is read.
const pinned = "7ed98ddbad84ccac4cd0aeb3099049280713df825c610a8ed34543318f1b2c49"

// ErrNotTheModel reports data that is not the pinned model.
var ErrNotTheModel = errors.New("not the Silero VAD model")

// Model is the network with its weights; it is safe for concurrent use,
// each stream being independent.
type Model struct {
	weights *weights
}

// The model is speech's model.
var _ speech.Model = (*Model)(nil)

// loadEmbedded reads the model the app carries, once.
var loadEmbedded = sync.OnceValues(func() (*Model, error) { return Load(embedded) })

// New returns the model the app carries; it reads it on first use.
func New() (*Model, error) {
	return loadEmbedded()
}

// Embedded is the model the app carries, read on its first stream, so the
// app does not pay for it until the voice is followed.
type Embedded struct{}

var _ speech.Model = Embedded{}

// NewStream reads the model, the first time, and starts a stream of it.
func (Embedded) NewStream() (speech.Stream, error) {
	model, err := loadEmbedded()
	if err != nil {
		return nil, err
	}
	return model.NewStream()
}

// Load reads a model, which must be the pinned one byte for byte.
func Load(data []byte) (*Model, error) {
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != pinned {
		return nil, ErrNotTheModel
	}
	tensors, err := readInitializers(data)
	if err != nil {
		return nil, err
	}
	w, err := weightsOf(tensors)
	if err != nil {
		return nil, err
	}
	return &Model{weights: w}, nil
}

// NewStream starts judging a stretch of sound, from silence.
func (m *Model) NewStream() (speech.Stream, error) {
	return newStream(m.weights), nil
}
