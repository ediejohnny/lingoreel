package app

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

var (
	errProfileExists   = worded(func() string { return ptbr.T("error.profile.exists") })
	errProfileMissing  = worded(func() string { return ptbr.T("error.profile.missing") })
	errLastProfile     = worded(func() string { return ptbr.T("error.profile.last") })
	errBadProfileName  = worded(func() string { return ptbr.T("error.profile.name") })
	errBadPlayerPath   = worded(func() string { return ptbr.T("error.player.path") })
	errSwitchWhileOpen = worded(func() string { return ptbr.T("error.profile.switchWhileWatching") })
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
	return a.changeConfig(func(next *settings.Config) error {
		profile, found := a.profiles.Find(name)
		if !found {
			return errProfileMissing
		}
		next.ActiveProfile = profile.Name
		return nil
	})
}

// CreateProfile adds a profile, a copy of the active one, and makes it the
// active one.
func (a *API) CreateProfile(name string) error {
	if current := a.current(); current != nil && current.session != nil {
		return errSwitchWhileOpen
	}
	return a.changeSettings(writeProfiles|writeConfig, func(config *settings.Config, profiles *settings.Profiles) error {
		next, err := profiles.Add(name, a.profile())
		if err != nil {
			return profileError(err)
		}
		*profiles, config.ActiveProfile = next, name
		return nil
	})
}

// RenameProfile renames a profile; the active one stays active.
func (a *API) RenameProfile(from, to string) error {
	// Whether it is the active one is told with the change, so a switch made
	// meanwhile is not lost; the configuration is written either way.
	return a.changeSettings(writeProfiles|writeConfig, func(config *settings.Config, profiles *settings.Profiles) error {
		wasActive := strings.EqualFold(a.profile().Name, from)
		next, err := profiles.Rename(from, to)
		if err != nil {
			return profileError(err)
		}
		*profiles = next
		if wasActive {
			config.ActiveProfile = to
		}
		return nil
	})
}

// DeleteProfile removes a profile; when it was the active one, the first
// profile left becomes active.
func (a *API) DeleteProfile(name string) error {
	if current := a.current(); current != nil && current.session != nil && strings.EqualFold(a.activeName(), name) {
		return errSwitchWhileOpen
	}
	return a.changeSettings(writeProfiles|writeConfig, func(config *settings.Config, profiles *settings.Profiles) error {
		wasActive := strings.EqualFold(a.profile().Name, name)
		next, err := profiles.Remove(name)
		if err != nil {
			return profileError(err)
		}
		*profiles = next
		if wasActive {
			config.ActiveProfile = next.Items[0].Name
		}
		return nil
	})
}

func (a *API) activeName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.profile().Name
}

// ResetProfile puts the active profile back to the defaults, keeping its
// name, and applies them to the video being watched.
func (a *API) ResetProfile() error {
	var previous, profile settings.Profile
	err := a.changeProfiles(func(next *settings.Profiles) error {
		previous = a.profile()
		profile = settings.DefaultProfile()
		profile.Name = previous.Name
		var err error
		*next, err = next.Replace(profile)
		return err
	})
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

// setPlayerPath keeps where a player's program is, for a player installed
// where the app does not look; only a program the user chose comes here.
func (a *API) setPlayerPath(id, path string) error {
	if !player.ID(id).Valid() || !a.deps.Registry.Supported(player.ID(id)) {
		return ErrUnknownPlayer
	}
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".exe") {
		return errBadPlayerPath
	}
	return a.changeConfig(func(next *settings.Config) error {
		next.PlayerPaths = maps.Clone(next.PlayerPaths)
		if next.PlayerPaths == nil {
			next.PlayerPaths = map[string]string{}
		}
		next.PlayerPaths[id] = path
		return nil
	})
}

// ChoosePlayerProgram asks for a player's program and keeps it; it returns
// the path, or empty when the user cancelled.
func (a *API) ChoosePlayerProgram(id string) (string, error) {
	if !a.deps.Registry.Supported(player.ID(id)) {
		return "", ErrUnknownPlayer
	}
	ui := a.window()
	if ui == nil {
		return "", ErrNotReady
	}
	path, err := ui.OpenProgramDialog(ptbr.T("dialog.playerProgram", "player", a.playerName(player.ID(id))))
	if err != nil || path == "" {
		return "", err
	}
	if err := a.setPlayerPath(id, path); err != nil {
		return "", err
	}
	return path, nil
}

// ClearPlayerPath forgets the path set for a player: the app looks for it
// again where players usually are.
func (a *API) ClearPlayerPath(id string) error {
	a.mu.Lock()
	_, set := a.config.PlayerPaths[id]
	a.mu.Unlock()
	if !set {
		return nil
	}
	return a.changeConfig(func(next *settings.Config) error {
		next.PlayerPaths = maps.Clone(next.PlayerPaths)
		delete(next.PlayerPaths, id)
		return nil
	})
}
