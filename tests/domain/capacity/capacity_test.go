package capacity_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
)

// A computer is weak when any one measure falls under its threshold; a
// measure that could not be taken (zero) counts for nothing.
func TestAWeakComputerIsTold(t *testing.T) {
	t.Parallel()
	const gib = 1 << 30
	strong := capacity.Machine{Processors: 8, Memory: 24 * gib, ModelSpeed: 90}
	for _, c := range []struct {
		name    string
		machine capacity.Machine
		weak    bool
	}{
		{"strong", strong, false},
		{"nothing measured", capacity.Machine{}, false},
		{"two processors", capacity.Machine{Processors: 2, Memory: 24 * gib, ModelSpeed: 90}, true},
		{"three processors", capacity.Machine{Processors: 3, Memory: 24 * gib, ModelSpeed: 90}, false},
		{"just under 4 GiB", capacity.Machine{Processors: 8, Memory: 4*gib - 1, ModelSpeed: 90}, true},
		{"4 GiB", capacity.Machine{Processors: 8, Memory: 4 * gib, ModelSpeed: 90}, false},
		{"a slow model", capacity.Machine{Processors: 8, Memory: 24 * gib, ModelSpeed: 24.9}, true},
		{"a model just fast enough", capacity.Machine{Processors: 8, Memory: 24 * gib, ModelSpeed: 25}, false},
		{"only the speed measured, slow", capacity.Machine{ModelSpeed: 10}, true},
	} {
		if got := c.machine.Weak(); got != c.weak {
			t.Errorf("%s: Weak() = %v, want %v", c.name, got, c.weak)
		}
	}
}
