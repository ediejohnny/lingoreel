//go:build windows

package win32

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// maxDIBSide bounds a bitmap side; no monitor is anywhere near it.
const maxDIBSide = 16384

// DIB is a top-down 32-bit bitmap in premultiplied BGRA, selected into a
// memory device context so it can be drawn on and shown.
type DIB struct {
	Width, Height int
	// Pixels aliases the bitmap memory: 4 bytes per pixel, rows top to bottom.
	Pixels []byte
	DC     windows.Handle
	bitmap windows.Handle
	old    uintptr
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// NewDIB allocates a cleared bitmap.
func NewDIB(width, height int) (*DIB, error) {
	if width <= 0 || height <= 0 || width > maxDIBSide || height > maxDIBSide {
		return nil, fmt.Errorf("bitmap size %dx%d out of range", width, height)
	}
	dc, _, err := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC: %w", callErr(err))
	}
	header := bitmapInfoHeader{
		Width: int32(width), Height: -int32(height), // negative: top-down rows
		Planes: 1, BitCount: 32,
	}
	header.Size = uint32(unsafe.Sizeof(header))
	var bits unsafe.Pointer
	bitmap, _, err := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		procDeleteDC.Call(dc)
		return nil, fmt.Errorf("CreateDIBSection: %w", callErr(err))
	}
	old, _, _ := procSelectObject.Call(dc, bitmap)
	return &DIB{
		Width: width, Height: height,
		Pixels: unsafe.Slice((*byte)(bits), width*height*4),
		DC:     windows.Handle(dc), bitmap: windows.Handle(bitmap), old: old,
	}, nil
}

// Clear makes every pixel fully transparent.
func (d *DIB) Clear() {
	clear(d.Pixels)
}

// Close releases the bitmap and its device context.
func (d *DIB) Close() {
	if d == nil || d.DC == 0 {
		return
	}
	procSelectObject.Call(uintptr(d.DC), d.old)
	procDeleteObject.Call(uintptr(d.bitmap))
	procDeleteDC.Call(uintptr(d.DC))
	d.Pixels, d.DC, d.bitmap = nil, 0, 0
}
