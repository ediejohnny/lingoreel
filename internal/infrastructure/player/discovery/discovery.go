// Package discovery finds the video players installed on this computer.
package discovery

import (
	"path/filepath"
	"regexp"
	"strings"

	"lingoreel/internal/domain/player"
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
	appPathsKey  = `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`
	uninstallKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
	uninstall32  = `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`
)

// registryValue is a registry value that holds a path to a player.
type registryValue struct {
	root Root
	path string
	name string
	// isDirectory marks values holding the install folder instead of the exe.
	isDirectory bool
}

// candidate describes where a player usually lives.
type candidate struct {
	id player.ID
	// executables are the file names the player may have, in preference order.
	executables []string
	registry    []registryValue
	// programFiles are paths relative to the Program Files folders.
	programFiles []string
	// localAppData are paths relative to %LOCALAPPDATA%, which may use globs.
	localAppData []string
	// uninstallName matches the DisplayName of the player's uninstall entry.
	uninstallName *regexp.Regexp
}

var candidates = []candidate{
	{
		id:            player.MPV,
		executables:   []string{"mpv.exe"},
		registry:      []registryValue{{LocalMachine, appPathsKey + "mpv.exe", "", false}, {CurrentUser, appPathsKey + "mpv.exe", "", false}},
		programFiles:  []string{`mpv\mpv.exe`},
		localAppData:  []string{`Microsoft\WinGet\Links\mpv.exe`, `Microsoft\WinGet\Packages\mpv-player.mpv*\mpv.exe`, `Programs\mpv\mpv.exe`},
		uninstallName: regexp.MustCompile(`(?i)^mpv\b`),
	},
	{
		id:          player.VLC,
		executables: []string{"vlc.exe"},
		registry: []registryValue{
			{LocalMachine, `SOFTWARE\VideoLAN\VLC`, "", false},
			{LocalMachine, `SOFTWARE\VideoLAN\VLC`, "InstallDir", true},
			{LocalMachine, `SOFTWARE\WOW6432Node\VideoLAN\VLC`, "InstallDir", true},
			{LocalMachine, appPathsKey + "vlc.exe", "", false},
		},
		programFiles:  []string{`VideoLAN\VLC\vlc.exe`},
		uninstallName: regexp.MustCompile(`(?i)^VLC media player`),
	},
	{
		id:          player.PotPlayer,
		executables: []string{"PotPlayerMini64.exe", "PotPlayerMini.exe"},
		registry: []registryValue{
			{CurrentUser, `SOFTWARE\DAUM\PotPlayer64`, "ProgramPath", false},
			{CurrentUser, `SOFTWARE\DAUM\PotPlayer`, "ProgramPath", false},
			{LocalMachine, appPathsKey + "PotPlayerMini64.exe", "", false},
		},
		programFiles:  []string{`DAUM\PotPlayer\PotPlayerMini64.exe`, `PotPlayer\PotPlayerMini64.exe`, `DAUM\PotPlayer\PotPlayerMini.exe`},
		uninstallName: regexp.MustCompile(`(?i)^PotPlayer`),
	},
	{
		id:            player.MPCHC,
		executables:   []string{"mpc-hc64.exe", "mpc-hc.exe"},
		registry:      []registryValue{{CurrentUser, `SOFTWARE\MPC-HC\MPC-HC`, "ExePath", false}},
		programFiles:  []string{`MPC-HC\mpc-hc64.exe`, `K-Lite Codec Pack\MPC-HC64\mpc-hc64.exe`, `MPC-HC\mpc-hc.exe`},
		uninstallName: regexp.MustCompile(`(?i)^MPC-HC`),
	},
	{
		id:            player.MPCBE,
		executables:   []string{"mpc-be64.exe", "mpc-be.exe"},
		registry:      []registryValue{{CurrentUser, `SOFTWARE\MPC-BE`, "ExePath", false}},
		programFiles:  []string{`MPC-BE x64\mpc-be64.exe`, `MPC-BE\mpc-be64.exe`, `MPC-BE\mpc-be.exe`},
		uninstallName: regexp.MustCompile(`(?i)^MPC-BE`),
	},
}

// Detect returns the players found on the computer, one installation per
// player. A path configured by the user wins when the file still exists.
func Detect(env Environment, configured map[player.ID]string) []player.Installation {
	var found []player.Installation
	for _, c := range candidates {
		if path, ok := locate(env, c, configured[c.id]); ok {
			found = append(found, player.Installation{ID: c.id, Path: path})
		}
	}
	return found
}

func locate(env Environment, c candidate, configured string) (string, bool) {
	check := func(path string) (string, bool) {
		path = cleanPath(path)
		if path != "" && filepath.IsAbs(path) && isExecutableOf(c, path) && env.FileExists(path) {
			return path, true
		}
		return "", false
	}

	if path, ok := check(configured); ok {
		return path, true
	}
	for _, value := range c.registry {
		raw, found := env.RegistryString(value.root, value.path, value.name)
		if !found {
			continue
		}
		if value.isDirectory {
			for _, executable := range c.executables {
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
		for _, relative := range c.programFiles {
			if base == "" {
				continue
			}
			if path, ok := check(filepath.Join(base, relative)); ok {
				return path, true
			}
		}
	}
	if localAppData := env.Getenv("LOCALAPPDATA"); localAppData != "" {
		for _, relative := range c.localAppData {
			for _, match := range env.Glob(filepath.Join(localAppData, relative)) {
				if path, ok := check(match); ok {
					return path, true
				}
			}
		}
	}
	if path, ok := fromUninstallEntries(env, c, check); ok {
		return path, true
	}
	for _, executable := range c.executables {
		if found, ok := env.LookPath(executable); ok {
			if path, ok := check(found); ok {
				return path, true
			}
		}
	}
	return "", false
}

func fromUninstallEntries(env Environment, c candidate, check func(string) (string, bool)) (string, bool) {
	for _, root := range []Root{LocalMachine, CurrentUser} {
		for _, key := range []string{uninstallKey, uninstall32} {
			for _, subkey := range env.RegistrySubkeys(root, key) {
				entry := key + `\` + subkey
				name, _ := env.RegistryString(root, entry, "DisplayName")
				if !c.uninstallName.MatchString(name) {
					continue
				}
				if icon, found := env.RegistryString(root, entry, "DisplayIcon"); found {
					if path, ok := check(icon); ok {
						return path, true
					}
				}
				if location, found := env.RegistryString(root, entry, "InstallLocation"); found {
					for _, executable := range c.executables {
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
func isExecutableOf(c candidate, path string) bool {
	name := filepath.Base(path)
	for _, executable := range c.executables {
		if strings.EqualFold(name, executable) {
			return true
		}
	}
	return false
}
