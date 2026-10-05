package app

import (
	"errors"
	"slices"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// ErrUnknownPlayer reports a player id the app has no plugin for.
var ErrUnknownPlayer = worded(func() string { return ptbr.T("error.player.unsupported") })

// Players lists the players on the computer again, after an installation
// or when the user asks.
func (a *API) Players() (Players, error) {
	if _, err := a.context(); err != nil {
		return Players{}, err
	}
	return a.players(), nil
}

// players detects the players and decides which one will be used: the
// user's preferred one when installed, else the default order. Installation
// cards appear only when no supported player is installed.
func (a *API) players() Players {
	a.mu.Lock()
	configured := make(map[player.ID]string, len(a.config.PlayerPaths))
	for id, path := range a.config.PlayerPaths {
		configured[player.ID(id)] = path
	}
	preferred := player.ID(a.profile().PreferredPlayer)
	dismissed := a.config.InstallRecommendations
	a.mu.Unlock()

	var installed []player.Installation
	if a.deps.Detect != nil {
		installed = a.deps.Detect(configured)
	}
	choice, found := player.Choose(installed, a.deps.Registry.Supported, preferred)
	// Empty lists, never null: the screens read them as lists.
	result := Players{
		Preferred: string(preferred), PreferredMissing: choice.PreferredMissing,
		Installed: []PlayerOption{}, Recommendations: []PlayerInstall{},
	}
	if found {
		result.Choice = string(choice.Installation.ID)
	}
	for _, installation := range installed {
		option := PlayerOption{
			ID: string(installation.ID), Name: a.playerName(installation.ID), Path: installation.Path,
			Supported: a.deps.Registry.Supported(installation.ID),
		}
		if plugin, ok := a.deps.Registry.Plugin(installation.ID); ok {
			option.FixedAudio = plugin.Descriptor().FixedAudioTrack
		}
		result.Installed = append(result.Installed, option)
	}
	if found {
		return result
	}
	autoInstall := a.deps.Installer != nil && a.deps.Installer.Available()
	for _, descriptor := range a.deps.Registry.Descriptors() {
		if dismissed.IsDismissed(string(descriptor.ID)) {
			continue
		}
		result.Recommendations = append(result.Recommendations, PlayerInstall{
			ID: string(descriptor.ID), Name: descriptor.Name, Benefit: descriptor.Install.Benefit,
			AutoInstall: autoInstall && descriptor.Install.WingetID != "",
		})
	}
	return result
}

// playerName is the plugin's name for a player, or its usual name.
func (a *API) playerName(id player.ID) string {
	if plugin, found := a.deps.Registry.Plugin(id); found {
		return plugin.Descriptor().Name
	}
	return id.DisplayName()
}

// installation finds an installed player the app can drive.
func (a *API) installation(id string) (player.Installation, error) {
	if !a.deps.Registry.Supported(player.ID(id)) {
		return player.Installation{}, ErrUnknownPlayer
	}
	a.mu.Lock()
	configured := make(map[player.ID]string, len(a.config.PlayerPaths))
	for key, path := range a.config.PlayerPaths {
		configured[player.ID(key)] = path
	}
	a.mu.Unlock()
	var installed []player.Installation
	if a.deps.Detect != nil {
		installed = a.deps.Detect(configured)
	}
	if i := slices.IndexFunc(installed, func(found player.Installation) bool { return found.ID == player.ID(id) }); i >= 0 {
		return installed[i], nil
	}
	return player.Installation{}, errors.New(ptbr.T("error.player.missing"))
}

// InstallPlayer installs a player with winget in the background; the window
// hears the result through the player:install event.
func (a *API) InstallPlayer(id string) error {
	ctx, err := a.context()
	if err != nil {
		return err
	}
	plugin, found := a.deps.Registry.Plugin(player.ID(id))
	if !found {
		return ErrUnknownPlayer
	}
	packageID := plugin.Descriptor().Install.WingetID
	if a.deps.Installer == nil || !a.deps.Installer.Available() || packageID == "" {
		return errors.New(ptbr.T("error.install.unavailable"))
	}
	go func() {
		progress := InstallProgress{ID: id, Done: true}
		if err := a.deps.Installer.Install(ctx, packageID); err != nil {
			a.logger.Warn("player installation failed", "player", id, "error", err)
			progress.Error = ptbr.T("error.install.failed")
		}
		a.emit(EventInstall, progress)
	}()
	return nil
}

// OpenDownloadPage opens the player's official download page in the browser.
func (a *API) OpenDownloadPage(id string) error {
	plugin, found := a.deps.Registry.Plugin(player.ID(id))
	if !found {
		return ErrUnknownPlayer
	}
	a.mu.Lock()
	ui := a.ui
	a.mu.Unlock()
	if ui == nil {
		return ErrNotReady
	}
	// The descriptor's address is checked to be https when registered.
	ui.OpenURL(plugin.Descriptor().Install.DownloadURL)
	return nil
}

// DismissRecommendation hides the installation card of one player for good;
// an empty id hides all of them.
func (a *API) DismissRecommendation(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	recommendations := &a.config.InstallRecommendations
	switch {
	case id == "":
		recommendations.DismissAll = true
	case !player.ID(id).Valid():
		return ErrUnknownPlayer
	case !slices.Contains(recommendations.Dismissed, id):
		recommendations.Dismissed = append(recommendations.Dismissed, id)
	}
	return a.saveConfig()
}

// ShowRecommendations shows the installation cards again.
func (a *API) ShowRecommendations() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.InstallRecommendations = settings.InstallRecommendations{}
	return a.saveConfig()
}
