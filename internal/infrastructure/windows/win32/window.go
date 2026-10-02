//go:build windows

package win32

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Functions of an ordinary top-level window: the player window the app
// draws the video into, with its mouse, keys and fullscreen.
var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procGetWindowLongPtrW   = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW   = user32.NewProc("SetWindowLongPtrW")
	procGetWindowPlacement  = user32.NewProc("GetWindowPlacement")
	procSetWindowPlacement  = user32.NewProc("SetWindowPlacement")
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procMonitorFromRect     = user32.NewProc("MonitorFromRect")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procTrackMouseEvent     = user32.NewProc("TrackMouseEvent")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procSetCursor           = user32.NewProc("SetCursor")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procEnableWindow        = user32.NewProc("EnableWindow")
	procSetFocus            = user32.NewProc("SetFocus")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procScreenToClient      = user32.NewProc("ScreenToClient")
	procGetAncestor         = user32.NewProc("GetAncestor")
	procExtractIconExW      = shell32.NewProc("ExtractIconExW")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procGetStockObject      = gdi32.NewProc("GetStockObject")
)

// Messages, styles and keys of the player window.
const (
	WM_MOVE           = 0x0003
	WM_SIZE           = 0x0005
	WM_SETFOCUS       = 0x0007
	WM_ACTIVATE       = 0x0006
	WM_SETCURSOR      = 0x0020
	WM_GETMINMAXINFO  = 0x0024
	WM_KILLFOCUS      = 0x0008
	WM_KEYDOWN        = 0x0100
	WM_KEYUP          = 0x0101
	WM_SYSKEYDOWN     = 0x0104
	WM_SYSKEYUP       = 0x0105
	WM_MOUSEMOVE      = 0x0200
	WM_LBUTTONDOWN    = 0x0201
	WM_LBUTTONUP      = 0x0202
	WM_LBUTTONDBLCLK  = 0x0203
	WM_PARENTNOTIFY   = 0x0210
	WM_CAPTURECHANGED = 0x0215
	WM_MOUSELEAVE     = 0x02A3
	WM_DPICHANGED     = 0x02E0
	WM_ERASEBKGND     = 0x0014

	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_CLIPCHILDREN     = 0x02000000
	WS_VISIBLE          = 0x10000000

	CS_DBLCLKS = 0x0008

	GWL_STYLE = -16

	SW_SHOW           = 5
	SW_SHOWNORMAL     = 1
	SW_SHOWNOACTIVATE = 4
	SWP_NOMOVE        = 0x0002
	SWP_NOOWNERZORDER = 0x0200
	SWP_FRAMECHANGED  = 0x0020
	SWP_SHOWWINDOW    = 0x0040

	MONITOR_DEFAULTTONEAREST = 0x00000002

	HTCLIENT = 1

	VK_SPACE  = 0x20
	VK_ESCAPE = 0x1B
	VK_LEFT   = 0x25
	VK_UP     = 0x26
	VK_RIGHT  = 0x27
	VK_DOWN   = 0x28
	VK_F      = 0x46
	VK_M      = 0x4D

	tmeLeave   = 0x00000002
	idcArrow   = 32512
	blackBrush = 4
)

// WindowPlacement is a WINDOWPLACEMENT (without rcDevice, which only the
// Mac version has: SetWindowPlacement refuses any other length).
type WindowPlacement struct {
	Length         uint32
	Flags          uint32
	ShowCmd        uint32
	MinPosition    Point
	MaxPosition    Point
	NormalPosition Rect
}

type trackMouseEvent struct {
	Size      uint32
	Flags     uint32
	Track     windows.HWND
	HoverTime uint32
}

// CreateTitledWindow creates a window with a title, a position and a size;
// owner (or parent, for a child style) may be zero.
func CreateTitledWindow(exStyle, style uint32, className, title *uint16, bounds Rect, owner windows.HWND, instance windows.Handle) (windows.HWND, error) {
	hwnd, _, err := procCreateWindowExW.Call(
		uintptr(exStyle), uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), uintptr(style),
		uintptr(bounds.Left), uintptr(bounds.Top), uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top),
		uintptr(owner), 0, uintptr(instance), 0)
	if hwnd == 0 {
		return 0, callErr(err)
	}
	return windows.HWND(hwnd), nil
}

// WindowRect returns a window's outer rectangle on screen.
func WindowRect(hwnd windows.HWND) (Rect, error) {
	var r Rect
	if ok, _, err := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return Rect{}, callErr(err)
	}
	return r, nil
}

// SetWindowPos moves, sizes and orders a window.
func SetWindowPos(hwnd, after windows.HWND, bounds Rect, flags uint32) error {
	r, _, err := procSetWindowPos.Call(uintptr(hwnd), uintptr(after),
		uintptr(bounds.Left), uintptr(bounds.Top), uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top), uintptr(flags))
	if r == 0 {
		return callErr(err)
	}
	return nil
}

// Style returns a window's style bits.
func Style(hwnd windows.HWND) uint32 {
	index := int32(GWL_STYLE)
	r, _, _ := procGetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index))
	return uint32(r)
}

// SetStyle replaces a window's style bits; SetWindowPos with
// SWP_FRAMECHANGED must follow for the frame to change.
func SetStyle(hwnd windows.HWND, style uint32) {
	index := int32(GWL_STYLE)
	procSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index), uintptr(style))
}

// Placement returns a window's restored position and show state.
func Placement(hwnd windows.HWND) (WindowPlacement, error) {
	p := WindowPlacement{Length: uint32(unsafe.Sizeof(WindowPlacement{}))}
	if ok, _, err := procGetWindowPlacement.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&p))); ok == 0 {
		return WindowPlacement{}, callErr(err)
	}
	return p, nil
}

// SetPlacement restores a placement read by Placement.
func SetPlacement(hwnd windows.HWND, p WindowPlacement) error {
	p.Length = uint32(unsafe.Sizeof(WindowPlacement{}))
	if ok, _, err := procSetWindowPlacement.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&p))); ok == 0 {
		return callErr(err)
	}
	return nil
}

// OnAnyMonitor reports whether a screen rectangle touches a monitor.
func OnAnyMonitor(r Rect) bool {
	const monitorDefaultToNull = 0
	monitor, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&r)), monitorDefaultToNull)
	return monitor != 0
}

// MonitorBounds returns the whole area of the display showing most of a
// window, in the calling thread's coordinates.
func MonitorBounds(hwnd windows.HWND) (Rect, error) {
	monitor, _, _ := procMonitorFromWindow.Call(uintptr(hwnd), MONITOR_DEFAULTTONEAREST)
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, err := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return Rect{}, callErr(err)
	}
	return info.Monitor, nil
}

// SetCapture sends the mouse to a window while a button is held.
func SetCapture(hwnd windows.HWND) {
	procSetCapture.Call(uintptr(hwnd))
}

// ReleaseCapture ends SetCapture.
func ReleaseCapture() {
	procReleaseCapture.Call()
}

// TrackMouseLeave asks for one WM_MOUSELEAVE when the mouse leaves the window.
func TrackMouseLeave(hwnd windows.HWND) {
	event := trackMouseEvent{Size: uint32(unsafe.Sizeof(trackMouseEvent{})), Flags: tmeLeave, Track: hwnd}
	procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&event)))
}

// ArrowCursor returns the system's arrow cursor.
func ArrowCursor() windows.Handle {
	r, _, _ := procLoadCursorW.Call(0, idcArrow)
	return windows.Handle(r)
}

// SetCursor shows a cursor; zero hides it.
func SetCursor(cursor windows.Handle) {
	procSetCursor.Call(uintptr(cursor))
}

// PostMessage posts a message to a window's queue.
func PostMessage(hwnd windows.HWND, message uint32, wparam, lparam uintptr) error {
	if r, _, err := procPostMessageW.Call(uintptr(hwnd), uintptr(message), wparam, lparam); r == 0 {
		return callErr(err)
	}
	return nil
}

// EnableWindow lets a window take input or not; a disabled child lets
// mouse clicks reach its parent.
func EnableWindow(hwnd windows.HWND, enable bool) {
	var flag uintptr
	if enable {
		flag = 1
	}
	procEnableWindow.Call(uintptr(hwnd), flag)
}

// SetFocus gives the keyboard to a window of the calling thread.
func SetFocus(hwnd windows.HWND) {
	procSetFocus.Call(uintptr(hwnd))
}

// SetForegroundWindow brings a window to the front and activates it, when
// Windows allows the calling process to.
func SetForegroundWindow(hwnd windows.HWND) {
	procSetForegroundWindow.Call(uintptr(hwnd))
}

// CursorInClient returns the mouse position in a window's client pixels.
func CursorInClient(hwnd windows.HWND) (Point, error) {
	var p Point
	if ok, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p))); ok == 0 {
		return Point{}, callErr(err)
	}
	if ok, _, err := procScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&p))); ok == 0 {
		return Point{}, callErr(err)
	}
	return p, nil
}

// RootOwner returns the top-level window above a window: itself for a
// top-level window, its owners' root for an owned one.
func RootOwner(hwnd windows.HWND) windows.HWND {
	const gaRootOwner = 3
	r, _, _ := procGetAncestor.Call(uintptr(hwnd), gaRootOwner)
	return windows.HWND(r)
}

// ExecutableIcons returns the first large and small icons of a program
// file, zero when it has none; DestroyIcon frees them.
func ExecutableIcons(path string) (large, small windows.Handle) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}
	procExtractIconExW.Call(uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)
	return large, small
}

// DestroyIcon frees an icon from ExecutableIcons.
func DestroyIcon(icon windows.Handle) {
	if icon != 0 {
		procDestroyIcon.Call(uintptr(icon))
	}
}

var procRtlMoveMemory = kernel32.NewProc("RtlMoveMemory")

// RectAt copies the RECT a message points to, such as the suggested
// bounds of WM_DPICHANGED.
func RectAt(address uintptr) Rect {
	var r Rect
	if address != 0 {
		procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&r)), address, unsafe.Sizeof(r))
	}
	return r
}

// BlackBrush is the system's black brush, for a window background.
func BlackBrush() windows.Handle {
	r, _, _ := procGetStockObject.Call(blackBrush)
	return windows.Handle(r)
}

// PointOf reads the point of a mouse message: two signed 16-bit coordinates.
func PointOf(lparam uintptr) Point {
	return Point{X: int32(int16(lparam & 0xFFFF)), Y: int32(int16(lparam >> 16 & 0xFFFF))}
}
