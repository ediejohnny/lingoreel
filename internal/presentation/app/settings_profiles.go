package app

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
)

var (
	errProfileExists   = errors.New("Já existe um perfil com esse nome.")
	errProfileMissing  = errors.New("Esse perfil não existe mais.")
	errLastProfile     = errors.New("É preciso manter pelo menos um perfil.")
	errBadProfileName  = errors.New("Dê ao perfil um nome de até 60 letras, sem espaços no começo ou no fim.")
	errBadPlayerPath   = errors.New("Escolha o programa do player: um arquivo .exe.")
	errSwitchWhileOpen = errors.New("Encerre o vídeo antes de trocar de perfil.")
)

// profileError words a domain error of the profile changes.
func profileError(err error) error {
	switch {
	case errors.Is(err, settings.ErrProfileExists):
		return errProfileExists
	case errors.Is(err, settings.ErrProfileNotFound):
		return errProfileMissing
	case errors.Is(err, settings.ErrLastProfile):
		return errLastProfile
	case errors.Is(err, settings.ErrInvalidSettings):
		return errBadProfileName
	}
	return err
}

// SwitchProfile makes another profile the active one. A video being watched
// keeps its profile: the switch waits for it to end.
func (a *API) SwitchProfile(name string) error {
	if current := a.current(); current != nil && current.session != nil {
		return errSwitchWhileOpen
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	profile, found := a.profiles.Find(name)
	if !found {
		return errProfileMissing
	}
	next := a.config
	next.ActiveProfile = profile.Name
	return a.commitConfig(next)
}

// CreateProfile adds a profile, a copy of the active one, and makes it the
// active one.
func (a *API) CreateProfile(name string) error {
	if current := a.current(); current != nil && current.session != nil {
		return errSwitchWhileOpen
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next, err := a.profiles.Add(name, a.profile())
	if err != nil {
		return profileError(err)
	}
	if err := a.commitProfiles(next); err != nil {
		return err
	}
	config := a.config
	config.ActiveProfile = name
	return a.commitConfig(config)
}

// RenameProfile renames a profile; the active one stays active.
func (a *API) RenameProfile(from, to string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	wasActive := strings.EqualFold(a.profile().Name, from)
	next, err := a.profiles.Rename(from, to)
	if err != nil {
		return profileError(err)
	}
	if err := a.commitProfiles(next); err != nil {
		return err
	}
	if !wasActive {
		return nil
	}
	config := a.config
	config.ActiveProfile = to
	return a.commitConfig(config)
}

// DeleteProfile removes a profile; when it was the active one, the first
// profile left becomes active.
func (a *API) DeleteProfile(name string) error {
	if current := a.current(); current != nil && current.session != nil && strings.EqualFold(a.activeName(), name) {
		return errSwitchWhileOpen
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	wasActive := strings.EqualFold(a.profile().Name, name)
	next, err := a.profiles.Remove(name)
	if err != nil {
		return profileError(err)
	}
	if err := a.commitProfiles(next); err != nil {
		return err
	}
	if !wasActive {
		return nil
	}
	config := a.config
	config.ActiveProfile = next.Items[0].Name
	return a.commitConfig(config)
}

func (a *API) activeName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.profile().Name
}

// ResetProfile puts the active profile back to the defaults, keeping its
// name, and applies them to the video being watched.
func (a *API) ResetProfile() error {
	a.mu.Lock()
	previous := a.profile()
	profile := settings.DefaultProfile()
	profile.Name = previous.Name
	next, err := a.profiles.Replace(profile)
	if err == nil {
		err = a.commitProfiles(next)
	}
	a.mu.Unlock()
	if err != nil {
		return profileError(err)
	}
	a.applyToSession(previous, profile)
	return nil
}

// playerSettings lists the players the app knows: the supported ones, and
// any other found on the computer.
func (a *API) playerSettings(config settings.Config) []PlayerSetting {
	configured := make(map[player.ID]string, len(config.PlayerPaths))
	for id, path := range config.PlayerPaths {
		configured[player.ID(id)] = path
	}
	var installed []player.Installation
	if a.deps.Detect != nil {
		installed = a.deps.Detect(configured)
	}
	found := map[player.ID]string{}
	for _, installation := range installed {
		found[installation.ID] = installation.Path
	}
	list := []PlayerSetting{}
	seen := map[player.ID]bool{}
	add := func(id player.ID) {
		if seen[id] {
			return
		}
		seen[id] = true
		_, custom := config.PlayerPaths[string(id)]
		list = append(list, PlayerSetting{
			ID: string(id), Name: a.playerName(id), Supported: a.deps.Registry.Supported(id),
			Path: found[id], Custom: custom,
		})
	}
	for _, descriptor := range a.deps.Registry.Descriptors() {
		add(descriptor.ID)
	}
	for _, id := range slices.Sorted(maps.Keys(found)) {
		add(id)
	}
	return list
}

// SetPlayerPath tells where a player's program is, for a player installed
// where the app does not look.
func (a *API) SetPlayerPath(id, path string) error {
	if !player.ID(id).Valid() || !a.deps.Registry.Supported(player.ID(id)) {
		return ErrUnknownPlayer
	}
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".exe") {
		return errBadPlayerPath
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.config
	next.PlayerPaths = maps.Clone(a.config.PlayerPaths)
	if next.PlayerPaths == nil {
		next.PlayerPaths = map[string]string{}
	}
	next.PlayerPaths[id] = path
	return a.commitConfig(next)
}

// ChoosePlayerProgram asks for a player's program and keeps it; it returns
// the path, or empty when the user cancelled.
func (a *API) ChoosePlayerProgram(id string) (string, error) {
	if !a.deps.Registry.Supported(player.ID(id)) {
		return "", ErrUnknownPlayer
	}
	a.mu.Lock()
	ui := a.ui
	a.mu.Unlock()
	if ui == nil {
		return "", ErrNotReady
	}
	path, err := ui.OpenProgramDialog("Onde está o " + a.playerName(player.ID(id)) + "?")
	if err != nil || path == "" {
		return "", err
	}
	if err := a.SetPlayerPath(id, path); err != nil {
		return "", err
	}
	return path, nil
}

// ClearPlayerPath forgets the path set for a player: the app looks for it
// again where players usually are.
func (a *API) ClearPlayerPath(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, set := a.config.PlayerPaths[id]; !set {
		return nil
	}
	next := a.config
	next.PlayerPaths = maps.Clone(a.config.PlayerPaths)
	delete(next.PlayerPaths, id)
	return a.commitConfig(next)
}
