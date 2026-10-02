package settings

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Profiles is the collection of study profiles.
type Profiles struct {
	Items []Profile `json:"profiles"`
}

// DefaultProfiles returns the profiles created on the first run.
func DefaultProfiles() Profiles {
	return Profiles{Items: []Profile{DefaultProfile()}}
}

// Find returns the profile with the given name, ignoring case.
func (p Profiles) Find(name string) (Profile, bool) {
	for _, profile := range p.Items {
		if strings.EqualFold(profile.Name, name) {
			return profile, true
		}
	}
	return Profile{}, false
}

// Errors of the profile changes.
var (
	ErrProfileExists   = errors.New("a profile with this name exists")
	ErrProfileNotFound = errors.New("profile not found")
	ErrLastProfile     = errors.New("the last profile cannot be removed")
)

// Add returns the profiles with a copy of base named name, at the end.
func (p Profiles) Add(name string, base Profile) (Profiles, error) {
	if err := validateProfileName(name); err != nil {
		return p, err
	}
	if _, taken := p.Find(name); taken {
		return p, fmt.Errorf("%w: %q", ErrProfileExists, name)
	}
	base.Name = name
	base.Shortcuts = maps.Clone(base.Shortcuts)
	items := append(slices.Clone(p.Items), base)
	return Profiles{Items: items}, nil
}

// Rename returns the profiles with a profile renamed; a change of case only
// is allowed.
func (p Profiles) Rename(from, to string) (Profiles, error) {
	if err := validateProfileName(to); err != nil {
		return p, err
	}
	index := p.index(from)
	if index < 0 {
		return p, fmt.Errorf("%w: %q", ErrProfileNotFound, from)
	}
	if other := p.index(to); other >= 0 && other != index {
		return p, fmt.Errorf("%w: %q", ErrProfileExists, to)
	}
	items := slices.Clone(p.Items)
	items[index].Name = to
	return Profiles{Items: items}, nil
}

// Remove returns the profiles without one; the last one stays.
func (p Profiles) Remove(name string) (Profiles, error) {
	index := p.index(name)
	if index < 0 {
		return p, fmt.Errorf("%w: %q", ErrProfileNotFound, name)
	}
	if len(p.Items) == 1 {
		return p, ErrLastProfile
	}
	return Profiles{Items: slices.Delete(slices.Clone(p.Items), index, index+1)}, nil
}

// Replace returns the profiles with the profile of the same name, ignoring
// case, swapped for profile.
func (p Profiles) Replace(profile Profile) (Profiles, error) {
	index := p.index(profile.Name)
	if index < 0 {
		return p, fmt.Errorf("%w: %q", ErrProfileNotFound, profile.Name)
	}
	items := slices.Clone(p.Items)
	items[index] = profile
	return Profiles{Items: items}, nil
}

func (p Profiles) index(name string) int {
	return slices.IndexFunc(p.Items, func(profile Profile) bool { return strings.EqualFold(profile.Name, name) })
}

// Validate requires at least one profile, valid profiles and unique names.
func (p Profiles) Validate() error {
	if len(p.Items) == 0 {
		return fmt.Errorf("%w: at least one profile is required", ErrInvalidSettings)
	}
	seen := make(map[string]bool, len(p.Items))
	for _, profile := range p.Items {
		if err := profile.Validate(); err != nil {
			return fmt.Errorf("profile %q: %w", profile.Name, err)
		}
		key := strings.ToLower(profile.Name)
		if seen[key] {
			return fmt.Errorf("%w: duplicated profile name %q", ErrInvalidSettings, profile.Name)
		}
		seen[key] = true
	}
	return nil
}
