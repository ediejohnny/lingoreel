package subtitle_test

import (
	"slices"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func TestALinesSpokenWordsLeaveOutWhatIsNotSaid(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		`{\i1}Where are you going?{\i0}`:         {"Where", "are", "you", "going?"},
		"<i>Come back</i>\\Nright now.":          {"Come", "back", "right", "now."},
		"- I can't.\n- Yes, you can.":            {"I", "can't.", "Yes,", "you", "can."},
		"[door slams] Who's there? (whispering)": {"Who's", "there?"},
		"♪ Happy birthday to you ♪":              nil,
		"[thunder]":                              nil,
		"Bem-vindo — disse ele.":                 {"Bem-vindo", "disse", "ele."},
	}
	for text, want := range cases {
		if got := subtitle.SpokenWords(text); !slices.Equal(got, want) {
			t.Errorf("SpokenWords(%q) = %q, want %q", text, got, want)
		}
	}
}
