package ptbr

import (
	"fmt"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

// VolumeNotice words the volume for the notice on the video after a volume
// key: the level, the player's ceiling when reached, and at 100% on a player
// that goes further, how to pass it (a held key stops there).
func VolumeNotice(volume, ceiling playback.Volume, muted bool) string {
	if muted {
		return "Sem som"
	}
	text := fmt.Sprintf("Volume %d%%", volume.Percent())
	switch {
	case volume.Percent() >= ceiling.Percent():
		return text + " · máximo deste player"
	case volume.Percent() == playback.FullVolume.Percent():
		return text + " · toque de novo para passar de 100%"
	}
	return text
}
