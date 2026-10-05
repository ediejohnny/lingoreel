// Package players is the contract every video player plugin follows, and the
// registry the app builds from the plugins it ships with. A plugin is an
// adapter: it describes its player, launches it and drives it. Anyone can
// add a player by writing one package that implements Plugin and adding it
// to the list in the composition root; see docs/plugins-de-player.md.
package players

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
)

// ErrUnknownPlayer reports a player without a registered plugin.
var ErrUnknownPlayer = errors.New("no plugin for this player")

// Plugin is a video player the app can drive.
type Plugin interface {
	// Descriptor names the player and tells how to install it.
	Descriptor() player.Descriptor
	// Launch opens a video in the player found at executable. It returns
	// once the video is open, or an error; a cancelled context stops it
	// early and leaves no process behind.
	Launch(ctx context.Context, executable string, request player.LaunchRequest) (Running, error)
}

// Running is a player with a video open.
type Running interface {
	// session.Player holds the commands. Every operation whose capability is
	// false must return an error (never pretend to work), so the app can
	// show it as unavailable.
	session.Player
	// Samples delivers observations of the player; it is closed when the
	// player exits or stops answering.
	Samples() <-chan playback.Sample
	// ProcessID identifies the player process, whose window the overlay
	// follows.
	ProcessID() int
	// Err tells why the samples ended: playback.ErrPlayerLost for a player
	// that crashed or stopped answering, nil for one the user closed.
	Err() error
	// Close ends the player within a bounded time; calling it again does
	// nothing. No process of the player may remain.
	Close() error
}

// Reconnector is what a Running may also offer: whether it is trying again
// to reach a player that stopped answering, before the session is lost.
type Reconnector interface {
	Reconnecting() bool
}

// Windowed is what a Running that draws in its own window may also offer:
// the windows as its plugin knows them, so the subtitles need not be put
// over the largest window of the process. Both may be zero at any moment,
// for a player that has not opened them yet or has closed them.
type Windowed interface {
	// Window is the player's main window, the one the subtitles follow.
	Window() uintptr
	// VideoArea is the window inside Window that holds the picture, so the
	// player's own list and buttons stay uncovered; zero covers it whole.
	VideoArea() uintptr
}

// Registry holds the plugins the app ships with.
type Registry struct {
	plugins []Plugin
}

// NewRegistry checks the plugins and keeps them in the given order. It
// refuses a plugin with an invalid descriptor or an id already taken.
func NewRegistry(plugins ...Plugin) (*Registry, error) {
	var ids []player.ID
	for i, plugin := range plugins {
		if plugin == nil {
			return nil, fmt.Errorf("%w: plugin %d is nil", player.ErrInvalidDescriptor, i)
		}
		descriptor := plugin.Descriptor()
		if err := descriptor.Validate(); err != nil {
			return nil, fmt.Errorf("plugin %d: %w", i, err)
		}
		if slices.Contains(ids, descriptor.ID) {
			return nil, fmt.Errorf("%w: two plugins for %s", player.ErrInvalidDescriptor, descriptor.ID)
		}
		ids = append(ids, descriptor.ID)
	}
	return &Registry{plugins: slices.Clone(plugins)}, nil
}

// Supported reports whether a plugin drives the player.
func (r *Registry) Supported(id player.ID) bool {
	_, found := r.Plugin(id)
	return found
}

// Plugin returns the plugin of a player.
func (r *Registry) Plugin(id player.ID) (Plugin, bool) {
	for _, plugin := range r.plugins {
		if plugin.Descriptor().ID == id {
			return plugin, true
		}
	}
	return nil, false
}

// Descriptors lists the registered players, in registration order.
func (r *Registry) Descriptors() []player.Descriptor {
	descriptors := make([]player.Descriptor, len(r.plugins))
	for i, plugin := range r.plugins {
		descriptors[i] = plugin.Descriptor()
	}
	return descriptors
}

// Launch opens a video with the plugin of an installed player.
func (r *Registry) Launch(ctx context.Context, installation player.Installation, request player.LaunchRequest) (Running, error) {
	plugin, found := r.Plugin(installation.ID)
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrUnknownPlayer, installation.ID)
	}
	return plugin.Launch(ctx, installation.Path, request)
}
