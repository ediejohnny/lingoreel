package settings

import (
	"fmt"
	"time"
)

// Duration stores a time.Duration as readable text ("250ms", "1s") so the
// configuration files stay editable by hand.
type Duration time.Duration

// Std returns the value as a time.Duration.
func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

// MarshalText implements encoding.TextMarshaler.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("%w: duration %q", ErrInvalidSettings, text)
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) within(low, high time.Duration) bool {
	return time.Duration(d) >= low && time.Duration(d) <= high
}
