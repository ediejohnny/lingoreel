package app

import (
	"errors"

	"github.com/ediejohnny/lingoreel/internal/application/speechpack"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// EventSpeechPacks tells the screens a speech pack changed: one began or
// stopped being fetched, went further, or was removed (SpeechPacksView).
const EventSpeechPacks = "speechpacks:changed"

// SpeechPacks are the languages' speech packs kept in the app's folder.
type SpeechPacks interface {
	States() []speechpack.State
	Fetch(language string) error
	Cancel()
	Remove(language string) error
}

var (
	errPackBusy         = worded(func() string { return ptbr.T("error.speechPack.busy") })
	errPackNotPublished = worded(func() string { return ptbr.T("error.speechPack.notPublished") })
	errPackDamaged      = worded(func() string { return ptbr.T("error.speechPack.notAsPublished") })
	errPackFailed       = worded(func() string { return ptbr.T("error.speechPack.failed") })
	errPackNotRemoved   = worded(func() string { return ptbr.T("error.speechPack.notRemoved") })
)

// errPackUnsuited reports a speech pack the computer's level does not suit,
// which the karaoke mode's fill does without.
var errPackUnsuited = errors.New("app: the speech pack does not suit this computer's level")

// openWords opens a language's speech pack for the karaoke mode's fill to
// follow the words: one that is here and suits the computer's level.
func (a *API) openWords(language string) (voice.WordModel, error) {
	if !a.machine().Level().SuitsSpeechPack(language) {
		return nil, errPackUnsuited
	}
	return a.deps.OpenSpeech(language)
}

// SpeechPacks tells every language's pack, with the computer's level, which
// marks the packs that suit it.
func (a *API) SpeechPacks() SpeechPacksView {
	level := a.machine().Level()
	view := SpeechPacksView{Level: string(level), Packs: []SpeechPackView{}}
	if a.deps.SpeechPacks == nil {
		return view
	}
	view.Available = true
	for _, state := range a.deps.SpeechPacks.States() {
		pack := SpeechPackView{
			Language: state.Language,
			Name:     ptbr.Capitalized(ptbr.LanguageName(media.LanguageTag(state.Language))),
			Size:     state.Size, Ready: state.Ready, Fetching: state.Fetching, Done: state.Done, Total: state.Total,
			Suits: level.SuitsSpeechPack(state.Language),
		}
		if state.Failed != nil {
			pack.Error = a.packFault(state.Failed).Error()
		}
		view.Packs = append(view.Packs, pack)
	}
	return view
}

// FetchSpeechPack starts fetching a language's pack in the background; the
// screens follow it through the speechpacks:changed event.
func (a *API) FetchSpeechPack(language string) error {
	if a.deps.SpeechPacks == nil {
		return errPackFailed
	}
	err := a.deps.SpeechPacks.Fetch(language)
	if errors.Is(err, speechpack.ErrBusy) {
		return errPackBusy
	}
	if err != nil {
		a.logger.Warn("speech pack not fetched", "language", language, "error", err)
		return errPackFailed
	}
	a.logger.Info("speech pack asked for", "language", language)
	return nil
}

// CancelSpeechPack stops the pack being fetched, if any.
func (a *API) CancelSpeechPack() {
	if a.deps.SpeechPacks != nil {
		a.deps.SpeechPacks.Cancel()
	}
}

// RemoveSpeechPack takes a language's pack off the computer.
func (a *API) RemoveSpeechPack(language string) error {
	if a.deps.SpeechPacks == nil {
		return errPackNotRemoved
	}
	err := a.deps.SpeechPacks.Remove(language)
	if errors.Is(err, speechpack.ErrBusy) {
		return errPackBusy
	}
	if err != nil {
		a.logger.Warn("speech pack not removed", "language", language, "error", err)
		return a.because(errPackNotRemoved, err)
	}
	a.logger.Info("speech pack removed", "language", language)
	return nil
}

// SpeechPacksChanged tells the screens a pack changed.
func (l Lifecycle) SpeechPacksChanged() {
	l.api.emit(EventSpeechPacks, l.api.SpeechPacks())
}

// packFault words why a pack's fetch stopped.
func (a *API) packFault(err error) error {
	switch {
	case errors.Is(err, speechpack.ErrNotPublished):
		return errPackNotPublished
	case errors.Is(err, speechpack.ErrNotAsPublished):
		return errPackDamaged
	}
	return a.because(errPackFailed, err)
}
