//go:build windows

// Package win32 binds the few user32 and gdi32 functions the overlay needs
// that golang.org/x/sys/windows does not provide.
package win32

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"lingoreel/internal/domain/layout"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW            = user32.NewProc("PostThreadMessageW")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procUpdateLayeredWindow           = user32.NewProc("UpdateLayeredWindow")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procSystemParametersInfoW         = user32.NewProc("SystemParametersInfoW")
	procGetClientRect                 = user32.NewProc("GetClientRect")
	procClientToScreen                = user32.NewProc("ClientToScreen")
	procIsIconic                      = user32.NewProc("IsIconic")
	procSetWinEventHook               = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent                = user32.NewProc("UnhookWinEvent")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procSetThreadDpiAwarenessContext  = user32.NewProc("SetThreadDpiAwarenessContext")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetThreadDpiAwarenessContext  = user32.NewProc("GetThreadDpiAwarenessContext")
	procAreDpiAwarenessContextsEqual  = user32.NewProc("AreDpiAwarenessContextsEqual")
	procPeekMessageW                  = user32.NewProc("PeekMessageW")
	procSetTimer                      = user32.NewProc("SetTimer")
	procKillTimer                     = user32.NewProc("KillTimer")
	procRegisterHotKey                = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey              = user32.NewProc("UnregisterHotKey")
	procGetForegroundWindow           = user32.NewProc("GetForegroundWindow")
	procGetAsyncKeyState              = user32.NewProc("GetAsyncKeyState")
	procCreateCompatibleDC            = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection              = gdi32.NewProc("CreateDIBSection")
	procSelectObject                  = gdi32.NewProc("SelectObject")
	procDeleteObject                  = gdi32.NewProc("DeleteObject")
	procDeleteDC                      = gdi32.NewProc("DeleteDC")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

// Window messages and constants used by the overlay.
const (
	WM_DESTROY       = 0x0002
	WM_CLOSE         = 0x0010
	WM_NCHITTEST     = 0x0084
	WM_MOUSEACTIVATE = 0x0021
	WM_HOTKEY        = 0x0312
	WM_TIMER         = 0x0113
	WM_APP           = 0x8000

	// HWND_MESSAGE is the parent of message-only windows: never shown, they
	// only receive messages.
	HWND_MESSAGE = windows.HWND(^uintptr(2)) // -3

	MOD_ALT     = 0x0001
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004

	VK_OEM_COMMA  = 0xBC
	VK_OEM_PERIOD = 0xBE

	HTTRANSPARENT = ^uintptr(0) // -1
	MA_NOACTIVATE = 3

	WS_POPUP = 0x80000000

	WS_EX_LAYERED     = 0x00080000
	WS_EX_TRANSPARENT = 0x00000020
	WS_EX_NOACTIVATE  = 0x08000000
	WS_EX_TOOLWINDOW  = 0x00000080

	SW_HIDE   = 0
	SW_SHOWNA = 8

	SWP_NOSIZE     = 0x0001
	SWP_NOZORDER   = 0x0004
	SWP_NOACTIVATE = 0x0010

	ULW_ALPHA    = 0x00000002
	AC_SRC_OVER  = 0x00
	AC_SRC_ALPHA = 0x01

	PM_NOREMOVE = 0x0000
)

// WinEvent constants.
const (
	EVENT_SYSTEM_FOREGROUND     = 0x0003
	EVENT_SYSTEM_MINIMIZESTART  = 0x0016
	EVENT_SYSTEM_MINIMIZEEND    = 0x0017
	EVENT_OBJECT_DESTROY        = 0x8001
	EVENT_OBJECT_SHOW           = 0x8002
	EVENT_OBJECT_HIDE           = 0x8003
	EVENT_OBJECT_LOCATIONCHANGE = 0x800B

	WINEVENT_OUTOFCONTEXT   = 0x0000
	WINEVENT_SKIPOWNPROCESS = 0x0002

	OBJID_WINDOW = 0
	CHILDID_SELF = 0
)

// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 makes a thread see physical
// pixels on every monitor.
const DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = ^uintptr(3) // -4

// ErrCall wraps a Win32 call that failed without an error code.
var ErrCall = errors.New("win32 call failed")

// Point is a POINT.
type Point struct{ X, Y int32 }

// Size is a SIZE.
type Size struct{ CX, CY int32 }

// Rect is a RECT.
type Rect struct{ Left, Top, Right, Bottom int32 }

// Msg is a MSG.
type Msg struct {
	HWnd    windows.HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      Point
	private uint32
}

// WndClassEx is a WNDCLASSEXW.
type WndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

func callErr(err error) error {
	if errno, ok := err.(syscall.Errno); ok && errno != 0 {
		return errno
	}
	return ErrCall
}

// ModuleHandle returns the handle of the running executable.
func ModuleHandle() windows.Handle {
	h, _, _ := procGetModuleHandleW.Call(0)
	return windows.Handle(h)
}

// RegisterClass registers a window class and returns its atom.
func RegisterClass(class *WndClassEx) (uint16, error) {
	class.Size = uint32(unsafe.Sizeof(*class))
	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(class)))
	if atom == 0 {
		return 0, callErr(err)
	}
	return uint16(atom), nil
}

// CreateWindow creates a window; owner may be zero.
func CreateWindow(exStyle, style uint32, className *uint16, owner windows.HWND, instance windows.Handle) (windows.HWND, error) {
	hwnd, _, err := procCreateWindowExW.Call(
		uintptr(exStyle), uintptr(unsafe.Pointer(className)), 0, uintptr(style),
		0, 0, 0, 0, uintptr(owner), 0, uintptr(instance), 0)
	if hwnd == 0 {
		return 0, callErr(err)
	}
	return windows.HWND(hwnd), nil
}

// DefWindowProc runs the default window procedure.
func DefWindowProc(hwnd windows.HWND, msg uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

// DestroyWindow destroys a window created by this thread.
func DestroyWindow(hwnd windows.HWND) error {
	if r, _, err := procDestroyWindow.Call(uintptr(hwnd)); r == 0 {
		return callErr(err)
	}
	return nil
}

// GetMessage waits for the next message; it returns false on WM_QUIT.
func GetMessage(msg *Msg) (bool, error) {
	r, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(msg)), 0, 0, 0)
	switch int32(r) {
	case -1:
		return false, callErr(err)
	case 0:
		return false, nil
	}
	return true, nil
}

// PeekMessage checks for a message without removing it; calling it once
// creates the thread's message queue.
func PeekMessage(msg *Msg) bool {
	r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(msg)), 0, 0, 0, PM_NOREMOVE)
	return r != 0
}

// DispatchMessage translates and dispatches a message.
func DispatchMessage(msg *Msg) {
	procTranslateMessage.Call(uintptr(unsafe.Pointer(msg)))
	procDispatchMessageW.Call(uintptr(unsafe.Pointer(msg)))
}

// PostThreadMessage posts a message to a thread's queue.
func PostThreadMessage(threadID uint32, msg uint32) error {
	if r, _, err := procPostThreadMessageW.Call(uintptr(threadID), uintptr(msg), 0, 0); r == 0 {
		return callErr(err)
	}
	return nil
}

// PostQuitMessage ends the calling thread's message loop.
func PostQuitMessage() {
	procPostQuitMessage.Call(0)
}

// ShowWindow shows or hides a window without activating it.
func ShowWindow(hwnd windows.HWND, command int32) {
	procShowWindow.Call(uintptr(hwnd), uintptr(command))
}

// ClientRectOnScreen returns the client area of a window in screen pixels.
func ClientRectOnScreen(hwnd windows.HWND) (Rect, error) {
	var r Rect
	if ok, _, err := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return Rect{}, callErr(err)
	}
	origin := Point{}
	if ok, _, err := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&origin))); ok == 0 {
		return Rect{}, callErr(err)
	}
	return Rect{Left: origin.X, Top: origin.Y, Right: origin.X + r.Right, Bottom: origin.Y + r.Bottom}, nil
}

// MoveWindow moves a window without resizing, activating or reordering it.
func MoveWindow(hwnd windows.HWND, x, y int32) error {
	const flags = SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE
	if r, _, err := procSetWindowPos.Call(uintptr(hwnd), 0, uintptr(x), uintptr(y), 0, 0, flags); r == 0 {
		return callErr(err)
	}
	return nil
}

// WorkArea returns the primary monitor's area not covered by the taskbar.
func WorkArea() (Rect, error) {
	const spiGetWorkArea = 0x0030
	var r Rect
	if ok, _, err := procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0); ok == 0 {
		return Rect{}, callErr(err)
	}
	return r, nil
}

// IsIconic reports whether a window is minimized.
func IsIconic(hwnd windows.HWND) bool {
	r, _, _ := procIsIconic.Call(uintptr(hwnd))
	return r != 0
}

// DpiForWindow returns the DPI of the monitor showing a window, or the base
// DPI (100% scaling) when it cannot tell.
func DpiForWindow(hwnd windows.HWND) int {
	if procGetDpiForWindow.Find() != nil {
		return layout.BaseDPI
	}
	r, _, _ := procGetDpiForWindow.Call(uintptr(hwnd))
	if r == 0 {
		return layout.BaseDPI
	}
	return int(r)
}

// UsePerMonitorDPI makes the calling thread per-monitor DPI aware (v2), so it
// works in physical pixels. It is a no-op before Windows 10 1703.
func UsePerMonitorDPI() {
	if procSetThreadDpiAwarenessContext.Find() == nil {
		procSetThreadDpiAwarenessContext.Call(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
	}
}

// UsePerMonitorDPIForProcess makes every thread of the process per-monitor
// DPI aware (v2) unless it chose otherwise, so screen sizes such as the work
// area come in physical pixels, the unit the players use. Call it before any
// window exists; a process whose awareness was already set (by a manifest)
// keeps it. It is a no-op before Windows 10 1703.
func UsePerMonitorDPIForProcess() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		procSetProcessDpiAwarenessContext.Call(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
	}
}

// ThreadUsesPerMonitorDPI reports whether the calling thread is per-monitor
// DPI aware (v2).
func ThreadUsesPerMonitorDPI() bool {
	if procGetThreadDpiAwarenessContext.Find() != nil || procAreDpiAwarenessContextsEqual.Find() != nil {
		return false
	}
	current, _, _ := procGetThreadDpiAwarenessContext.Call()
	equal, _, _ := procAreDpiAwarenessContextsEqual.Call(current, DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
	return equal != 0
}

// SetWinEventHook installs an out-of-context hook for one process; the
// callback runs on the calling thread's message loop.
func SetWinEventHook(eventMin, eventMax uint32, processID uint32, callback uintptr) (windows.Handle, error) {
	h, _, err := procSetWinEventHook.Call(uintptr(eventMin), uintptr(eventMax), 0, callback,
		uintptr(processID), 0, WINEVENT_OUTOFCONTEXT|WINEVENT_SKIPOWNPROCESS)
	if h == 0 {
		return 0, callErr(err)
	}
	return windows.Handle(h), nil
}

// UnhookWinEvent removes a hook.
func UnhookWinEvent(hook windows.Handle) {
	procUnhookWinEvent.Call(uintptr(hook))
}

// SetTimer starts or restarts a window timer that posts WM_TIMER with the id.
func SetTimer(hwnd windows.HWND, id uintptr, milliseconds uint32) error {
	if r, _, err := procSetTimer.Call(uintptr(hwnd), id, uintptr(milliseconds), 0); r == 0 {
		return callErr(err)
	}
	return nil
}

// KillTimer stops a window timer; stopping one that is not running is harmless.
func KillTimer(hwnd windows.HWND, id uintptr) {
	procKillTimer.Call(uintptr(hwnd), id)
}

// RegisterHotKey makes a key combination post WM_HOTKEY with the id to the
// window, and keeps it from reaching other programs, until unregistered.
func RegisterHotKey(hwnd windows.HWND, id int32, modifiers, key uint32) error {
	if r, _, err := procRegisterHotKey.Call(uintptr(hwnd), uintptr(id), uintptr(modifiers), uintptr(key)); r == 0 {
		return callErr(err)
	}
	return nil
}

// UnregisterHotKey releases a key combination registered by the window.
func UnregisterHotKey(hwnd windows.HWND, id int32) {
	procUnregisterHotKey.Call(uintptr(hwnd), uintptr(id))
}

// ForegroundWindow returns the window the user is working in, or zero.
func ForegroundWindow() windows.HWND {
	r, _, _ := procGetForegroundWindow.Call()
	return windows.HWND(r)
}

// KeyDown reports whether a key is physically down now.
func KeyDown(key uint32) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(key))
	return r&0x8000 != 0
}

// UpdateLayered shows a premultiplied 32-bit bitmap as the whole content of a
// layered window at a screen position.
func UpdateLayered(hwnd windows.HWND, position Point, size Size, source windows.Handle) error {
	origin := Point{}
	blend := blendFunction{BlendOp: AC_SRC_OVER, SourceConstantAlpha: 255, AlphaFormat: AC_SRC_ALPHA}
	r, _, err := procUpdateLayeredWindow.Call(uintptr(hwnd), 0,
		uintptr(unsafe.Pointer(&position)), uintptr(unsafe.Pointer(&size)),
		uintptr(source), uintptr(unsafe.Pointer(&origin)), 0,
		uintptr(unsafe.Pointer(&blend)), ULW_ALPHA)
	if r == 0 {
		return callErr(err)
	}
	return nil
}
