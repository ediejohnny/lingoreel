package voice_test

import (
	"errors"
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/silero"
)

// brokenModel cannot start a stream.
type brokenModel struct{}

func (brokenModel) NewStream() (speech.Stream, error) { return nil, errors.New("no weights") }

// The embedded model's speed is measured on made-up sound, as a number the
// weak computer's verdict can use; a model that cannot run says so instead
// of passing for a fast one.
func TestTheVoiceModelsSpeedIsMeasured(t *testing.T) {
	speed, err := voice.ModelSpeed(silero.Embedded{})
	if err != nil || !(speed > 0) || math.IsInf(speed, 0) {
		t.Fatalf("ModelSpeed(embedded) = %v, %v", speed, err)
	}
	t.Logf("the embedded model judges %.0f s of sound per second here", speed)
	if speed, err := voice.ModelSpeed(brokenModel{}); err == nil || speed != 0 {
		t.Fatalf("ModelSpeed(broken) = %v, %v; want an error", speed, err)
	}
}
