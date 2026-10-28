package speechpack_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/speechpack"
)

// Every pack in the catalogue carries its vocabulary and meta inside the app,
// readable as an alphabet, and the meta names its source, never a folder of
// the computer it was made on.
func TestEveryPackCarriesItsAlphabetAndSource(t *testing.T) {
	t.Parallel()
	for _, pack := range speech.Packs() {
		extras, err := speechpack.Extras(pack.Language)
		if err != nil {
			t.Fatalf("%s: %v", pack.Language, err)
		}
		var meta struct {
			Model     string `json:"model"`
			Rate      int    `json:"sampling_rate"`
			Pad       string `json:"pad_token"`
			Delimiter string `json:"word_delimiter_token"`
		}
		var vocabulary map[string]int
		if err := json.Unmarshal(extras["meta.json"], &meta); err != nil {
			t.Fatalf("%s meta.json: %v", pack.Language, err)
		}
		if err := json.Unmarshal(extras["vocab.json"], &vocabulary); err != nil {
			t.Fatalf("%s vocab.json: %v", pack.Language, err)
		}
		if _, err := speech.NewAlphabet(vocabulary, meta.Pad, meta.Delimiter); err != nil {
			t.Errorf("%s: its vocabulary makes no alphabet: %v", pack.Language, err)
		}
		if meta.Rate != 16000 {
			t.Errorf("%s: sampling rate %d, want 16000", pack.Language, meta.Rate)
		}
		if bytes.Contains(extras["meta.json"], []byte(`:\\`)) { // a drive path, as JSON writes it
			t.Errorf("%s: meta.json names a local folder", pack.Language)
		}
		if !bytes.Contains([]byte(meta.Model), []byte("/")) || bytes.Contains([]byte(meta.Model), []byte(`\`)) {
			t.Errorf("%s: meta names %q, want its source on Hugging Face (owner/name@commit)", pack.Language, meta.Model)
		}
		// The license beside the model is the Apache License its authors
		// published it under, naming the very revision it was made from.
		_, revision, _ := strings.Cut(meta.Model, "@")
		license := string(extras["LICENSE.txt"])
		if len(revision) != 40 || !strings.Contains(license, revision) || !strings.Contains(license, "Apache License") {
			t.Errorf("%s: the license beside the model does not name its revision %q under the Apache License", pack.Language, revision)
		}
	}
	if _, err := speechpack.Extras("xx"); err == nil {
		t.Error("a language without a pack has extras")
	}
}

// ONNX Runtime's license, third-party notices and where their source is go
// beside its DLL (the MPL-2.0 of Eigen, inside it, asks for the source's
// whereabouts).
func TestTheRuntimeCarriesItsLicenseAndNotices(t *testing.T) {
	t.Parallel()
	notices, err := speechpack.RuntimeNotices()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(notices["onnxruntime-LICENSE.txt"]), "MIT License") {
		t.Error("ONNX Runtime's license is not its MIT License")
	}
	if !strings.Contains(string(notices["onnxruntime-ThirdPartyNotices.txt"]), "Eigen") {
		t.Error("ONNX Runtime's third-party notices do not name what it is built with (Eigen)")
	}
	if !strings.Contains(string(notices["onnxruntime-SOURCE.txt"]), "github.com/microsoft/onnxruntime/tree/v1.30.0") {
		t.Error("the runtime's notices do not say where its source is")
	}
}
