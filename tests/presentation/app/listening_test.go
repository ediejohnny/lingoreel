package app_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"
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
	r := findingRig(t, func(context.Context, string) (voice.Decoder, error) {
		found.Add(1)
		return sayingDecoder{&found}, nil
	})
	return r, &found
}

// findingRig is the app with that video and the voice model, its FFmpeg
// looked for with find; a computer too busy to start it rests 10 ms.
func findingRig(t *testing.T, find func(context.Context, string) (voice.Decoder, error)) *rig {
	t.Helper()
	r := newRigWith(t, func(deps *app.Deps) {
		service := analysis.NewService(unsubtitledReader{}, besideFiles{})
		deps.Service, deps.Ports.Service = service, service
		deps.Ports.Model = testkit.LoudModel{}
		deps.Ports.FindDecoder = find
		deps.BusyRest = 10 * time.Millisecond
	})
	r.attach(t)
	return r
}

// heardTheRightFile waits for the video's hearing to end and tells whether
// the file that fits its voice went on top.
func heardTheRightFile(t *testing.T, r *rig) bool {
	t.Helper()
	testkit.EventuallyWithin(t, "the hearing over", time.Minute, func() bool {
		states := r.ui.listenings()
		return len(states) > 0 && !states[len(states)-1].Active
	})
	told, _ := r.ui.last(app.EventVideo)
	return told.(app.Video).Suggested.Primary == media.ExternalTrackBase
}

// A computer too busy to start FFmpeg (a game, an antivirus) only delays the
// hearing: it rests and looks again while the video is open. No FFmpeg at
// all ends it after one look.
func TestABusyComputerDelaysTheHearingWithoutEndingIt(t *testing.T) {
	t.Parallel()
	var looks atomic.Int32
	busy := findingRig(t, func(context.Context, string) (voice.Decoder, error) {
		if looks.Add(1) <= 2 {
			return nil, voice.ErrDecoderBusy
		}
		return sayingDecoder{new(atomic.Int32)}, nil
	})
	if _, err := busy.api.OpenVideo(unsubtitledVideo); err != nil {
		t.Fatal(err)
	}
	if !heardTheRightFile(t, busy) || looks.Load() != 3 {
		t.Fatalf("FFmpeg looked for %d times; want the video heard on the third look, after two busy ones", looks.Load())
	}

	var missing atomic.Int32
	none := findingRig(t, func(context.Context, string) (voice.Decoder, error) {
		missing.Add(1)
		return nil, errors.New("ffmpeg: not found")
	})
	if _, err := none.api.OpenVideo(unsubtitledVideo); err != nil {
		t.Fatal(err)
	}
	if heardTheRightFile(t, none) || missing.Load() != 1 {
		t.Fatalf("FFmpeg looked for %d times; want one look, and the files left unheard", missing.Load())
	}
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
	// Six stretches heard at a quarter of a core: real work, slowed down by -race.
	testkit.EventuallyWithin(t, "the video heard", time.Minute, func() bool {
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

// A file kept from an earlier opening was chosen before anyone knew whether
// it fits: when the video is first heard and it does not, the file in its
// language that fits takes its place, once.
func TestTheHearingReplacesAKeptFileThatDoesNotFit(t *testing.T) {
	t.Parallel()
	r, _ := hearingRig(t)
	kept := project.New()
	kept.Choice.Primary = "file:subs/wrong.en.srt"
	if err := r.projects.Save(unsubtitledVideo, kept); err != nil {
		t.Fatal(err)
	}

	video, err := r.api.OpenVideo(unsubtitledVideo)
	if err != nil {
		t.Fatal(err)
	}
	if !video.Listening.Active || video.Suggested.Primary != media.ExternalTrackBase+1 {
		t.Fatalf("as it opens: listening %+v, primary %d; want the kept file, still heard", video.Listening, video.Suggested.Primary)
	}
	testkit.EventuallyWithin(t, "the video heard", time.Minute, func() bool {
		states := r.ui.listenings()
		return len(states) > 0 && !states[len(states)-1].Active
	})
	told, _ := r.ui.last(app.EventVideo)
	if heard := told.(app.Video); heard.Suggested.Primary != media.ExternalTrackBase {
		t.Fatalf("after hearing: primary %d; want the file that fits in place of the kept one", heard.Suggested.Primary)
	}
}

// Files found not to fit are not a missing language: the note says the ones
// found do not fit this video, beside the language that has none at all.
func TestTheNoteTellsFilesThatDoNotFitFromNone(t *testing.T) {
	t.Parallel()
	r, _ := hearingRig(t)
	kept := project.New()
	for key, seed := range map[string]uint64{"file:subs/right.en.srt": 1, "file:subs/wrong.en.srt": 2} {
		fit := project.FitFrom(subtitle.Alignment{Verdict: subtitle.VerdictNoMatch, Rate: 1, Score: 0.5}, project.FitOf(spokenLines(seed).Events))
		kept.Tracks[key] = project.Track{Fit: &fit}
	}
	if err := r.projects.Save(unsubtitledVideo, kept); err != nil {
		t.Fatal(err)
	}
	video, err := r.api.OpenVideo(unsubtitledVideo)
	if err != nil {
		t.Fatal(err)
	}
	const want = "Não há legenda em português do Brasil neste vídeo, e a legenda em inglês que o app achou não bate com ele"
	if !strings.HasPrefix(video.LanguageNote, want) {
		t.Fatalf("language note = %q, want it to start with %q", video.LanguageNote, want)
	}
}

// Opening a video tells how far it went, from 0 to 100, for a bar that
// fills while a large video not yet read takes a moment.
func TestOpeningAVideoTellsHowFarItWent(t *testing.T) {
	t.Parallel()
	r, _ := hearingRig(t)
	if _, err := r.api.OpenVideo(unsubtitledVideo); err != nil {
		t.Fatal(err)
	}
	r.ui.mu.Lock()
	var percents []int
	for _, e := range r.ui.events {
		if opening, ok := e.data.(app.Opening); ok && e.name == app.EventOpening && opening.Path == unsubtitledVideo {
			percents = append(percents, opening.Percent)
		}
	}
	r.ui.mu.Unlock()
	if len(percents) < 3 || percents[0] != 0 || percents[len(percents)-1] != 100 || !slices.IsSorted(percents) {
		t.Fatalf("told %v, want it rising from 0 to 100", percents)
	}
}
