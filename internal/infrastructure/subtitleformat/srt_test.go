package subtitleformat

import "testing"

func TestSRTParsesWellFormedDocument(t *testing.T) {
	document := "1\r\n00:10:41,120 --> 00:10:43,800\r\nI'm not going there.\r\n\r\n" +
		"2\r\n00:10:44,000 --> 00:10:46,500\r\n<i>Why not?</i>\r\nTell me.\r\n"

	result := mustParse(t, SRTParser{}, document)

	assertEvents(t, result.Events, []wantEvent{
		{641_120, 643_800, "I'm not going there."},
		{644_000, 646_500, "Why not?\nTell me."},
	})
	if result.Skipped != 0 {
		t.Fatalf("Skipped = %d, want 0", result.Skipped)
	}
	if result.Events[0].ID != "1" || result.Events[1].ID != "2" {
		t.Fatalf("IDs = %q, %q; want document order", result.Events[0].ID, result.Events[1].ID)
	}
}

func TestSRTToleratesCommonDefects(t *testing.T) {
	document := "\xEF\xBB\xBF" + // UTF-8 byte order mark
		"00:00:01.000 --> 00:00:02.000\n" + // no index, dot separator
		"no index\n" +
		"\n\n\n" +
		"x7\n" + // non-numeric index
		"00:00:03,000 --> 00:00:04,000 X1:10 X2:20 Y1:30 Y2:40\n" + // position coordinates
		"{\\an8}top line\n" +
		"4\n" + // next cue without a blank line
		"00:00:05,000 --> 00:00:06,000\n" +
		"<font color=\"#ffff00\">yellow</font>\n"

	result := mustParse(t, SRTParser{}, document)

	assertEvents(t, result.Events, []wantEvent{
		{1_000, 2_000, "no index"},
		{3_000, 4_000, "top line"},
		{5_000, 6_000, "yellow"},
	})
}

func TestSRTSkipsBrokenCuesAndKeepsGoing(t *testing.T) {
	document := "1\n00:00:01,000 --> 00:00:02,000\nfirst\n\n" +
		"garbage line\nmore garbage\n\n" +
		"2\n00:00:09,000 --> 00:00:03,000\nends before it starts\n\n" +
		"3\n00:00:04,000 --> 00:00:05,000\n\n" + // no text
		"4\n00:00:61,000 --> 00:00:62,000\nbad seconds\n\n" +
		"5\n00:00:07,000 --> 00:00:08,000\nlast\n"

	result := mustParse(t, SRTParser{}, document)

	assertEvents(t, result.Events, []wantEvent{
		{1_000, 2_000, "first"},
		{7_000, 8_000, "last"},
	})
	if result.Skipped != 4 {
		t.Fatalf("Skipped = %d, want 4 (garbage, inverted, empty, bad seconds)", result.Skipped)
	}
}

func TestSRTDropsExactDuplicates(t *testing.T) {
	document := "1\n00:00:01,000 --> 00:00:02,000\nsame\n\n2\n00:00:01,000 --> 00:00:02,000\nsame\n"

	result := mustParse(t, SRTParser{}, document)

	if len(result.Events) != 1 || result.Skipped != 1 {
		t.Fatalf("got %d events and %d skipped, want 1 and 1", len(result.Events), result.Skipped)
	}
}

func TestSRTKeepsLiteralBracesThatAreNotOverrides(t *testing.T) {
	document := "1\n00:00:01,000 --> 00:00:02,000\n{laughs} a < b\n"

	result := mustParse(t, SRTParser{}, document)

	assertEvents(t, result.Events, []wantEvent{{1_000, 2_000, "{laughs} a < b"}})
}

func TestSRTReadsWindows1252Portuguese(t *testing.T) {
	document := []byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nVoc\xea n\xe3o vai\x85\r\n")

	result, err := SRTParser{}.Parse(document)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	assertEvents(t, result.Events, []wantEvent{{1_000, 2_000, "Você não vai…"}})
}
