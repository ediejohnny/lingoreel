package settings

import (
	"fmt"
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
