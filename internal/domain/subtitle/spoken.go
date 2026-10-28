package subtitle

import (
	"regexp"
	"strings"
)

// unspoken is what a line carries besides what is said: ASS overrides
// ({\i1}), HTML-like tags (<i>), and sounds described for the deaf, in
// brackets or parentheses ([door slams], (laughs)).
var unspoken = regexp.MustCompile(`\{[^}]*\}|<[^>]*>|\[[^\]]*\]|\([^)]*\)`)

// SpokenWords are a line's words as they are said: its markup, its line
// breaks, its described sounds and the dashes that open a speaker's turn
// left out. A sung line (♪) says nothing a speech model can follow, so it
// has no spoken words.
func SpokenWords(text string) []string {
	if strings.ContainsAny(text, "♪♫") {
		return nil
	}
	text = strings.NewReplacer(`\N`, " ", `\n`, " ", "\n", " ").Replace(text)
	var words []string
	for word := range strings.FieldsSeq(unspoken.ReplaceAllString(text, " ")) {
		if strings.Trim(word, "-–—") != "" {
			words = append(words, word)
		}
	}
	return words
}
