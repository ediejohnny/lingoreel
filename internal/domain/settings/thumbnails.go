package settings

// ThumbnailPreferences are the seek bar's stills on this computer: whether
// they are taken at all, in the background on the battery too (else only
// the moment under the pointer), and kept on the disk between openings
// (else taken again at each one, in memory only).
type ThumbnailPreferences struct {
	On        bool `json:"on"`
	OnBattery bool `json:"onBattery"`
	OnDisk    bool `json:"onDisk"`
}

// DefaultThumbnails takes the stills and keeps them on the disk, but on the
// battery only the moment under the pointer.
func DefaultThumbnails() ThumbnailPreferences {
	return ThumbnailPreferences{On: true, OnDisk: true}
}
