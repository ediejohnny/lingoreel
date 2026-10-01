package player

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ErrInvalidDescriptor reports a player plugin that does not describe itself
// well enough to be offered to the user.
var ErrInvalidDescriptor = errors.New("invalid player descriptor")

// idPattern keeps player ids short, lower case and safe in file names and
// settings: "vlc", "mpv", "mpc-hc".
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// Valid reports an id a plugin may use and the settings may store.
func (id ID) Valid() bool {
	return idPattern.MatchString(string(id))
}

// maxDescriptorText bounds the names and texts a plugin shows.
const maxDescriptorText = 300

// Descriptor is what a player plugin tells about its player: how to call it
// and how the user can install it.
type Descriptor struct {
	ID   ID
	Name string
	// Install is how to get the player; both ways are offered to the user.
	Install Install
	// DrawsInHost tells the player can draw its video inside a window of
	// the app (LaunchRequest.Host). The app then owns the window: closing,
	// fullscreen, the keys and its own controls over the video.
	DrawsInHost bool
}

// Install tells how the user can get a player.
type Install struct {
	// WingetID is the package the automatic installation uses; empty when
	// the player has no winget package.
	WingetID string
	// DownloadURL is the official download page, always HTTPS.
	DownloadURL string
	// Benefit is the one-sentence reason to choose this player, shown by
	// the (?) next to it.
	Benefit string
}

// wingetIDPattern is the shape of a winget package id: "VideoLAN.VLC". It
// never starts with a dash, so it cannot become a command-line option.
var wingetIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_-]{0,99}$`)

// ValidWingetID reports a winget package id safe to install.
func ValidWingetID(id string) bool {
	return wingetIDPattern.MatchString(id)
}

// Validate checks every field a plugin must fill.
func (d Descriptor) Validate() error {
	switch {
	case !d.ID.Valid():
		return fmt.Errorf("%w: id %q", ErrInvalidDescriptor, d.ID)
	case !usableText(d.Name):
		return fmt.Errorf("%w: %s has no usable name", ErrInvalidDescriptor, d.ID)
	case !usableText(d.Install.Benefit):
		return fmt.Errorf("%w: %s has no usable benefit text", ErrInvalidDescriptor, d.ID)
	case d.Install.WingetID != "" && !ValidWingetID(d.Install.WingetID):
		return fmt.Errorf("%w: %s winget id %q", ErrInvalidDescriptor, d.ID, d.Install.WingetID)
	}
	link, err := url.Parse(d.Install.DownloadURL)
	if err != nil || link.Scheme != "https" || link.Host == "" || link.User != nil {
		return fmt.Errorf("%w: %s download page %q must be an https address", ErrInvalidDescriptor, d.ID, d.Install.DownloadURL)
	}
	return nil
}

func usableText(text string) bool {
	trimmed := strings.TrimSpace(text)
	return trimmed != "" && len(text) <= maxDescriptorText && !strings.ContainsAny(text, "\r\n\x00")
}
