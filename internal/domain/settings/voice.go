package settings

import (
	"fmt"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// MaxVoiceDelay bounds the output's delay a user can set: wireless
// headphones take about 0.2 s, a receiver over HDMI a little more.
const MaxVoiceDelay = time.Second

// VoicePreferences are how the Modo Karaokê follows the actors' voice on
// this computer: whether at all, how readily quiet sound counts as voice,
// the FFmpeg the user chose (empty: looked for), the output's delay, and
// whether to go on when the computer cannot keep up.
type VoicePreferences struct {
	Follow       bool               `json:"follow"`
	Sensitivity  speech.Sensitivity `json:"sensitivity"`
	FFmpegPath   string             `json:"ffmpegPath"`
	Delay        Duration           `json:"delay"`
	EvenWhenBusy bool               `json:"evenWhenBusy"`
}

// DefaultVoice follows the voice at the normal sensitivity, with FFmpeg
// looked for, no delay, and the estimate on a computer that cannot keep up.
func DefaultVoice() VoicePreferences {
	return VoicePreferences{Follow: true, Sensitivity: speech.SensitivityNormal}
}

func (v VoicePreferences) validate() error {
	if !v.Sensitivity.Valid() {
		return fmt.Errorf("%w: voice sensitivity %q", ErrInvalidSettings, v.Sensitivity)
	}
	if !v.Delay.within(0, MaxVoiceDelay) {
		return fmt.Errorf("%w: voice delay %v", ErrInvalidSettings, time.Duration(v.Delay))
	}
	// Whether it is an absolute .exe path is an operating system rule,
	// checked where the settings are stored (settingsfile).
	if v.FFmpegPath != "" && strings.TrimSpace(v.FFmpegPath) == "" {
		return fmt.Errorf("%w: blank FFmpeg path", ErrInvalidSettings)
	}
	return nil
}
