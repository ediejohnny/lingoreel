//go:build windows && amd64

package gdiplus

import (
	"math"
	"strconv"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// CueLook is how a preview shows the time left before its line.
type CueLook struct {
	Indicator study.Indicator
	// Progress goes from 1 when the preview begins to 0 at the line.
	Progress float64
	Seconds  int
}

// Indicator geometry, in style units (scaled like the text).
const (
	cueBarThickness = 5.0
	cueBarMinPixels = 3.0
	cueGap          = 8.0
	countdownScale  = 0.8
)

// drawPrimary draws the line on top: as a preview with its indicator, or
// normally with the part already spoken filled in.
func (r *Renderer) drawPrimary(c *canvas, target *win32.DIB, block *layout.Block, look Look) error {
	if look.Cue != nil {
		style := look.Primary
		style.Color = look.Preview
		if err := r.drawBlock(c, target, block, style); err != nil {
			return err
		}
		return r.drawCue(c, target, block, style, *look.Cue)
	}
	if err := r.drawBlock(c, target, block, look.Primary); err != nil {
		return err
	}
	return r.drawFill(c, block, look)
}

// drawCue draws the indicator above the preview: a bar the width of the
// text shrinking to nothing, or the seconds left.
func (r *Renderer) drawCue(c *canvas, target *win32.DIB, block *layout.Block, style appearance.SubtitleStyle, cue CueLook) error {
	bounds := block.Bounds
	gap := cueGap * block.Scale
	if cue.Indicator == study.IndicatorCountdown {
		if cue.Seconds <= 0 {
			return nil
		}
		text := strconv.Itoa(cue.Seconds)
		font := block.Font
		font.Size *= countdownScale
		width, height := r.Width(text, font), r.LineHeight(font)
		number := &layout.Block{Font: font, Scale: block.Scale, Lines: []layout.Line{{
			Text: text, X: bounds.X + (bounds.W-width)/2, Y: bounds.Y - gap - height, Width: width, Height: height,
		}}}
		number.Bounds = layout.Rect{X: number.Lines[0].X, Y: number.Lines[0].Y, W: width, H: height}
		style.Background.Enabled = false
		return r.drawBlock(c, target, number, style)
	}
	progress := math.Min(math.Max(cue.Progress, 0), 1)
	if progress <= 0 {
		return nil
	}
	thickness := math.Max(cueBarThickness*block.Scale, cueBarMinPixels)
	bar := layout.Rect{X: bounds.X, Y: bounds.Y - gap - thickness, W: bounds.W * progress, H: thickness}
	return newShape().rounded(bar, thickness/2).fill(c.graphics, style.Color)
}

// drawFill paints the spoken part of each line over the text, clipped at
// the line's fill width, like a karaoke screen.
func (r *Renderer) drawFill(c *canvas, block *layout.Block, look Look) error {
	painted := false
	for _, width := range look.FillWidths {
		painted = painted || width > 0
	}
	if !painted {
		return nil
	}
	path, err := r.path(block)
	if err != nil {
		return err
	}
	defer procDeletePath.Call(path)
	// The clip reaches above and below the line for accents and descenders.
	reach := antialiasMargin + look.Primary.Outline.Width*block.Scale
	for i, width := range look.FillWidths {
		if i >= len(block.Lines) || width <= 0 {
			continue
		}
		line := block.Lines[i]
		if err := call(procSetClipRect, c.graphics, f32(line.X), f32(line.Y-reach), f32(width), f32(line.Height+2*reach), combineModeReplace); err != nil {
			return err
		}
		err := fillPath(c.graphics, path, look.FillColor)
		procResetClip.Call(c.graphics)
		if err != nil {
			return err
		}
	}
	return nil
}
