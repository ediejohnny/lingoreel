package media_test

import (
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

func TestHintsFromFileName(t *testing.T) {
	t.Parallel()
	const video = "Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.mkv"
	cases := []struct {
		file string
		want media.FileHints
	}{
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.en.srt", media.FileHints{Language: "en"}},
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.eng.srt", media.FileHints{Language: "en"}},
		{"filme.exemplo.2001.bluray.1080p.x264.dual-grupo.PT-BR.srt", media.FileHints{Language: "pt-BR"}},
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.pt_br.forced.srt", media.FileHints{Language: "pt-BR", Forced: true}},
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.en.sdh.srt", media.FileHints{Language: "en", HearingImpaired: true}},
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.en.hi.srt", media.FileHints{Language: "en", HearingImpaired: true}},
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.por.srt", media.FileHints{Language: "pt"}},
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.es-419.srt", media.FileHints{Language: "es-419"}},
		// Regions only the next token gives (no name hint knows them).
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.fr-CA.srt", media.FileHints{Language: "fr-CA"}},
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.de_at.srt", media.FileHints{Language: "de-AT"}},
		// YTS style, inside Subs/.
		{"2_English.srt", media.FileHints{Language: "en"}},
		{"3_Portuguese (Brazil).srt", media.FileHints{Language: "pt-BR"}},
		{"English (SDH).srt", media.FileHints{Language: "en", HearingImpaired: true}},
		{"Brazilian.Portuguese.srt", media.FileHints{Language: "pt-BR"}},
		{"Legendas em Português - Forçadas.ass", media.FileHints{Language: "pt", Forced: true}},
		{"Inglês.vtt", media.FileHints{Language: "en"}},
		// Nothing about the language.
		{"Filme.Exemplo.2001.BluRay.1080p.x264.DUAL-GRUPO.srt", media.FileHints{}},
		{"subtitle.srt", media.FileHints{}},
		{"", media.FileHints{}},
	}
	for _, c := range cases {
		if got := media.HintsFromFileName(c.file, video); got != c.want {
			t.Errorf("HintsFromFileName(%q) = %+v, want %+v", c.file, got, c.want)
		}
	}
}

// Words of the video name must never be read as the language: "Quillon" is
// not a code, but "It" (Italian's code) and "Hi" (hearing impaired) are
// words that appear in titles.
func TestVideoNameWordsAreIgnored(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"It.Was.Raining.2017.srt":   "It.Was.Raining.2017.mkv",
		"Say.Hi.To.The.Bad.Guy.srt": "Say.Hi.To.The.Bad.Guy.mkv",
		"The.Portuguese.Job.srt":    "The.Portuguese.Job.mkv",
		"ENGLISH.ROSE.GARDEN.srt":   "English.Rose.Garden.mkv",
	}
	for file, video := range cases {
		if got := media.HintsFromFileName(file, video); got != (media.FileHints{}) {
			t.Errorf("HintsFromFileName(%q, %q) = %+v, want nothing", file, video, got)
		}
	}
	// With the language after the title, the title still does not count.
	if got := media.HintsFromFileName("The.Portuguese.Job.en.srt", "The.Portuguese.Job.mkv"); got.Language != "en" {
		t.Errorf("language = %q, want en", got.Language)
	}
}

func TestShortWordsAreNotLanguages(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"to.srt", "of the day.srt", "a.b.c.srt", "xx.srt", "zzz.srt", "und.srt", "12345.srt"} {
		if got := media.HintsFromFileName(file, "Movie.mkv"); got.Language != "" {
			t.Errorf("HintsFromFileName(%q) language = %q, want none", file, got.Language)
		}
	}
}

func TestHintsSurviveHostileNames(t *testing.T) {
	t.Parallel()
	hostile := []string{
		strings.Repeat("en.", 5000) + "srt",
		"....srt",
		"\x00\x01.srt",
		"💥.pt.srt",
		`..\..\Windows\en.srt`,
	}
	for _, file := range hostile {
		media.HintsFromFileName(file, "Movie.mkv") // must not panic
	}
	if got := media.HintsFromFileName("💥.pt.srt", "Movie.mkv"); got.Language != "pt" {
		t.Errorf("emoji name: language = %q, want pt", got.Language)
	}
}
