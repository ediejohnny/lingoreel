package ptbr

import (
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

// VolumeNotice words the volume for the notice on the video after a volume
// key: the level, and the player's ceiling when reached.
func VolumeNotice(volume, ceiling playback.Volume, muted bool) string {
	if muted {
		return T("notice.muted")
	}
	percent := volume.Percent()
	if percent >= ceiling.Percent() {
		return T("notice.volumeCeiling", "percent", percent)
	}
	return T("notice.volume", "percent", percent)
}
