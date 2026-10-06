//go:build windows

package audiotap_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/audiotap"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// Hearing this silent test process: Windows hears it alone and hands over
// 16 kHz stereo at the pace of the clock, each packet stamped a moment ago.
func TestTheTapHearsAProcessAlone(t *testing.T) {
	testkit.Slow(t)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	samples, packets, system := 0, 0, false
	var oldest time.Duration
	err := audiotap.Tap{}.Listen(ctx, os.Getpid(), func(packet voice.Packet) {
		packets++
		samples += len(packet.Samples) / packet.Channels
		system = system || packet.System
		oldest = max(oldest, time.Since(packet.At))
	})
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case system:
		t.Fatal("heard the speakers, not the process alone")
	case samples < 6000 || samples > 9000: // about half a second at 16 kHz
		t.Fatalf("%d samples in %d packets in half a second, want about 8000", samples, packets)
	case oldest > 200*time.Millisecond:
		t.Fatalf("a packet came %v after it was taken", oldest)
	}
}
