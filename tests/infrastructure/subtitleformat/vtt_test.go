package subtitleformat_test

import (
	"errors"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitleformat"
)

func TestWebVTTParsesCuesAndIgnoresMetadata(t *testing.T) {
	t.Parallel()
	document := "WEBVTT - sample file\nKind: captions\n\n" +
		"NOTE this is a comment\nspanning lines\n\n" +
		"STYLE\n::cue { color: yellow }\n\n" +
		"REGION\nid:fred\n\n" +
		"intro\n00:01.000 --> 00:04.000 align:start position:10%\n<v Roger>Hello &amp; <b>welcome</b>!\n\n" +
		"00:00:05.000 --> 00:00:07.500\nKaraoke <00:00:06.000>time\n"

	result := mustParse(t, subtitleformat.WebVTTParser{}, document)

	assertEvents(t, result.Events, []wantEvent{
		{1_000, 4_000, "Hello & welcome!"},
		{5_000, 7_500, "Karaoke time"},
	})
}

func TestWebVTTRequiresSignature(t *testing.T) {
	t.Parallel()
	for _, document := range []string{"", "WEBVTTX\n\n00:01.000 --> 00:02.000\nx\n", "1\n00:00:01,000 --> 00:00:02,000\nx\n", "webvtt\n"} {
		if _, err := (subtitleformat.WebVTTParser{}).Parse([]byte(document)); !errors.Is(err, subtitleformat.ErrMalformed) {
			t.Fatalf("Parse(%q) error = %v, want ErrMalformed", document, err)
		}
	}
}

func TestWebVTTSkipsBlocksWithoutTiming(t *testing.T) {
	t.Parallel()
	document := "WEBVTT\n\njust text\nwithout timing\n\n00:01.000 --> 00:02.000\nok\n"

	result := mustParse(t, subtitleformat.WebVTTParser{}, document)

	assertEvents(t, result.Events, []wantEvent{{1_000, 2_000, "ok"}})
	if result.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", result.Skipped)
	}
}

func TestWebVTTNoteWordInsideCueTextIsNotMetadata(t *testing.T) {
	t.Parallel()
	document := "WEBVTT\n\n00:01.000 --> 00:02.000\nNOTE the time\n"

	result := mustParse(t, subtitleformat.WebVTTParser{}, document)

	assertEvents(t, result.Events, []wantEvent{{1_000, 2_000, "NOTE the time"}})
}
