package app

import (
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

var (
	errWelcomeWhileWatching = worded(func() string { return ptbr.T("error.welcome.watching") })
	errWelcomeLanguage      = worded(func() string { return ptbr.T("error.welcome.language") })
)

// Welcome is what the welcome asks, with the answers it suggests: on the
// first run, what Windows tells; opened again from the settings, what the
// active profile holds.
func (a *API) Welcome() WelcomeView {
	a.mu.Lock()
	pending, language := a.welcoming, a.config.Language
	profile := a.profile()
	a.mu.Unlock()
	languages, mode := profile.Languages, profile.Study.Mode
	if pending {
		languages = settings.SuggestedLanguages(a.systemLanguage())
	}
	view := WelcomeView{
		Pending: pending, AppLanguage: language, AppLanguages: interfaceLanguages(),
		Spoken: languages.Translation, Studied: languages.Primary, Mode: string(mode), Modes: modeOptions(),
		PreferredPlayer: profile.PreferredPlayer, Languages: []Option{},
	}
	listed := map[string]bool{}
	add := func(tag string) {
		if tag == "" || listed[tag] {
			return
		}
		listed[tag] = true
		view.Languages = append(view.Languages, Option{Value: tag, Label: ptbr.Capitalized(ptbr.LanguageName(media.LanguageTag(tag)))})
	}
	for _, tag := range previewLanguages {
		add(string(tag))
	}
	// A language the app does not list (the Windows one, or set by hand) is
	// offered too, so the answer shown is one of the choices.
	add(view.Spoken)
	add(view.Studied)
	return view
}

// FinishWelcome keeps the welcome's answers. On the first run they make the
// first profile, named after the language studied; afterwards they change
// the active profile, or, for another language studied, a profile of that
// language (a copy of the active one when there is none), which becomes the
// active one. The app's language was applied when it was chosen.
func (a *API) FinishWelcome(choices WelcomeChoices) error {
	if current := a.current(); current != nil && current.session != nil {
		return errWelcomeWhileWatching
	}
	if !study.Mode(choices.Mode).Valid() {
		return errUnknownMode
	}
	if choices.PreferredPlayer != "" && !a.deps.Registry.Supported(player.ID(choices.PreferredPlayer)) {
		return ErrUnknownPlayer
	}
	languages := settings.LanguagePreferences{Audio: choices.Studied, Primary: choices.Studied, Translation: choices.Spoken}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.answerWelcome(languages, study.Mode(choices.Mode), choices.PreferredPlayer)
}

// SkipWelcome answers the first run's welcome with what Windows tells; once
// answered, it does nothing.
func (a *API) SkipWelcome() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.welcoming {
		return nil
	}
	profile := a.profile()
	return a.answerWelcome(settings.SuggestedLanguages(a.systemLanguage()), profile.Study.Mode, profile.PreferredPlayer)
}

// answerWelcome keeps a welcome's answers. Called with a.mu held.
func (a *API) answerWelcome(languages settings.LanguagePreferences, mode study.Mode, preferred string) error {
	name := ptbr.Capitalized(ptbr.LanguageName(media.LanguageTag(languages.Primary)))
	target := a.profile()
	profiles := a.profiles
	switch {
	case a.welcoming && untouched(profiles):
		// The first profile is the person's own, not the defaults' "Inglês".
		target = settings.FirstProfile(name, languages)
		profiles = settings.Profiles{Items: []settings.Profile{target}}
	case !strings.EqualFold(target.Languages.Primary, languages.Primary):
		if existing, found := profiles.Find(name); found {
			target = existing
		} else {
			added, err := profiles.Add(name, target)
			if err != nil {
				return profileError(err)
			}
			profiles, target.Name = added, name
		}
	}
	target.Languages, target.Study.Mode, target.PreferredPlayer = languages, mode, preferred
	if err := target.Validate(); err != nil {
		return errWelcomeLanguage
	}
	profiles, err := profiles.Replace(target)
	if err != nil {
		return profileError(err)
	}
	if err := a.commitProfiles(profiles); err != nil {
		return err
	}
	config := a.config
	config.ActiveProfile = target.Name
	if err := a.commitConfig(config); err != nil {
		return err
	}
	a.welcoming = false
	a.logger.Info("welcome answered", "studies", languages.Primary, "speaks", languages.Translation, "mode", mode)
	return nil
}

// untouched tells the profiles are still the defaults a first run starts
// with: one profile, the default one's name.
func untouched(profiles settings.Profiles) bool {
	return len(profiles.Items) == 1 && profiles.Items[0].Name == settings.DefaultProfileName
}

// systemLanguage is the Windows display language, empty when unknown.
func (a *API) systemLanguage() string {
	if a.deps.SystemLanguage == nil {
		return ""
	}
	return a.deps.SystemLanguage()
}
