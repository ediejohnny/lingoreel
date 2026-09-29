//go:build windows && amd64

// Package gdiplus draws subtitle text with GDI+ (flat API, no cgo): text is
// turned into a path, so outline, shadow and fill are exact and antialiased.
//
// Floating point arguments are passed as their bits: on windows/amd64 the Go
// runtime copies the first four integer argument registers into XMM0-XMM3,
// which is where the callee reads float parameters.
package gdiplus

import (
	"fmt"
	"math"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	dll = windows.NewLazySystemDLL("gdiplus.dll")

	procStartup                  = dll.NewProc("GdiplusStartup")
	procShutdown                 = dll.NewProc("GdiplusShutdown")
	procCreateBitmapFromScan0    = dll.NewProc("GdipCreateBitmapFromScan0")
	procGetImageGraphicsContext  = dll.NewProc("GdipGetImageGraphicsContext")
	procDisposeImage             = dll.NewProc("GdipDisposeImage")
	procDeleteGraphics           = dll.NewProc("GdipDeleteGraphics")
	procSetSmoothingMode         = dll.NewProc("GdipSetSmoothingMode")
	procCreateFontFamilyFromName = dll.NewProc("GdipCreateFontFamilyFromName")
	procGetGenericSansSerif      = dll.NewProc("GdipGetGenericFontFamilySansSerif")
	procDeleteFontFamily         = dll.NewProc("GdipDeleteFontFamily")
	procGetEmHeight              = dll.NewProc("GdipGetEmHeight")
	procGetLineSpacing           = dll.NewProc("GdipGetLineSpacing")
	procCreateFont               = dll.NewProc("GdipCreateFont")
	procDeleteFont               = dll.NewProc("GdipDeleteFont")
	procGenericTypographic       = dll.NewProc("GdipStringFormatGetGenericTypographic")
	procCloneStringFormat        = dll.NewProc("GdipCloneStringFormat")
	procSetStringFormatFlags     = dll.NewProc("GdipSetStringFormatFlags")
	procDeleteStringFormat       = dll.NewProc("GdipDeleteStringFormat")
	procMeasureString            = dll.NewProc("GdipMeasureString")
	procCreatePath               = dll.NewProc("GdipCreatePath")
	procDeletePath               = dll.NewProc("GdipDeletePath")
	procAddPathString            = dll.NewProc("GdipAddPathString")
	procCreateSolidFill          = dll.NewProc("GdipCreateSolidFill")
	procDeleteBrush              = dll.NewProc("GdipDeleteBrush")
	procCreatePen1               = dll.NewProc("GdipCreatePen1")
	procDeletePen                = dll.NewProc("GdipDeletePen")
	procSetPenLineJoin           = dll.NewProc("GdipSetPenLineJoin")
	procFillPath                 = dll.NewProc("GdipFillPath")
	procDrawPath                 = dll.NewProc("GdipDrawPath")
	procFillRectangle            = dll.NewProc("GdipFillRectangle")
	procTranslateWorldTransform  = dll.NewProc("GdipTranslateWorldTransform")
	procResetWorldTransform      = dll.NewProc("GdipResetWorldTransform")
)

const (
	pixelFormat32bppPARGB = 0x000E200B
	unitPixel             = 2
	smoothingAntiAlias    = 4
	fillModeWinding       = 1
	lineJoinRound         = 2
	matrixOrderAppend     = 1

	fontStyleBold   = 1
	fontStyleItalic = 2

	stringFormatNoWrap                = 0x00001000
	stringFormatMeasureTrailingSpaces = 0x00000800
)

// Status is a GDI+ status code; zero means success.
type Status uintptr

func (s Status) Error() string { return fmt.Sprintf("GDI+ status %d", uintptr(s)) }

func call(proc *windows.LazyProc, args ...uintptr) error {
	r, _, _ := proc.Call(args...)
	if r != 0 {
		return fmt.Errorf("%s: %w", proc.Name, Status(r))
	}
	return nil
}

func f32(v float64) uintptr { return uintptr(math.Float32bits(float32(v))) }

type rectF struct{ X, Y, W, H float32 }

type startupInput struct {
	Version                  uint32
	DebugEventCallback       uintptr
	SuppressBackgroundThread int32
	SuppressExternalCodecs   int32
}

func startup() (uintptr, error) {
	var token uintptr
	input := startupInput{Version: 1}
	if err := call(procStartup, uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&input)), 0); err != nil {
		return 0, err
	}
	return token, nil
}

// toUTF16 converts text for GDI+, NUL-terminated, dropping NUL characters it
// cannot carry (windows.StringToUTF16 would panic on them).
func toUTF16(text string) []uint16 {
	encoded := utf16.Encode([]rune(text))
	out := encoded[:0]
	for _, u := range encoded {
		if u != 0 {
			out = append(out, u)
		}
	}
	return append(out, 0)
}

func argb(r, g, b, a uint8) uintptr {
	return uintptr(uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}
