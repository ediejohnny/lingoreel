package silero_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/silero"
)

// reference is what the official runtime (onnxruntime) said for the same
// model on the same signals, chunk by chunk.
type reference struct {
	Runtime       string               `json:"onnxruntime_version"`
	ModelSHA256   string               `json:"model_sha256"`
	SampleRate    int                  `json:"sample_rate"`
	WindowSize    int                  `json:"window_size"`
	Probabilities map[string][]float64 `json:"probabilities"`
}

// The network computed here says what the official runtime says, chunk by
// chunk, carrying its state through 60 s of each kind of sound: within
// 1e-4 of a probability.
func TestTheNetworkSaysWhatTheOfficialRuntimeSays(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("testdata", "silero_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want reference
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if want.SampleRate != signalRate || want.WindowSize != speech.ChunkSamples {
		t.Fatalf("reference of %d Hz in %d samples, want %d Hz in %d", want.SampleRate, want.WindowSize, signalRate, speech.ChunkSamples)
	}
	model, err := silero.New()
	if err != nil {
		t.Fatal(err)
	}
	const tolerance = 1e-4
	for _, sig := range signals() {
		chunks := want.Probabilities[sig.name]
		if len(chunks) != len(sig.samples)/speech.ChunkSamples {
			t.Fatalf("%s: %d reference chunks for %d samples", sig.name, len(chunks), len(sig.samples))
		}
		stream, err := model.NewStream()
		if err != nil {
			t.Fatal(err)
		}
		worst := 0.0
		for i, p := range chunks {
			got := stream.Speech(sig.samples[i*speech.ChunkSamples : (i+1)*speech.ChunkSamples])
			worst = max(worst, math.Abs(float64(got)-p))
			if worst > tolerance {
				t.Fatalf("%s chunk %d: %.7f, the runtime says %.7f", sig.name, i, got, p)
			}
		}
		t.Logf("%s: %d chunks, at most %.2e off", sig.name, len(chunks), worst)
	}
}

// Only the pinned model is read: a file with one byte changed, or cut
// short, is refused before anything in it is trusted.
func TestOnlyThePinnedModelIsRead(t *testing.T) {
	t.Parallel()
	original, err := os.ReadFile(filepath.Join("..", "..", "..", "internal", "infrastructure", "silero", "model", "silero_vad_16k_op15.onnx"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := silero.Load(original); err != nil {
		t.Fatalf("the model itself was refused: %v", err)
	}
	changed := append([]byte(nil), original...)
	changed[len(changed)/2] ^= 1
	for name, data := range map[string][]byte{"one byte changed": changed, "cut short": original[:len(original)/2], "empty": nil} {
		if _, err := silero.Load(data); !errors.Is(err, silero.ErrNotTheModel) {
			t.Errorf("%s: %v, want it refused as not the model", name, err)
		}
	}
}

// What a weak computer pays to judge the voice: one chunk is 32 ms of
// sound.
func BenchmarkStream(b *testing.B) {
	model, err := silero.New()
	if err != nil {
		b.Fatal(err)
	}
	stream, err := model.NewStream()
	if err != nil {
		b.Fatal(err)
	}
	sound := speechPulses()
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		at := (i * speech.ChunkSamples) % (len(sound) - speech.ChunkSamples)
		stream.Speech(sound[at : at+speech.ChunkSamples])
	}
	b.ReportMetric(float64(b.N)*0.032/b.Elapsed().Seconds(), "x-realtime")
}
