package matroska_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/infrastructure/media/matroska"
)

// Fixtures made by scripts\make-fixtures.ps1 with mkvmerge.
var fixtureFiles = []string{"subtitles.mkv", "subtitles-zlib.mkv"}

func fixturePath(name string) string {
	return filepath.Join("..", "..", "..", "testdata", "matroska", name)
}

func TestFixturesMadeByMkvmerge(t *testing.T) {
	want := map[uint64]struct {
		language            media.LanguageTag
		name                string
		format              subtitle.Format
		isDefault, isForced bool
		texts               []string
	}{
		1: {"en", "English", subtitle.FormatSRT, true, false, []string{"I'm gonna have to think about it.", "That's sick!", "I'm not going there."}},
		2: {"pt-BR", "Português (Brasil)", subtitle.FormatASS, false, false, []string{"Vou ter que pensar sobre isso.", "Que loucura!", "Eu não vou para lá.\nNão mesmo."}},
		3: {"es", "", subtitle.FormatSRT, false, true, []string{"¡Vámonos!"}},
	}
	for _, file := range fixtureFiles {
		t.Run(file, func(t *testing.T) {
			reader := matroska.FileReader{}
			info, err := reader.Analyze(fixturePath(file))
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			subtitles := info.TracksOf(media.TrackSubtitle)
			if len(subtitles) != len(want) {
				t.Fatalf("found %d subtitle tracks, want %d", len(subtitles), len(want))
			}
			numbers := make([]uint64, 0, len(subtitles))
			for _, track := range subtitles {
				expected := want[track.Number]
				if track.Language != expected.language || track.Name != expected.name || track.SubtitleFormat != expected.format ||
					track.Default != expected.isDefault || track.Forced != expected.isForced || !track.Readable {
					t.Fatalf("track %d = %+v, want %+v", track.Number, track, expected)
				}
				numbers = append(numbers, track.Number)
			}

			content, err := reader.ReadSubtitles(context.Background(), fixturePath(file), numbers, nil)
			if err != nil {
				t.Fatalf("ReadSubtitles: %v", err)
			}
			for number, expected := range want {
				events := content[number].Events
				if len(events) != len(expected.texts) || content[number].Skipped != 0 {
					t.Fatalf("track %d: %d events, %d skipped; want %d events", number, len(events), content[number].Skipped, len(expected.texts))
				}
				for i, text := range expected.texts {
					if events[i].OriginalText != text {
						t.Fatalf("track %d event %d = %q, want %q", number, i, events[i].OriginalText, text)
					}
				}
			}
			english := content[1].Events
			if english[0].Start != ms(1_000) || english[0].End != ms(3_500) || english[2].Start != ms(7_250) || english[2].End != ms(9_000) {
				t.Fatalf("English timing = %+v", english)
			}

			selection := media.SelectTracks(info, media.Preferences{Audio: "en", Primary: "en", Translation: "pt-BR"})
			if selection.Primary != 1 || selection.Translation != 2 {
				t.Fatalf("SelectTracks = %+v, want primary 1 and translation 2", selection)
			}
		})
	}
}

// mkvmergeIdentification is the part of `mkvmerge -J` used for the cross-check.
type mkvmergeIdentification struct {
	Tracks []struct {
		ID         int    `json:"id"`
		Type       string `json:"type"`
		Codec      string `json:"codec"`
		Properties struct {
			Number       uint64 `json:"number"`
			CodecID      string `json:"codec_id"`
			Language     string `json:"language"`
			LanguageIETF string `json:"language_ietf"`
			TrackName    string `json:"track_name"`
			Default      bool   `json:"default_track"`
			Forced       bool   `json:"forced_track"`
		} `json:"properties"`
	} `json:"tracks"`
}

func findMkvmerge() string {
	if path, err := exec.LookPath("mkvmerge"); err == nil {
		return path
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if base == "" {
			continue
		}
		if candidate := filepath.Join(base, "MKVToolNix", "mkvmerge.exe"); fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// Compares our reading of the fixtures with MKVToolNix's own identification.
func TestFixturesAgreeWithMkvmerge(t *testing.T) {
	mkvmerge := findMkvmerge()
	if mkvmerge == "" {
		t.Skip("MKVToolNix is not installed; cross-check skipped")
	}
	for _, file := range fixtureFiles {
		output, err := exec.Command(mkvmerge, "-J", fixturePath(file)).Output()
		if err != nil {
			t.Fatalf("mkvmerge -J %s: %v", file, err)
		}
		var identified mkvmergeIdentification
		if err := json.Unmarshal(output, &identified); err != nil {
			t.Fatalf("decoding mkvmerge output: %v", err)
		}
		info, err := matroska.FileReader{}.Analyze(fixturePath(file))
		if err != nil {
			t.Fatalf("Analyze: %v", err)
		}
		if len(identified.Tracks) != len(info.Tracks) {
			t.Fatalf("%s: mkvmerge sees %d tracks, we see %d", file, len(identified.Tracks), len(info.Tracks))
		}
		for _, theirs := range identified.Tracks {
			ours, found := info.Track(theirs.Properties.Number)
			if !found {
				t.Fatalf("%s: track %d missing", file, theirs.Properties.Number)
			}
			if ours.Codec != theirs.Properties.CodecID || ours.Name != theirs.Properties.TrackName ||
				ours.Default != theirs.Properties.Default || ours.Forced != theirs.Properties.Forced ||
				string(ours.Language) != theirs.Properties.LanguageIETF {
				t.Fatalf("%s: track %d differs\n ours:   %+v\n theirs: %+v", file, theirs.Properties.Number, ours, theirs.Properties)
			}
		}
	}
}
