//go:build windows && amd64

package gdiplus

import (
	"errors"
	"image"
	"math"
	"unsafe"

	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/layout"
	"lingoreel/internal/infrastructure/windows/win32"
)

// maxCachedFonts bounds the font cache; layouts use a handful of sizes.
const maxCachedFonts = 64

// Shadow geometry.
const (
	// antialiasMargin covers the pixels antialiasing paints around a glyph.
	antialiasMargin = 2.0
	// blurReach is how many blur radii a soft shadow spreads beyond its shape.
	blurReach = 3
)

// Look says how to draw a frame: the style of each line and whether the
// translation is currently shown.
type Look struct {
	Primary            appearance.SubtitleStyle
	Translation        appearance.SubtitleStyle
	TranslationVisible bool
	Notice             appearance.SubtitleStyle
	// PrimaryHidden leaves the line on top out.
	PrimaryHidden bool
	// Cue draws the primary as a preview, in Preview, with its indicator.
	Cue     *CueLook
	Preview appearance.Color
	// FillWidths paint each line of the primary in FillColor from its left
	// edge, as the line is spoken; nil paints nothing.
	FillWidths []float64
	FillColor  appearance.Color
}

// Renderer measures and draws subtitle text. It is not safe for concurrent
// use; the overlay calls it from its UI thread only.
type Renderer struct {
	token    uintptr
	format   uintptr
	measure  uintptr // graphics of a 1x1 bitmap, used only to measure
	bitmap   uintptr
	families map[string]uintptr
	owned    []uintptr // families created here, deleted on Close
	fonts    map[layout.Font]uintptr
	shadow   *win32.DIB // scratch layer for blurred shadows
	group    *win32.DIB // scratch layer for partly transparent text
}

// NewRenderer starts GDI+ and prepares the measuring context.
func NewRenderer() (*Renderer, error) {
	token, err := startup()
	if err != nil {
		return nil, err
	}
	r := &Renderer{token: token, families: map[string]uintptr{}, fonts: map[layout.Font]uintptr{}}
	// The generic typographic format is shared by GDI+: clone it before
	// changing its flags.
	var typographic uintptr
	err = call(procGenericTypographic, uintptr(unsafe.Pointer(&typographic)))
	if err == nil {
		err = call(procCloneStringFormat, typographic, uintptr(unsafe.Pointer(&r.format)))
	}
	if err == nil {
		err = call(procSetStringFormatFlags, r.format, stringFormatNoWrap|stringFormatMeasureTrailingSpaces)
	}
	if err == nil {
		err = call(procCreateBitmapFromScan0, 1, 1, 0, pixelFormat32bppPARGB, 0, uintptr(unsafe.Pointer(&r.bitmap)))
	}
	if err == nil {
		err = call(procGetImageGraphicsContext, r.bitmap, uintptr(unsafe.Pointer(&r.measure)))
	}
	if err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}

// Close releases every GDI+ object and shuts GDI+ down.
func (r *Renderer) Close() {
	if r.token == 0 {
		return
	}
	for _, font := range r.fonts {
		procDeleteFont.Call(font)
	}
	for _, family := range r.owned {
		procDeleteFontFamily.Call(family)
	}
	if r.measure != 0 {
		procDeleteGraphics.Call(r.measure)
	}
	if r.bitmap != 0 {
		procDisposeImage.Call(r.bitmap)
	}
	if r.format != 0 {
		procDeleteStringFormat.Call(r.format)
	}
	r.shadow.Close()
	r.group.Close()
	procShutdown.Call(r.token)
	*r = Renderer{}
}

// family returns the font family, falling back to the system sans serif
// when the name is unknown.
func (r *Renderer) family(name string) uintptr {
	if family, found := r.families[name]; found {
		return family
	}
	var family uintptr
	if name == "" || call(procCreateFontFamilyFromName, uintptr(unsafe.Pointer(&toUTF16(name)[0])), 0, uintptr(unsafe.Pointer(&family))) != nil {
		family = 0
		if call(procGetGenericSansSerif, uintptr(unsafe.Pointer(&family))) != nil {
			family = 0
		}
	} else {
		r.owned = append(r.owned, family)
	}
	r.families[name] = family
	return family
}

func styleBits(font layout.Font) uintptr {
	var bits uintptr
	if font.Bold {
		bits |= fontStyleBold
	}
	if font.Italic {
		bits |= fontStyleItalic
	}
	return bits
}

func usable(font layout.Font) bool {
	return font.Size > 0 && !math.IsInf(font.Size, 1)
}

func (r *Renderer) font(font layout.Font) uintptr {
	if handle, found := r.fonts[font]; found {
		return handle
	}
	if len(r.fonts) >= maxCachedFonts {
		for key, handle := range r.fonts {
			procDeleteFont.Call(handle)
			delete(r.fonts, key)
		}
	}
	var handle uintptr
	if call(procCreateFont, r.family(font.Family), f32(font.Size), styleBits(font), unitPixel, uintptr(unsafe.Pointer(&handle))) != nil {
		return 0
	}
	r.fonts[font] = handle
	return handle
}

// Width returns the advance width of one line of text.
func (r *Renderer) Width(text string, font layout.Font) float64 {
	if text == "" || !usable(font) {
		return 0
	}
	handle := r.font(font)
	if handle == 0 {
		return 0
	}
	encoded := toUTF16(text)
	var layoutRect, box rectF
	if call(procMeasureString, r.measure, uintptr(unsafe.Pointer(&encoded[0])), uintptr(len(encoded)-1), handle,
		uintptr(unsafe.Pointer(&layoutRect)), r.format, uintptr(unsafe.Pointer(&box)), 0, 0) != nil {
		return 0
	}
	return float64(box.W)
}

// LineHeight returns the font's line spacing in pixels.
func (r *Renderer) LineHeight(font layout.Font) float64 {
	if !usable(font) {
		return 0
	}
	family := r.family(font.Family)
	var em, spacing uint16
	if call(procGetEmHeight, family, styleBits(font), uintptr(unsafe.Pointer(&em))) != nil ||
		call(procGetLineSpacing, family, styleBits(font), uintptr(unsafe.Pointer(&spacing))) != nil || em == 0 {
		return font.Size
	}
	return font.Size * float64(spacing) / float64(em)
}

// Draw paints a frame onto a cleared bitmap.
func (r *Renderer) Draw(target *win32.DIB, frame layout.Frame, look Look) error {
	canvas, err := r.canvasOn(target)
	if err != nil {
		return err
	}
	defer canvas.close()
	if frame.Primary != nil && !look.PrimaryHidden {
		if err := r.drawPrimary(canvas, target, frame.Primary, look); err != nil {
			return err
		}
	}
	if frame.Translation != nil && look.TranslationVisible {
		if err := r.drawBlock(canvas, target, frame.Translation, look.Translation); err != nil {
			return err
		}
	}
	if frame.Notice != nil {
		return r.drawBlock(canvas, target, frame.Notice, look.Notice)
	}
	return nil
}

// canvas is a GDI+ graphics drawing straight into a bitmap's pixels.
type canvas struct {
	bitmap, graphics uintptr
}

func (r *Renderer) canvasOn(target *win32.DIB) (*canvas, error) {
	if target == nil || len(target.Pixels) == 0 {
		return nil, errors.New("no bitmap to draw on")
	}
	c := &canvas{}
	err := call(procCreateBitmapFromScan0, uintptr(target.Width), uintptr(target.Height), uintptr(target.Width*4),
		pixelFormat32bppPARGB, uintptr(unsafe.Pointer(&target.Pixels[0])), uintptr(unsafe.Pointer(&c.bitmap)))
	if err == nil {
		err = call(procGetImageGraphicsContext, c.bitmap, uintptr(unsafe.Pointer(&c.graphics)))
	}
	if err == nil {
		err = call(procSetSmoothingMode, c.graphics, smoothingAntiAlias)
	}
	if err != nil {
		c.close()
		return nil, err
	}
	return c, nil
}

func (c *canvas) close() {
	if c.graphics != 0 {
		procDeleteGraphics.Call(c.graphics)
	}
	if c.bitmap != 0 {
		procDisposeImage.Call(c.bitmap)
	}
}

func (r *Renderer) path(block *layout.Block) (uintptr, error) {
	var path uintptr
	if err := call(procCreatePath, fillModeWinding, uintptr(unsafe.Pointer(&path))); err != nil {
		return 0, err
	}
	family := r.family(block.Font.Family)
	for _, line := range block.Lines {
		encoded := toUTF16(line.Text)
		if len(encoded) < 2 {
			continue
		}
		origin := rectF{X: float32(line.X), Y: float32(line.Y)}
		if err := call(procAddPathString, path, uintptr(unsafe.Pointer(&encoded[0])), uintptr(len(encoded)-1), family,
			styleBits(block.Font), f32(block.Font.Size), uintptr(unsafe.Pointer(&origin)), r.format); err != nil {
			procDeletePath.Call(path)
			return 0, err
		}
	}
	return path, nil
}

func (r *Renderer) drawBlock(c *canvas, target *win32.DIB, block *layout.Block, style appearance.SubtitleStyle) error {
	if !usable(block.Font) || len(block.Lines) == 0 {
		return nil
	}
	path, err := r.path(block)
	if err != nil {
		return err
	}
	defer procDeletePath.Call(path)

	if style.Background.Enabled {
		if err := fillRect(c.graphics, style.Background.Color, block.Bounds); err != nil {
			return err
		}
	}
	opacity := math.Max(0, math.Min(style.Opacity, 1))
	if opacity >= 1 {
		return r.drawText(c, target, block, style, path)
	}
	if opacity == 0 {
		return nil
	}
	// Opacity applies to the text as a whole: drawn opaque on its own layer,
	// the outline under the fill does not show through it.
	region := textRegion(block, style, target)
	if region.Empty() {
		return nil
	}
	group, err := ensureLayer(&r.group, target)
	if err != nil {
		return err
	}
	clearRegion(group.Pixels, group.Width, region)
	layer, err := r.canvasOn(group)
	if err != nil {
		return err
	}
	err = r.drawText(layer, group, block, style, path)
	layer.close()
	if err != nil {
		return err
	}
	compositeRegion(target.Pixels, group.Pixels, target.Width, region, int(math.Round(opacity*255)))
	return nil
}

// drawText paints shadow, outline and fill, in that order, at full opacity.
func (r *Renderer) drawText(c *canvas, surface *win32.DIB, block *layout.Block, style appearance.SubtitleStyle, path uintptr) error {
	if style.Shadow.Enabled {
		if err := r.drawShadow(c, surface, block, style, path); err != nil {
			return err
		}
	}
	if style.Outline.Enabled && style.Outline.Width > 0 {
		if err := strokePath(c.graphics, path, style.Outline.Color, style.Outline.Width*block.Scale); err != nil {
			return err
		}
	}
	return fillPath(c.graphics, path, style.Color)
}

// textRegion is the area the text, its outline and its shadow can reach.
func textRegion(block *layout.Block, style appearance.SubtitleStyle, target *win32.DIB) image.Rectangle {
	margin := antialiasMargin
	if style.Outline.Enabled {
		margin += style.Outline.Width * block.Scale
	}
	if style.Shadow.Enabled {
		s := style.Shadow
		margin += (math.Abs(s.OffsetX) + math.Abs(s.OffsetY) + blurReach*s.Blur) * block.Scale
	}
	b := block.Bounds
	return image.Rect(
		int(math.Floor(b.X-margin)), int(math.Floor(b.Y-margin)),
		int(math.Ceil(b.Right()+margin)), int(math.Ceil(b.Bottom()+margin)),
	).Intersect(image.Rect(0, 0, target.Width, target.Height))
}

// ensureLayer returns a scratch bitmap the size of the target, reusing it
// while the size does not change.
func ensureLayer(layer **win32.DIB, target *win32.DIB) (*win32.DIB, error) {
	if current := *layer; current != nil && current.Width == target.Width && current.Height == target.Height {
		return current, nil
	}
	(*layer).Close()
	fresh, err := win32.NewDIB(target.Width, target.Height)
	if err != nil {
		*layer = nil
		return nil, err
	}
	*layer = fresh
	return fresh, nil
}

// drawShadow paints the text shape offset and, when asked, blurred. The
// shadow includes the outline so it matches the visible glyph.
func (r *Renderer) drawShadow(c *canvas, surface *win32.DIB, block *layout.Block, style appearance.SubtitleStyle, path uintptr) error {
	shadow := style.Shadow
	dx, dy := shadow.OffsetX*block.Scale, shadow.OffsetY*block.Scale
	outline := 0.0
	if style.Outline.Enabled {
		outline = style.Outline.Width * block.Scale
	}
	blur := shadow.Blur * block.Scale

	paint := func(graphics uintptr) error {
		if err := call(procTranslateWorldTransform, graphics, f32(dx), f32(dy), matrixOrderAppend); err != nil {
			return err
		}
		defer procResetWorldTransform.Call(graphics)
		if outline > 0 {
			if err := strokePath(graphics, path, shadow.Color, outline); err != nil {
				return err
			}
		}
		return fillPath(graphics, path, shadow.Color)
	}
	if blur < 1 {
		return paint(c.graphics)
	}

	region := textRegion(block, style, surface)
	if region.Empty() {
		return nil
	}
	scratch, err := ensureLayer(&r.shadow, surface)
	if err != nil {
		return err
	}
	clearRegion(scratch.Pixels, scratch.Width, region)
	layer, err := r.canvasOn(scratch)
	if err != nil {
		return err
	}
	err = paint(layer.graphics)
	layer.close()
	if err != nil {
		return err
	}
	// Each pass reaches its radius, so the passes together reach the blur.
	blurRegion(scratch.Pixels, scratch.Width, region, int(math.Round(blur/blurPasses)))
	compositeRegion(surface.Pixels, scratch.Pixels, surface.Width, region, 255)
	return nil
}

func colorOf(color appearance.Color) uintptr {
	return argb(color.R, color.G, color.B, color.A)
}

func fillPath(graphics, path uintptr, color appearance.Color) error {
	var brush uintptr
	if err := call(procCreateSolidFill, colorOf(color), uintptr(unsafe.Pointer(&brush))); err != nil {
		return err
	}
	defer procDeleteBrush.Call(brush)
	return call(procFillPath, graphics, brush, path)
}

// strokePath draws an outline of the given width outside the glyphs: the pen
// is centered on the edge and the fill covers its inner half.
func strokePath(graphics, path uintptr, color appearance.Color, width float64) error {
	var pen uintptr
	if err := call(procCreatePen1, colorOf(color), f32(2*width), unitPixel, uintptr(unsafe.Pointer(&pen))); err != nil {
		return err
	}
	defer procDeletePen.Call(pen)
	if err := call(procSetPenLineJoin, pen, lineJoinRound); err != nil {
		return err
	}
	return call(procDrawPath, graphics, pen, path)
}

func fillRect(graphics uintptr, color appearance.Color, rect layout.Rect) error {
	var brush uintptr
	if err := call(procCreateSolidFill, colorOf(color), uintptr(unsafe.Pointer(&brush))); err != nil {
		return err
	}
	defer procDeleteBrush.Call(brush)
	return call(procFillRectangle, graphics, brush, f32(rect.X), f32(rect.Y), f32(rect.W), f32(rect.H))
}
