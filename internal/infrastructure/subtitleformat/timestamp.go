package subtitleformat

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrBadTimestamp reports a timestamp that cannot be read.
var ErrBadTimestamp = errors.New("invalid subtitle timestamp")

const (
	// maxTimestampHours rejects absurd values before they overflow a Duration.
	maxTimestampHours = 999
	maxClockDigits    = 3
	maxFractionDigits = 9
	minutesPerHour    = 60
	secondsPerMinute  = 60
)

// ParseTimestamp reads "[h:]mm:ss[.,]fff" as used by SRT, WebVTT and ASS. The
// fraction is decimal, so ",5", ".50" and ".500" all mean half a second.
func ParseTimestamp(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("%w: %q", ErrBadTimestamp, value)
	}

	secondsField := strings.Replace(parts[len(parts)-1], ",", ".", 1)
	wholeSeconds, fraction, _ := strings.Cut(secondsField, ".")

	hours := 0
	var err error
	if len(parts) == 3 {
		if hours, err = parseClockField(parts[0]); err != nil || hours > maxTimestampHours {
			return 0, fmt.Errorf("%w: %q", ErrBadTimestamp, value)
		}
	}
	minutes, minutesErr := parseClockField(parts[len(parts)-2])
	seconds, secondsErr := parseClockField(wholeSeconds)
	nanoseconds, fractionErr := parseFraction(fraction)
	if minutesErr != nil || secondsErr != nil || fractionErr != nil ||
		minutes >= minutesPerHour || seconds >= secondsPerMinute {
		return 0, fmt.Errorf("%w: %q", ErrBadTimestamp, value)
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		nanoseconds, nil
}

func parseClockField(field string) (int, error) {
	if field == "" || len(field) > maxClockDigits {
		return 0, ErrBadTimestamp
	}
	value := 0
	for _, digit := range field {
		if digit < '0' || digit > '9' {
			return 0, ErrBadTimestamp
		}
		value = value*10 + int(digit-'0')
	}
	return value, nil
}

// parseFraction turns the digits after the decimal separator into a duration,
// ignoring precision beyond nanoseconds.
func parseFraction(digits string) (time.Duration, error) {
	if len(digits) > maxFractionDigits {
		digits = digits[:maxFractionDigits]
	}
	nanoseconds := 0
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return 0, ErrBadTimestamp
		}
		nanoseconds = nanoseconds*10 + int(digit-'0')
	}
	for range maxFractionDigits - len(digits) {
		nanoseconds *= 10
	}
	return time.Duration(nanoseconds), nil
}
