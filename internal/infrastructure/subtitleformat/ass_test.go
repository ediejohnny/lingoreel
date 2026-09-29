package subtitleformat

import (
	"errors"
	"testing"
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
	result := mustParse(t, ASSParser{}, assDocument)

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

func TestASSHonorsCustomFormatOrder(t *testing.T) {
	document := "[Events]\nFormat: Start, End, Text\nDialogue: 0:00:01.5,0:00:02.25,text, with, commas\n"

	result := mustParse(t, ASSParser{}, document)

	assertEvents(t, result.Events, []wantEvent{{1_500, 2_250, "text, with, commas"}})
}

func TestASSReadsLegacySSAMarkedField(t *testing.T) {
	document := "[Events]\nFormat: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: Marked=0,0:00:01.00,0:00:02.00,Default,,0000,0000,0000,,old style\n"

	result := mustParse(t, ASSParser{}, document)

	assertEvents(t, result.Events, []wantEvent{{1_000, 2_000, "old style"}})
}

func TestASSWithoutEventsSectionIsMalformed(t *testing.T) {
	_, err := ASSParser{}.Parse([]byte("[Script Info]\nTitle: nothing here\n"))
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("error = %v, want ErrMalformed", err)
	}
}
