package app_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/speechpack"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// dubbedVideo's audio track does not tell its language (Matroska's default,
// English, guessed); its subtitles inside are in English and Portuguese.
const dubbedVideo = `D:\Filmes\Dublado.mkv`

type untoldAudioReader struct{}

func (untoldAudioReader) Analyze(string) (media.Info, error) {
	return media.Info{Duration: time.Hour, Tracks: []media.Track{
		{Number: 1, Type: media.TrackVideo, Width: 1920, Height: 1080},
		{Number: 2, Type: media.TrackAudio, Language: "en", LanguageGuessed: true, Codec: "A_AC3", Channels: 2},
		{Number: 3, Type: media.TrackSubtitle, Language: "en", Codec: "S_TEXT/UTF8", Readable: true},
		{Number: 4, Type: media.TrackSubtitle, Language: "pt-BR", Codec: "S_TEXT/UTF8", Readable: true},
	}}, nil
}
func (untoldAudioReader) Check(string) error { return nil }
func (untoldAudioReader) ReadSubtitles(_ context.Context, _ string, tracks []uint64, _ func(int64, int64)) (map[uint64]media.SubtitleContent, error) {
	contents := map[uint64]media.SubtitleContent{}
	for _, number := range tracks {
		contents[number] = spokenLines(uint64(number))
	}
	return contents, nil
}

// spokenIn is a language's model as sure of every line as told.
type spokenIn struct{ sureness float64 }

func (m spokenIn) Align(_ []float32, words []string) ([]speech.Word, error) {
	sureness := m.sureness
	result := make([]speech.Word, len(words))
	for i := range result {
		result[i] = speech.Word{Heard: true, Confidence: sureness}
	}
	return result, nil
}
func (spokenIn) Close() {}

// englishTrack is how sure each language's model is of a track spoken in English.
var englishTrack = map[string]float64{"en": 0.9, "pt": 0.2}

// spokenRig is the app with that video, the packs given and an FFmpeg, each
// language's model as sure as told; opened counts the packs opened, and
// looked the times an FFmpeg was looked for to hear the video.
func spokenRig(t *testing.T, packs *fakePacks, sureness map[string]float64) (r *rig, opened, looked *atomic.Int32) {
	t.Helper()
	opened, looked = new(atomic.Int32), new(atomic.Int32)
	var mu sync.Mutex
	r = newRigWith(t, func(deps *app.Deps) {
		service := analysis.NewService(untoldAudioReader{}, nil)
		deps.Service, deps.Ports.Service = service, service
		deps.Ports.FindDecoder = func(context.Context, string) voice.Decoder {
			looked.Add(1)
			return sayingDecoder{new(atomic.Int32)}
		}
		deps.SpeechPacks = packs
		deps.OpenSpeech = func(language string) (app.SpeechModel, error) {
			mu.Lock()
			defer mu.Unlock()
			opened.Add(1)
			return spokenIn{sureness[language]}, nil
		}
	})
	r.attach(t)
	return r, opened, looked
}

func bothPacks() *fakePacks {
	return &fakePacks{states: []speechpack.State{{Language: "en", Ready: true}, {Language: "pt", Ready: true}}}
}

func audioLabel(video app.Video) string {
	if len(video.Audio) == 0 {
		return ""
	}
	return video.Audio[0].Label
}

// An audio track whose file does not tell its language is heard in the
// profile's two languages as the video opens; the one its sound confirms
// names it, the project keeps it, and the next opening hears nothing.
func TestAnAudioTrackWithoutItsLanguageIsHeardOnce(t *testing.T) {
	t.Parallel()
	r, opened, _ := spokenRig(t, bothPacks(), englishTrack)
	video, err := r.api.OpenVideo(dubbedVideo)
	if err != nil {
		t.Fatal(err)
	}
	if label := audioLabel(video); !strings.HasPrefix(label, ptbr.T("track.audio.untold")) {
		t.Fatalf("as it opens the track reads %q, want that its file does not tell", label)
	}
	heard := ptbr.T("track.audio.heard", "language", ptbr.Capitalized(ptbr.LanguageName("en")))
	testkit.Eventually(t, "the track's language heard", func() bool {
		told, ok := r.ui.last(app.EventVideo)
		return ok && strings.HasPrefix(audioLabel(told.(app.Video)), heard)
	})
	if n := opened.Load(); n != 2 {
		t.Errorf("%d packs opened, want English's and Portuguese's", n)
	}
	testkit.Eventually(t, "the hearing kept", func() bool {
		kept, err := r.projects.Load(dubbedVideo)
		if err != nil {
			return false
		}
		for _, track := range kept.Tracks {
			if h := track.Hearing; h != nil && h.Language == "en" && h.Covers([]string{"en", "pt"}) {
				return true
			}
		}
		return false
	})

	again, err := r.api.OpenVideo(dubbedVideo)
	if err != nil {
		t.Fatal(err)
	}
	if label := audioLabel(again); !strings.HasPrefix(label, heard) {
		t.Errorf("opened again the track reads %q, want %q at once", label, heard)
	}
	time.Sleep(100 * time.Millisecond) // a hearing would have opened a pack by now
	if n := opened.Load(); n != 2 {
		t.Errorf("opened again, %d packs opened in all, want none more", n)
	}
}

// With the voice AI off, nothing is heard and the track keeps saying its
// file does not tell its language.
func TestWithTheVoiceAIOffTheAudioIsNotHeard(t *testing.T) {
	t.Parallel()
	r, opened, _ := spokenRig(t, bothPacks(), englishTrack)
	voiceOff := settingsOf(t, r).Voice
	voiceOff.Model = false
	if err := r.api.SetVoice(voiceOff); err != nil {
		t.Fatal(err)
	}
	if _, err := r.api.OpenVideo(dubbedVideo); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := opened.Load(); n != 0 {
		t.Errorf("%d packs opened with the voice AI off, want none", n)
	}
}

// keptHearing is what the project kept of the dubbed video's audio track.
func keptHearing(r *rig) *project.Hearing {
	kept, err := r.projects.Load(dubbedVideo)
	if err != nil {
		return nil
	}
	for _, track := range kept.Tracks {
		if track.Hearing != nil {
			return track.Hearing
		}
	}
	return nil
}

// A track the sound did not decide is not heard again in the languages
// tried; a language whose pack comes later is heard, and only it.
func TestAnUndecidedTrackIsHeardAgainOnlyInANewLanguage(t *testing.T) {
	t.Parallel()
	packs := &fakePacks{states: []speechpack.State{{Language: "en", Ready: true}, {Language: "pt"}}}
	r, opened, looked := spokenRig(t, packs, map[string]float64{"en": 0.3, "pt": 0.9})
	if _, err := r.api.OpenVideo(dubbedVideo); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "English tried", func() bool {
		h := keptHearing(r)
		return h != nil && h.Covers([]string{"en"}) && !h.Covers([]string{"pt"})
	})
	if h := keptHearing(r); h.Language != "" || opened.Load() != 1 {
		t.Fatalf("undecided in English: kept %+v, %d packs opened; want no language and only English's pack", h, opened.Load())
	}

	before := looked.Load()
	video, err := r.api.OpenVideo(dubbedVideo)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // a hearing would have opened a pack by now
	if looked.Load() != before {
		t.Errorf("opened again, an FFmpeg was looked for to hear a track heard in every language it can be")
	}
	if label := audioLabel(video); opened.Load() != 1 || !strings.HasPrefix(label, ptbr.T("track.audio.untold")) {
		t.Fatalf("opened again: %d packs opened, track %q; want nothing heard and the track still untold", opened.Load(), label)
	}

	packs.mu.Lock()
	packs.states[1].Ready = true
	packs.mu.Unlock()
	if _, err := r.api.OpenVideo(dubbedVideo); err != nil {
		t.Fatal(err)
	}
	heard := ptbr.T("track.audio.heard", "language", ptbr.Capitalized(ptbr.LanguageName("pt")))
	testkit.Eventually(t, "the track heard in Portuguese", func() bool {
		told, ok := r.ui.last(app.EventVideo)
		return ok && strings.HasPrefix(audioLabel(told.(app.Video)), heard)
	})
	if n := opened.Load(); n != 2 {
		t.Errorf("%d packs opened in all, want English's once and Portuguese's once", n)
	}
	testkit.Eventually(t, "both languages kept as tried", func() bool {
		h := keptHearing(r)
		return h != nil && h.Language == "pt" && h.Covers([]string{"en", "pt"})
	})
}
