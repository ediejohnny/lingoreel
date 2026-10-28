package speech

import (
	"errors"
	"math"
	"slices"
	"time"
	"unicode"
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

// minWordsSureness is how sure the model must be of a line's words, the
// median of the heard ones' confidence, for the karaoke mode to paint the
// line word by word. Measured on Sintel: a line said plainly scores 0.9 or
// more, a shout or a whisper 0.01 to 0.5.
const minWordsSureness = 0.5

// WordsSure tells whether a line's words were placed surely enough to paint
// it word by word: some were heard, and their median confidence reaches
// minWordsSureness.
func WordsSure(words []Word) bool {
	var heard []float64
	for _, w := range words {
		if w.Heard {
			heard = append(heard, w.Confidence)
		}
	}
	return len(heard) > 0 && median(heard) >= minWordsSureness
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
	if base, ok := plainLetter(r); ok {
		return a.letter(base)
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
// frame. A word's time runs over its letters' frames, and its confidence is
// how likely the model found them, on average.
func (a Alphabet) Align(logits [][]float32, frame time.Duration, words []string) ([]Word, error) {
	lines, err := a.AlignLines(logits, frame, [][]string{words})
	if err != nil {
		return nil, err
	}
	return lines[0], nil
}

// AlignLines places several lines' words in one sound, the lines said in
// their order with anything between them (a song's lyrics in the song, its
// solos and refrains unwritten between the lines); each line's words are as
// Align tells them.
func (a Alphabet) AlignLines(logits [][]float32, frame time.Duration, lines [][]string) ([][]Word, error) {
	path, err := a.lettersPath(logits, lines)
	if err != nil {
		return nil, err
	}
	result := make([][]Word, len(lines))
	sums, counts := make([][]float64, len(lines)), make([][]int, len(lines))
	for i, words := range lines {
		result[i], sums[i], counts[i] = make([]Word, len(words)), make([]float64, len(words)), make([]int, len(words))
	}
	for t, at := range path {
		if at.word < 0 {
			continue
		}
		w := &result[at.line][at.word]
		start, end := time.Duration(t)*frame, time.Duration(t+1)*frame
		if !w.Heard {
			*w = Word{Span: Span{Start: start, End: end}, Heard: true}
		}
		w.End = end
		sums[at.line][at.word] += math.Exp(at.logProb)
		counts[at.line][at.word]++
	}
	for i := range result {
		for j := range result[i] {
			if counts[i][j] > 0 {
				result[i][j].Confidence = sums[i][j] / float64(counts[i][j])
			}
		}
	}
	return result, nil
}

// pathState is a state of the best path through the lines: outside every
// line, a line's blank between two of its letters, or one of its letters.
type pathState struct {
	kind  stateKind
	label int // the model's output heard in it, for a blank or a letter
	// line and word own a letter; word is -1 for the delimiter between words,
	// and for a blank or a frame outside the lines.
	line, word int
}

type stateKind int8

const (
	outsideLines stateKind = iota
	blankState
	letterState
)

// step is one frame of the best path: the line and word of the letter it is
// in (word -1 elsewhere) and how likely the model found it.
type step struct {
	line, word int
	logProb    float64
}

// Moves into a state of the best path: staying, coming from the state
// before, or skipping the blank between two different letters.
const (
	stay int8 = iota
	advance
	skipBlank
)

// pathStates spells the lines into the states of the best path: outside
// before the first line, between two lines and after the last; inside a
// line its letters, the delimiter between its words and a blank between two
// letters (the same letter twice in a row needs it). A line with no letter
// of the alphabet has no states; letters tells how many there are in all.
func (a Alphabet) pathStates(lines [][]string) (states []pathState, needed, letters int) {
	states = []pathState{{kind: outsideLines, word: -1}}
	for i, words := range lines {
		tokens, owners := a.spell(words)
		if len(tokens) == 0 {
			continue
		}
		for j, token := range tokens {
			if j > 0 {
				states = append(states, pathState{kind: blankState, label: a.blank, line: i, word: -1})
				if token == tokens[j-1] {
					needed++
				}
			}
			states = append(states, pathState{kind: letterState, label: token, line: i, word: owners[j]})
		}
		states = append(states, pathState{kind: outsideLines, word: -1})
		letters += len(tokens)
		needed += len(tokens) + 1
	}
	return states, needed - 1, letters
}

// lettersPath finds the most likely way through the lines' letters (Viterbi
// over CTC's states: a letter may last several frames, a blank may come
// between two letters). The sound around and between lines may hold other
// voices, music or silence, so a line may start and end at any frame, and a
// frame outside the lines is whatever the model heard most there: a silence
// is its blank, a neighbour's voice its own letter. Were those frames the
// line's blank, the path would pay for every frame of a neighbour's voice
// and find it cheaper to pull the line's last letters onto it (a "flowing"
// ending on the next line's "thank"). On a tie a line keeps the frame, so a
// letter heard surely is never cut short.
func (a Alphabet) lettersPath(logits [][]float32, lines [][]string) ([]step, error) {
	states, needed, letters := a.pathStates(lines)
	if letters == 0 {
		return nil, ErrNothingToAlign
	}
	frames := len(logits)
	if frames < needed {
		return nil, ErrTooShort
	}
	for _, row := range logits {
		if len(row) < a.size {
			return nil, ErrAlphabet
		}
	}
	logProbs := logSoftmax(logits)
	heard := func(t, s int) float64 {
		if states[s].kind == outsideLines {
			return slices.Max(logProbs[t])
		}
		return logProbs[t][states[s].label]
	}
	impossible := math.Inf(-1)
	previous, current := make([]float64, len(states)), make([]float64, len(states))
	for s := range previous {
		previous[s] = impossible
	}
	previous[0] = 0 // before the first frame, the path is outside the lines
	back := make([][]int8, frames)
	for t := range frames {
		back[t] = make([]int8, len(states))
		for s, state := range states {
			best, from := previous[s], stay
			// Leaving a line on a tie keeps its last letter a frame longer.
			if s >= 1 && (previous[s-1] > best || state.kind == outsideLines && previous[s-1] == best) {
				best, from = previous[s-1], advance
			}
			if state.kind == letterState && s >= 2 && states[s-1].kind == blankState &&
				states[s-2].label != state.label && previous[s-2] > best {
				best, from = previous[s-2], skipBlank
			}
			current[s] = best + heard(t, s)
			back[t][s] = from
		}
		previous, current = current, previous
	}
	last := len(states) - 1
	s := last
	if previous[last-1] >= previous[last] {
		s = last - 1 // a line ending with the sound keeps its last frame
	}
	if math.IsInf(previous[s], -1) {
		return nil, ErrTooShort
	}
	path := make([]step, frames)
	for t := frames - 1; t >= 0; t-- {
		path[t] = step{line: states[s].line, word: -1}
		if states[s].kind == letterState {
			path[t].word, path[t].logProb = states[s].word, heard(t, s)
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
