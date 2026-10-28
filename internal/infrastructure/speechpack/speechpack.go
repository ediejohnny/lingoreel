// Package speechpack carries inside the app the small files of each
// language's speech pack: the model's vocabulary, how its sound is read
// (meta.json, which names the model's source on Hugging Face) and the
// models' license; and ONNX Runtime's license and notices. Only the model
// and the runtime themselves are fetched.
package speechpack

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed packs licenses
var files embed.FS

// Extras are a language's vocab.json, meta.json and the models' license
// (LICENSE.txt), by file name, to go beside its model.
func Extras(language string) (map[string][]byte, error) {
	return read(map[string]string{
		"vocab.json":  "packs/" + language + "/vocab.json",
		"meta.json":   "packs/" + language + "/meta.json",
		"LICENSE.txt": "licenses/models-LICENSE.txt",
	})
}

// RuntimeNotices are ONNX Runtime's license, its third-party notices and
// where their source is, by file name, to go beside its DLL.
func RuntimeNotices() (map[string][]byte, error) {
	return read(map[string]string{
		"onnxruntime-LICENSE.txt":           "licenses/onnxruntime-LICENSE.txt",
		"onnxruntime-ThirdPartyNotices.txt": "licenses/onnxruntime-ThirdPartyNotices.txt",
		"onnxruntime-SOURCE.txt":            "licenses/onnxruntime-SOURCE.txt",
	})
}

// read reads embedded files, by the names they are written under.
func read(names map[string]string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for name, path := range names {
		data, err := fs.ReadFile(files, path)
		if err != nil {
			return nil, fmt.Errorf("speech pack: %w", err)
		}
		out[name] = data
	}
	return out, nil
}
