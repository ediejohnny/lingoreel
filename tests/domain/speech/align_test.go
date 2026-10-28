package speech_test

import (
	"errors"
	"math"
	"slices"
	"testing"
	"time"
	"unicode"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"golang.org/x/text/unicode/norm"
)

const alignFrame = 20 * time.Millisecond

// english is an alphabet like wav2vec2-base-960h's: capitals, the
// apostrophe, "|" between words and "<pad>" as the blank.
func english(t *testing.T) speech.Alphabet {
	t.Helper()
	a, err := speech.NewAlphabet(englishVocabulary(), "<pad>", "|")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// emitted builds a model's output where each frame is sure of one token:
// "." is the blank, "|" the delimiter, any other the letter.
func emitted(a speech.Alphabet, vocabulary map[string]int, frames string) [][]float32 {
	logits := make([][]float32, 0, len(frames))
	for _, r := range frames {
		row := make([]float32, a.Size())
		token := string(r)
		if token == "." {
			token = "<pad>"
		}
		row[vocabulary[token]] = 8
		logits = append(logits, row)
	}
	return logits
}

func englishVocabulary() map[string]int {
	vocabulary := map[string]int{"<pad>": 0, "<s>": 1, "</s>": 2, "<unk>": 3, "|": 4, "'": 5}
	for i, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		vocabulary[string(r)] = 6 + i
	}
	return vocabulary
}

func at(frames int) time.Duration { return time.Duration(frames) * alignFrame }

// A letter heard surely over several frames keeps them all, at the line's
// first and last letters too: before another voice (the "TT" after "DOGG")
// and when the sound ends with the line.
func TestWordsArePlacedWhereTheirLettersAreHeard(t *testing.T) {
	t.Parallel()
	a := english(t)
	//                      0    5    10   15   20
	logits := emitted(a, englishVocabulary(), ".....CCAT..||..DOGGTT...")
	want := []speech.Span{{Start: at(5), End: at(9)}, {Start: at(15), End: at(19)}}
	for _, sound := range [][][]float32{logits, logits[:19]} {
		words, err := a.Align(sound, alignFrame, []string{"Cat,", "dog!"})
		if err != nil {
			t.Fatal(err)
		}
		for i, w := range words {
			if !w.Heard || w.Span != want[i] {
				t.Errorf("%d frames, word %d: %+v, want %v heard", len(sound), i, w, want[i])
			}
			if w.Confidence < 0.9 {
				t.Errorf("%d frames, word %d: confidence %.2f, want the model's certainty", len(sound), i, w.Confidence)
			}
		}
	}
}

// heardFrame is one frame of a model's output, from how likely it found each
// token; the others are nearly never heard.
func heardFrame(a speech.Alphabet, vocabulary map[string]int, likely map[string]float64) []float32 {
	row := make([]float32, a.Size())
	for i := range row {
		row[i] = float32(math.Log(1e-6))
	}
	for token, p := range likely {
		row[vocabulary[token]] = float32(math.Log(p))
	}
	return row
}

// The sound taken around a line holds its neighbours' voices, and a voice
// is no blank: paying for every frame of it, the best path found it cheaper
// to pull a line's last letters onto the next line's (Sintel's "flowing"
// ended on the "thiNK" of "Thank you", 2.6 s late). The frames outside the
// line cost nothing, so "sing" stays where its letters were heard, before
// the next line's voice and, read backwards, after the previous line's.
func TestALinesLettersStayOffItsNeighboursVoices(t *testing.T) {
	t.Parallel()
	a := english(t)
	vocabulary := englishVocabulary()
	sure := func(token string) map[string]float64 { return map[string]float64{token: 1} }
	frames := []map[string]float64{
		sure("<pad>"), sure("S"), sure("I"),
		{"N": 0.855, "<pad>": 0.13}, {"G": 0.468, "<pad>": 0.26, "N": 0.25},
		sure("<pad>"), sure("<pad>"), sure("<pad>"), sure("<pad>"), sure("<pad>"),
		{"T": 0.99, "<pad>": 0.01}, {"I": 0.74, "A": 0.24, "<pad>": 0.01},
		{"N": 0.94, "<pad>": 0.06}, {"N": 0.89, "<pad>": 0.11}, {"K": 0.99, "G": 0.006, "<pad>": 0.01},
	}
	logits := make([][]float32, 0, len(frames))
	for _, f := range frames {
		logits = append(logits, heardFrame(a, vocabulary, f))
	}
	words, err := a.Align(logits, alignFrame, []string{"sing"})
	if err != nil {
		t.Fatal(err)
	}
	if want := (speech.Span{Start: at(1), End: at(5)}); words[0].Span != want {
		t.Errorf("sing before the next line's voice: %v, want %v, where its letters were heard", words[0].Span, want)
	}

	slices.Reverse(logits)
	words, err = a.Align(logits, alignFrame, []string{"gnis"})
	if err != nil {
		t.Fatal(err)
	}
	if want := (speech.Span{Start: at(len(frames) - 5), End: at(len(frames) - 1)}); words[0].Span != want {
		t.Errorf("gnis after the previous line's voice: %v, want %v, where its letters were heard", words[0].Span, want)
	}
}

// A song's lines are placed in one sound, in their order, with whatever is
// between them (a solo, a refrain not written) left outside them: "yo" also
// sounds before "sing" but is the later one, and the voice between the lines
// does not pull "sing"'s last letters onto it.
func TestASongsLinesArePlacedInOrderWithAnythingBetween(t *testing.T) {
	t.Parallel()
	a := english(t)
	vocabulary := englishVocabulary()
	sure := func(token string) map[string]float64 { return map[string]float64{token: 1} }
	frames := []map[string]float64{
		sure("Y"), sure("O"), sure("<pad>"), sure("S"), sure("I"),
		{"N": 0.855, "<pad>": 0.13}, {"G": 0.468, "<pad>": 0.26, "N": 0.25},
		sure("<pad>"), sure("<pad>"),
		{"T": 0.99, "<pad>": 0.01}, {"I": 0.74, "A": 0.24, "<pad>": 0.01},
		{"N": 0.94, "<pad>": 0.06}, {"N": 0.89, "<pad>": 0.11}, {"K": 0.99, "G": 0.006, "<pad>": 0.01},
		sure("<pad>"), sure("Y"), sure("O"), sure("<pad>"),
	}
	logits := make([][]float32, 0, len(frames))
	for _, f := range frames {
		logits = append(logits, heardFrame(a, vocabulary, f))
	}
	lines, err := a.AlignLines(logits, alignFrame, [][]string{{"sing"}, {"yo"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := (speech.Span{Start: at(3), End: at(7)}); lines[0][0].Span != want {
		t.Errorf("sing: %v, want %v, where its letters were heard", lines[0][0].Span, want)
	}
	if want := (speech.Span{Start: at(15), End: at(17)}); lines[1][0].Span != want {
		t.Errorf("yo: %v, want %v, after sing", lines[1][0].Span, want)
	}
}

func TestADoubledLetterNeedsABlankBetween(t *testing.T) {
	t.Parallel()
	a := english(t)
	vocabulary := englishVocabulary()
	words, err := a.Align(emitted(a, vocabulary, "..BO.OK.."), alignFrame, []string{"book"})
	if err != nil {
		t.Fatal(err)
	}
	if words[0].Span != (speech.Span{Start: at(2), End: at(7)}) {
		t.Errorf("book: %v, want frames 2 to 7", words[0].Span)
	}
	// Without the blank, the two Os cannot both be heard in "BOOK": the
	// line needs one frame more than BOK.
	if _, err := a.Align(emitted(a, vocabulary, "BOK"), alignFrame, []string{"book"}); !errors.Is(err, speech.ErrTooShort) {
		t.Errorf("BOOK in 3 frames: %v, want ErrTooShort", err)
	}
}

func TestAccentsAndCaseFollowTheAlphabet(t *testing.T) {
	t.Parallel()
	a := english(t)
	words, err := a.Align(emitted(a, englishVocabulary(), "..CAFE.."), alignFrame, []string{"Café"})
	if err != nil {
		t.Fatal(err)
	}
	if !words[0].Heard || words[0].Span != (speech.Span{Start: at(2), End: at(6)}) {
		t.Errorf("café on an alphabet without accents: %+v", words[0])
	}

	// An alphabet with accents (like the XLSR models', in lower case) keeps
	// them: "é" is its own letter.
	vocabulary := map[string]int{"[PAD]": 0, "|": 1, "c": 2, "a": 3, "f": 4, "e": 5, "é": 6}
	accented, err := speech.NewAlphabet(vocabulary, "[PAD]", "|")
	if err != nil {
		t.Fatal(err)
	}
	logits := make([][]float32, 0, 6)
	for _, id := range []int{0, 2, 3, 4, 6, 0} {
		row := make([]float32, accented.Size())
		row[id] = 8
		logits = append(logits, row)
	}
	words, err = accented.Align(logits, alignFrame, []string{"CAFÉ"})
	if err != nil {
		t.Fatal(err)
	}
	if words[0].Span != (speech.Span{Start: at(1), End: at(5)}) || words[0].Confidence < 0.9 {
		t.Errorf("CAFÉ on an alphabet with é: %+v", words[0])
	}
}

// On an alphabet without accents, every Latin letter of Latin-1 and Latin
// Extended-A is heard as its plain letter exactly when Unicode decomposes it
// into that letter and its accents (NFD), or its capital is that letter
// (ı, ſ); a letter of its own (ø, ß, ł) is not in the alphabet.
func TestAnAccentedLetterIsHeardAsItsPlainLetter(t *testing.T) {
	t.Parallel()
	a := english(t)
	vocabulary := englishVocabulary()
	for r := rune(0xC0); r <= 0x17F; r++ {
		if !unicode.IsLetter(r) {
			continue
		}
		decomposed := []rune(norm.NFD.String(string(r)))
		base := unicode.ToUpper(decomposed[0])
		if len(decomposed) == 1 {
			base = unicode.ToUpper(r)
		}
		accented := (len(decomposed) > 1 || base != r) && base >= 'A' && base <= 'Z'
		words, err := a.Align(emitted(a, vocabulary, "."+string(base)+"."), alignFrame, []string{string(r)})
		switch {
		case accented && (err != nil || words[0].Span != (speech.Span{Start: at(1), End: at(2)})):
			t.Errorf("%c (%U): %v %+v, want heard as %c", r, r, err, words, base)
		case !accented && !errors.Is(err, speech.ErrNothingToAlign):
			t.Errorf("%c (%U): %v, want no letter of the alphabet", r, r, err)
		}
	}
}

func TestAWordWithoutLettersHasNoTimeOfItsOwn(t *testing.T) {
	t.Parallel()
	a := english(t)
	words, err := a.Align(emitted(a, englishVocabulary(), "..IN..|..LA.."), alignFrame, []string{"in", "1984", "LA"})
	if err != nil {
		t.Fatal(err)
	}
	if !words[0].Heard || words[1].Heard || !words[2].Heard {
		t.Fatalf("heard: %v %v %v, want only the words with letters", words[0].Heard, words[1].Heard, words[2].Heard)
	}
	if words[2].Span != (speech.Span{Start: at(9), End: at(11)}) {
		t.Errorf("LA: %v", words[2].Span)
	}
}

func TestWhatCannotBeAlignedSaysWhy(t *testing.T) {
	t.Parallel()
	a := english(t)
	if _, err := a.Align(emitted(a, englishVocabulary(), "...."), alignFrame, []string{"42", "!"}); !errors.Is(err, speech.ErrNothingToAlign) {
		t.Errorf("no letters: %v", err)
	}
	if _, err := a.Align(emitted(a, englishVocabulary(), "HI"), alignFrame, []string{"hello"}); !errors.Is(err, speech.ErrTooShort) {
		t.Errorf("too short: %v", err)
	}
	if _, err := speech.NewAlphabet(map[string]int{"A": 0}, "<pad>", "|"); !errors.Is(err, speech.ErrAlphabet) {
		t.Errorf("no blank: %v", err)
	}
}
