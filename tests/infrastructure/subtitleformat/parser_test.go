package subtitleformat_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitleformat"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

func TestNewParserSelectsStrategyByFormat(t *testing.T) {
	t.Parallel()
	cases := map[subtitle.Format]subtitleformat.Parser{
		subtitle.FormatSRT:    subtitleformat.SRTParser{},
		subtitle.FormatASS:    subtitleformat.ASSParser{},
		subtitle.FormatSSA:    subtitleformat.ASSParser{},
		subtitle.FormatWebVTT: subtitleformat.WebVTTParser{},
	}
	for format, want := range cases {
		got, err := subtitleformat.NewParser(format)
		if err != nil || got != want {
			t.Fatalf("NewParser(%q) = %T, %v; want %T", format, got, err, want)
		}
	}
	for _, unknown := range []subtitle.Format{"pgs", "", "SRT"} {
		if _, err := subtitleformat.NewParser(unknown); !errors.Is(err, subtitleformat.ErrUnsupportedFormat) {
			t.Fatalf("NewParser(%q) error = %v, want ErrUnsupportedFormat", unknown, err)
		}
	}
}

func TestDetectFormat(t *testing.T) {
	t.Parallel()
	const (
		srt = "1\n00:00:01,000 --> 00:00:02,000\nx"
		ass = "[Script Info]\nTitle: x"
		vtt = "WEBVTT\n\n00:01.000 --> 00:02.000\nx"
	)
	cases := []struct {
		name     string
		fileName string
		content  string
		want     subtitle.Format
		wantOK   bool
	}{
		{"srt", "Movie.PT-BR.SRT", srt, subtitle.FormatSRT, true},
		{"ass", "movie.ass", ass, subtitle.FormatASS, true},
		{"ssa reads as ass, the name tells", "movie.ssa", ass, subtitle.FormatSSA, true},
		{"vtt", "movie.vtt", vtt, subtitle.FormatWebVTT, true},
		{"vtt renamed", "movie.txt", vtt, subtitle.FormatWebVTT, true},
		{"ass named srt", "movie.srt", ass, subtitle.FormatASS, true},
		{"no name", "", srt, subtitle.FormatSRT, true},
		{"utf-16 with its bom", "movie.srt", "\xff\xfe1\x00\n\x000\x000\x00:\x000\x000\x00:\x000\x001\x00,\x000\x000\x000\x00 \x00-\x00-\x00>\x00", subtitle.FormatSRT, true},
		// Not subtitles, whatever their name says.
		{"a page renamed", "movie.srt", "<html><script>alert(1)</script></html>", "", false},
		{"a script renamed", "movie.ass", "Remove-Item -Recurse C:\\", "", false},
		{"a program renamed", "movie.srt", "MZ\x90\x00\x03\x00\x00\x00 1\n00:00:01,000 --> 00:00:02,000\n", "", false},
		{"an empty srt", "movie.srt", "", "", false},
		{"text", "notes.txt", "hello", "", false},
		{"empty", "", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := subtitleformat.DetectFormat(tc.fileName, []byte(tc.content))
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("DetectFormat = %q,%v; want %q,%v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParsersRejectOversizedInputButAcceptTheLimit(t *testing.T) {
	t.Parallel()
	testkit.Slow(t)
	huge := bytes.Repeat([]byte("a"), subtitleformat.MaxInputBytes+1)
	for _, parser := range []subtitleformat.Parser{subtitleformat.SRTParser{}, subtitleformat.ASSParser{}, subtitleformat.WebVTTParser{}} {
		if _, err := parser.Parse(huge); !errors.Is(err, subtitleformat.ErrInputTooLarge) {
			t.Fatalf("%T.Parse(limit+1) error = %v, want ErrInputTooLarge", parser, err)
		}
	}
	if _, err := (subtitleformat.SRTParser{}).Parse(huge[:subtitleformat.MaxInputBytes]); err != nil {
		t.Fatalf("SRTParser rejected a document exactly at the limit: %v", err)
	}
}

func TestBlockText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		format  subtitle.Format
		payload string
		want    string
	}{
		{"matroska ASS block", subtitle.FormatASS, `12,0,Default,,0,0,0,,{\i1}Hi,{\i0} there\Nfriend`, "Hi, there\nfriend"},
		{"matroska SSA block", subtitle.FormatSSA, "3,Marked=0,Default,,0,0,0,,old text", "old text"},
		{"ASS block with drawing only", subtitle.FormatASS, `1,0,Sign,,0,0,0,,{\p1}m 0 0 l 5 5`, ""},
		{"SRT block", subtitle.FormatSRT, "<i>Hello</i>\r\nworld", "Hello\nworld"},
		{"WebVTT block", subtitle.FormatWebVTT, "<c.yellow>Tom &amp; Jerry</c>", "Tom & Jerry"},
		{"empty SRT block", subtitle.FormatSRT, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := subtitleformat.BlockText(tc.format, []byte(tc.payload))
			if err != nil || got != tc.want {
				t.Fatalf("BlockText = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestBlockTextRejectsBrokenASSAndUnknownFormats(t *testing.T) {
	t.Parallel()
	if _, err := subtitleformat.BlockText(subtitle.FormatASS, []byte("only,three,fields")); !errors.Is(err, subtitleformat.ErrMalformed) {
		t.Fatalf("broken ASS block error = %v, want ErrMalformed", err)
	}
	if _, err := subtitleformat.BlockText("pgs", []byte("x")); !errors.Is(err, subtitleformat.ErrUnsupportedFormat) {
		t.Fatalf("unknown format error = %v, want ErrUnsupportedFormat", err)
	}
}
