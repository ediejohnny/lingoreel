// Package overlay draws the subtitles in a transparent window over the
// player's video window: click-through, never focused, owned by the player
// window so it stays right above it, and following it as it moves, resizes,
// minimizes, goes fullscreen or closes.
package overlay

// Candidate is a top-level window of the player process.
type Candidate struct {
	Handle        uintptr
	Visible       bool
	Width, Height int
}

// ChooseTarget picks the player's video window: the visible window with the
// largest client area. Hidden helper windows (hotkeys, IME) and empty ones
// are never chosen; on a tie the first one wins.
func ChooseTarget(candidates []Candidate) (Candidate, bool) {
	var best Candidate
	found := false
	for _, c := range candidates {
		if !c.Visible || c.Width <= 0 || c.Height <= 0 {
			continue
		}
		if !found || c.Width*c.Height > best.Width*best.Height {
			best, found = c, true
		}
	}
	return best, found
}
