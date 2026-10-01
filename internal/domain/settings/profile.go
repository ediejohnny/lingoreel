package settings

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
)

// DefaultProfileName is the profile created on the first run.
const DefaultProfileName = "Inglês"

// MaxProfileNameRunes bounds the length of a profile name.
const MaxProfileNameRunes = 60

// languageTagPattern accepts BCP 47 style tags such as "en", "pt-BR" or "zh-Hant".
var languageTagPattern = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// Allowed ranges for the synchronization steps.
const (
	MinPrecisionSeekStep = 10 * time.Millisecond
	MaxPrecisionSeekStep = 10 * time.Second
)

// LanguagePreferences lists the preferred languages as BCP 47 tags. An empty
// Translation means no translation line.
type LanguagePreferences struct {
	Audio       string `json:"audio"`
	Primary     string `json:"primary"`
	Translation string `json:"translation"`
}

// StudyPreferences configures how the translation is revealed.
type StudyPreferences struct {
	Mode   study.Mode           `json:"mode"`
	Reveal study.RevealBehavior `json:"reveal"`
}

// SyncPreferences configures the fine time adjustments. OffsetStep and
// OffsetLargeStep are the sync shortcuts' steps (a press, and while held).
type SyncPreferences struct {
	PrecisionSeekStep Duration `json:"precisionSeekStep"`
	OffsetStep        Duration `json:"offsetStep"`
	OffsetLargeStep   Duration `json:"offsetLargeStep"`
}

// Nudge returns the sync shortcuts' steps.
func (s SyncPreferences) Nudge() subtitle.NudgeSteps {
	return subtitle.NudgeSteps{Step: time.Duration(s.OffsetStep), FastStep: time.Duration(s.OffsetLargeStep)}
}

// Profile is a named set of study preferences, such as "Inglês" or "Francês".
type Profile struct {
	Name string `json:"name"`
	// PreferredPlayer is a player id; empty means automatic choice.
	PreferredPlayer string                   `json:"preferredPlayer"`
	Languages       LanguagePreferences      `json:"languages"`
	Primary         appearance.SubtitleStyle `json:"primaryStyle"`
	Translation     appearance.SubtitleStyle `json:"translationStyle"`
	Layout          appearance.Layout        `json:"layout"`
	Study           StudyPreferences         `json:"study"`
	Speed           playback.Speed           `json:"speed"`
	Sync            SyncPreferences          `json:"sync"`
}

// DefaultProfile returns the English study profile described in the PRD.
func DefaultProfile() Profile {
	return Profile{
		Name:        DefaultProfileName,
		Languages:   LanguagePreferences{Audio: "en", Primary: "en", Translation: "pt-BR"},
		Primary:     appearance.DefaultPrimaryStyle(),
		Translation: appearance.DefaultTranslationStyle(),
		Layout:      appearance.DefaultLayout(),
		Study:       StudyPreferences{Mode: study.ModeOnDemand, Reveal: study.RevealToggle},
		Speed:       playback.NormalSpeed,
		Sync: SyncPreferences{
			PrecisionSeekStep: Duration(100 * time.Millisecond),
			OffsetStep:        Duration(subtitle.DefaultNudgeStep),
			OffsetLargeStep:   Duration(subtitle.DefaultFastNudgeStep),
		},
	}
}

// Validate checks every preference of the profile.
func (p Profile) Validate() error {
	if err := validateProfileName(p.Name); err != nil {
		return err
	}
	if p.PreferredPlayer != "" && !player.ID(p.PreferredPlayer).Valid() {
		return fmt.Errorf("%w: preferredPlayer %q", ErrInvalidSettings, p.PreferredPlayer)
	}
	if !languageTagPattern.MatchString(p.Languages.Audio) || !languageTagPattern.MatchString(p.Languages.Primary) {
		return fmt.Errorf("%w: audio and primary languages must be language tags", ErrInvalidSettings)
	}
	if p.Languages.Translation != "" && !languageTagPattern.MatchString(p.Languages.Translation) {
		return fmt.Errorf("%w: translation language %q", ErrInvalidSettings, p.Languages.Translation)
	}
	if err := p.Primary.Validate(); err != nil {
		return fmt.Errorf("primaryStyle: %w", err)
	}
	if err := p.Translation.Validate(); err != nil {
		return fmt.Errorf("translationStyle: %w", err)
	}
	if err := p.Layout.Validate(); err != nil {
		return err
	}
	if !p.Study.Mode.Valid() || !p.Study.Reveal.Valid() {
		return fmt.Errorf("%w: study preferences", ErrInvalidSettings)
	}
	if !p.Speed.Valid() {
		return fmt.Errorf("%w: speed %v", ErrInvalidSettings, p.Speed)
	}
	return p.Sync.validate()
}

func (s SyncPreferences) validate() error {
	switch {
	case !s.PrecisionSeekStep.within(MinPrecisionSeekStep, MaxPrecisionSeekStep):
		return fmt.Errorf("%w: precisionSeekStep", ErrInvalidSettings)
	case s.Nudge().Validate() != nil:
		return fmt.Errorf("%w: offsetStep and offsetLargeStep: %w", ErrInvalidSettings, s.Nudge().Validate())
	}
	return nil
}

func validateProfileName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed != name || utf8.RuneCountInString(name) > MaxProfileNameRunes {
		return fmt.Errorf("%w: profile name %q", ErrInvalidSettings, name)
	}
	return nil
}
