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

// The app's icon and logo, drawn here so it stays sharp at every size: a
// dark rounded square with a play sign split across the middle, its halves
// in two reds, a color few icons on a taskbar have.
var (
	iconBackground = color.NRGBA{R: 0x0C, G: 0x11, B: 0x1F, A: 0xFF}
	iconTop        = color.NRGBA{R: 0xE5, G: 0x09, B: 0x14, A: 0xFF}
	iconBottom     = color.NRGBA{R: 0xFF, G: 0x6F, B: 0x61, A: 0xFF}
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

func iconShapes(size int) []shape {
	const corner = 0.19
	roundedSquare := func(x, y float64) bool {
		dx := math.Max(math.Max(corner-x, x-(1-corner)), 0)
		dy := math.Max(math.Max(corner-y, y-(1-corner)), 0)
		return x >= 0 && x <= 1 && y >= 0 && y <= 1 && dx*dx+dy*dy <= corner*corner
	}
	// The play sign: a triangle pointing right, its tip on the middle line,
	// a tenth larger than the first drawing around the same center.
	const left, top, bottom, tip = 0.2477, 0.1535, 0.8465, 0.8263
	play := func(x, y float64) bool {
		if x < left || x > tip {
			return false
		}
		half := (bottom - top) / 2 * (tip - x) / (tip - left)
		return math.Abs(y-0.5) <= half
	}
	// A thin dark line splits the halves; under a pixel wide, in the small
	// sizes, it would only blur them, and the colors split them already.
	split := 0.005
	if float64(size)*2*split < 1 {
		split = 0
	}
	return []shape{
		{roundedSquare, iconBackground},
		{func(x, y float64) bool { return play(x, y) && y < 0.5-split }, iconTop},
		{func(x, y float64) bool { return play(x, y) && y > 0.5+split }, iconBottom},
	}
}

// drawIcon draws the icon at a size, each pixel the average of its
// subsamples, the last shape over a point winning.
func drawIcon(size int) *image.NRGBA {
	shapes := iconShapes(size)
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
