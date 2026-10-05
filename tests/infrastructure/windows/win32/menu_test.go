//go:build windows

package win32_test

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// sdkMenuItemInfo is MENUITEMINFOW as the Windows SDK lays it out, written
// here apart from the package's own, so the package is not its own oracle.
type sdkMenuItemInfo struct {
	cbSize        uint32
	fMask         uint32
	fType         uint32
	fState        uint32
	wID           uint32
	hSubMenu      uintptr
	hbmpChecked   uintptr
	hbmpUnchecked uintptr
	dwItemData    uintptr
	dwTypeData    *uint16
	cch           uint32
	hbmpItem      uintptr
}

// The SDK's values of the masks, types and states the test reads.
const (
	sdkMIIM_STATE     = 0x0001
	sdkMIIM_ID        = 0x0002
	sdkMIIM_SUBMENU   = 0x0004
	sdkMIIM_STRING    = 0x0040
	sdkMIIM_BITMAP    = 0x0080
	sdkMIIM_FTYPE     = 0x0100
	sdkMFT_SEPARATOR  = 0x0800
	sdkMFT_RADIOCHECK = 0x0200
	sdkMFS_CHECKED    = 0x0008
	sdkMFS_GRAYED     = 0x0003
)

var procGetMenuItemInfo = windows.NewLazySystemDLL("user32.dll").NewProc("GetMenuItemInfoW")

// sdkItem reads an item the way any Windows program would.
func sdkItem(t *testing.T, menu windows.Handle, position int) (sdkMenuItemInfo, string) {
	t.Helper()
	text := make([]uint16, 128)
	info := sdkMenuItemInfo{
		fMask:      sdkMIIM_STATE | sdkMIIM_ID | sdkMIIM_SUBMENU | sdkMIIM_STRING | sdkMIIM_BITMAP | sdkMIIM_FTYPE,
		dwTypeData: &text[0], cch: uint32(len(text)),
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if ok, _, err := procGetMenuItemInfo.Call(uintptr(menu), uintptr(position), 1, uintptr(unsafe.Pointer(&info))); ok == 0 {
		t.Fatalf("GetMenuItemInfoW(%d): %v", position, err)
	}
	return info, windows.UTF16ToString(text)
}

// The items go into a real menu with their text and key, marks, states,
// submenu and picture, as Windows itself reads them back.
func TestMenuItemsReachWindowsAsBuilt(t *testing.T) {
	t.Parallel()
	if size := unsafe.Sizeof(sdkMenuItemInfo{}); size != 80 {
		t.Fatalf("the SDK's MENUITEMINFOW is 80 bytes on amd64, the test's is %d", size)
	}
	menu, err := win32.CreatePopupMenu()
	if err != nil {
		t.Fatal(err)
	}
	defer win32.DestroyMenu(menu)
	sub, err := win32.CreatePopupMenu()
	if err != nil {
		t.Fatal(err)
	}
	icon, err := win32.NewDIB(16, 16)
	if err != nil {
		t.Fatal(err)
	}
	bitmap := icon.Release()
	defer win32.DeleteBitmap(bitmap)
	items := []win32.MenuItem{
		{Text: "Mudo\tM", ID: 1, Checked: true},
		{Separator: true},
		{Text: "1,25×", ID: 2, Radio: true, Checked: true},
		{Text: "Quadro anterior\t,", ID: 3, Disabled: true},
		{Text: "Legendas", SubMenu: sub},
		{Text: "Fechar\tAlt+F4", ID: 4, Bitmap: bitmap},
	}
	for _, item := range items {
		if err := win32.AppendMenuItem(menu, item); err != nil {
			t.Fatal(err)
		}
	}
	if got := win32.MenuItemCount(menu); got != len(items) {
		t.Fatalf("%d items in the menu, want %d", got, len(items))
	}
	for i, want := range []struct {
		text              string
		id                uint32
		separator, radio  bool
		checked, disabled bool
		sub               bool
		bitmap            uintptr
	}{
		{text: "Mudo\tM", id: 1, checked: true},
		{separator: true},
		{text: "1,25×", id: 2, radio: true, checked: true},
		{text: "Quadro anterior\t,", id: 3, disabled: true},
		{text: "Legendas", sub: true},
		{text: "Fechar\tAlt+F4", id: 4, bitmap: uintptr(bitmap)},
	} {
		info, text := sdkItem(t, menu, i)
		got := struct {
			text              string
			id                uint32
			separator, radio  bool
			checked, disabled bool
			sub               bool
			bitmap            uintptr
		}{
			text: text, id: info.wID, separator: info.fType&sdkMFT_SEPARATOR != 0, radio: info.fType&sdkMFT_RADIOCHECK != 0,
			checked: info.fState&sdkMFS_CHECKED != 0, disabled: info.fState&sdkMFS_GRAYED == sdkMFS_GRAYED,
			sub: info.hSubMenu == uintptr(sub), bitmap: info.hbmpItem,
		}
		if want.separator {
			// Windows itself grays a separator.
			got.text, got.disabled = "", false
		}
		if got != want {
			t.Errorf("item %d = %+v, want %+v", i, got, want)
		}
	}
	// The package reads its items back the same way.
	if back, err := win32.MenuItemAt(menu, 5); err != nil || back.Text != "Fechar\tAlt+F4" || back.Bitmap != bitmap {
		t.Fatalf("MenuItemAt(5) = %+v, %v", back, err)
	}
	win32.DestroyMenu(menu)
	if win32.IsMenu(menu) || win32.IsMenu(sub) {
		t.Fatal("the menu or its submenu outlived DestroyMenu")
	}
}

// A released bitmap is free of its device context: it is a 32-bit bitmap
// that another DC can take, until it is freed.
func TestAReleasedBitmapCanBeUsedElsewhere(t *testing.T) {
	t.Parallel()
	dib, err := win32.NewDIB(20, 20)
	if err != nil {
		t.Fatal(err)
	}
	bitmap := dib.Release()
	if dib.DC != 0 || dib.Pixels != nil {
		t.Fatal("the DIB still holds its DC after the release")
	}
	if width, height, bits, ok := win32.BitmapBits(bitmap); !ok || width != 20 || height != 20 || bits != 32 {
		t.Fatalf("released bitmap = %dx%d, %d bits, %v; want 20x20 at 32 bits", width, height, bits, ok)
	}
	gdi := windows.NewLazySystemDLL("gdi32.dll")
	dc, _, _ := gdi.NewProc("CreateCompatibleDC").Call(0)
	defer gdi.NewProc("DeleteDC").Call(dc)
	previous, _, _ := gdi.NewProc("SelectObject").Call(dc, uintptr(bitmap))
	if previous == 0 {
		t.Fatal("a new DC could not take the released bitmap: it is still selected elsewhere")
	}
	gdi.NewProc("SelectObject").Call(dc, previous)
	win32.DeleteBitmap(bitmap)
	if _, _, _, ok := win32.BitmapBits(bitmap); ok {
		t.Fatal("the bitmap is still there after DeleteBitmap")
	}
}
