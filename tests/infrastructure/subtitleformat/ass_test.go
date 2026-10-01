package subtitleformat_test

import (
	"errors"
	"testing"

	"lingoreel/internal/infrastructure/subtitleformat"
)

const assDocument = `[Script Info]
Title: Sample
ScriptType: v4.00+

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour
Style: Default,Arial,20,&H00FFFFFF

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Comment: 0,0:00:00.00,0:00:05.00,Default,,0,0,0,,this is a comment
Dialogue: 0,0:00:01.00,0:00:03.50,Default,,0,0,0,,{\i1}Well,{\i0} I'm gonna go.
Dialogue: 0,0:00:04.00,0:00:06.00,Default,,0,0,0,,First line\NSecond\hline
Dialogue: 1,0:00:04.00,0:00:06.00,Sign,,0,0,0,,{\p1}m 0 0 l 100 0 100 100 0 100{\p0}EXIT
Dialogue: 0,0:00:07.00,0:00:08.00,Default,,0,0,0,,{\pos(10,20)\b1}Bold at a position
Dialogue: 0,0:00:09.00,0:00:10.00,Default,,0,0,0,,{\p1}m 0 0 l 1 1
Dialogue: 0,broken,0:00:12.00,Default,,0,0,0,,bad start
`

func TestASSParsesDialogueAndStripsOverrides(t *testing.T) {
	result := mustParse(t, subtitleformat.ASSParser{}, assDocument)

	assertEvents(t, result.Events, []wantEvent{
		{1_000, 3_500, "Well, I'm gonna go."},
		{4_000, 6_000, "First line\nSecond line"},
		{4_000, 6_000, "EXIT"},
		{7_000, 8_000, "Bold at a position"},
	})
	if result.Skipped != 2 {
		t.Fatalf("Skipped = %d, want 2 (drawing only, bad start)", result.Skipped)
	}
}

// In ASS, \N always breaks the line, but the lowercase \n is a soft break:
// it only breaks under WrapStyle 2 and reads as a space otherwise.
func TestASSLowercaseBreakFollowsTheWrapStyle(t *testing.T) {
	events := "[Events]\nFormat: Start, End, Text\nDialogue: 0:00:01.00,0:00:02.00,one\\ntwo\\Nthree\n"

	soft := mustParse(t, subtitleformat.ASSParser{}, "[Script Info]\nWrapStyle: 0\n\n"+events)
	assertEvents(t, soft.Events, []wantEvent{{1_000, 2_000, "one two\nthree"}})

	noStyle := mustParse(t, subtitleformat.ASSParser{}, events)
	assertEvents(t, noStyle.Events, []wantEvent{{1_000, 2_000, "one two\nthree"}})

	hard := mustParse(t, subtitleformat.ASSParser{}, "[Script Info]\nwrapstyle:  2 \n\n"+events)
	assertEvents(t, hard.Events, []wantEvent{{1_000, 2_000, "one\ntwo\nthree"}})
}

func TestASSHonorsCustomFormatOrder(t *testing.T) {
	document := "[Events]\nFormat: Start, End, Text\nDialogue: 0:00:01.5,0:00:02.25,text, with, commas\n"

	result := mustParse(t, subtitleformat.ASSParser{}, document)

	assertEvents(t, result.Events, []wantEvent{{1_500, 2_250, "text, with, commas"}})
}

func TestASSReadsLegacySSAMarkedField(t *testing.T) {
	document := "[Events]\nFormat: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: Marked=0,0:00:01.00,0:00:02.00,Default,,0000,0000,0000,,old style\n"

	result := mustParse(t, subtitleformat.ASSParser{}, document)

	assertEvents(t, result.Events, []wantEvent{{1_000, 2_000, "old style"}})
}

func TestASSSkipsDialogueWithTooFewFields(t *testing.T) {
	document := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default\n"

	result := mustParse(t, subtitleformat.ASSParser{}, document)

	if len(result.Events) != 0 || result.Skipped != 1 {
		t.Fatalf("got %d events, %d skipped; want 0 and 1", len(result.Events), result.Skipped)
	}
}

func TestASSKeepsUnterminatedOverrideAsText(t *testing.T) {
	document := "[Events]\nFormat: Start, End, Text\nDialogue: 0:00:01.00,0:00:02.00,Hello {world\n"

	result := mustParse(t, subtitleformat.ASSParser{}, document)

	assertEvents(t, result.Events, []wantEvent{{1_000, 2_000, "Hello {world"}})
}

func TestASSWithoutEventsSectionIsMalformed(t *testing.T) {
	_, err := subtitleformat.ASSParser{}.Parse([]byte("[Script Info]\nTitle: nothing here\n"))
	if !errors.Is(err, subtitleformat.ErrMalformed) {
		t.Fatalf("error = %v, want ErrMalformed", err)
	}
}
