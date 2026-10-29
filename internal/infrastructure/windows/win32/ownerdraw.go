//go:build windows

package win32

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	msimg32 = windows.NewLazySystemDLL("msimg32.dll")

	procAlphaBlend                 = msimg32.NewProc("AlphaBlend")
	procBitBlt                     = gdi32.NewProc("BitBlt")
	procExcludeClipRect            = gdi32.NewProc("ExcludeClipRect")
	procSystemParametersInfoForDpi = user32.NewProc("SystemParametersInfoForDpi")
)

// The messages a window gets for the menu items it draws, and what
// DRAWITEMSTRUCT says of the item: a menu's, highlighted, disabled, checked.
const (
	WM_DRAWITEM    = 0x002B
	WM_MEASUREITEM = 0x002C

	ODT_MENU     = 1
	ODS_SELECTED = 0x0001
	ODS_GRAYED   = 0x0002
	ODS_DISABLED = 0x0004
	ODS_CHECKED  = 0x0008

	srcCopy             = 0x00CC0020
	spiGetNonClientMets = 0x0029
	// blendSourceAlpha is a BLENDFUNCTION that lays premultiplied pixels
	// over the target: AC_SRC_OVER, whole opacity, AC_SRC_ALPHA.
	blendSourceAlpha = 0x01FF0000
)

// DrawItem is a DRAWITEMSTRUCT: which item to draw, where, on what.
type DrawItem struct {
	CtlType, CtlID, ItemID, ItemAction, ItemState uint32
	Window                                        windows.Handle
	DC                                            windows.Handle
	Bounds                                        Rect
	ItemData                                      uintptr
}

// MeasureItem is a MEASUREITEMSTRUCT: the item asked about, and the size
// the window answers.
type MeasureItem struct {
	CtlType, CtlID, ItemID, ItemWidth, ItemHeight uint32
	ItemData                                      uintptr
}

// DrawItemAt copies the DRAWITEMSTRUCT a WM_DRAWITEM points to.
func DrawItemAt(address uintptr) DrawItem {
	var item DrawItem
	if address != 0 {
		procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&item)), address, unsafe.Sizeof(item))
	}
	return item
}

// MeasureItemAt copies the MEASUREITEMSTRUCT a WM_MEASUREITEM points to.
func MeasureItemAt(address uintptr) MeasureItem {
	var item MeasureItem
	if address != 0 {
		procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&item)), address, unsafe.Sizeof(item))
	}
	return item
}

// AnswerMeasure writes the size of an item back into the MEASUREITEMSTRUCT
// a WM_MEASUREITEM points to.
func AnswerMeasure(address uintptr, item MeasureItem) {
	if address != 0 {
		procRtlMoveMemory.Call(address, uintptr(unsafe.Pointer(&item)), unsafe.Sizeof(item))
	}
}

// CopyDIB puts a whole bitmap on a device context at a point.
func CopyDIB(dc windows.Handle, x, y int32, from *DIB) {
	if from == nil || from.DC == 0 {
		return
	}
	procBitBlt.Call(uintptr(dc), uintptr(x), uintptr(y), uintptr(from.Width), uintptr(from.Height), uintptr(from.DC), 0, 0, srcCopy)
}

// BlendBitmap lays a square 32-bit premultiplied bitmap of a side over a
// bitmap at a point, through its transparency.
func BlendBitmap(target *DIB, bitmap windows.Handle, x, y int32, side int) {
	if target == nil || target.DC == 0 || bitmap == 0 {
		return
	}
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return
	}
	defer procDeleteDC.Call(dc)
	old, _, _ := procSelectObject.Call(dc, uintptr(bitmap))
	defer procSelectObject.Call(dc, old)
	procAlphaBlend.Call(uintptr(target.DC), uintptr(x), uintptr(y), uintptr(side), uintptr(side),
		dc, 0, 0, uintptr(side), uintptr(side), blendSourceAlpha)
}

// ExcludeClip keeps whatever is drawn on a device context afterwards out of
// a rectangle.
func ExcludeClip(dc windows.Handle, r Rect) {
	procExcludeClipRect.Call(uintptr(dc), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom))
}

// logFont is a LOGFONTW.
type logFont struct {
	Height, Width, Escapement, Orientation, Weight int32
	Italic, Underline, StrikeOut, CharSet          byte
	OutPrecision, ClipPrecision, Quality, Pitch    byte
	FaceName                                       [32]uint16
}

// nonClientMetrics is a NONCLIENTMETRICSW.
type nonClientMetrics struct {
	Size                                   uint32
	BorderWidth, ScrollWidth, ScrollHeight int32
	CaptionWidth, CaptionHeight            int32
	CaptionFont                            logFont
	SmallCaptionWidth, SmallCaptionHeight  int32
	SmallCaptionFont                       logFont
	MenuWidth, MenuHeight                  int32
	MenuFont, StatusFont, MessageFont      logFont
	PaddedBorderWidth                      int32
}

// MenuFont is the typeface and the height, in pixels at a DPI, of the
// font Windows writes its menus in; false when Windows cannot tell.
func MenuFont(dpi int) (face string, size float64, ok bool) {
	var metrics nonClientMetrics
	metrics.Size = uint32(unsafe.Sizeof(metrics))
	if done, _, _ := procSystemParametersInfoForDpi.Call(spiGetNonClientMets, uintptr(metrics.Size), uintptr(unsafe.Pointer(&metrics)), 0, uintptr(dpi)); done == 0 {
		return "", 0, false
	}
	height := metrics.MenuFont.Height
	if height < 0 {
		height = -height
	}
	face = windows.UTF16ToString(metrics.MenuFont.FaceName[:])
	return face, float64(height), face != "" && height > 0
}
