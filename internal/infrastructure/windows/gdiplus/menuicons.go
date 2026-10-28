//go:build windows && amd64

package gdiplus

import (
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/vector"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// The context menu's icons are of the same family as the app's screens
// (frontend/src/components/ui/icons.tsx): SVG path data on a 24-unit grid,
// every drawing inside 2..22, stroked two units wide with round ends and
// joins, in one color and no fill.
const (
	menuGrid   = 24
	menuStroke = 2
)

var menuDrawings = map[contextmenu.Icon]string{
	contextmenu.IconPlay:           "M7 4.5v15L19 12Z",
	contextmenu.IconPause:          "M8 5v14M16 5v14",
	contextmenu.IconStop:           "M7 5.5h10a1.5 1.5 0 0 1 1.5 1.5v10a1.5 1.5 0 0 1-1.5 1.5H7A1.5 1.5 0 0 1 5.5 17V7A1.5 1.5 0 0 1 7 5.5Z",
	contextmenu.IconBack:           "M11 6.5 4.5 12l6.5 5.5ZM19.5 6.5 13 12l6.5 5.5Z",
	contextmenu.IconForward:        "m13 6.5 6.5 5.5-6.5 5.5ZM4.5 6.5 11 12l-6.5 5.5Z",
	contextmenu.IconFrameBack:      "M6 6v12M18 6.5 9.5 12l8.5 5.5Z",
	contextmenu.IconFrameForward:   "M18 6v12M6 6.5l8.5 5.5L6 17.5Z",
	contextmenu.IconFullscreen:     "M4 9V4h5M15 4h5v5M20 15v5h-5M9 20H4v-5",
	contextmenu.IconExitFullscreen: "M9 4v5H4M20 9h-5V4M15 20v-5h5M4 15h5v5",
	contextmenu.IconSpeaker:        "M3.5 9.5h4L12 5.5v13l-4.5-4h-4ZM15.5 9a4.5 4.5 0 0 1 0 6M18.5 6a8.5 8.5 0 0 1 0 12",
	contextmenu.IconSubtitles:      "M5 5.5h14a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-9a2 2 0 0 1 2-2ZM10.5 10.3a2 2 0 1 0 0 3.4M16.5 10.3a2 2 0 1 0 0 3.4",
	contextmenu.IconStudy:          "M12 6.5C10 5 7 4.5 3 4.5v14c4 0 7 .5 9 2 2-1.5 5-2 9-2v-14c-4 0-7 .5-9 2ZM12 6.5v14M14.5 12l1.8 1.8 3.2-3.8",
	contextmenu.IconKaraoke:        "M9.5 17.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 1 1 5 0ZM19.5 15.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 1 1 5 0ZM9.5 17.5v-12l10-2v12",
	contextmenu.IconPassage:        "M5.5 21V4M5.5 4.5h12L15 9l2.5 4.5h-12",
	contextmenu.IconFolder:         "M3.5 18.5V5.5A1.5 1.5 0 0 1 5 4h4l2 2.5h6.5A1.5 1.5 0 0 1 19 8v2.5M3.5 18.5l3-8h15l-3 8Z",
	contextmenu.IconSettings:       "M19 12a7 7 0 1 1-14 0 7 7 0 1 1 14 0ZM14.5 12a2.5 2.5 0 1 1-5 0 2.5 2.5 0 1 1 5 0ZM12 2.5V5M12 19v2.5M2.5 12H5M19 12h2.5M5.3 5.3 7 7M17 17l1.7 1.7M5.3 18.7 7 17M17 7l1.7-1.7",
	contextmenu.IconClose:          "M6.5 6.5l11 11M17.5 6.5l-11 11",
}

// DrawIcon paints one of the context menu's icons over a whole bitmap,
// cleared first, in one color: its grid fills the bitmap's square, at the
// top left. An unknown icon draws nothing.
func (r *Renderer) DrawIcon(target *win32.DIB, icon contextmenu.Icon, color appearance.Color) error {
	if target == nil {
		return nil
	}
	target.Clear()
	data, known := menuDrawings[icon]
	if !known {
		return nil
	}
	figures, err := vector.Parse(data)
	if err != nil {
		return err
	}
	c, err := r.canvasOn(target)
	if err != nil {
		return err
	}
	defer c.close()
	// Pixel x covers x..x+1 on the grid, as in the screens' svg: a line on a
	// whole unit then lights whole pixels instead of two halves.
	if err := call(procSetPixelOffsetMode, c.graphics, pixelOffsetHalf); err != nil {
		return err
	}
	scale := float64(min(target.Width, target.Height)) / menuGrid
	return newShape().figures(figures, 0, 0, scale).stroke(c.graphics, color, menuStroke*scale)
}
