//go:build windows

package win32

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

var (
	shcore = windows.NewLazySystemDLL("shcore.dll")

	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procGetDpiForMonitor    = shcore.NewProc("GetDpiForMonitor")
)

const (
	monitorInfoPrimary = 0x00000001
	mdtEffectiveDPI    = 0
)

// Monitor describes one display.
type Monitor struct {
	Bounds   Rect
	WorkArea Rect
	DPI      int
	Primary  bool
}

type monitorInfo struct {
	Size    uint32
	Monitor Rect
	Work    Rect
	Flags   uint32
}

// monitors collects displays during one EnumDisplayMonitors call. Callbacks
// made with syscall.NewCallback are never freed, so one serves every call,
// serialized by the mutex.
var monitors struct {
	sync.Mutex
	found []Monitor
}

var monitorProc = syscall.NewCallback(func(handle windows.Handle, _ windows.Handle, _ *Rect, _ uintptr) uintptr {
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := procGetMonitorInfoW.Call(uintptr(handle), uintptr(unsafe.Pointer(&info))); ok == 0 {
		return 1 // skip it and keep enumerating
	}
	monitors.found = append(monitors.found, Monitor{
		Bounds: info.Monitor, WorkArea: info.Work, DPI: monitorDPI(handle),
		Primary: info.Flags&monitorInfoPrimary != 0,
	})
	return 1
})

// Monitors lists the displays with their bounds, work areas and scaling, in
// the coordinates of the calling thread's DPI awareness.
func Monitors() ([]Monitor, error) {
	monitors.Lock()
	defer monitors.Unlock()
	monitors.found = nil
	if ok, _, err := procEnumDisplayMonitors.Call(0, 0, monitorProc, 0); ok == 0 {
		return nil, callErr(err)
	}
	return append([]Monitor(nil), monitors.found...), nil
}

// monitorDPI returns a display's effective DPI, or the base DPI when the
// system cannot tell (before Windows 8.1).
func monitorDPI(handle windows.Handle) int {
	if procGetDpiForMonitor.Find() != nil {
		return layout.BaseDPI
	}
	var x, y uint32
	if r, _, _ := procGetDpiForMonitor.Call(uintptr(handle), mdtEffectiveDPI, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y))); r != 0 || x == 0 {
		return layout.BaseDPI
	}
	return int(x)
}
