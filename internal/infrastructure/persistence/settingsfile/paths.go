// Package settingsfile holds the checks on stored settings that depend on the
// operating system, kept out of the domain.
package settingsfile

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"lingoreel/internal/domain/settings"
)

// executableExtension is what a player program file ends with on Windows.
const executableExtension = ".exe"

// ErrInvalidPlayerPath reports a configured player that is not an absolute
// path to a program file.
var ErrInvalidPlayerPath = errors.New("player path must be an absolute .exe path")

// CheckPlayerPaths rejects player paths that are relative or that name
// anything but a program, so a hand-edited configuration cannot point the
// app at a script or at a file found through the current folder. The loader
// runs it next to the domain's own Validate.
func CheckPlayerPaths(config settings.Config) error {
	for playerID, path := range config.PlayerPaths {
		if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), executableExtension) {
			return fmt.Errorf("%w: %q for %q", ErrInvalidPlayerPath, path, playerID)
		}
	}
	return nil
}
