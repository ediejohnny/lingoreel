//go:build windows

package win32

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Child controls, fonts and colors: the box that corrects a line where the
// video shows it.
var (
	procSendMessageW     = user32.NewProc("SendMessageW")
	procCallWindowProcW  = user32.NewProc("CallWindowProcW")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procSetBkColor       = gdi32.NewProc("SetBkColor")
)

// Messages and styles of the controls.
const (
	WS_CHILD      = 0x40000000
	WS_BORDER     = 0x00800000
	WS_EX_TOPMOST = 0x00000008

	ES_CENTER      = 0x0001
	ES_MULTILINE   = 0x0004
	ES_AUTOVSCROLL = 0x0040
	SS_CENTER      = 0x0001

	WM_SETTEXT        = 0x000C
	WM_GETTEXT        = 0x000D
	WM_GETTEXTLENGTH  = 0x000E
	WM_SETFONT        = 0x0030
	WM_CHAR           = 0x0102
	WM_CTLCOLOREDIT   = 0x0133
	WM_CTLCOLORSTATIC = 0x0138
	EM_SETSEL         = 0x00B1

	VK_SHIFT = 0x10

	gwlpWndProc      = -4
	fwNormal         = 400
	fwBold           = 700
	defaultCharset   = 1
	cleartypeQuality = 5
)

// SendMessage sends a message to a window of this process and returns its
// answer.
func SendMessage(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := procSendMessageW.Call(uintptr(hwnd), uintptr(message), wparam, lparam)
	return r
}

// CreateChild creates a control of a system class ("EDIT", "STATIC") inside
// parent, at bounds in the parent's client pixels.
func CreateChild(class string, style uint32, parent windows.HWND, bounds Rect) (windows.HWND, error) {
	name, err := windows.UTF16PtrFromString(class)
	if err != nil {
		return 0, err
	}
	return CreateTitledWindow(0, WS_CHILD|WS_VISIBLE|style, name, nil, bounds, parent, ModuleHandle())
}

// SetText replaces a window's text.
func SetText(hwnd windows.HWND, text string) error {
	utf16, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return err
	}
	SendMessage(hwnd, WM_SETTEXT, 0, uintptr(unsafe.Pointer(utf16)))
	return nil
}

// Text is a window's text.
func Text(hwnd windows.HWND) string {
	length := SendMessage(hwnd, WM_GETTEXTLENGTH, 0, 0)
	buffer := make([]uint16, length+1)
	n := SendMessage(hwnd, WM_GETTEXT, uintptr(len(buffer)), uintptr(unsafe.Pointer(&buffer[0])))
	return windows.UTF16ToString(buffer[:n])
}

// CreateFont makes a GDI font of a family, pixelHeight tall (the height of
// its characters, as the subtitles measure it); DeleteObject frees it.
func CreateFont(family string, pixelHeight int32, bold, italic bool) (windows.Handle, error) {
	face, err := windows.UTF16PtrFromString(family)
	if err != nil {
		return 0, err
	}
	weight, slant := uintptr(fwNormal), uintptr(0)
	if bold {
		weight = fwBold
	}
	if italic {
		slant = 1
	}
	// A negative height asks for the character height, not the cell's.
	font, _, callError := procCreateFontW.Call(uintptr(-pixelHeight), 0, 0, 0, weight, slant, 0, 0,
		defaultCharset, 0, 0, cleartypeQuality, 0, uintptr(unsafe.Pointer(face)))
	if font == 0 {
		return 0, callErr(callError)
	}
	return windows.Handle(font), nil
}

// DeleteObject frees a font or a brush.
func DeleteObject(object windows.Handle) {
	if object != 0 {
		procDeleteObject.Call(uintptr(object))
	}
}

// RGB is a GDI color.
func RGB(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

// SolidBrush makes a brush of one color; DeleteObject frees it.
func SolidBrush(color uint32) windows.Handle {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return windows.Handle(brush)
}

// SetColors sets the text and background colors a control draws with, in
// answer to WM_CTLCOLOREDIT or WM_CTLCOLORSTATIC (the message's wparam).
func SetColors(dc uintptr, text, background uint32) {
	procSetTextColor.Call(dc, uintptr(text))
	procSetBkColor.Call(dc, uintptr(background))
}

// Subclass gives a window a new procedure and returns the one it had, which
// the new one hands what it leaves alone through CallWindowProc.
func Subclass(hwnd windows.HWND, proc uintptr) uintptr {
	index := int32(gwlpWndProc)
	previous, _, _ := procSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index), proc)
	return previous
}

// CallWindowProc runs a window's previous procedure.
func CallWindowProc(previous uintptr, hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := procCallWindowProcW.Call(previous, uintptr(hwnd), uintptr(message), wparam, lparam)
	return r
}
