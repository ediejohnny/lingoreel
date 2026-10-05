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
	cases := []struct {
		name     string
		fileName string
		content  string
		want     subtitle.Format
		wantOK   bool
	}{
		{"srt extension", "Movie.PT-BR.SRT", "", subtitle.FormatSRT, true},
		{"ass extension", "movie.ass", "", subtitle.FormatASS, true},
		{"ssa extension", "movie.ssa", "", subtitle.FormatSSA, true},
		{"vtt extension", "movie.vtt", "", subtitle.FormatWebVTT, true},
		{"sniff webvtt", "movie.txt", "WEBVTT\n\n00:01.000 --> 00:02.000\nx", subtitle.FormatWebVTT, true},
		{"sniff ass", "movie.sub", "[Script Info]\nTitle: x", subtitle.FormatASS, true},
		{"sniff srt", "", "1\n00:00:01,000 --> 00:00:02,000\nx", subtitle.FormatSRT, true},
		{"unknown", "notes.txt", "hello", "", false},
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
	if _, err := subtitleformat.BlockText(subtitle.FormatASS, []byte("only,three,fields")); !errors.Is(err, subtitleformat.ErrMalformed) {
		t.Fatalf("broken ASS block error = %v, want ErrMalformed", err)
	}
	if _, err := subtitleformat.BlockText("pgs", []byte("x")); !errors.Is(err, subtitleformat.ErrUnsupportedFormat) {
		t.Fatalf("unknown format error = %v, want ErrUnsupportedFormat", err)
	}
}
