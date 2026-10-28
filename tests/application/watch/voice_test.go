package watch_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
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
	r.request.Voice = watch.VoiceSetup{Model: true, Follow: true}
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
	s.SetVoiceSetup(watch.VoiceSetup{Model: true, Follow: false})
	testkit.Eventually(t, "no longer hearing with following off", func() bool { return tap.listening.Load() == 0 })
	if _, following := s.VoiceMode(); following {
		t.Fatal("the voice is said to be followed with following off")
	}
	s.SetVoiceSetup(watch.VoiceSetup{Model: true, Follow: true})
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
	r.ports.FindDecoder = func(context.Context, string) (voice.Decoder, error) {
		found <- struct{}{}
		return nil, errors.New("no FFmpeg yet")
	}
	r.request.Karaoke = paintingKaraoke()
	r.request.Voice = watch.VoiceSetup{Model: true, Follow: true}
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
	r.ports.FindDecoder = func(_ context.Context, path string) (voice.Decoder, error) {
		chosen.Store(path)
		return decoder, nil
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

// Keeping the study line on the voice hears the film even with the karaoke
// mode off, without painting anything; turned off, or with the voice model
// off, nothing hears it.
func TestKeepingTheLineOnTheVoiceHearsTheFilm(t *testing.T) {
	t.Parallel()
	start := func(setup watch.VoiceSetup) (*watch.Session, *stretchDecoder) {
		r := newRig(t)
		r.ports.Model = testkit.LoudModel{}
		decoder := &stretchDecoder{}
		r.ports.FindDecoder = func(context.Context, string) (voice.Decoder, error) { return decoder, nil }
		r.request.Voice = setup
		return r.start(t), decoder
	}
	kept, decoder := start(watch.VoiceSetup{Model: true, Sync: true})
	testkit.Eventually(t, "hearing the film for its sync", func() bool { return decoder.asked() != nil })
	if _, painting := kept.VoiceMode(); painting {
		t.Fatal("the karaoke mode, off, follows the voice")
	}

	_, resting := start(watch.VoiceSetup{Model: false, Sync: true})
	_, off := start(watch.VoiceSetup{Model: true, Sync: false})
	time.Sleep(300 * time.Millisecond) // the one above was heard within milliseconds
	if resting.asked() != nil || off.asked() != nil {
		t.Fatal("the film is heard with the voice model, or keeping the line, off")
	}
}

// power is a computer whose battery a test plugs in and out.
type power struct {
	mu        sync.Mutex
	onBattery bool
	changes   chan bool
}

func newPower(onBattery bool) *power { return &power{onBattery: onBattery, changes: make(chan bool)} }

func (p *power) HasBattery() bool { return true }

func (p *power) OnBattery() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.onBattery
}

func (p *power) Watch(ctx context.Context, changed func(bool)) {
	for {
		select {
		case <-ctx.Done():
			return
		case onBattery := <-p.changes:
			p.mu.Lock()
			p.onBattery = onBattery
			p.mu.Unlock()
			changed(onBattery)
		}
	}
}

// On the battery the voice model rests, unless the user wants it there too:
// the player is not heard and the session says why; plugging the computer
// in wakes the model, and switching it off rests it again.
func TestTheVoiceModelRestsOnTheBattery(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.ports.Model = testkit.LoudModel{}
	tap, battery := &listeningTap{}, newPower(true)
	r.ports.Tap, r.ports.Power = tap, battery
	r.request.Karaoke = paintingKaraoke()
	r.request.Voice = watch.VoiceSetup{Model: true, Follow: true}
	r.request.Tracks.Audio = 3
	s := r.start(t)

	testkit.Eventually(t, "the estimate on the battery", func() bool {
		mode, following := s.VoiceMode()
		return following && mode == voice.ModeEstimate
	})
	if tap.listening.Load() != 0 || s.ModelRest() != watch.ModelOnBattery {
		t.Fatalf("on the battery: %d hearing, rest %q", tap.listening.Load(), s.ModelRest())
	}
	battery.changes <- false // plugged in
	testkit.Eventually(t, "hearing once plugged in", func() bool { return tap.listening.Load() == 1 })
	if s.ModelRest() != watch.ModelAtWork {
		t.Fatalf("plugged in, the model rests: %q", s.ModelRest())
	}

	s.SetVoiceSetup(watch.VoiceSetup{Model: true, ModelOnBattery: true, Follow: true})
	battery.changes <- true
	if s.ModelRest() != watch.ModelAtWork {
		t.Fatalf("wanted on the battery too, the model rests: %q", s.ModelRest())
	}
	s.SetVoiceSetup(watch.VoiceSetup{Model: false, Follow: true})
	testkit.Eventually(t, "no longer hearing with the model off", func() bool { return tap.listening.Load() == 0 })
	if s.ModelRest() != watch.ModelOff {
		t.Fatalf("switched off, the rest is %q", s.ModelRest())
	}
}

// Watching without subtitles hears nothing: the voice is followed once a
// subtitle goes on top from the panel, and no longer when it is taken away.
func TestWithoutALineOnTopTheVoiceIsNotFollowed(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.ports.Model = testkit.LoudModel{}
	tap := &listeningTap{}
	r.ports.Tap = tap
	r.request.Karaoke = paintingKaraoke()
	r.request.Voice = watch.VoiceSetup{Model: true, Follow: true}
	r.request.Tracks.Audio, r.request.Tracks.Primary, r.request.Tracks.Translation = 3, 0, 0
	s := r.start(t)

	time.Sleep(100 * time.Millisecond) // a follower wrongly started would be hearing
	if tap.listening.Load() != 0 {
		t.Fatal("the voice is followed with no line on top")
	}
	if err := s.SetLine(session.LinePrimary, 4, loadedFile(t, "Hello."), project.Track{}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "hearing with a line on top", func() bool { return tap.listening.Load() == 1 })
	if err := s.SetLine(session.LinePrimary, 0, analysis.LoadedTrack{}, project.Track{}); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "no longer hearing without it", func() bool { return tap.listening.Load() == 0 })
}
