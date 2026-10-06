//go:build windows && amd64

package overlay

import (
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// Spot is where a line was drawn and in which font: the place of a box that
// corrects the line where it shows.
type Spot struct {
	Box  layout.Rect
	Font layout.Font
}

// LineAt tells which line was drawn at a point of the covered window's
// client area, so a click there can pick it for editing.
func (o *Overlay) LineAt(x, y float64) (session.Line, bool) {
	o.boxesMu.Lock()
	defer o.boxesMu.Unlock()
	for _, line := range []session.Line{session.LinePrimary, session.LineTranslation} {
		if box := o.boxes[line]; box.W > 0 && box.H > 0 && box.Contains(x, y) {
			return line, true
		}
	}
	return 0, false
}

// SpotOf is where a line was last drawn and in which font, in the covered
// window's client pixels; false when the line is not on screen.
func (o *Overlay) SpotOf(line session.Line) (Spot, bool) {
	if !line.Valid() {
		return Spot{}, false
	}
	o.boxesMu.Lock()
	defer o.boxesMu.Unlock()
	box := o.boxes[line]
	if box.W <= 0 || box.H <= 0 {
		return Spot{}, false
	}
	return Spot{Box: box, Font: o.fonts[line]}, true
}

// cueRoom is the room the karaoke indicator takes above a previewed line,
// kept for the whole preview so the lines do not move when it ends.
func cueRoom(cue *session.CueScreen, style appearance.CueStyle) *layout.CueRoom {
	if cue == nil {
		return nil
	}
	return &layout.CueRoom{Countdown: cue.Indicator == study.IndicatorCountdown, CountdownSize: style.CountdownSize, BarThickness: style.BarThickness}
}

// drawnBoxes are the boxes of the lines a frame draws: a hidden line or one
// without text has none.
func drawnBoxes(frame layout.Frame, content session.Screen) [2]layout.Rect {
	var boxes [2]layout.Rect
	if frame.Primary != nil && !content.PrimaryHidden && content.Primary != "" {
		boxes[session.LinePrimary] = frame.Primary.Bounds
	}
	if frame.Translation != nil && content.TranslationVisible && content.Translation != "" {
		boxes[session.LineTranslation] = frame.Translation.Bounds
	}
	return boxes
}
