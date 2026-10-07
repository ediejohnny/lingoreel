//go:build windows && amd64

package gdiplus

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"unsafe"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// The seek preview: a still of the film over the time under the pointer,
// both in the bar's colors.

// blurShrink is how many times smaller than its box a still standing for a
// moment it was not taken at is drawn before it is enlarged back: blurred,
// as video sites show a still that is only near.
const blurShrink = 8.0

// Still is a still's picture as rows of premultiplied BGRA, ready to draw.
type Still struct {
	Width, Height int
	Pixels        []byte
}

// StillOf turns a decoded picture into a Still, opaque.
func StillOf(picture image.Image) *Still {
	bounds := picture.Bounds()
	still := &Still{Width: bounds.Dx(), Height: bounds.Dy(), Pixels: make([]byte, bounds.Dx()*bounds.Dy()*4)}
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var r, g, b uint8
			if ycc, ok := picture.(*image.YCbCr); ok {
				c := ycc.YCbCrAt(x, y)
				r, g, b = color.YCbCrToRGB(c.Y, c.Cb, c.Cr)
			} else {
				r16, g16, b16, _ := picture.At(x, y).RGBA()
				r, g, b = uint8(r16>>8), uint8(g16>>8), uint8(b16>>8)
			}
			still.Pixels[i], still.Pixels[i+1], still.Pixels[i+2], still.Pixels[i+3] = b, g, r, 0xff
			i += 4
		}
	}
	return still
}

// PreviewLook is what the seek preview shows besides its layout.
type PreviewLook struct {
	// Style is the bar's colors and font, at Scale.
	Style appearance.BarStyle
	Scale float64
	// Still is the picture, nil for an empty box; Blurred shows it as the
	// nearest still taken, not the moment's own.
	Still   *Still
	Blurred bool
	// Time is the moment under the pointer, worded.
	Time string
}

// PreviewLabel is the size of the box the time is written in, as the bar's
// tips are.
func (r *Renderer) PreviewLabel(text string, style appearance.BarStyle, scale float64) (width, height float64) {
	font := layout.Font{Family: style.FontFamily, Size: barFontSize * scale}
	return r.Width(text, font) + 2*barTipPadding*scale, r.LineHeight(font) + 2*barTipPadding*scale
}

// DrawPreview paints the seek preview onto a cleared bitmap covering
// preview.Box: the still (or an empty box in the panel's color) framed in
// the menus' color, and the time under it like a tip.
func (r *Renderer) DrawPreview(target *win32.DIB, preview controlbar.Preview, look PreviewLook) error {
	c, err := r.canvasOn(target)
	if err != nil {
		return err
	}
	defer c.close()
	if err := call(procTranslateWorldTransform, c.graphics, f32(-preview.Box.X), f32(-preview.Box.Y), matrixOrderAppend); err != nil {
		return err
	}
	g, s := c.graphics, look.Scale
	p := paletteOf(look.Style)
	if preview.Picture.Valid() {
		if look.Still != nil {
			err = drawStill(g, look.Still, preview.Picture, look.Blurred)
		} else {
			err = fillRect(g, p.panel, preview.Picture)
		}
		if err != nil {
			return err
		}
		// The frame also hides the half-see-through edge GDI+ leaves around a
		// scaled picture.
		if err := newShape().rounded(preview.Picture, barRadius*s).stroke(g, p.menu, s); err != nil {
			return err
		}
	}
	if err := newShape().rounded(preview.Label, barRadius*s).fill(g, p.tip); err != nil {
		return err
	}
	font := layout.Font{Family: look.Style.FontFamily, Size: barFontSize * s}
	return r.writeLine(g, look.Time, font, preview.Label, true, p.icon)
}

// drawStill draws a still scaled into a box; blurred, it is made small
// first and enlarged back.
func drawStill(g uintptr, still *Still, box layout.Rect, blurred bool) error {
	if still.Width < 1 || still.Height < 1 || len(still.Pixels) < still.Width*still.Height*4 {
		return nil
	}
	defer runtime.KeepAlive(still)
	var picture uintptr
	if err := call(procCreateBitmapFromScan0, uintptr(still.Width), uintptr(still.Height), uintptr(still.Width*4),
		pixelFormat32bppPARGB, uintptr(unsafe.Pointer(&still.Pixels[0])), uintptr(unsafe.Pointer(&picture))); err != nil {
		return err
	}
	defer procDisposeImage.Call(picture)
	source := picture
	if blurred {
		small, err := shrink(picture, max(int(math.Round(box.W/blurShrink)), 1), max(int(math.Round(box.H/blurShrink)), 1))
		if err != nil {
			return err
		}
		defer procDisposeImage.Call(small)
		source = small
	}
	if err := call(procSetInterpolationMode, g, interpolationHighQualityBicubic); err != nil {
		return err
	}
	if err := call(procSetPixelOffsetMode, g, pixelOffsetHalf); err != nil {
		return err
	}
	x, y := math.Round(box.X), math.Round(box.Y)
	width, height := math.Round(box.Right())-x, math.Round(box.Bottom())-y
	return call(procDrawImageRectI, g, source, uintptr(int32(x)), uintptr(int32(y)), uintptr(int32(width)), uintptr(int32(height)))
}

// shrink draws a picture into a new, smaller one.
func shrink(picture uintptr, width, height int) (uintptr, error) {
	var small, graphics uintptr
	if err := call(procCreateBitmapFromScan0, uintptr(width), uintptr(height), 0, pixelFormat32bppPARGB, 0, uintptr(unsafe.Pointer(&small))); err != nil {
		return 0, err
	}
	err := call(procGetImageGraphicsContext, small, uintptr(unsafe.Pointer(&graphics)))
	if err == nil {
		defer procDeleteGraphics.Call(graphics)
		err = call(procSetInterpolationMode, graphics, interpolationHighQualityBilinear)
	}
	if err == nil {
		err = call(procDrawImageRectI, graphics, picture, 0, 0, uintptr(width), uintptr(height))
	}
	if err != nil {
		procDisposeImage.Call(small)
		return 0, err
	}
	return small, nil
}
