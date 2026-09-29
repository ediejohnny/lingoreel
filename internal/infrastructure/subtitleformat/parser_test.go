package subtitleformat

import (
	"bytes"
	"errors"
	"testing"

	"lingoreel/internal/domain/subtitle"
)

func TestNewParserSelectsStrategyByFormat(t *testing.T) {
	cases := map[subtitle.Format]Parser{
		subtitle.FormatSRT:    SRTParser{},
		subtitle.FormatASS:    ASSParser{},
		subtitle.FormatSSA:    ASSParser{},
		subtitle.FormatWebVTT: WebVTTParser{},
	}
	for format, want := range cases {
		got, err := NewParser(format)
		if err != nil || got != want {
			t.Fatalf("NewParser(%q) = %T, %v; want %T", format, got, err, want)
		}
	}
	if _, err := NewParser("pgs"); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("NewParser(pgs) error = %v, want ErrUnsupportedFormat", err)
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := DetectFormat(tc.fileName, []byte(tc.content))
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("DetectFormat = %q,%v; want %q,%v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParsersRejectOversizedInput(t *testing.T) {
	huge := bytes.Repeat([]byte("a"), MaxInputBytes+1)
	for _, parser := range []Parser{SRTParser{}, ASSParser{}, WebVTTParser{}} {
		if _, err := parser.Parse(huge); !errors.Is(err, ErrInputTooLarge) {
			t.Fatalf("%T.Parse(huge) error = %v, want ErrInputTooLarge", parser, err)
		}
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
		{"SRT block", subtitle.FormatSRT, "<i>Hello</i>\r\nworld", "Hello\nworld"},
		{"WebVTT block", subtitle.FormatWebVTT, "<c.yellow>Tom &amp; Jerry</c>", "Tom & Jerry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BlockText(tc.format, []byte(tc.payload))
			if err != nil || got != tc.want {
				t.Fatalf("BlockText = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestBlockTextRejectsBrokenASSAndUnknownFormats(t *testing.T) {
	if _, err := BlockText(subtitle.FormatASS, []byte("only,three,fields")); !errors.Is(err, ErrMalformed) {
		t.Fatalf("broken ASS block error = %v, want ErrMalformed", err)
	}
	if _, err := BlockText("pgs", []byte("x")); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("unknown format error = %v, want ErrUnsupportedFormat", err)
	}
}
