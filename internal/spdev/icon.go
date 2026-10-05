package spdev

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
)

// The app's icon, drawn here so its art is the project's own: a dark square
// with a play sign in the app's blue and two subtitle lines below it, the top
// one half painted in the Modo Karaokê's fill.
var (
	iconBackground = color.NRGBA{R: 0x0B, G: 0x12, B: 0x20, A: 0xFF}
	iconPlay       = color.NRGBA{R: 0x5B, G: 0x9B, B: 0xFF, A: 0xFF}
	iconLine       = color.NRGBA{R: 0xF2, G: 0xF2, B: 0xF2, A: 0xFF}
	iconFill       = color.NRGBA{R: 0x00, G: 0xE5, B: 0xFF, A: 0xFF}
	iconSecondLine = color.NRGBA{R: 0x8A, G: 0x9B, B: 0xB4, A: 0xFF}
)

// iconSizes are the sizes in the Windows icon: the taskbar, the title bar,
// Explorer's views and high-DPI screens each pick the nearest.
var iconSizes = []int{16, 20, 24, 32, 40, 48, 64, 96, 128, 256}

// appIconSize is the picture Wails takes as the app's icon.
const appIconSize = 1024

// iconSamples is the subsamples per pixel side for the edges' antialiasing.
const iconSamples = 4

// shape tells whether a point, in units of the icon's side (0 to 1), is
// inside, and its color.
type shape struct {
	inside func(x, y float64) bool
	color  color.NRGBA
}

func iconShapes() []shape {
	const corner = 0.09 // a square, barely rounded, like the app's interface
	roundedSquare := func(x, y float64) bool {
		dx := math.Max(math.Max(corner-x, x-(1-corner)), 0)
		dy := math.Max(math.Max(corner-y, y-(1-corner)), 0)
		return x >= 0 && x <= 1 && y >= 0 && y <= 1 && dx*dx+dy*dy <= corner*corner
	}
	// The play sign: a triangle pointing right, its weight centered.
	ax, ay, bx, by, cx, cy := 0.38, 0.17, 0.38, 0.55, 0.68, 0.36
	play := func(x, y float64) bool {
		d1 := (x-bx)*(ay-by) - (ax-bx)*(y-by)
		d2 := (x-cx)*(by-cy) - (bx-cx)*(y-cy)
		d3 := (x-ax)*(cy-ay) - (cx-ax)*(y-ay)
		negative := d1 < 0 || d2 < 0 || d3 < 0
		positive := d1 > 0 || d2 > 0 || d3 > 0
		return !(negative && positive)
	}
	bar := func(left, top, right, bottom float64) func(x, y float64) bool {
		return func(x, y float64) bool { return x >= left && x <= right && y >= top && y <= bottom }
	}
	return []shape{
		{roundedSquare, iconBackground},
		{play, iconPlay},
		{bar(0.16, 0.66, 0.84, 0.735), iconLine},
		{bar(0.16, 0.66, 0.50, 0.735), iconFill},
		{bar(0.26, 0.80, 0.74, 0.855), iconSecondLine},
	}
}

// drawIcon draws the icon at a size, each pixel the average of its
// subsamples, the last shape over a point winning.
func drawIcon(size int) *image.NRGBA {
	shapes := iconShapes()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := range size {
		for px := range size {
			var r, g, b, a float64
			for sy := range iconSamples {
				for sx := range iconSamples {
					x := (float64(px) + (float64(sx)+0.5)/iconSamples) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/iconSamples) / float64(size)
					for i := len(shapes) - 1; i >= 0; i-- {
						if shapes[i].inside(x, y) {
							c := shapes[i].color
							r, g, b, a = r+float64(c.R), g+float64(c.G), b+float64(c.B), a+255
							break
						}
					}
				}
			}
			if a == 0 {
				continue
			}
			// Colors averaged over the covered subsamples only, so edges
			// fade in alpha instead of darkening toward black.
			covered := a / 255
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(math.Round(r / covered)), G: uint8(math.Round(g / covered)), B: uint8(math.Round(b / covered)),
				A: uint8(math.Round(a / (iconSamples * iconSamples))),
			})
		}
	}
	return img
}

// writeICO writes a Windows icon with one picture per size: the 256-pixel
// one as PNG, the smaller ones as bitmaps, the convention every reader of
// icons follows (some refuse a small PNG).
func writeICO(w io.Writer, sizes []int) error {
	pictures := make([][]byte, len(sizes))
	for i, size := range sizes {
		if size >= 256 {
			var buf bytes.Buffer
			if err := png.Encode(&buf, drawIcon(size)); err != nil {
				return err
			}
			pictures[i] = buf.Bytes()
			continue
		}
		pictures[i] = iconBitmap(drawIcon(size))
	}
	const headerSize, entrySize = 6, 16
	header := []uint16{0, 1, uint16(len(sizes))} // reserved, type 1 (icon), count
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}
	offset := uint32(headerSize + entrySize*len(sizes))
	for i, size := range sizes {
		side := uint8(size)
		if size >= 256 {
			side = 0 // 0 means 256
		}
		entry := struct {
			Width, Height, Colors, Reserved uint8
			Planes, BitCount                uint16
			Size, Offset                    uint32
		}{side, side, 0, 0, 1, 32, uint32(len(pictures[i])), offset}
		if err := binary.Write(w, binary.LittleEndian, entry); err != nil {
			return err
		}
		offset += uint32(len(pictures[i]))
	}
	for _, picture := range pictures {
		if _, err := w.Write(picture); err != nil {
			return err
		}
	}
	return nil
}

// iconBitmap is a picture as an icon stores a bitmap: a BITMAPINFOHEADER
// with twice the height (the color rows, then the mask's), the 32-bit BGRA
// rows bottom up, then an empty 1-bit mask, the alpha doing its work.
func iconBitmap(img *image.NRGBA) []byte {
	size := img.Rect.Dx()
	maskRow := (size + 31) / 32 * 4
	var buf bytes.Buffer
	header := struct {
		Size                     uint32
		Width, Height            int32
		Planes, BitCount         uint16
		Compression, ImageSize   uint32
		XPerMeter, YPerMeter     int32
		ColorsUsed, ColorsNeeded uint32
	}{40, int32(size), int32(2 * size), 1, 32, 0, uint32(size*size*4 + maskRow*size), 0, 0, 0, 0}
	_ = binary.Write(&buf, binary.LittleEndian, header) // a bytes.Buffer does not fail
	for y := size - 1; y >= 0; y-- {
		for x := range size {
			c := img.NRGBAAt(x, y)
			buf.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	buf.Write(make([]byte, maskRow*size))
	return buf.Bytes()
}

// icon writes the app's icon into a build folder: appicon.png, the picture
// Wails takes, and windows\icon.ico, the one inside the executable.
func icon(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("icon", flag.ContinueOnError)
	flags.SetOutput(stdout)
	out := flags.String("out", "build", "the build folder to write into")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return errors.New("icon takes no arguments besides --out")
	}
	var picture bytes.Buffer
	if err := png.Encode(&picture, drawIcon(appIconSize)); err != nil {
		return err
	}
	pngPath := filepath.Join(*out, "appicon.png")
	if err := os.WriteFile(pngPath, picture.Bytes(), 0o644); err != nil {
		return err
	}
	var ico bytes.Buffer
	if err := writeICO(&ico, iconSizes); err != nil {
		return err
	}
	icoPath := filepath.Join(*out, "windows", "icon.ico")
	if err := os.MkdirAll(filepath.Dir(icoPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(icoPath, ico.Bytes(), 0o644); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "wrote %s and %s (%d sizes)\n", pngPath, icoPath, len(iconSizes))
	return err
}
