package watch

import (
	"context"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// While the karaoke mode paints the line, or the study line is kept on the
// voice, a follower hears the actor's voice. It starts in the background
// (finding the user's FFmpeg takes a moment, and the video must not wait for
// it), starts again when the audio or the line on top changes, and stops
// when neither wants it any more or the session ends.

// findTimeout bounds the search for the user's FFmpeg.
const findTimeout = 10 * time.Second

// VoiceSetup is how the user wants the voice followed on this computer.
type VoiceSetup struct {
	// Model lets the voice model work on this computer; ModelOnBattery, on
	// its battery too. Resting, it leaves every line to the estimate.
	Model, ModelOnBattery bool
	// Follow off leaves every line to the estimate from its letters.
	Follow bool
	// Sync keeps the study line on the voice when the film slips.
	Sync bool
	// FFmpegPath is the FFmpeg the user chose; empty looks for one.
	FFmpegPath string
	Settings   voice.Settings
}

// ModelRest tells why the voice model rests on purpose, if it does.
type ModelRest string

const (
	ModelAtWork ModelRest = ""
	// ModelOff: the user switched the model off on this computer.
	ModelOff ModelRest = "off"
	// ModelOnBattery: the computer runs on its battery, and the user keeps
	// the model for when it is plugged in.
	ModelOnBattery ModelRest = "battery"
)

// Rest tells whether the model rests with this setup, and why: the one
// place the rule is written.
func (s VoiceSetup) Rest(onBattery bool) ModelRest {
	switch {
	case !s.Model:
		return ModelOff
	case onBattery && !s.ModelOnBattery:
		return ModelOnBattery
	}
	return ModelAtWork
}

type voicing struct {
	model     speech.Model
	tap       voice.Tap
	find      func(ctx context.Context, chosen string) (voice.Decoder, error)
	openWords func(language string) (voice.WordModel, error)
	file      string
	pid       int

	mu        sync.Mutex
	setup     VoiceSetup
	onBattery bool
	follower  *voice.Follower
	// keeper keeps the study line on the voice the follower hears.
	keeper voice.LineKeeper
	// starting numbers the starts; a start that finds a newer one asked for,
	// or the fill off, closes what it started. inFlight counts those running.
	starting int
	inFlight int
	ended    bool
}

// followVoice starts or stops following the voice as the karaoke mode and
// keeping the line on the voice ask.
func (s *Session) followVoice() {
	s.reviewVoice(false)
}

// fillWanted tells whether the karaoke mode paints the line on top by the
// voice: its fill is on, there is a line, and the user follows the voice;
// callers hold v.mu.
func (s *Session) fillWanted() bool {
	karaoke := s.subtitles.Karaoke()
	return karaoke.Enabled && karaoke.Fill && s.voice.setup.Follow && s.Tracks().Current.Primary != 0
}

// keepWanted tells whether the line on top is kept on the voice: the user
// asks for it and there is a line (a model at rest hears nothing, so it
// moves nothing); callers hold v.mu.
func (s *Session) keepWanted() bool {
	return s.voice.setup.Sync && s.Tracks().Current.Primary != 0
}

// voiceWanted tells whether the voice is to be heard at all; callers hold
// v.mu.
func (s *Session) voiceWanted() bool {
	return s.fillWanted() || s.keepWanted()
}

// paintByVoice gives the karaoke mode's fill the follower's voice when the
// fill wants it, and takes it away otherwise; callers hold v.mu.
func (s *Session) paintByVoice(follower *voice.Follower) {
	if follower == nil || !s.fillWanted() {
		s.subtitles.SetVoice(nil)
		return
	}
	s.subtitles.SetVoice(followedVoice{follower})
}

// followedVoice is a follower as the subtitles ask for a line's voice.
type followedVoice struct{ follower *voice.Follower }

func (v followedVoice) Fill(text string, start, end, position time.Duration) (float64, bool) {
	return v.follower.Fill(voice.Line{Text: text, Start: start, End: end}, position)
}

func (v followedVoice) Begins(text string, start, end time.Duration) (time.Duration, bool) {
	return v.follower.Begins(voice.Line{Text: text, Start: start, End: end})
}

// reviewVoice hears the voice while something wants it; again (restart)
// when what it hears changed, such as the audio track.
func (s *Session) reviewVoice(restart bool) {
	v := s.voice
	if v == nil || v.model == nil || (v.tap == nil && v.find == nil) {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	wanted := s.voiceWanted()
	if v.ended {
		return
	}
	if v.follower != nil && (restart || !wanted) {
		s.subtitles.SetVoice(nil)
		go v.follower.Close()
		v.follower = nil
	}
	s.paintByVoice(v.follower)
	if wanted && v.follower == nil && (v.inFlight == 0 || restart) {
		v.starting++
		v.inFlight++
		go s.startVoice(v.starting)
	}
}

// startVoice starts a follower with the sources and the setup there are now.
func (s *Session) startVoice(attempt int) {
	v := s.voice
	v.mu.Lock()
	setup, rest := v.setup, v.setup.Rest(v.onBattery)
	v.mu.Unlock()
	sources := voice.Sources{} // a model at rest hears nothing: the estimate
	if rest == ModelAtWork {
		sources = s.voiceSources()
	}
	if v.find != nil && sources.Voice.File != "" {
		ctx, cancel := context.WithTimeout(context.Background(), findTimeout)
		// Without a decoder, busy or missing, the player is heard instead.
		if decoder, err := v.find(ctx, setup.FFmpegPath); err == nil {
			sources.Decoder = decoder
		}
		cancel()
	}
	// The words are placed in the sound decoded ahead, which only a decoder
	// gives; without their pack the fill keeps the voice found.
	if sources.Decoder != nil && v.openWords != nil {
		if words, err := v.openWords(s.studyLanguage().Base()); err == nil {
			sources.Words, sources.Lines = words, s.studyLines
		}
	}
	settings := setup.Settings
	if !settings.Sensitivity.Valid() {
		settings.Sensitivity = speech.SensitivityNormal
	}
	follower := voice.Follow(context.Background(), sources, s.clock, settings)
	v.mu.Lock()
	defer v.mu.Unlock()
	v.inFlight--
	if v.ended || attempt != v.starting || v.follower != nil || !s.voiceWanted() {
		go follower.Close()
		return
	}
	v.follower = follower
	s.paintByVoice(follower)
}

// voiceSources are the ways to the voice of the line on top: its language's
// audio track, decoded ahead, and the player heard as it plays, but only
// while the audio playing is that track (a dubbed voice says the line at
// other times).
func (s *Session) voiceSources() voice.Sources {
	v := s.voice
	tracks := s.Tracks()
	info := media.Info{Tracks: tracks.List}
	sources := voice.Sources{Model: v.model}
	track, position, found := media.VoiceTrack(info, s.studyLanguage())
	if found {
		sources.Voice = voice.Stretch{File: v.file, Track: position, Channels: track.Channels, Codec: track.Codec}
	}
	if v.tap != nil && v.pid > 0 && (!found || playedAudio(info, tracks.Current.Audio) == track.Number) {
		sources.Tap, sources.PID = v.tap, v.pid
	}
	return sources
}

// studyLanguage is the language of the line on top, the voice followed.
func (s *Session) studyLanguage() media.LanguageTag {
	tracks := s.Tracks()
	primary, _ := media.Info{Tracks: tracks.List}.Track(tracks.Current.Primary)
	return primary.Language
}

// studyLines lists the lines on top shown between two moments of the
// playback clock, with their times on it, as the fill asks for them.
func (s *Session) studyLines(from, to time.Duration) []voice.Line {
	track := s.subtitles.Track(session.LinePrimary)
	if track == nil || track.Timeline == nil {
		return nil
	}
	shown := track.Shown()
	events := track.Timeline.Overlapping(shown.TrackTime(from), shown.TrackTime(to))
	lines := make([]voice.Line, len(events))
	for i, event := range events {
		lines[i] = voice.Line{Text: event.Text(), Start: shown.PlaybackTime(event.Start), End: shown.PlaybackTime(event.End)}
	}
	return lines
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

// SetVoiceSetup changes how the voice is followed; a follower running starts
// again with it.
func (s *Session) SetVoiceSetup(setup VoiceSetup) {
	v := s.voice
	if v == nil {
		return
	}
	v.mu.Lock()
	changed := v.setup != setup
	v.setup = setup
	v.mu.Unlock()
	if changed {
		s.reviewVoice(true)
	}
}

// ModelRest tells why the voice model rests now, if it does.
func (s *Session) ModelRest() ModelRest {
	v := s.voice
	if v == nil {
		return ModelAtWork
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.setup.Rest(v.onBattery)
}

// watchPower follows the computer going on and off its battery: the voice
// starts again when that wakes the model or puts it to rest, and the seek
// bar's stills take, on the battery, only the pointer's.
func (s *Session) watchPower(ctx context.Context, power voice.Power) {
	power.Watch(ctx, func(onBattery bool) {
		s.restStills(onBattery)
		v := s.voice
		v.mu.Lock()
		changed := v.setup.Rest(v.onBattery) != v.setup.Rest(onBattery)
		v.onBattery = onBattery
		v.mu.Unlock()
		if changed {
			s.reviewVoice(true)
		}
	})
}

// VoiceMode tells how the karaoke mode follows the voice now; false when it
// does not (its fill is off, or following is).
func (s *Session) VoiceMode() (voice.Mode, bool) {
	v := s.voice
	if v == nil {
		return voice.ModeEstimate, false
	}
	v.mu.Lock()
	follower, painting := v.follower, s.fillWanted()
	v.mu.Unlock()
	if follower == nil || !painting {
		return voice.ModeEstimate, false
	}
	return follower.Mode(), true
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
