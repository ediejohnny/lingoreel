package watch_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// listeningTap counts who is hearing the player, and which player.
type listeningTap struct {
	listening atomic.Int32
	pid       atomic.Int32
}

func (t *listeningTap) Listen(ctx context.Context, pid int, _ func(voice.Packet)) error {
	t.pid.Store(int32(pid))
	t.listening.Add(1)
	<-ctx.Done()
	t.listening.Add(-1)
	return nil
}

// stretchDecoder keeps what it was asked to decode, and decodes nothing.
type stretchDecoder struct {
	mu      sync.Mutex
	stretch *voice.Stretch
}

func (d *stretchDecoder) Decode(ctx context.Context, stretch voice.Stretch, _ func([]float32, int)) error {
	d.mu.Lock()
	d.stretch = &stretch
	d.mu.Unlock()
	return nil
}

func (d *stretchDecoder) asked() *voice.Stretch {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stretch
}

func paintingKaraoke() session.Karaoke {
	return session.Karaoke{Enabled: true, Rules: study.DefaultCueRules(), Fill: true}
}

// The player is heard only while the karaoke mode paints the line, and no
// longer once the session ends.
func TestThePlayerIsHeardWhileTheLineIsPainted(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.ports.Model = testkit.LoudModel{}
	tap := &listeningTap{}
	r.ports.Tap = tap
	r.request.Karaoke = paintingKaraoke()
	r.request.Voice = watch.VoiceSetup{Follow: true}
	r.request.Tracks.Audio = 3 // English, the line's language
	s := r.start(t)

	testkit.Eventually(t, "hearing the player", func() bool { return tap.listening.Load() == 1 })
	if tap.pid.Load() != 4242 {
		t.Fatalf("heard process %d, want the player's 4242", tap.pid.Load())
	}
	s.SetKaraoke(false)
	testkit.Eventually(t, "no longer hearing with the karaoke mode off", func() bool { return tap.listening.Load() == 0 })
	s.SetKaraoke(true)
	testkit.Eventually(t, "hearing again", func() bool { return tap.listening.Load() == 1 })
	s.SetVoiceSetup(watch.VoiceSetup{Follow: false})
	testkit.Eventually(t, "no longer hearing with following off", func() bool { return tap.listening.Load() == 0 })
	if _, following := s.VoiceMode(); following {
		t.Fatal("the voice is said to be followed with following off")
	}
	s.SetVoiceSetup(watch.VoiceSetup{Follow: true})
	testkit.Eventually(t, "hearing with following on", func() bool { return tap.listening.Load() == 1 })
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "no longer hearing after the session", func() bool { return tap.listening.Load() == 0 })
}

// A dubbed voice says the line at other times: while the Portuguese track
// plays under an English line, the player is not followed; switching to the
// English audio follows it. The film's English track is decoded ahead by its
// place among the audio tracks, with the FFmpeg the user chose.
func TestTheVoiceFollowedSpeaksTheLinesLanguage(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.ports.Model = testkit.LoudModel{}
	tap, decoder := &listeningTap{}, &stretchDecoder{}
	found := make(chan struct{}, 4)
	r.ports.Tap = tap
	r.ports.FindDecoder = func(context.Context, string) voice.Decoder {
		found <- struct{}{}
		return nil // no FFmpeg yet
	}
	r.request.Karaoke = paintingKaraoke()
	r.request.Voice = watch.VoiceSetup{Follow: true}
	r.request.Tracks.Audio = 2 // the Portuguese dub
	r.running.audio = true
	s := r.start(t)

	<-found
	time.Sleep(100 * time.Millisecond) // a follower that wrongly heard the dub would have started
	if tap.listening.Load() != 0 {
		t.Fatal("the dubbed voice is followed")
	}
	if err := s.SelectAudio(3); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "hearing the English audio", func() bool { return tap.listening.Load() == 1 })

	var chosen atomic.Value
	r.ports.FindDecoder = func(_ context.Context, path string) voice.Decoder {
		chosen.Store(path)
		return decoder
	}
	r.request.Voice.FFmpegPath = `C:\ffmpeg\bin\ffmpeg.exe`
	ahead := r.start(t)
	defer ahead.Stop()
	testkit.Eventually(t, "decoding ahead", func() bool { return decoder.asked() != nil })
	if stretch := decoder.asked(); stretch.File != `C:\v.mkv` || stretch.Track != 1 {
		t.Fatalf("decoding %+v, want the second audio track of the video", *stretch)
	}
	if path, _ := chosen.Load().(string); path != `C:\ffmpeg\bin\ffmpeg.exe` {
		t.Fatalf("looked for FFmpeg at %q, want the one chosen", path)
	}
	if mode, following := ahead.VoiceMode(); !following || mode != voice.ModeAhead {
		t.Fatalf("voice mode %v (following %v), want decoding ahead", mode, following)
	}
}
