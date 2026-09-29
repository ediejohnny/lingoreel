//go:build windows

package discovery

import (
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// SystemEnvironment reads the real registry, file system and environment.
type SystemEnvironment struct{}

func hive(root Root) registry.Key {
	if root == CurrentUser {
		return registry.CURRENT_USER
	}
	return registry.LOCAL_MACHINE
}

// RegistryString reads a string value; name "" reads the key's default value.
func (SystemEnvironment) RegistryString(root Root, path, name string) (string, bool) {
	key, err := registry.OpenKey(hive(root), path, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if err != nil {
		return "", false
	}
	expanded, err := registry.ExpandString(value)
	if err != nil {
		return value, true
	}
	return expanded, true
}

// RegistrySubkeys lists the subkeys of a key.
func (SystemEnvironment) RegistrySubkeys(root Root, path string) []string {
	key, err := registry.OpenKey(hive(root), path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer key.Close()
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}
	return names
}

// FileExists reports whether path is an existing regular file.
func (SystemEnvironment) FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// Getenv reads an environment variable.
func (SystemEnvironment) Getenv(name string) string {
	return os.Getenv(name)
}

// LookPath searches the PATH. A match in the current directory is refused:
// a file dropped next to a video must never be run as the player.
func (SystemEnvironment) LookPath(file string) (string, bool) {
	path, err := exec.LookPath(file)
	if err != nil || !filepath.IsAbs(path) {
		return "", false
	}
	return path, true
}

// Glob expands a file pattern.
func (SystemEnvironment) Glob(pattern string) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	return matches
}
