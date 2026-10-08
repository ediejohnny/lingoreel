package speech_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
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

func TestWordsArePlacedWhereTheirLettersAreHeard(t *testing.T) {
	t.Parallel()
	a := english(t)
	//                      0    5    10   15   20
	logits := emitted(a, englishVocabulary(), ".....CCAT..||..DOOG.....")
	words, err := a.Align(logits, alignFrame, []string{"Cat,", "dog!"})
	if err != nil {
		t.Fatal(err)
	}
	want := []speech.Span{{Start: at(5), End: at(9)}, {Start: at(15), End: at(19)}}
	for i, w := range words {
		if !w.Heard || w.Span != want[i] {
			t.Errorf("word %d: %+v, want %v heard", i, w, want[i])
		}
		if w.Confidence < 0.9 {
			t.Errorf("word %d: confidence %.2f, want the model's certainty", i, w.Confidence)
		}
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
