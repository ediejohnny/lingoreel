package speech

import (
	"errors"
	"math"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Aligning a line's words with its sound. A CTC model (wav2vec2) hears the
// sound frame by frame and tells, for each frame, how likely each letter of
// its alphabet is, or none (the blank). Knowing what is said, the line's
// text, the most likely path through those letters places every word in
// time, much closer than spreading the words by their syllables.

var (
	// ErrAlphabet reports an alphabet without its blank or its word
	// delimiter.
	ErrAlphabet = errors.New("speech: the model's alphabet lacks its blank or its word delimiter")
	// ErrNothingToAlign reports a line with no letter of the alphabet.
	ErrNothingToAlign = errors.New("speech: no letter of the line is in the model's alphabet")
	// ErrTooShort reports a sound with fewer frames than the line's letters
	// need.
	ErrTooShort = errors.New("speech: the sound is too short for the line's letters")
)

// Alphabet is a CTC model's letters by their numbers in its output, with
// the blank (no letter) and the letter between words.
type Alphabet struct {
	letters          map[rune]int
	blank, delimiter int
	size             int
}

// NewAlphabet reads a model's vocabulary (each token and its number) and
// the names of its blank and word delimiter tokens; tokens longer than one
// letter ("<unk>", "</s>") are never spelled.
func NewAlphabet(vocabulary map[string]int, blank, delimiter string) (Alphabet, error) {
	blankID, okBlank := vocabulary[blank]
	delimiterID, okDelimiter := vocabulary[delimiter]
	if !okBlank || !okDelimiter {
		return Alphabet{}, ErrAlphabet
	}
	a := Alphabet{letters: map[rune]int{}, blank: blankID, delimiter: delimiterID}
	for token, id := range vocabulary {
		a.size = max(a.size, id+1)
		if letters := []rune(token); len(letters) == 1 && id != blankID && id != delimiterID {
			a.letters[letters[0]] = id
		}
	}
	return a, nil
}

// Size is how many outputs the model has per frame.
func (a Alphabet) Size() int { return a.size }

// Word is where a line's word was heard; Heard is false for a word none of
// whose letters is in the alphabet (a number, a symbol), which has no time
// of its own. Confidence is how sure the model was of its letters, from 0
// to 1.
type Word struct {
	Span
	Heard      bool
	Confidence float64
}

// letter finds a written letter in the alphabet: as written, in the other
// case, or without its accent when the alphabet has none ("é" as "E").
func (a Alphabet) letter(r rune) (int, bool) {
	if r == '’' || r == 'ʼ' {
		r = '\''
	}
	for _, candidate := range []rune{r, unicode.ToUpper(r), unicode.ToLower(r)} {
		if id, ok := a.letters[candidate]; ok {
			return id, true
		}
	}
	if base := []rune(norm.NFD.String(string(r))); len(base) > 1 && base[0] != r {
		return a.letter(base[0])
	}
	return 0, false
}

// spell turns the words into the model's letters, with the delimiter
// between words, and tells which word each letter belongs to.
func (a Alphabet) spell(words []string) (tokens, owners []int) {
	for i, word := range words {
		first := true
		for _, r := range word {
			id, ok := a.letter(r)
			if !ok {
				continue
			}
			if first && len(tokens) > 0 {
				tokens, owners = append(tokens, a.delimiter), append(owners, -1)
			}
			first = false
			tokens, owners = append(tokens, id), append(owners, i)
		}
	}
	return tokens, owners
}

// Align places a line's words in its sound, given the model's raw output
// (logits) for each frame of frame length; the times count from the first
// frame.
func (a Alphabet) Align(logits [][]float32, frame time.Duration, words []string) ([]Word, error) {
	tokens, owners := a.spell(words)
	if len(tokens) == 0 {
		return nil, ErrNothingToAlign
	}
	path, err := a.bestPath(logSoftmax(logits), tokens)
	if err != nil {
		return nil, err
	}
	result := make([]Word, len(words))
	sums := make([]float64, len(words))
	counts := make([]int, len(words))
	for t, state := range path {
		if state.token < 0 || owners[state.token] < 0 {
			continue
		}
		i := owners[state.token]
		start, end := time.Duration(t)*frame, time.Duration(t+1)*frame
		if !result[i].Heard {
			result[i] = Word{Span: Span{Start: start, End: end}, Heard: true}
		}
		result[i].End = end
		sums[i] += math.Exp(state.logProb)
		counts[i]++
	}
	for i := range result {
		if counts[i] > 0 {
			result[i].Confidence = sums[i] / float64(counts[i])
		}
	}
	return result, nil
}

// step is one frame of the best path: the letter it is in (an index into
// the line's letters, -1 for a blank) and how likely the model found it.
type step struct {
	token   int
	logProb float64
}

// bestPath finds the most likely way through the line's letters (Viterbi
// over CTC's states: a blank before, between and after the letters; a
// letter may last several frames; the same letter twice in a row needs a
// blank between).
func (a Alphabet) bestPath(logProbs [][]float64, tokens []int) ([]step, error) {
	frames, states := len(logProbs), 2*len(tokens)+1
	needed := len(tokens)
	for i := 1; i < len(tokens); i++ {
		if tokens[i] == tokens[i-1] {
			needed++
		}
	}
	if frames < needed {
		return nil, ErrTooShort
	}
	label := func(s int) int {
		if s%2 == 0 {
			return a.blank
		}
		return tokens[s/2]
	}
	for _, row := range logProbs {
		if len(row) < a.size {
			return nil, ErrAlphabet
		}
	}
	impossible := math.Inf(-1)
	previous, current := make([]float64, states), make([]float64, states)
	back := make([][]int8, frames)
	for s := range previous {
		previous[s] = impossible
	}
	previous[0] = logProbs[0][a.blank]
	previous[1] = logProbs[0][tokens[0]]
	back[0] = make([]int8, states)
	for t := 1; t < frames; t++ {
		back[t] = make([]int8, states)
		for s := range states {
			best, from := previous[s], int8(0)
			if s >= 1 && previous[s-1] > best {
				best, from = previous[s-1], 1
			}
			if s >= 2 && s%2 == 1 && label(s) != label(s-2) && previous[s-2] > best {
				best, from = previous[s-2], 2
			}
			current[s] = best + logProbs[t][label(s)]
			back[t][s] = from
		}
		previous, current = current, previous
	}
	s := states - 1
	if previous[states-2] > previous[s] {
		s = states - 2
	}
	if math.IsInf(previous[s], -1) {
		return nil, ErrTooShort
	}
	path := make([]step, frames)
	for t := frames - 1; t >= 0; t-- {
		path[t] = step{token: -1, logProb: logProbs[t][label(s)]}
		if s%2 == 1 {
			path[t].token = s / 2
		}
		s -= int(back[t][s])
	}
	return path, nil
}

// logSoftmax turns each frame's logits into log-probabilities.
func logSoftmax(logits [][]float32) [][]float64 {
	result := make([][]float64, len(logits))
	for t, row := range logits {
		top := math.Inf(-1)
		for _, v := range row {
			top = max(top, float64(v))
		}
		sum := 0.0
		for _, v := range row {
			sum += math.Exp(float64(v) - top)
		}
		total := top + math.Log(sum)
		result[t] = make([]float64, len(row))
		for i, v := range row {
			result[t][i] = float64(v) - total
		}
	}
	return result
}
