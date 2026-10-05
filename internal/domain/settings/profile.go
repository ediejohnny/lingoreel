package settings

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
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

// StudyPreferences configures how the translation is revealed and the
// Modo Karaokê.
type StudyPreferences struct {
	Mode    study.Mode           `json:"mode"`
	Reveal  study.RevealBehavior `json:"reveal"`
	Karaoke KaraokePreferences   `json:"karaoke"`
}

// KaraokePreferences configure the Modo Karaokê: the next line shows before
// its time after a long silence, and fills in as it is spoken.
type KaraokePreferences struct {
	Enabled bool `json:"enabled"`
	// MinGap is the shortest silence that brings a preview; Lead is how
	// long before the line it shows.
	MinGap       Duration         `json:"minGap"`
	Lead         Duration         `json:"lead"`
	Indicator    study.Indicator  `json:"indicator"`
	PreviewColor appearance.Color `json:"previewColor"`
	// Fill paints the line as it is spoken, in FillColor.
	Fill      bool             `json:"fill"`
	FillColor appearance.Color `json:"fillColor"`
	// Repeat plays each line again that many times; 0 is off.
	Repeat int `json:"repeat"`
}

// DefaultKaraoke is off, with a 10 s gap, a 5 s lead, the bar, a light grey
// preview and the karaoke yellow fill.
func DefaultKaraoke() KaraokePreferences {
	rules := study.DefaultCueRules()
	return KaraokePreferences{
		MinGap: Duration(rules.MinGap), Lead: Duration(rules.Lead), Indicator: study.IndicatorBar,
		PreviewColor: appearance.Color{R: 0xC8, G: 0xCD, B: 0xD4, A: 0xFF},
		Fill:         true, FillColor: appearance.Color{R: 0xFF, G: 0xD2, B: 0x3F, A: 0xFF},
	}
}

// Rules are the preview's timing rules.
func (k KaraokePreferences) Rules() study.CueRules {
	return study.CueRules{MinGap: time.Duration(k.MinGap), Lead: time.Duration(k.Lead)}
}

func (k KaraokePreferences) validate() error {
	if err := k.Rules().Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSettings, err)
	}
	if !k.Indicator.Valid() {
		return fmt.Errorf("%w: karaoke indicator %q", ErrInvalidSettings, k.Indicator)
	}
	if k.Repeat < 0 || k.Repeat > study.MaxRepeats {
		return fmt.Errorf("%w: karaoke repeat %d outside 0..%d", ErrInvalidSettings, k.Repeat, study.MaxRepeats)
	}
	return nil
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
	// Shortcuts are the profile's own chords; commands left out keep the
	// default one.
	Shortcuts shortcut.Map `json:"shortcuts"`
}

// DefaultProfile returns the English study profile the app starts with.
func DefaultProfile() Profile {
	return Profile{
		Name:        DefaultProfileName,
		Languages:   LanguagePreferences{Audio: "en", Primary: "en", Translation: "pt-BR"},
		Primary:     appearance.DefaultPrimaryStyle(),
		Translation: appearance.DefaultTranslationStyle(),
		Layout:      appearance.DefaultLayout(),
		Study:       StudyPreferences{Mode: study.ModeOnDemand, Reveal: study.RevealToggle, Karaoke: DefaultKaraoke()},
		Speed:       playback.NormalSpeed,
		Sync: SyncPreferences{
			PrecisionSeekStep: Duration(100 * time.Millisecond),
			OffsetStep:        Duration(subtitle.DefaultNudgeStep),
			OffsetLargeStep:   Duration(subtitle.DefaultFastNudgeStep),
		},
		Shortcuts: shortcut.Defaults(),
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
	if err := p.Study.Karaoke.validate(); err != nil {
		return err
	}
	if !p.Speed.Valid() {
		return fmt.Errorf("%w: speed %v", ErrInvalidSettings, p.Speed)
	}
	if err := p.Shortcuts.Validate(); err != nil {
		return fmt.Errorf("%w: shortcuts: %w", ErrInvalidSettings, err)
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
