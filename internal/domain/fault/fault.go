// Package fault names why the app could not keep something on the disk, so
// the screens can say it in words, with what to do about it.
package fault

// Kind is why a file or a folder could not be written, renamed or removed.
type Kind int

const (
	// None is no failure at all.
	None Kind = iota
	// Other is a failure none of the kinds below tells.
	Other
	// DiskFull is a disk with no room left.
	DiskFull
	// Denied is Windows refusing the app: the folder's rights, a file
	// marked read-only, or an antivirus.
	Denied
	// InUse is another program holding the file.
	InUse
	// ReadOnly is a disk that takes no writing at all, as a locked card.
	ReadOnly
	// Unavailable is a disk, a folder or a network share that is not there
	// any more: removed, disconnected or renamed.
	Unavailable
	// TooLong is a path longer than Windows takes.
	TooLong
)
