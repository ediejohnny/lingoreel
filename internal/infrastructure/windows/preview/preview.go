//go:build windows && amd64

// Package preview draws sample subtitles with a look over a sample picture,
// with the overlay's own renderer, so the settings screen shows exactly what
// the video will.
package preview

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/uithread"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// Limits of a preview picture.
const (
	minSide = 16
	maxSide = 4096
	minDPI  = 48
	maxDPI  = 480
)

// ErrInvalidSize reports a picture size or a scaling outside the limits.
var ErrInvalidSize = errors.New("invalid preview size")

// Renderer draws previews on a thread of its own, where its GDI+ objects
// live. It is safe for concurrent use: drawings wait for each other.
type Renderer struct {
	thread   *uithread.Thread
	renderer *gdiplus.Renderer
}

// Start prepares the renderer.
func Start() (*Renderer, error) {
	thread, err := uithread.Start()
	if err != nil {
		return nil, fmt.Errorf("starting the preview thread: %w", err)
	}
	r := &Renderer{thread: thread}
	var startErr error
	if err := thread.Do(func() { r.renderer, startErr = gdiplus.NewRenderer() }); err != nil || startErr != nil {
		return nil, errors.Join(err, startErr, thread.Close())
	}
	return r, nil
}

// Preview draws the screen's two lines with a look on a picture of width ×
// height pixels at a display scaling (96 is 100%), as PNG. Both lines show;
// the screen's highlight outlines the line being edited, its cue shows the
// line on top before its time, and its fill paints that share of the line
// in the look's fill color. A notice, when not empty, shows as the video's
// would.
func (r *Renderer) Preview(look session.Look, width, height, dpi int, screen session.Screen, notice string) ([]byte, error) {
	if width < minSide || height < minSide || width > maxSide || height > maxSide || dpi < minDPI || dpi > maxDPI {
		return nil, fmt.Errorf("%w: %d×%d at %d dpi", ErrInvalidSize, width, height, dpi)
	}
	var picture *image.RGBA
	var drawErr error
	if err := r.thread.Do(func() { picture, drawErr = r.draw(look, width, height, dpi, screen, notice) }); err != nil {
		return nil, err
	}
	if drawErr != nil {
		return nil, drawErr
	}
	var out bytes.Buffer
	if err := png.Encode(&out, picture); err != nil {
		return nil, fmt.Errorf("encoding the preview: %w", err)
	}
	return out.Bytes(), nil
}

// draw runs on the renderer's thread.
func (r *Renderer) draw(look session.Look, width, height, dpi int, screen session.Screen, noticeText string) (*image.RGBA, error) {
	dib, err := win32.NewDIB(width, height)
	if err != nil {
		return nil, fmt.Errorf("creating the preview bitmap: %w", err)
	}
	defer dib.Close()
	area := layout.Rect{W: float64(width), H: float64(height)}
	notice := look.Notice.Style()
	frame := layout.Compute(layout.Input{
		Video: area, DPI: dpi, Layout: look.Layout,
		Primary:     layout.Text{Style: look.Primary, Content: screen.Primary},
		Translation: layout.Text{Style: look.Translation, Content: screen.Translation},
		Notice: layout.Notice{
			Text:   layout.Text{Style: notice, Content: noticeText},
			Margin: look.Notice.Margin, MaxWidth: look.Notice.MaxWidth,
		},
	}, r.renderer)
	drawn := gdiplus.Look{
		Primary: look.Primary, Translation: look.Translation, TranslationVisible: true, Notice: notice,
		Preview: look.PreviewColor, FillColor: look.FillColor,
	}
	if cue := screen.Cue; cue != nil {
		drawn.Cue = &gdiplus.CueLook{Indicator: cue.Indicator, Progress: cue.Progress, Seconds: cue.Seconds, Style: look.Cue}
	} else if screen.Fill > 0 {
		drawn.FillWidths = layout.FillWidths(frame.Primary, screen.Fill, r.renderer)
	}
	if screen.Highlighted {
		drawn.Highlight = gdiplus.OutlineAround(boxOf(frame, screen.Highlight), dpi, look.Selection)
	}
	if err := r.renderer.Draw(dib, frame, drawn); err != nil {
		return nil, fmt.Errorf("drawing the preview: %w", err)
	}
	picture := scene(width, height)
	over(picture, dib.Pixels, 0)
	return picture, nil
}

// The bar's sample: a video a quarter played, at 150% of a player that goes
// to 200%, so every color of the bar shows.
const (
	sampleBarPlayed  = 0.25
	sampleBarVolume  = 1.5
	sampleBarCeiling = 2
)

// PreviewBar draws the control bar in a style at the bottom of the sample
// picture, width × height pixels at a display scaling (96 is 100%), as PNG:
// the speed list open, the mouse over the play button, the volume past
// 100%. clock is the bar's time text, and speed words a speed.
func (r *Renderer) PreviewBar(style appearance.BarStyle, width, height, dpi int, clock string, speed func(playback.Speed) string) ([]byte, error) {
	if width < minSide || height < minSide || width > maxSide || height > maxSide || dpi < minDPI || dpi > maxDPI {
		return nil, fmt.Errorf("%w: %d×%d at %d dpi", ErrInvalidSize, width, height, dpi)
	}
	var picture *image.RGBA
	var drawErr error
	if err := r.thread.Do(func() { picture, drawErr = r.drawBar(style, width, height, dpi, clock, speed) }); err != nil {
		return nil, err
	}
	if drawErr != nil {
		return nil, drawErr
	}
	var out bytes.Buffer
	if err := png.Encode(&out, picture); err != nil {
		return nil, fmt.Errorf("encoding the preview: %w", err)
	}
	return out.Bytes(), nil
}

// drawBar runs on the renderer's thread.
func (r *Renderer) drawBar(style appearance.BarStyle, width, height, dpi int, clock string, speed func(playback.Speed) string) (*image.RGBA, error) {
	picture := scene(width, height)
	const duration = time.Hour
	state := controlbar.State{
		Position: time.Duration(sampleBarPlayed * float64(duration)), Duration: duration,
		Speed: playback.NormalSpeed, Volume: sampleBarVolume, SpeedMenuOpen: true, HasTranslation: true, TranslationVisible: true,
		Capabilities: playback.Capabilities{
			Pause: true, Seek: true, Speed: true, Volume: true, MaxVolume: sampleBarCeiling, FrameStepForward: true, FrameStepBackward: true,
		},
	}
	bar := controlbar.Layout(layout.Rect{W: float64(width), H: float64(height)}, controlbar.Scale(dpi)*style.Size, state)
	if !bar.Band.Valid() {
		return picture, nil
	}
	// Whole pixels, as the player window draws it.
	top := math.Floor(bar.Band.Y)
	bar.Band = layout.Rect{X: 0, Y: top, W: float64(width), H: math.Ceil(bar.Band.Bottom() - top)}
	dib, err := win32.NewDIB(width, int(bar.Band.H))
	if err != nil {
		return nil, fmt.Errorf("creating the preview bitmap: %w", err)
	}
	defer dib.Close()
	look := gdiplus.BarLook{
		Style: style, Time: clock, Thumb: true,
		Label: func(item controlbar.Item) string { return speed(item.Speed) },
	}
	for _, item := range bar.Items {
		if item.Action == controlbar.ActionPlayPause {
			look.Hover = item
		}
	}
	if err := r.renderer.DrawBar(dib, bar, look); err != nil {
		return nil, fmt.Errorf("drawing the bar's preview: %w", err)
	}
	over(picture, dib.Pixels, int(top))
	return picture, nil
}

// boxOf is where a line was laid out; empty when the frame has no such line.
func boxOf(frame layout.Frame, line session.Line) layout.Rect {
	block := frame.Primary
	if line == session.LineTranslation {
		block = frame.Translation
	}
	if block == nil {
		return layout.Rect{}
	}
	return block.Bounds
}

// Fonts lists the font families installed on the computer.
func (r *Renderer) Fonts() ([]string, error) {
	var families []string
	var listErr error
	if err := r.thread.Do(func() { families, listErr = r.renderer.Families() }); err != nil {
		return nil, err
	}
	return families, listErr
}

// Close releases the renderer and its thread.
func (r *Renderer) Close() error {
	err := r.thread.Do(func() {
		r.renderer.Close()
	})
	return errors.Join(err, r.thread.Close())
}

// Colors of the sample picture: a dusk sky over a darker ground, light
// enough at the top and dark enough at the bottom to judge both.
var (
	skyTop    = color.RGBA{0x5B, 0x7C, 0x99, 0xFF}
	skyBottom = color.RGBA{0xC9, 0x8E, 0x6A, 0xFF}
	ground    = color.RGBA{0x2A, 0x2D, 0x33, 0xFF}
)

// horizon is where the ground begins, as a share of the height.
const horizon = 0.62

// scene paints the sample picture behind the subtitles.
func scene(width, height int) *image.RGBA {
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	line := int(float64(height) * horizon)
	for y := range height {
		c := ground
		if y < line {
			c = mix(skyTop, skyBottom, float64(y)/float64(max(line-1, 1)))
		}
		row := picture.Pix[y*picture.Stride : y*picture.Stride+width*4]
		for x := 0; x < len(row); x += 4 {
			row[x], row[x+1], row[x+2], row[x+3] = c.R, c.G, c.B, c.A
		}
	}
	return picture
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	channel := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.RGBA{channel(a.R, b.R), channel(a.G, b.G), channel(a.B, b.B), 0xFF}
}

// over composes a drawing (premultiplied BGRA, as wide as the picture) onto
// the picture, its first row at the picture's row top.
func over(picture *image.RGBA, bgra []byte, top int) {
	offset := top * picture.Stride
	for i := 0; i+3 < len(bgra) && offset+i+3 < len(picture.Pix); i += 4 {
		alpha := uint32(bgra[i+3])
		if alpha == 0 {
			continue
		}
		keep := 255 - alpha
		p := picture.Pix[offset+i : offset+i+4 : offset+i+4]
		p[0] = uint8(uint32(bgra[i+2]) + (uint32(p[0])*keep+127)/255)
		p[1] = uint8(uint32(bgra[i+1]) + (uint32(p[1])*keep+127)/255)
		p[2] = uint8(uint32(bgra[i]) + (uint32(p[2])*keep+127)/255)
	}
}
