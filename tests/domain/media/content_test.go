package media_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// What a file says inside decides its language: a title word read as a code
// ("Ice", Icelandic's) or a wrong name loses to the text; the name only adds
// its region, or speaks when the text cannot tell.
func TestTheTextDecidesTheLanguage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		named media.LanguageTag
		text  string
		want  media.LanguageTag
	}{
		{"is", englishDialogue, "en"},
		{"pt", englishDialogue, "en"},
		{"", englishDialogue, "en"},
		{"pt-BR", portugueseDialogue, "pt-BR"},
		{"ja", "Ok.", "ja"},
		{"", "Ok.", ""},
	}
	for _, c := range cases {
		if got := media.SubtitleLanguage(c.named, c.text); got != c.want {
			t.Errorf("SubtitleLanguage(%q, %.20q…) = %q, want %q", c.named, c.text, got, c.want)
		}
	}
}

func spoken(texts ...string) []subtitle.Event {
	events := make([]subtitle.Event, len(texts))
	for i, text := range texts {
		events[i] = subtitle.Event{Start: time.Duration(i) * time.Second, End: time.Duration(i)*time.Second + 900*time.Millisecond, OriginalText: text}
	}
	return events
}

func repeated(text string, times int) []string {
	texts := make([]string, times)
	for i := range texts {
		texts[i] = text
	}
	return texts
}

// A subtitle for the hearing impaired is told by its lines: enough of them
// describe sounds. A few in an ordinary subtitle, or a parenthesis in the
// middle of a sentence, do not make one.
func TestSoundDescriptionsTellASubtitleForTheHearingImpaired(t *testing.T) {
	t.Parallel()
	dialogue := repeated("Where's the line?", 80)
	cases := map[string]struct {
		texts []string
		want  bool
	}{
		"a fifth in parentheses": {append(repeated("(BIRDS CHIRPING)", 20), dialogue...), true},
		"a fifth in brackets":    {append(repeated("Run! [DOOR SLAMS]", 20), dialogue...), true},
		"after a dash":           {append(repeated("- (laughs) Yes.", 20), dialogue...), true},
		"a few":                  {append(repeated("(GASPS)", 3), dialogue...), false},
		"mid-sentence":           {append(repeated("I said (quietly) no.", 20), dialogue...), false},
		"none at all":            {nil, false},
	}
	for name, c := range cases {
		if got := media.DescribesSounds(spoken(c.texts...)); got != c.want {
			t.Errorf("%s: DescribesSounds = %v, want %v", name, got, c.want)
		}
	}
}

// A forced subtitle carries only a few lines for its video, whatever its
// name; a short video says nothing by its count.
func TestFewLinesTellAForcedSubtitle(t *testing.T) {
	t.Parallel()
	film := 95 * time.Minute
	switch {
	case !media.LooksForced(spoken(repeated("¡Hola!", 40)...), film):
		t.Error("40 lines in a 95-minute film are not forced")
	case media.LooksForced(spoken(repeated("Hello.", 1100)...), film):
		t.Error("1100 lines in a 95-minute film are forced")
	case media.LooksForced(spoken(repeated("Hello.", 5)...), 10*time.Minute):
		t.Error("a 10-minute video tells a forced subtitle by its count")
	case media.LooksForced(spoken(repeated("Hello.", 5)...), 0):
		t.Error("a video of unknown length tells a forced subtitle by its count")
	}
}
