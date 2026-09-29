// Package player models the external video players the LingoReel can
// drive and the rule for choosing one.
package player

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
// supported; otherwise the first supported player in the default priority.
// It reports false when no supported player is installed.
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
	return Choice{PreferredMissing: preferred != ""}, false
}
