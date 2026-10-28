package analysis_test

import (
	"context"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// heardOf is what the model hears of a video whose lines are said as the
// content times them, in the stretches the video is heard.
func heardOf(said media.SubtitleContent, duration time.Duration) []speech.Track {
	return heardIn(said, subtitle.HearingStretches(duration), func(int) bool { return false })
}

// heardIn is what the model hears of the lines in some stretches, taking for
// voice all through the ones under music.
func heardIn(said media.SubtitleContent, stretches []subtitle.HearingStretch, music func(stretch int) bool) []speech.Track {
	var heard []speech.Track
	for i, stretch := range stretches {
		track := speech.Track{Start: stretch.From}
		for at := stretch.From; at < stretch.From+stretch.Length; at += speech.FrameLength {
			p := float32(0.05)
			for _, line := range said.Events {
				if music(i) || (at >= line.Start && at < line.End) {
					p = 0.9
				}
			}
			track.Frames = append(track.Frames, speech.Frame{Speech: p})
		}
		heard = append(heard, track)
	}
	return heard
}

// Music under four of the six stretches heard leaves the file that fits
// undecided: those groups find chance offsets. The stretches between them,
// heard then, tell it fits; the other film's file still does not.
func TestTheStretchesBetweenTellAnUndecidedFile(t *testing.T) {
	t.Parallel()
	_, result, _ := unsubtitled(t, 2*time.Second)
	said := dialogue(60, 900, 0, englishLine)
	first := heardIn(said, subtitle.HearingStretches(time.Hour), func(i int) bool { return i != 2 && i != 3 })
	between := heardIn(said, subtitle.HearingStretchesBetween(time.Hour), func(int) bool { return false })
	right, wrong := media.ExternalTrackBase, media.ExternalTrackBase+1
	if _, fits := analysis.FitByVoice(result, first, nil, preferences); fits[right].Verdict != subtitle.VerdictNoMatch || !analysis.Undecided(result, first) {
		t.Fatalf("the first stretches alone: %+v, undecided %v; want no match, undecided", fits[right], analysis.Undecided(result, first))
	}
	_, fits := analysis.FitByVoice(result, first, between, preferences)
	if fits[right].Verdict != subtitle.VerdictMatch || fits[wrong].Verdict != subtitle.VerdictNoMatch {
		t.Fatalf("with the stretches between: right %+v, wrong %+v", fits[right], fits[wrong])
	}
}

// unsubtitled is an hour-long video with no subtitle inside, whose voice
// says the lines of seed 60, and two English files beside it: the one for
// it, shown shift early, and another film's.
func unsubtitled(t *testing.T, shift time.Duration) (*analysis.Service, analysis.Analysis, []speech.Track) {
	t.Helper()
	reader := &fakeReader{info: media.Info{Duration: time.Hour, Tracks: []media.Track{{Number: 1, Type: media.TrackAudio, Language: "en"}}}}
	files := &fakeFiles{
		candidates: []string{`D:\Filmes\subs\right.en.srt`, `D:\Filmes\subs\wrong.en.srt`},
		content: map[string]media.SubtitleContent{
			`D:\Filmes\subs\right.en.srt`: dialogue(60, 900, -shift, englishLine),
			`D:\Filmes\subs\wrong.en.srt`: dialogue(61, 900, 0, englishLine),
		},
	}
	service := analysis.NewService(reader, files)
	result, err := service.Analyze(context.Background(), `D:\Filmes\Exemplo.mkv`, preferences)
	if err != nil {
		t.Fatal(err)
	}
	return service, result, heardOf(dialogue(60, 900, 0, englishLine), time.Hour)
}

// With no subtitle in the video to compare with, its voice tells the files
// apart: the one whose lines fall where people speak is chosen and moved
// onto the voice; the other film's is never chosen.
func TestTheVoiceTellsWhichFileFitsAVideoWithoutSubtitles(t *testing.T) {
	t.Parallel()
	service, result, heard := unsubtitled(t, 2*time.Second)
	right, wrong := media.ExternalTrackBase, media.ExternalTrackBase+1
	if unheard := result.Unheard(); len(unheard) != 2 || result.Suggested.Primary != 0 {
		t.Fatalf("before hearing: unheard %v, primary %d; want both files unheard and none chosen", unheard, result.Suggested.Primary)
	}

	fitted, fits := analysis.FitByVoice(result, heard, nil, preferences)
	if fits[right].Verdict != subtitle.VerdictMatch || fits[wrong].Verdict != subtitle.VerdictNoMatch {
		t.Fatalf("fits = %+v", fits)
	}
	if fitted.Suggested.Primary != right || len(fitted.Unheard()) != 0 {
		t.Fatalf("after hearing: primary %d, unheard %v; want the file that fits, none unheard", fitted.Suggested.Primary, fitted.Unheard())
	}
	if result.Suggested.Primary != 0 {
		t.Fatal("hearing changed the analysis it was given")
	}
	loaded, err := service.LoadSubtitles(context.Background(), fitted, []uint64{right}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded[right]; !got.Corrected || (got.Correction.Offset-2*time.Second).Abs() > 60*time.Millisecond {
		t.Fatalf("correction %v %+v, want about +2 s", got.Corrected, got.Correction)
	}
}

// A file that fits the voice within what the voice can tell keeps the
// timing its authors gave it.
func TestAFileThatFitsTheVoiceKeepsItsOwnTiming(t *testing.T) {
	t.Parallel()
	_, result, heard := unsubtitled(t, 100*time.Millisecond)
	fitted, _ := analysis.FitByVoice(result, heard, nil, preferences)
	track, _ := fitted.Info.Track(media.ExternalTrackBase)
	if track.Alignment.Verdict != subtitle.VerdictMatch {
		t.Fatalf("verdict %s, want match", track.Alignment.Verdict)
	}
	if _, moved := analysis.Correction(track); moved {
		t.Fatalf("moved by %v", track.Alignment.Offset)
	}
}
