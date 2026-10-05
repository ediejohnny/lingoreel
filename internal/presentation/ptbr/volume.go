package ptbr

import (
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

// VolumeNotice words the volume for the notice on the video after a volume
// key: the level, the player's ceiling when reached, and at 100% on a player
// that goes further, how to pass it (a held key stops there).
func VolumeNotice(volume, ceiling playback.Volume, muted bool) string {
	if muted {
		return T("notice.muted")
	}
	percent := volume.Percent()
	switch {
	case percent >= ceiling.Percent():
		return T("notice.volumeCeiling", "percent", percent)
	case percent == playback.FullVolume.Percent():
		return T("notice.volumeFull", "percent", percent)
	}
	return T("notice.volume", "percent", percent)
}
