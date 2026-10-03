package ptbr_test

import (
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/presentation/ptbr"
)

func TestLanguageName(t *testing.T) {
	cases := map[media.LanguageTag]string{
		"en": "inglês", "en-US": "inglês", "pt": "português", "pt-BR": "português do Brasil",
		"pt-PT": "português de Portugal", "es": "espanhol", "": "idioma desconhecido", "tlh": "tlh",
	}
	for tag, want := range cases {
		if got := ptbr.LanguageName(tag); got != want {
			t.Errorf("LanguageName(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestOffset(t *testing.T) {
	cases := map[time.Duration]string{
		0:                         "+0,000 s",
		150 * time.Millisecond:    "+0,150 s",
		-150 * time.Millisecond:   "−0,150 s",
		-12500 * time.Millisecond: "−12,500 s",
	}
	for offset, want := range cases {
		if got := ptbr.Offset(subtitle.Offset(offset)); got != want {
			t.Errorf("Offset(%v) = %q, want %q", offset, got, want)
		}
	}
}

func TestClock(t *testing.T) {
	cases := map[time.Duration]string{
		0:                                     "0:00",
		59*time.Second + 999*time.Millisecond: "0:59",
		61 * time.Second:                      "1:01",
		time.Hour + 29*time.Minute + 32*time.Second: "1:29:32",
		-time.Second: "0:00",
	}
	for d, want := range cases {
		if got := ptbr.Clock(d); got != want {
			t.Errorf("Clock(%v) = %q, want %q", d, got, want)
		}
	}
}

// A passage saved by its shortcut is named by its times, the way the panel
// writes them, so ParseClock reads each side back.
func TestPassageName(t *testing.T) {
	cases := []struct {
		start, end time.Duration
		want       string
	}{
		{10*time.Minute + 38*time.Second, 11*time.Minute + 30*time.Second, "Trecho 10:38–11:30"},
		{4 * time.Second, 9 * time.Second, "Trecho 0:04–0:09"},
		{time.Hour + 10*time.Minute + 23*time.Second, time.Hour + 11*time.Minute + 4*time.Second, "Trecho 1:10:23–1:11:04"},
		// Whole seconds, to the nearest: a start fitted half a second back
		// keeps the second the user marked.
		{10*time.Minute + 37*time.Second + 500*time.Millisecond, 11*time.Minute + 30*time.Second, "Trecho 10:38–11:30"},
		{2400 * time.Millisecond, 4 * time.Second, "Trecho 0:02–0:04"},
	}
	for _, c := range cases {
		if got := ptbr.PassageName(c.start, c.end); got != c.want {
			t.Errorf("PassageName(%v, %v) = %q, want %q", c.start, c.end, got, c.want)
		}
	}
	if got := ptbr.PassageMarkNotice(10*time.Minute + 38*time.Second); got != "Início do trecho: 10:38" {
		t.Errorf("PassageMarkNotice = %q", got)
	}
	if got := ptbr.PassageSavedNotice("Trecho 10:38–11:30"); got != "Salvo: Trecho 10:38–11:30" {
		t.Errorf("PassageSavedNotice = %q", got)
	}
	marks := map[watch.MarkEvent]string{
		watch.MarkStarted:   "Início do trecho: 10:38",
		watch.MarkNoStart:   "Marque o início do trecho primeiro",
		watch.MarkTooShort:  "O fim precisa vir pelo menos 1 s depois do início",
		watch.MarkEvent(99): "",
	}
	for event, want := range marks {
		if got := ptbr.MarkNotice(event, study.Segment{Start: 10*time.Minute + 38*time.Second}); got != want {
			t.Errorf("MarkNotice(%d) = %q, want %q", event, got, want)
		}
	}
}

func TestLineNamesAndNotice(t *testing.T) {
	info := media.Info{Tracks: []media.Track{
		{Number: 3, Type: media.TrackSubtitle, Language: "en"},
		{Number: 5, Type: media.TrackSubtitle, Language: "pt-BR"},
		{Number: 6, Type: media.TrackSubtitle},
	}}
	names := ptbr.NewLineNames(info, 3, 5)
	if names[0] != "Legenda de cima (inglês)" || names[1] != "Legenda de baixo (português do Brasil)" {
		t.Fatalf("names = %q", names)
	}
	if got := names.Notice(session.LinePrimary, subtitle.Offset(-50*time.Millisecond)); got != "Legenda de cima (inglês): −0,050 s" {
		t.Fatalf("notice = %q", got)
	}
	unknown := ptbr.NewLineNames(info, 6, 99)
	if unknown[0] != "Legenda de cima" || unknown[1] != "Legenda de baixo" {
		t.Fatalf("names without a language = %q", unknown)
	}
	if got := names.Notice(session.Line(9), subtitle.Offset(time.Second)); got != "+1,000 s" {
		t.Fatalf("notice for an unknown line = %q", got)
	}
}
