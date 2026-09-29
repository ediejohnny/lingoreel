package subtitleformat_test

import (
	"errors"
	"testing"
	"time"

	"lingoreel/internal/infrastructure/subtitleformat"
)

func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		value string
		want  time.Duration
	}{
		{"00:10:41,120", 10*time.Minute + 41*time.Second + 120*time.Millisecond},
		{"00:10:41.120", 10*time.Minute + 41*time.Second + 120*time.Millisecond},
		{"1:02:03.5", time.Hour + 2*time.Minute + 3*time.Second + 500*time.Millisecond},
		{"0:00:01.25", time.Second + 250*time.Millisecond},
		{"01:05.001", time.Minute + 5*time.Second + time.Millisecond},
		{"00:00:00,000", 0},
		{"00:59:59,999", 59*time.Minute + 59*time.Second + 999*time.Millisecond},
		{"123:00:00,000", 123 * time.Hour},
		{" 00:00:02,000 ", 2 * time.Second},
		{"00:00:02", 2 * time.Second},
		{"00:00:02.", 2 * time.Second},
		{"00:00:00,123456789123", 123456789 * time.Nanosecond},
	}
	for _, tc := range cases {
		got, err := subtitleformat.ParseTimestamp(tc.value)
		if err != nil || got != tc.want {
			t.Fatalf("ParseTimestamp(%q) = %v, %v; want %v", tc.value, got, err, tc.want)
		}
	}
}

func TestParseTimestampRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "10", "00:60:00,000", "00:00:60,000", "aa:bb:cc", "00:00:01,5x", "-1:00:00", "1:2:3:4", "1000:00:00,000", "00::01", ":00:01", "00:00:01,2,3"} {
		if _, err := subtitleformat.ParseTimestamp(value); !errors.Is(err, subtitleformat.ErrBadTimestamp) {
			t.Fatalf("ParseTimestamp(%q) error = %v, want ErrBadTimestamp", value, err)
		}
	}
}
