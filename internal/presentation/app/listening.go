package app

import (
	"context"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
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
func (a *API) startListening(ctx context.Context, result analysis.Analysis, preferences media.Preferences, onBattery bool) {
	ports := a.deps.Ports
	setup := voiceSetup(a.config.Voice)
	stretches := subtitle.HearingStretches(result.HearingLength())
	if len(result.Unheard()) == 0 || len(stretches) == 0 || ports.Model == nil || ports.FindDecoder == nil ||
		setup.Rest(onBattery) != watch.ModelAtWork {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	l := &listening{cancel: cancel, state: Listening{Path: result.Path, Active: true, Stretch: 1, Stretches: len(stretches)}}
	a.listen = l
	go a.listenTo(ctx, l, result, setup.FFmpegPath, preferences)
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

// listenTo hears the stretches of a video's voice in the language of the
// line on top, and the ones between them when the first leave a file
// undecided, then lines its unheard files up with it.
func (a *API) listenTo(ctx context.Context, l *listening, result analysis.Analysis, ffmpegPath string, preferences media.Preferences) {
	decoder := a.decoderFor(ctx, ffmpegPath, "the video")
	if decoder == nil {
		a.endListening(l, nil) // the files stay unheard
		return
	}
	heard, err := analysis.HearVoice(ctx, result, decoder, a.deps.Ports.Model, preferences.Primary, func(stretch int, done float64) {
		a.listened(l, stretch, done)
	})
	var between []speech.Track
	if more := subtitle.HearingStretchesBetween(result.HearingLength()); err == nil && len(more) > 0 && analysis.Undecided(result, heard) {
		first := len(heard)
		a.mu.Lock()
		l.state.Stretches = first + len(more)
		a.mu.Unlock()
		between, err = analysis.HearMoreVoice(ctx, result, decoder, a.deps.Ports.Model, preferences.Primary, func(stretch int, done float64) {
			a.listened(l, first+stretch, done)
		})
	}
	if err != nil {
		if ctx.Err() == nil {
			a.logger.Warn("hearing the video failed", "error", err)
		}
		a.endListening(l, nil)
		return
	}
	_, fits := analysis.FitByVoice(result, heard, between, preferences)
	a.logger.Info("the video was heard", "files", len(fits), "stretches", len(heard)+len(between))
	a.endListening(l, fits)
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
// project, where they may also replace a file kept from an earlier opening
// that does not fit, and tells the screens: the video described again, with
// the file that fits on top and the hearing over, even when nothing was found, so
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
	a.showFound(path, func(video analysis.Analysis, preferences media.Preferences) analysis.Analysis {
		return video.WithFits(fits, preferences)
	}, func(opened *videoProject, next analysis.Analysis) {
		for number, fit := range fits {
			if key := opened.keys[number]; key != "" {
				track := opened.track(key)
				kept := project.FitFrom(fit, project.FitOf(next.FileLines(number)))
				track.Fit = &kept
				opened.value.Tracks[key] = track
			}
		}
		opened.correctInherited(next.Info, fits)
	})
	a.emit(EventListening, Listening{Path: path})
}

// showFound gives the open video what a hearing found, while it is still the
// one at path: apply makes its analysis anew with the profile's preferences,
// keep puts in its project what is kept of it, the tracks are suggested again
// over what the project remembers, and the screens are told the video. It
// tells whether the video was still open. Called with a.mu held, which it
// lets go.
func (a *API) showFound(path string, apply func(analysis.Analysis, media.Preferences) analysis.Analysis, keep func(*videoProject, analysis.Analysis)) bool {
	if a.video == nil || a.video.Path != path {
		a.mu.Unlock()
		return false
	}
	mode := a.profile().Study.Mode
	next := apply(*a.video, preferencesOf(a.profile().Languages))
	a.video = &next
	opened, suggested := a.project, next.Suggested
	if opened != nil && opened.video == path {
		keep(opened, next)
		suggested, mode = opened.remembered(next.Info, next.Suggested, mode)
	} else {
		opened = nil
	}
	a.mu.Unlock()
	a.emit(EventVideo, a.describeVideo(next, opened, suggested, mode))
	return true
}
