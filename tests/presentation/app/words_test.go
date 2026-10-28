package app_test

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// portugueseFilm is spoken and subtitled in Brazilian Portuguese.
const portugueseFilm = `D:\Filmes\Falado.mkv`

type portugueseReader struct{}

func (portugueseReader) Analyze(string) (media.Info, error) {
	return media.Info{Duration: time.Hour, Tracks: []media.Track{
		{Number: 1, Type: media.TrackVideo, Width: 1920, Height: 1080},
		{Number: 2, Type: media.TrackAudio, Language: "pt-BR", Codec: "A_AC3", Channels: 2},
		{Number: 3, Type: media.TrackSubtitle, Language: "pt-BR", Codec: "S_TEXT/UTF8", Readable: true},
	}}, nil
}
func (portugueseReader) Check(string) error { return nil }
func (portugueseReader) ReadSubtitles(_ context.Context, _ string, tracks []uint64, _ func(int64, int64)) (map[uint64]media.SubtitleContent, error) {
	contents := map[uint64]media.SubtitleContent{}
	for _, number := range tracks {
		contents[number] = spokenLines(1)
	}
	return contents, nil
}

// wordsRig is the app on a computer watching that film, the karaoke mode's
// fill following the voice; it tells how many times an FFmpeg was looked for
// to decode the voice and the languages whose speech pack was opened.
func wordsRig(t *testing.T, machine capacity.Machine) (looked *atomic.Int32, opened func() []string) {
	t.Helper()
	looked = new(atomic.Int32)
	var mu sync.Mutex
	var languages []string
	r := newRigWith(t, func(deps *app.Deps) {
		service := analysis.NewService(portugueseReader{}, nil)
		deps.Service, deps.Ports.Service = service, service
		deps.Machine = func() capacity.Machine { return machine }
		deps.Ports.Model = testkit.LoudModel{}
		deps.Ports.FindDecoder = func(context.Context, string) (voice.Decoder, error) {
			looked.Add(1)
			return sayingDecoder{new(atomic.Int32)}, nil
		}
		deps.OpenSpeech = func(language string) (voice.WordModel, error) {
			mu.Lock()
			defer mu.Unlock()
			languages = append(languages, language)
			return spokenIn{0.9}, nil
		}
	})
	r.attach(t)
	following := settingsOf(t, r).Voice
	following.Model, following.Follow = true, true
	if err := r.api.SetVoice(following); err != nil {
		t.Fatal(err)
	}
	if _, err := r.api.OpenVideo(portugueseFilm); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 2, Primary: 3}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the session", func() bool { return r.api.PlaybackState().Active })
	t.Cleanup(func() { r.api.StopWatching() })
	if err := r.api.SetKaraoke(true); err != nil {
		t.Fatal(err)
	}
	return looked, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(languages)
	}
}

// The karaoke mode's fill places a line's words with its language's speech
// pack only on a computer whose level suits that pack: Portuguese's large
// model on a high computer, never on a medium one, which runs English's
// small model alone.
func TestTheWordsArePlacedOnlyWithAPackTheComputerSuits(t *testing.T) {
	t.Parallel()
	_, highOpened := wordsRig(t, studioComputer)
	testkit.Eventually(t, "Portuguese's pack opened on a high computer", func() bool {
		return slices.Equal(highOpened(), []string{"pt"})
	})

	looked, mediumOpened := wordsRig(t, homeComputer)
	testkit.Eventually(t, "the voice decoded on a medium computer", func() bool { return looked.Load() > 0 })
	time.Sleep(200 * time.Millisecond) // the pack is opened right after the decoder is found
	if languages := mediumOpened(); len(languages) != 0 {
		t.Errorf("packs opened on a medium computer: %q, want none for Portuguese", languages)
	}
}
