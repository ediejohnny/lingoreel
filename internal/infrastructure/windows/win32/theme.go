//go:build windows

package win32

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Menus in the dark or in the light mode. Windows has no documented call for
// it: uxtheme's SetPreferredAppMode (ordinal 135) and FlushMenuThemes (136),
// since Windows 10 1903, are what programs with dark menus use. Something in
// this process (the WebView2) already asks for dark menus when Windows' apps
// are dark; asking ourselves keeps them as the app's theme says.
const (
	ordinalSetPreferredAppMode = 135
	ordinalFlushMenuThemes     = 136
	appModeForceDark           = 2
	appModeForceLight          = 3
)

var menuModes struct {
	once                       sync.Once
	setPreferredAppMode, flush uintptr
}

// SetMenusDark makes the process's menus dark or light from now on; false
// when this Windows cannot.
func SetMenusDark(dark bool) bool {
	menuModes.once.Do(func() {
		uxtheme := windows.NewLazySystemDLL("uxtheme.dll")
		if uxtheme.Load() != nil {
			return
		}
		module := windows.Handle(uxtheme.Handle())
		set, setErr := windows.GetProcAddressByOrdinal(module, ordinalSetPreferredAppMode)
		flush, flushErr := windows.GetProcAddressByOrdinal(module, ordinalFlushMenuThemes)
		if setErr == nil && flushErr == nil {
			menuModes.setPreferredAppMode, menuModes.flush = set, flush
		}
	})
	if menuModes.setPreferredAppMode == 0 {
		return false
	}
	mode := uintptr(appModeForceLight)
	if dark {
		mode = appModeForceDark
	}
	syscall.SyscallN(menuModes.setPreferredAppMode, mode)
	syscall.SyscallN(menuModes.flush)
	return true
}

// AppsUseDarkMode tells whether Windows' apps are in the dark mode (Settings
// › Personalization › Colors).
func AppsUseDarkMode() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	light, _, err := key.GetIntegerValue("AppsUseLightTheme")
	return err == nil && light == 0
}

// highContrast is a HIGHCONTRASTW.
type highContrast struct {
	size, flags uint32
	scheme      *uint16
}

// HighContrast tells whether a high contrast theme is on: everything then
// takes the system's colors.
func HighContrast() bool {
	const spiGetHighContrast, hcfHighContrastOn = 0x0042, 0x1
	info := highContrast{size: uint32(unsafe.Sizeof(highContrast{}))}
	ok, _, _ := procSystemParametersInfoW.Call(spiGetHighContrast, uintptr(info.size), uintptr(unsafe.Pointer(&info)), 0)
	return ok != 0 && info.flags&hcfHighContrastOn != 0
}
