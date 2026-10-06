package app

import (
	"context"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// A video with subtitle files nothing could measure (no subtitle inside it
// to compare them with) is heard as it opens, in the background, to tell
// which file fits it: the session screen shows how far it went, puts the
// file that fits on top once it is known, and the project keeps what was
// found, so the video is not heard again the next time.

// EventListening tells the session screen how far hearing the open video
// has gone (a Listening).
const EventListening = "video:listening"

// findTimeout bounds the search for the user's FFmpeg before hearing.
const findTimeout = 10 * time.Second

// listening is the hearing of the open video's voice.
type listening struct {
	cancel context.CancelFunc
	state  Listening
}

// withKeptFits gives the files of a video the fits the project kept for
// them, while their lines are the ones heard then.
func withKeptFits(result analysis.Analysis, p *videoProject, preferences media.Preferences) analysis.Analysis {
	fits := map[uint64]subtitle.Alignment{}
	for _, number := range result.Unheard() {
		key := p.keys[number]
		if kept := p.value.Tracks[key].Fit; key != "" && kept != nil && kept.Of == project.FitOf(result.FileLines(number)) {
			fits[number] = kept.Alignment()
		}
	}
	return result.WithFits(fits, preferences)
}

// startListening starts hearing a video just opened, when some of its files
// are still unheard and the voice model may work. Called with a.mu held.
func (a *API) startListening(ctx context.Context, result analysis.Analysis, preferences media.Preferences) {
	ports := a.deps.Ports
	setup := voiceSetup(a.config.Voice)
	onBattery := ports.Power != nil && ports.Power.OnBattery()
	stretches := subtitle.HearingStretches(result.HearingLength())
	if len(result.Unheard()) == 0 || len(stretches) == 0 || ports.Model == nil || ports.FindDecoder == nil ||
		setup.Rest(onBattery) != watch.ModelAtWork {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	l := &listening{cancel: cancel, state: Listening{Path: result.Path, Active: true, Stretch: 1, Stretches: len(stretches)}}
	a.listen = l
	go a.listenTo(ctx, l, result, stretches, setup.FFmpegPath, preferences)
}

// stopListening stops hearing a video, which is no longer the one open.
// Called with a.mu held.
func (a *API) stopListening() {
	if a.listen != nil {
		a.listen.cancel()
		a.listen = nil
	}
}

// listeningTo is how far hearing a video has gone, inactive when it is not
// heard. Called with a.mu held.
func (a *API) listeningTo(path string) Listening {
	if l := a.listen; l != nil && l.state.Path == path {
		return l.state
	}
	return Listening{Path: path}
}

// listenTo hears the stretches of a video's voice, then lines its unheard
// files up with it.
func (a *API) listenTo(ctx context.Context, l *listening, result analysis.Analysis, stretches []subtitle.HearingStretch, ffmpegPath string, preferences media.Preferences) {
	found, cancelFind := context.WithTimeout(ctx, findTimeout)
	decoder := a.deps.Ports.FindDecoder(found, ffmpegPath)
	cancelFind()
	if decoder == nil {
		a.logger.Info("the video was not heard: no FFmpeg found")
		a.endListening(l, nil) // the files stay unheard
		return
	}
	base := voiceOf(result, preferences.Primary)
	heard := make([]speech.Track, 0, len(stretches))
	for i, s := range stretches {
		stretch := base
		stretch.From, stretch.Length = s.From, s.Length
		track, err := voice.Hear(ctx, decoder, a.deps.Ports.Model, stretch, func(done float64) { a.listened(l, i, done) })
		if err != nil {
			if ctx.Err() == nil {
				a.logger.Warn("hearing the video failed", "error", err)
			}
			a.endListening(l, nil)
			return
		}
		heard = append(heard, track)
	}
	_, fits := analysis.FitByVoice(result, heard, preferences)
	a.logger.Info("the video was heard", "files", len(fits))
	a.endListening(l, fits)
}

// voiceOf is the audio track heard: the one speaking the language of the
// line on top, the film's own (subtitles are timed to it, not to a dub);
// else the first.
func voiceOf(result analysis.Analysis, language media.LanguageTag) voice.Stretch {
	stretch := voice.Stretch{File: result.Path}
	track, position, found := media.VoiceTrack(result.Info, language)
	if !found {
		audio := result.Info.TracksOf(media.TrackAudio)
		if len(audio) == 0 {
			return stretch // a video whose tracks are not read: its first audio
		}
		track, position = audio[0], 0
	}
	stretch.Track, stretch.Channels, stretch.Codec = position, track.Channels, track.Codec
	return stretch
}

// listened tells the screen how far hearing went, when it moved by a percent.
func (a *API) listened(l *listening, stretch int, done float64) {
	a.mu.Lock()
	total := l.state.Stretches
	percent := min(int(100*(float64(stretch)+done)/float64(total)), 99)
	if a.listen != l || (percent == l.state.Percent && stretch+1 == l.state.Stretch) {
		a.mu.Unlock()
		return
	}
	l.state.Percent, l.state.Stretch = percent, stretch+1
	state := l.state
	a.mu.Unlock()
	a.emit(EventListening, state)
}

// endListening gives the open video the fits found, keeps them in its
// project and tells the screens: the video described again, with the file
// that fits on top and the hearing over, even when nothing was found, so
// the session screen hears of the end one way only. A video no longer open
// changes nothing; a session watching it is left as it is, and the next
// one starts with the files' fits.
func (a *API) endListening(l *listening, fits map[uint64]subtitle.Alignment) {
	a.mu.Lock()
	if a.listen != l {
		a.mu.Unlock()
		return
	}
	a.listen = nil
	path := l.state.Path
	if a.video == nil || a.video.Path != path {
		a.mu.Unlock()
		a.emit(EventListening, Listening{Path: path})
		return
	}
	languages := a.profile().Languages
	preferences := media.Preferences{
		Audio: media.LanguageTag(languages.Audio), Primary: media.LanguageTag(languages.Primary),
		Translation: media.LanguageTag(languages.Translation),
	}
	mode := a.profile().Study.Mode
	next := a.video.WithFits(fits, preferences)
	a.video = &next
	opened, suggested := a.project, next.Suggested
	if opened != nil && opened.video == path {
		for number, fit := range fits {
			if key := opened.keys[number]; key != "" {
				track := opened.track(key)
				kept := project.FitFrom(fit, project.FitOf(next.FileLines(number)))
				track.Fit = &kept
				opened.value.Tracks[key] = track
			}
		}
		suggested, mode = opened.remembered(next.Info, next.Suggested, mode)
	} else {
		opened = nil
	}
	a.mu.Unlock()
	a.emit(EventVideo, a.describeVideo(next, opened, suggested, mode))
	a.emit(EventListening, Listening{Path: path})
}
