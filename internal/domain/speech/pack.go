package speech

import "slices"

// File is one file the app fetches as published: its name where it is
// published, its exact size and its SHA-256 (hex).
type File struct {
	Name   string
	Size   int64
	SHA256 string
}

// Pack is what places a language's words in its sound: a CTC speech model
// (wav2vec2, int8 on its MatMul nodes) for that language.
type Pack struct {
	// Language is the language's code, as the profiles name it ("en", "pt").
	Language string
	Model    File
}

// Runtime is ONNX Runtime 1.30.0, built from its source with telemetry off:
// every pack runs on it, so it comes with the first one.
var Runtime = File{
	Name: "onnxruntime-1.30.0.dll", Size: 17573888,
	SHA256: "f0e85685f946015f33a800ad4bdb7b50d10445eba1d3aabd5d4393ed60ddd213",
}

// packs are English's base model and the XLSR-53 large models of the other
// languages, all Apache-2.0.
var packs = []Pack{
	{Language: "en", Model: File{Name: "speech-en.onnx", Size: 121923222,
		SHA256: "efe5c6faa894803c1328cafdcf98be2ea1bb2b4d9026ee88afa1e42d4fa83e52"}},
	{Language: "pt", Model: File{Name: "speech-pt.onnx", Size: 355018566,
		SHA256: "d6c29e16f1d1c53fb5514b55710c74dc35b550de2d5fbb25477b2bdfc6fbc143"}},
	{Language: "es", Model: File{Name: "speech-es.onnx", Size: 355013426,
		SHA256: "9e35f8db763c2ca1661004373974ce58f6731a36a350c9bf8a754b946bb587a8"}},
	{Language: "fr", Model: File{Name: "speech-fr.onnx", Size: 355031930,
		SHA256: "d201660feed3305172b087d0147046d19b23c8b3b7492a7b85978777917ae7ec"}},
	{Language: "de", Model: File{Name: "speech-de.onnx", Size: 355010342,
		SHA256: "1d2cc147ec2a395d2c23c3ea13f8c4a1a1027fb6946ee228c99b2cb2e2036b6b"}},
	{Language: "it", Model: File{Name: "speech-it.onnx", Size: 355016510,
		SHA256: "f39959425a0d414e58252bbe5426f0293f9a61388343aba777b8c5e5ed2be644"}},
}

// Packs lists every language with a pack, English first.
func Packs() []Pack { return slices.Clone(packs) }

// PackFor finds the pack of a language.
func PackFor(language string) (Pack, bool) {
	i := slices.IndexFunc(packs, func(p Pack) bool { return p.Language == language })
	if i < 0 {
		return Pack{}, false
	}
	return packs[i], true
}
