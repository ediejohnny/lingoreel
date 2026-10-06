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
	// Style is the bar's thickness and color and the seconds' size.
	Style appearance.CueStyle
}

// The indicator's geometry is the layout's, so the room the subtitles keep
// for it is the room it takes.
const (
	cueBarMinPixels = layout.CueBarMinPixels
	cueGap          = layout.CueGap
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
	return r.drawBlockThen(c, target, block, look.Primary, func(c *canvas, path uintptr) error {
		return drawFill(c, block, look, path)
	})
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
		font.Size *= cue.Style.CountdownSize
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
	thickness := math.Max(cue.Style.BarThickness*block.Scale, cueBarMinPixels)
	// The bar fades with the line's opacity, as the text does.
	color := cue.Style.BarColor
	alpha := float64(color.A) * math.Max(0, math.Min(style.Opacity, 1))
	if alpha < 1 {
		return nil
	}
	bar := layout.Rect{X: bounds.X, Y: bounds.Y - gap - thickness, W: bounds.W * progress, H: thickness}
	return newShape().rounded(bar, math.Min(thickness/2, barRadius*block.Scale)).fill(c.graphics, color.WithAlpha(uint8(math.Round(alpha))))
}

// drawFill paints the spoken part of each line over the text's path,
// clipped at the line's fill width, like a karaoke screen.
func drawFill(c *canvas, block *layout.Block, look Look, path uintptr) error {
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
