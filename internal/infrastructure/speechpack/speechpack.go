// Package speechpack carries inside the app the small files of each
// language's speech pack: the model's vocabulary and how its sound is read
// (meta.json, which names the model's source on Hugging Face). Only the
// model itself is fetched.
package speechpack

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed packs
var packs embed.FS

// Extras are a language's vocab.json and meta.json, by file name.
func Extras(language string) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, name := range []string{"vocab.json", "meta.json"} {
		data, err := fs.ReadFile(packs, "packs/"+language+"/"+name)
		if err != nil {
			return nil, fmt.Errorf("speech pack %q: %w", language, err)
		}
		files[name] = data
	}
	return files, nil
}
