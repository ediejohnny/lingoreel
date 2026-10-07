package ptbr

import "github.com/ediejohnny/lingoreel/internal/domain/fault"

// FaultReason words why the app could not keep something on the disk and
// what to do about it; empty when the system told nothing that helps.
func FaultReason(kind fault.Kind) string {
	switch kind {
	case fault.DiskFull:
		return T("error.disk.full")
	case fault.Denied:
		return T("error.disk.denied")
	case fault.InUse:
		return T("error.disk.inUse")
	case fault.ReadOnly:
		return T("error.disk.readOnly")
	case fault.Unavailable:
		return T("error.disk.unavailable")
	case fault.TooLong:
		return T("error.disk.tooLong")
	}
	return ""
}
