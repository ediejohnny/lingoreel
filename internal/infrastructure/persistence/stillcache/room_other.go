//go:build !windows

package stillcache

import "errors"

// freeRoom is unknown until each system tells it: the stills are then
// bounded by their own limits and by the disk filling up.
func freeRoom(string) (int64, error) { return 0, errors.ErrUnsupported }
