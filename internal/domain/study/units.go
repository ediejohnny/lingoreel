package study

import (
	"strings"
	"unicode"
)

// Break is the pause after a word: none, a comma's, a sentence's end, or
// another person starting to speak.
type Break int

const (
	BreakNone Break = iota
	BreakComma
	BreakStop
	BreakSpeaker
)

// Unit is a word of a line as it is spoken.
type Unit struct {
	// Chars is how many of the line's visible characters (spaces left out,
	// as the fill counts them) the word takes, its punctuation included.
	Chars int
	// Weight is how long it takes to say, in syllables; zero for what is not
	// spoken: a sound's description, a speaker's name, punctuation alone.
	Weight float64
	// Break is the pause after it.
	Break Break
}

// Units is a line as it is spoken, word by word.
type Units struct {
	Words []Unit
	// Lyrics marks a sung line (between ♪): music under the voice.
	Lyrics bool
}

// Spoken tells whether anything in the line is said aloud.
func (u Units) Spoken() bool {
	for _, word := range u.Words {
		if word.Weight > 0 {
			return true
		}
	}
	return false
}

// Speech timing: a word takes its syllables and a little more; a stretched
// sound ("Nooooo!") takes longer, within a limit.
const (
	wordWeight      = 0.3
	stretchWeight   = 0.25 // per letter past two of the same
	stretchMaxExtra = 2.0
)

// SpeechUnits splits a line into words as they are spoken. Subtitles for the
// deaf describe sounds ("[DOOR SLAMS]", "(laughs)") and name who speaks
// ("JOHN:"): those take no time. A hyphen inside a word splits syllables
// ("Ia-ba-da-ba-duuu"), never a pause; a hyphen opening a line is another
// person speaking.
func SpeechUnits(text string) Units {
	var units Units
	if strings.ContainsAny(text, "♪♫") {
		units.Lyrics = true
	}
	depth := 0 // inside [...] or (...)
	for line := range strings.SplitSeq(text, "\n") {
		tokens := strings.Fields(line)
		// A dash opening the line: someone else speaks.
		first := 0
		for first < len(tokens) && strings.Trim(tokens[first], "-–—") == "" {
			first++
		}
		if first > 0 || (first < len(tokens) && strings.HasPrefix(tokens[first], "-")) {
			units.markSpeaker()
		}
		labelEnd := first + speakerLabel(tokens[first:])
		for i, token := range tokens {
			unit := Unit{Chars: len([]rune(token))}
			silent := depth > 0 || i < labelEnd
			for _, r := range token {
				switch r {
				case '[', '(':
					depth++
					silent = true
				case ']', ')':
					if depth > 0 {
						depth--
					}
				}
			}
			if !silent {
				unit.Weight = syllables(token)
				if unit.Weight > 0 {
					unit.Weight += wordWeight
				}
			}
			unit.Break = breakAfter(token)
			if unit.Weight == 0 && unit.Break != BreakNone {
				// Punctuation standing alone ("How ...") pauses after the word before.
				units.mergeBreak(unit.Break)
				unit.Break = BreakNone
			}
			units.Words = append(units.Words, unit)
		}
	}
	return units
}

// markSpeaker turns the pause after the last word into another person's turn.
func (u *Units) markSpeaker() {
	for i := len(u.Words) - 1; i >= 0; i-- {
		if u.Words[i].Weight > 0 {
			u.Words[i].Break = BreakSpeaker
			return
		}
	}
}

// mergeBreak gives the last spoken word the longer of its pause and another.
func (u *Units) mergeBreak(b Break) {
	for i := len(u.Words) - 1; i >= 0; i-- {
		if u.Words[i].Weight > 0 {
			u.Words[i].Break = max(u.Words[i].Break, b)
			return
		}
	}
}

// speakerLabel is how many words a line opens with naming who speaks: one
// to three words in capitals, the last ending with a colon ("JOHN:",
// "MAN 2:"); zero for none. A shout like "WAIT: what?" reads the same way.
func speakerLabel(tokens []string) int {
	for end := 0; end < len(tokens) && end < 3; end++ {
		word := tokens[end]
		if !hasCapitals(strings.TrimSuffix(word, ":")) {
			return 0
		}
		if strings.HasSuffix(word, ":") {
			return end + 1
		}
	}
	return 0
}

// hasCapitals tells a word written in capitals: letters, all upper case
// (digits allowed: "MAN 2").
func hasCapitals(word string) bool {
	letters := 0
	for _, r := range word {
		if unicode.IsLetter(r) {
			if !unicode.IsUpper(r) {
				return false
			}
			letters++
		} else if !unicode.IsDigit(r) && !strings.ContainsRune("'-.", r) {
			return false
		}
	}
	return letters > 0 || strings.IndexFunc(word, unicode.IsDigit) >= 0
}

// breakAfter is the pause a word's last punctuation calls for: repeated
// marks ("!!", "?!", "...") count once.
func breakAfter(token string) Break {
	trimmed := strings.TrimRight(token, "\"'”’»)]")
	switch {
	case strings.HasSuffix(trimmed, "…"), strings.HasSuffix(trimmed, "."), strings.HasSuffix(trimmed, "!"), strings.HasSuffix(trimmed, "?"):
		return BreakStop
	case strings.HasSuffix(trimmed, ","), strings.HasSuffix(trimmed, ";"), strings.HasSuffix(trimmed, ":"), strings.HasSuffix(trimmed, "—"), strings.HasSuffix(trimmed, "–"):
		return BreakComma
	}
	return BreakNone
}

// syllables estimates how many syllables a word has: its groups of vowels,
// each piece between hyphens at least one, a silent final e dropped, and a
// stretched sound counted longer. Zero for a token with no letters.
func syllables(token string) float64 {
	total := 0.0
	letters := false
	for _, piece := range strings.FieldsFunc(strings.ToLower(token), func(r rune) bool { return r == '-' || r == '–' || r == '—' }) {
		groups, inVowel, count := 0, false, 0
		var word []rune
		for _, r := range piece {
			if !unicode.IsLetter(r) {
				continue
			}
			word = append(word, r)
			if isVowel(r) {
				if !inVowel {
					groups++
				}
				inVowel = true
			} else {
				inVowel = false
			}
		}
		if len(word) == 0 {
			continue
		}
		letters = true
		count = groups
		// English drops a final e after a consonant ("make", "time").
		if n := len(word); count > 1 && word[n-1] == 'e' && !isVowel(word[n-2]) {
			count--
		}
		total += float64(max(count, 1)) + stretched(word)
	}
	if !letters {
		return 0
	}
	return total
}

// stretched is the extra time of a sound held longer than written: each
// letter past the second in a run of the same letter ("duuuu").
func stretched(word []rune) float64 {
	extra, run := 0.0, 1
	for i := 1; i <= len(word); i++ {
		if i < len(word) && word[i] == word[i-1] {
			run++
			continue
		}
		if run > 2 {
			extra += float64(run-2) * stretchWeight
		}
		run = 1
	}
	return min(extra, stretchMaxExtra)
}

func isVowel(r rune) bool {
	return strings.ContainsRune("aeiouyáéíóúâêîôûãõàèìòùäëïöüåæœ", r)
}
