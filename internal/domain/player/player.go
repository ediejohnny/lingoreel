// Package player models the external video players LingoReel can
// drive and the rule for choosing one.
package player

import "time"

// ID identifies a player program.
type ID string

// Known players.
const (
	MPV       ID = "mpv"
	VLC       ID = "vlc"
	PotPlayer ID = "potplayer"
	MPCHC     ID = "mpc-hc"
	MPCBE     ID = "mpc-be"
)

// defaultPriority is the automatic order for users without a preference.
var defaultPriority = []ID{MPV, VLC, PotPlayer}

// DisplayName returns the name shown to the user.
func (id ID) DisplayName() string {
	switch id {
	case MPV:
		return "mpv"
	case VLC:
		return "VLC"
	case PotPlayer:
		return "PotPlayer"
	case MPCHC:
		return "MPC-HC"
	case MPCBE:
		return "MPC-BE"
	}
	return string(id)
}

// Placement is a window position and size in screen pixels.
type Placement struct {
	X, Y          int
	Width, Height int
}

// Set reports whether the placement has a usable size.
func (p Placement) Set() bool {
	return p.Width > 0 && p.Height > 0
}

// LaunchRequest describes how a player should open a video.
type LaunchRequest struct {
	File string
	// AudioIndex is the audio track by its position among the file's audio
	// tracks, starting at zero; negative keeps the player's choice.
	AudioIndex  int
	StartPaused bool
	// Window is where the video window opens; a zero size leaves it to
	// the player. It only sets the start: the user resizes it at will.
	Window Placement
	// VideoWidth is the picture's width, for players that size their
	// window as a zoom of it; zero when unknown.
	VideoWidth int
	Start      time.Duration
	// Host is a window of the app the player draws its video into, for
	// plugins whose descriptor says DrawsInHost; zero lets the player open
	// its own window at Window. It is an opaque handle for the domain.
	Host uintptr
}

// Installation is a player found on this computer.
type Installation struct {
	ID      ID
	Path    string
	Version string
}

// Choice is the player picked for a session.
type Choice struct {
	Installation Installation
	// PreferredMissing is set when the user prefers a player that is not
	// installed (or not supported) anymore, so the UI can say so.
	PreferredMissing bool
}

// Choose picks the user's preferred player when it is installed and
// supported; otherwise the first supported player in the default priority,
// then any other supported player (a plugin outside the default list), in
// the order found. It reports false when no supported player is installed.
func Choose(installed []Installation, supported func(ID) bool, preferred ID) (Choice, bool) {
	find := func(id ID) (Installation, bool) {
		for _, installation := range installed {
			if installation.ID == id && supported(id) {
				return installation, true
			}
		}
		return Installation{}, false
	}

	if preferred != "" {
		if installation, found := find(preferred); found {
			return Choice{Installation: installation}, true
		}
	}
	for _, id := range defaultPriority {
		if installation, found := find(id); found {
			return Choice{Installation: installation, PreferredMissing: preferred != ""}, true
		}
	}
	for _, installation := range installed {
		if supported(installation.ID) {
			return Choice{Installation: installation, PreferredMissing: preferred != ""}, true
		}
	}
	return Choice{PreferredMissing: preferred != ""}, false
}
