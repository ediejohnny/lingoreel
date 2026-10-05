package ptbr_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

func TestVolumeNotice(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		volume, ceiling float64
		muted           bool
		want            string
	}{
		{0.35, 2, false, "Volume 35%"},
		{1, 2, false, "Volume 100%"},
		{1.35, 2, false, "Volume 135%"},
		{2, 2, false, "Volume 200% · máximo deste player"},
		{1, 1, false, "Volume 100% · máximo deste player"},
		{0.8, 2, true, "Sem som"},
	} {
		if got := ptbr.VolumeNotice(volume(c.volume), volume(c.ceiling), c.muted); got != c.want {
			t.Errorf("VolumeNotice(%v, %v, %v) = %q, want %q", c.volume, c.ceiling, c.muted, got, c.want)
		}
	}
}
func volume(v float64) playback.Volume { return playback.Volume(v) }
