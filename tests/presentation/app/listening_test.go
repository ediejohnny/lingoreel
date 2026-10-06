package app_test

import (
	"context"
	"math/rand/v2"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// unsubtitledVideo has no subtitle inside: two English files sit in a folder
// beside it, the one for it and another film's.
const (
	unsubtitledVideo = `D:\Filmes\SemLegenda.mkv`
	rightFile        = `D:\Filmes\subs\right.en.srt`
	wrongFile        = `D:\Filmes\subs\wrong.en.srt`
)

const spokenLine = "You know where she is? No, I do not know. That is very good, thank you. Let us go now."

// spokenLines are an hour of a film's lines, the same ones for the same seed.
func spokenLines(seed uint64) media.SubtitleContent {
	random := rand.New(rand.NewPCG(seed, seed+1))
	var events []subtitle.Event
	for at := 30 * time.Second; at < 58*time.Minute; {
		events = append(events, subtitle.Event{ID: "l", Start: at, End: at + 900*time.Millisecond, OriginalText: spokenLine})
		at += 1500*time.Millisecond + time.Duration(random.Int64N(int64(4500*time.Millisecond)))
	}
	return media.SubtitleContent{Events: events}
}

type unsubtitledReader struct{}

func (unsubtitledReader) Analyze(string) (media.Info, error) {
	return media.Info{Duration: time.Hour, Tracks: []media.Track{
		{Number: 1, Type: media.TrackVideo, Width: 1920, Height: 1080},
		{Number: 2, Type: media.TrackAudio, Language: "en", Codec: "A_AC3", Channels: 2},
	}}, nil
}
func (unsubtitledReader) Check(string) error { return nil }
func (unsubtitledReader) ReadSubtitles(context.Context, string, []uint64, func(int64, int64)) (map[uint64]media.SubtitleContent, error) {
	return map[uint64]media.SubtitleContent{}, nil
}

type besideFiles struct{}

func (besideFiles) Find(string) ([]string, error) { return nil, nil }
func (besideFiles) FindCandidates(string) ([]string, error) {
	return []string{rightFile, wrongFile}, nil
}
func (besideFiles) Load(path string) (subtitle.Format, media.SubtitleContent, error) {
	if path == rightFile {
		return subtitle.FormatSRT, spokenLines(1), nil
	}
	return subtitle.FormatSRT, spokenLines(2), nil
}

// sayingDecoder decodes a film where someone speaks the lines of seed 1:
// sound in them, silence between.
type sayingDecoder struct{ asked *atomic.Int32 }

func (d sayingDecoder) Decode(ctx context.Context, stretch voice.Stretch, hear func([]float32, int)) error {
	lines := spokenLines(1).Events
	piece := make([]float32, 0, 1600)
	for at := stretch.From; at < stretch.From+stretch.Length; at += 100 * time.Millisecond {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		piece = piece[:0]
		first := sort.Search(len(lines), func(i int) bool { return lines[i].End > at })
		for i := range 1600 {
			now, level := at+time.Duration(i)*time.Second/16000, float32(0)
			for _, line := range lines[first:min(first+2, len(lines))] {
				if now >= line.Start && now < line.End {
					level = 0.3
				}
			}
			piece = append(piece, level)
		}
		hear(piece, 1)
	}
	return nil
}

// hearingRig is the app with that video, the voice model and an FFmpeg.
func hearingRig(t *testing.T) (*rig, *atomic.Int32) {
	t.Helper()
	var found atomic.Int32
	r := newRigWith(t, func(deps *app.Deps) {
		service := analysis.NewService(unsubtitledReader{}, besideFiles{})
		deps.Service, deps.Ports.Service = service, service
		deps.Ports.Model = testkit.LoudModel{}
		deps.Ports.FindDecoder = func(context.Context, string) voice.Decoder {
			found.Add(1)
			return sayingDecoder{&found}
		}
	})
	r.attach(t)
	return r, &found
}

// listenings are the hearing's states the screens were told, in order.
func (u *fakeUI) listenings() []app.Listening {
	u.mu.Lock()
	defer u.mu.Unlock()
	var states []app.Listening
	for _, e := range u.events {
		if e.name == app.EventListening {
			states = append(states, e.data.(app.Listening))
		}
	}
	return states
}

// A video with no subtitle inside is heard as it opens: the screen sees the
// hearing rise to its end, and then the video described again with the file
// that fits its voice on top.
func TestAVideoWithoutSubtitlesIsHeardToChooseItsFile(t *testing.T) {
	t.Parallel()
	r, _ := hearingRig(t)
	video, err := r.api.OpenVideo(unsubtitledVideo)
	if err != nil {
		t.Fatal(err)
	}
	if !video.Listening.Active || video.Listening.Stretches != 6 || video.Suggested.Primary != 0 {
		t.Fatalf("as it opens: listening %+v, primary %d; want hearing and no file chosen yet", video.Listening, video.Suggested.Primary)
	}
	testkit.Eventually(t, "the video heard", func() bool {
		states := r.ui.listenings()
		return len(states) > 0 && !states[len(states)-1].Active
	})
	rising := 0
	for _, state := range r.ui.listenings() {
		if state.Active && state.Percent > 0 && state.Percent < 100 {
			rising++
		}
	}
	if rising < 6 {
		t.Fatalf("the hearing was told %d times as it went, want the bar to fill", rising)
	}
	told, _ := r.ui.last(app.EventVideo)
	if heard := told.(app.Video); heard.Suggested.Primary != media.ExternalTrackBase || heard.Listening.Active {
		t.Fatalf("after hearing: primary %d, listening %+v; want the file that fits", heard.Suggested.Primary, heard.Listening)
	}
}

// What a video's hearing found is kept in its project: the next time it
// opens it is not heard again, and the file that fits is on top at once.
func TestAVideoHeardBeforeIsNotHeardAgain(t *testing.T) {
	t.Parallel()
	r, found := hearingRig(t)
	kept := project.New()
	right := project.FitFrom(subtitle.Alignment{Verdict: subtitle.VerdictMatch, Rate: 1, Score: 0.9}, project.FitOf(spokenLines(1).Events))
	wrong := project.FitFrom(subtitle.Alignment{Verdict: subtitle.VerdictNoMatch, Rate: 1, Score: 0.6}, project.FitOf(spokenLines(2).Events))
	kept.Tracks["file:subs/right.en.srt"] = project.Track{Fit: &right}
	kept.Tracks["file:subs/wrong.en.srt"] = project.Track{Fit: &wrong}
	if err := r.projects.Save(unsubtitledVideo, kept); err != nil {
		t.Fatal(err)
	}

	video, err := r.api.OpenVideo(unsubtitledVideo)
	if err != nil {
		t.Fatal(err)
	}
	if video.Listening.Active || video.Suggested.Primary != media.ExternalTrackBase || found.Load() != 0 {
		t.Fatalf("listening %+v, primary %d, FFmpeg looked for %d times; want the kept file on top, nothing heard",
			video.Listening, video.Suggested.Primary, found.Load())
	}
}
