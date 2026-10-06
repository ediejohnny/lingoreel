package watch

import (
	"context"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// While the Modo Karaokê paints the line, a follower finds the actor's voice
// for it. It starts in the background (finding the user's FFmpeg takes a
// moment, and the video must not wait for it), starts again when the audio
// or the line on top changes, and stops when the fill is turned off or the
// session ends.

// findTimeout bounds the search for the user's FFmpeg.
const findTimeout = 10 * time.Second

type voicing struct {
	tap  voice.Tap
	find func(ctx context.Context) voice.Decoder
	file string
	pid  int

	mu       sync.Mutex
	follower *voice.Follower
	// starting numbers the starts; a start that finds a newer one asked for,
	// or the fill off, closes what it started. inFlight counts those running.
	starting int
	inFlight int
	ended    bool
}

// followVoice starts or stops following the voice as the Modo Karaokê asks.
func (s *Session) followVoice() {
	s.reviewVoice(false)
}

// reviewVoice follows the voice when the fill is on; again (restart) when
// what it follows changed, such as the audio track.
func (s *Session) reviewVoice(restart bool) {
	v := s.voice
	if v == nil || (v.tap == nil && v.find == nil) {
		return
	}
	karaoke := s.subtitles.Karaoke()
	wanted := karaoke.Enabled && karaoke.Fill
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.ended {
		return
	}
	if v.follower != nil && (restart || !wanted) {
		s.subtitles.SetVoice(nil)
		go v.follower.Close()
		v.follower = nil
	}
	if wanted && v.follower == nil && (v.inFlight == 0 || restart) {
		v.starting++
		v.inFlight++
		go s.startVoice(v.starting)
	}
}

// startVoice starts a follower with the sources there are now.
func (s *Session) startVoice(attempt int) {
	v := s.voice
	sources := s.voiceSources()
	if v.find != nil && sources.Voice.File != "" {
		ctx, cancel := context.WithTimeout(context.Background(), findTimeout)
		sources.Decoder = v.find(ctx)
		cancel()
	}
	follower := voice.Follow(context.Background(), sources, s.clock, voice.Settings{Sensitivity: speech.SensitivityNormal})
	v.mu.Lock()
	defer v.mu.Unlock()
	v.inFlight--
	karaoke := s.subtitles.Karaoke()
	if v.ended || attempt != v.starting || v.follower != nil || !(karaoke.Enabled && karaoke.Fill) {
		go follower.Close()
		return
	}
	v.follower = follower
	s.subtitles.SetVoice(func(text string, start, end, position time.Duration) (float64, bool) {
		return follower.Fill(voice.Line{Text: text, Start: start, End: end}, position)
	})
}

// voiceSources are the ways to the voice of the line on top: its language's
// audio track, decoded ahead, and the player heard as it plays, but only
// while the audio playing is that track (a dubbed voice says the line at
// other times).
func (s *Session) voiceSources() voice.Sources {
	v := s.voice
	tracks := s.Tracks()
	language := media.LanguageTag("")
	for _, track := range tracks.List {
		if track.Number == tracks.Current.Primary {
			language = track.Language
		}
	}
	info := media.Info{Tracks: tracks.List}
	var sources voice.Sources
	track, position, found := media.VoiceTrack(info, language)
	if found {
		sources.Voice = voice.Stretch{File: v.file, Track: position, Channels: track.Channels, Codec: track.Codec}
	}
	if v.tap != nil && v.pid > 0 && (!found || playedAudio(info, tracks.Current.Audio) == track.Number) {
		sources.Tap, sources.PID = v.tap, v.pid
	}
	return sources
}

// playedAudio is the audio track playing: the one chosen, else the player's
// own choice, the default track or the first.
func playedAudio(info media.Info, chosen uint64) uint64 {
	if chosen != 0 {
		return chosen
	}
	audio := info.TracksOf(media.TrackAudio)
	for _, track := range audio {
		if track.Default {
			return track.Number
		}
	}
	if len(audio) > 0 {
		return audio[0].Number
	}
	return 0
}

// endVoice stops following for good; the follower closes in the background,
// so ending the session never waits for it.
func (s *Session) endVoice() {
	v := s.voice
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.ended = true
	if v.follower != nil {
		s.subtitles.SetVoice(nil)
		go v.follower.Close()
		v.follower = nil
	}
}
