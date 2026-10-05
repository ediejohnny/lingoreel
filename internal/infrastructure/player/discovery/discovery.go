// Package discovery finds the video players installed on this computer.
package discovery

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
)

// Root selects a registry hive.
type Root int

const (
	LocalMachine Root = iota
	CurrentUser
)

// Environment is the part of the operating system the detector reads, so
// detection can be tested without touching the real machine.
type Environment interface {
	RegistryString(root Root, path, name string) (string, bool)
	RegistrySubkeys(root Root, path string) []string
	FileExists(path string) bool
	Getenv(name string) string
	LookPath(file string) (string, bool)
	Glob(pattern string) []string
}

const (
	// AppPathsKey is where Windows records the full path of installed
	// programs; the default value of a subkey named after the executable
	// holds its path.
	AppPathsKey  = `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`
	uninstallKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
	uninstall32  = `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`
)

// RegistryValue is a registry value that holds a path to a player.
type RegistryValue struct {
	Root Root
	Path string
	Name string
	// IsDirectory marks values holding the install folder instead of the exe.
	IsDirectory bool
}

// Hints describe where a player usually lives. Each player plugin brings
// its own; players without a plugin are listed by Known.
type Hints struct {
	ID player.ID
	// Executables are the file names the player may have, in preference
	// order; a path found anywhere must end in one of them.
	Executables []string
	Registry    []RegistryValue
	// ProgramFiles are paths relative to the Program Files folders.
	ProgramFiles []string
	// LocalAppData are paths relative to %LOCALAPPDATA%, which may use globs.
	LocalAppData []string
	// UninstallName matches the DisplayName of the player's uninstall
	// entry; nil skips the uninstall entries.
	UninstallName *regexp.Regexp
}

// known are the players detected so the app can say "coming soon" for them,
// until a plugin brings their own hints.
var known = []Hints{
	{
		ID:            player.MPV,
		Executables:   []string{"mpv.exe"},
		Registry:      []RegistryValue{{LocalMachine, AppPathsKey + "mpv.exe", "", false}, {CurrentUser, AppPathsKey + "mpv.exe", "", false}},
		ProgramFiles:  []string{`mpv\mpv.exe`},
		LocalAppData:  []string{`Microsoft\WinGet\Links\mpv.exe`, `Microsoft\WinGet\Packages\mpv-player.mpv*\mpv.exe`, `Programs\mpv\mpv.exe`},
		UninstallName: regexp.MustCompile(`(?i)^mpv\b`),
	},
	{
		ID:            player.MPCHC,
		Executables:   []string{"mpc-hc64.exe", "mpc-hc.exe"},
		Registry:      []RegistryValue{{CurrentUser, `SOFTWARE\MPC-HC\MPC-HC`, "ExePath", false}},
		ProgramFiles:  []string{`MPC-HC\mpc-hc64.exe`, `K-Lite Codec Pack\MPC-HC64\mpc-hc64.exe`, `MPC-HC\mpc-hc.exe`},
		UninstallName: regexp.MustCompile(`(?i)^MPC-HC`),
	},
	{
		ID:            player.MPCBE,
		Executables:   []string{"mpc-be64.exe", "mpc-be.exe"},
		Registry:      []RegistryValue{{CurrentUser, `SOFTWARE\MPC-BE`, "ExePath", false}},
		ProgramFiles:  []string{`MPC-BE x64\mpc-be64.exe`, `MPC-BE\mpc-be64.exe`, `MPC-BE\mpc-be.exe`},
		UninstallName: regexp.MustCompile(`(?i)^MPC-BE`),
	},
}

// Known returns the hints of the players that have no plugin yet.
func Known() []Hints {
	return slices.Clone(known)
}

// Detect returns the players found on the computer, one installation per
// player, in the order of the hints; a player listed twice is looked for
// once, with its first hints. A path configured by the user wins when the
// file still exists.
func Detect(env Environment, hints []Hints, configured map[player.ID]string) []player.Installation {
	var found []player.Installation
	seen := map[player.ID]bool{}
	for _, h := range hints {
		if h.ID == "" || len(h.Executables) == 0 || seen[h.ID] {
			continue
		}
		seen[h.ID] = true
		if path, ok := locate(env, h, configured[h.ID]); ok {
			found = append(found, player.Installation{ID: h.ID, Path: path})
		}
	}
	return found
}

func locate(env Environment, h Hints, configured string) (string, bool) {
	check := func(path string) (string, bool) {
		path = cleanPath(path)
		if path != "" && filepath.IsAbs(path) && isExecutableOf(h, path) && env.FileExists(path) {
			return path, true
		}
		return "", false
	}

	if path, ok := check(configured); ok {
		return path, true
	}
	for _, value := range h.Registry {
		raw, found := env.RegistryString(value.Root, value.Path, value.Name)
		if !found {
			continue
		}
		if value.IsDirectory {
			for _, executable := range h.Executables {
				if path, ok := check(filepath.Join(cleanPath(raw), executable)); ok {
					return path, true
				}
			}
			continue
		}
		if path, ok := check(raw); ok {
			return path, true
		}
	}
	for _, base := range []string{env.Getenv("ProgramFiles"), env.Getenv("ProgramFiles(x86)"), env.Getenv("ProgramW6432")} {
		for _, relative := range h.ProgramFiles {
			if base == "" {
				continue
			}
			if path, ok := check(filepath.Join(base, relative)); ok {
				return path, true
			}
		}
	}
	if localAppData := env.Getenv("LOCALAPPDATA"); localAppData != "" {
		for _, relative := range h.LocalAppData {
			for _, match := range env.Glob(filepath.Join(localAppData, relative)) {
				if path, ok := check(match); ok {
					return path, true
				}
			}
		}
	}
	if path, ok := fromUninstallEntries(env, h, check); ok {
		return path, true
	}
	for _, executable := range h.Executables {
		if found, ok := env.LookPath(executable); ok {
			if path, ok := check(found); ok {
				return path, true
			}
		}
	}
	return "", false
}

func fromUninstallEntries(env Environment, h Hints, check func(string) (string, bool)) (string, bool) {
	if h.UninstallName == nil {
		return "", false
	}
	for _, root := range []Root{LocalMachine, CurrentUser} {
		for _, key := range []string{uninstallKey, uninstall32} {
			for _, subkey := range env.RegistrySubkeys(root, key) {
				entry := key + `\` + subkey
				name, _ := env.RegistryString(root, entry, "DisplayName")
				if !h.UninstallName.MatchString(name) {
					continue
				}
				if icon, found := env.RegistryString(root, entry, "DisplayIcon"); found {
					if path, ok := check(icon); ok {
						return path, true
					}
				}
				if location, found := env.RegistryString(root, entry, "InstallLocation"); found {
					for _, executable := range h.Executables {
						if path, ok := check(filepath.Join(cleanPath(location), executable)); ok {
							return path, true
						}
					}
				}
			}
		}
	}
	return "", false
}

// iconIndexPattern matches the ",0" icon index that DisplayIcon values carry.
var iconIndexPattern = regexp.MustCompile(`,\s*-?\d+$`)

// cleanPath removes quotes, icon indexes and surrounding spaces from a path
// read from the registry.
func cleanPath(raw string) string {
	path := strings.TrimSpace(raw)
	path = iconIndexPattern.ReplaceAllString(path, "")
	return strings.TrimSpace(strings.Trim(path, `"`))
}

// isExecutableOf rejects paths that point to some other program.
func isExecutableOf(h Hints, path string) bool {
	name := filepath.Base(path)
	for _, executable := range h.Executables {
		if strings.EqualFold(name, executable) {
			return true
		}
	}
	return false
}
