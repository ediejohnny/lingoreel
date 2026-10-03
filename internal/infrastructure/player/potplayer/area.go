package potplayer

import "strings"

// viewClassPrefix starts the class name of the views PotPlayer draws in.
// Its picture and its playlist are both such windows; the plain container
// around them ("AfxWnd…") is not, so the prefix keeps it out.
const viewClassPrefix = "Afx:"

// AreaCandidate is a child window of PotPlayer's main window.
type AreaCandidate struct {
	Handle        uintptr
	Class         string
	Visible       bool
	Width, Height int
}

// ChooseArea picks the window holding the picture: the largest visible view.
// The playlist is a view too, always the smaller one next to the picture; on
// a tie the first one wins. It reports false while PotPlayer has no view
// yet, and the subtitles then cover its whole window.
func ChooseArea(candidates []AreaCandidate) (uintptr, bool) {
	var best AreaCandidate
	found := false
	for _, c := range candidates {
		if !c.Visible || c.Width <= 0 || c.Height <= 0 || !strings.HasPrefix(c.Class, viewClassPrefix) {
			continue
		}
		if !found || c.Width*c.Height > best.Width*best.Height {
			best, found = c, true
		}
	}
	return best.Handle, found
}
