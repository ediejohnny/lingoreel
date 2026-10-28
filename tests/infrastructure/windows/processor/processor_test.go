package processor_test

import (
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/processor"
)

// Windows names every processor; the name comes without the padding some
// processors carry. (The name itself is not printed: it is the computer's.)
func TestTheProcessorIsNamed(t *testing.T) {
	t.Parallel()
	name := processor.Name()
	if name == "" || name != strings.TrimSpace(name) {
		t.Fatalf("Name() is empty or padded (%d characters)", len(name))
	}
}
