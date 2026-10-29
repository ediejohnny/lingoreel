//go:build windows

package win32

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

var (
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procInsertMenuItemW        = user32.NewProc("InsertMenuItemW")
	procGetMenuItemInfoW       = user32.NewProc("GetMenuItemInfoW")
	procGetMenuItemCount       = user32.NewProc("GetMenuItemCount")
	procSetMenuInfo            = user32.NewProc("SetMenuInfo")
	procTrackPopupMenuEx       = user32.NewProc("TrackPopupMenuEx")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procEndMenu                = user32.NewProc("EndMenu")
	procIsMenu                 = user32.NewProc("IsMenu")
	procGetSystemMetricsForDpi = user32.NewProc("GetSystemMetricsForDpi")
	procGetSysColor            = user32.NewProc("GetSysColor")
	procGetObjectW             = gdi32.NewProc("GetObjectW")
)

// Menu item masks, types and states (MENUITEMINFOW), and the menu style
// that leaves room for either a check mark or a bitmap, not both.
const (
	miimState   = 0x0001
	miimID      = 0x0002
	miimSubmenu = 0x0004
	miimData    = 0x0020
	miimString  = 0x0040
	miimBitmap  = 0x0080
	miimFType   = 0x0100

	mftSeparator  = 0x0800
	mftRadioCheck = 0x0200
	mftOwnerDraw  = 0x0100
	mfsChecked    = 0x0008
	mfsDisabled   = 0x0003

	mimBackground = 0x0002
	mimStyle      = 0x0010
	mnsCheckOrBmp = 0x04000000

	tpmRightButton = 0x0002
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100

	smCxSmIcon = 49
	// smallIconBase is a small icon's side at 100% scaling.
	smallIconBase = 16

	// COLOR_MENUTEXT and COLOR_GRAYTEXT: the system's colors for a menu's
	// text, enabled and not.
	COLOR_MENUTEXT = 7
	COLOR_GRAYTEXT = 17
)

// menuItemInfo is a MENUITEMINFOW: 80 bytes on amd64, as the SDK lays it out.
type menuItemInfo struct {
	Size      uint32
	Mask      uint32
	Type      uint32
	State     uint32
	ID        uint32
	SubMenu   windows.Handle
	Checked   windows.Handle
	Unchecked windows.Handle
	ItemData  uintptr
	TypeData  *uint16
	Cch       uint32
	Bitmap    windows.Handle
}

// menuInfo is a MENUINFO.
type menuInfo struct {
	Size          uint32
	Mask          uint32
	Style         uint32
	MaxHeight     uint32
	Background    windows.Handle
	ContextHelpID uint32
	MenuData      uintptr
}

// bitmapObject is a BITMAP, as GetObject fills it.
type bitmapObject struct {
	Type       int32
	Width      int32
	Height     int32
	WidthBytes int32
	Planes     uint16
	BitsPixel  uint16
	Bits       uintptr
}

// MenuItem is one item of a popup menu.
type MenuItem struct {
	// Text is the item's text, a tab and its key on the right ("Mudo\tM").
	Text string
	// ID is what TrackPopupMenu returns when the item is chosen; zero for a
	// submenu or a separator.
	ID                        uint32
	Separator, Radio, Checked bool
	Disabled                  bool
	SubMenu                   windows.Handle
	Bitmap                    windows.Handle
	// OwnerDraw leaves the item's drawing to the window the menu shows for,
	// through WM_MEASUREITEM and WM_DRAWITEM, which carry Data; the text
	// stays the item's name for screen readers.
	OwnerDraw bool
	Data      uintptr
}

// CreatePopupMenu makes an empty popup menu that takes check marks or
// bitmaps on its items (MNS_CHECKORBMP); DestroyMenu frees it.
func CreatePopupMenu() (windows.Handle, error) {
	menu, _, err := procCreatePopupMenu.Call()
	if menu == 0 {
		return 0, fmt.Errorf("CreatePopupMenu: %w", callErr(err))
	}
	info := menuInfo{Mask: mimStyle, Style: mnsCheckOrBmp}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, err := procSetMenuInfo.Call(menu, uintptr(unsafe.Pointer(&info))); ok == 0 {
		procDestroyMenu.Call(menu)
		return 0, fmt.Errorf("SetMenuInfo: %w", callErr(err))
	}
	return windows.Handle(menu), nil
}

// SetMenuBackground paints a menu's background with a brush, which must
// outlive the menu.
func SetMenuBackground(menu, brush windows.Handle) error {
	info := menuInfo{Mask: mimBackground, Background: brush}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, err := procSetMenuInfo.Call(uintptr(menu), uintptr(unsafe.Pointer(&info))); ok == 0 {
		return fmt.Errorf("SetMenuInfo: %w", callErr(err))
	}
	return nil
}

// AppendMenuItem adds an item at the end of a menu.
func AppendMenuItem(menu windows.Handle, item MenuItem) error {
	info := menuItemInfo{Mask: miimFType | miimState}
	info.Size = uint32(unsafe.Sizeof(info))
	if item.OwnerDraw {
		info.Type |= mftOwnerDraw
		info.Mask |= miimData
		info.ItemData = item.Data
	}
	var text []uint16
	switch {
	case item.Separator:
		info.Type |= mftSeparator
	default:
		var err error
		if text, err = windows.UTF16FromString(item.Text); err != nil {
			return fmt.Errorf("menu text: %w", err)
		}
		info.Mask |= miimString | miimID
		info.TypeData, info.Cch, info.ID = &text[0], uint32(len(text)-1), item.ID
		if item.Radio {
			info.Type |= mftRadioCheck
		}
		if item.Checked {
			info.State |= mfsChecked
		}
		if item.Disabled {
			info.State |= mfsDisabled
		}
		if item.SubMenu != 0 {
			info.Mask |= miimSubmenu
			info.SubMenu = item.SubMenu
		}
		if item.Bitmap != 0 {
			info.Mask |= miimBitmap
			info.Bitmap = item.Bitmap
		}
	}
	count, _, _ := procGetMenuItemCount.Call(uintptr(menu))
	ok, _, err := procInsertMenuItemW.Call(uintptr(menu), count, 1, uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(text)
	if ok == 0 {
		return fmt.Errorf("InsertMenuItemW: %w", callErr(err))
	}
	return nil
}

// MenuItemCount is how many items a menu has; -1 for no menu.
func MenuItemCount(menu windows.Handle) int {
	count, _, _ := procGetMenuItemCount.Call(uintptr(menu))
	return int(int32(count))
}

// MenuItemAt reads an item of a menu back, by its position.
func MenuItemAt(menu windows.Handle, position int) (MenuItem, error) {
	info := menuItemInfo{Mask: miimFType | miimState | miimID | miimSubmenu | miimString | miimBitmap | miimData}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, err := procGetMenuItemInfoW.Call(uintptr(menu), uintptr(position), 1, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return MenuItem{}, fmt.Errorf("GetMenuItemInfoW: %w", callErr(err))
	}
	item := MenuItem{
		ID: info.ID, Separator: info.Type&mftSeparator != 0, Radio: info.Type&mftRadioCheck != 0,
		Checked: info.State&mfsChecked != 0, Disabled: info.State&mfsDisabled != 0,
		SubMenu: info.SubMenu, Bitmap: info.Bitmap,
		OwnerDraw: info.Type&mftOwnerDraw != 0, Data: info.ItemData,
	}
	if item.Separator {
		return item, nil
	}
	text := make([]uint16, info.Cch+1)
	info.TypeData, info.Cch = &text[0], uint32(len(text))
	if ok, _, err := procGetMenuItemInfoW.Call(uintptr(menu), uintptr(position), 1, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return MenuItem{}, fmt.Errorf("GetMenuItemInfoW: %w", callErr(err))
	}
	item.Text = windows.UTF16ToString(text)
	return item, nil
}

// TrackPopupMenu shows a menu at a point on screen, owned by a window, and
// returns the ID of the item chosen, zero when none was. It runs a modal
// loop on the calling thread until the menu closes.
func TrackPopupMenu(menu windows.Handle, owner windows.HWND, x, y int32) uint32 {
	chosen, _, _ := procTrackPopupMenuEx.Call(uintptr(menu), tpmReturnCmd|tpmRightButton|tpmNoNotify,
		uintptr(x), uintptr(y), uintptr(owner), 0)
	return uint32(chosen)
}

// EndMenu closes the menu shown by this thread, if any.
func EndMenu() {
	procEndMenu.Call()
}

// DestroyMenu frees a menu and its submenus.
func DestroyMenu(menu windows.Handle) {
	if menu != 0 {
		procDestroyMenu.Call(uintptr(menu))
	}
}

// IsMenu tells whether a handle is a menu that still exists.
func IsMenu(menu windows.Handle) bool {
	ok, _, _ := procIsMenu.Call(uintptr(menu))
	return ok != 0
}

// SmallIconSize is the side of a small icon, as menus show them, at a DPI.
func SmallIconSize(dpi int) int {
	size, _, _ := procGetSystemMetricsForDpi.Call(smCxSmIcon, uintptr(dpi))
	if size == 0 {
		return smallIconBase * dpi / layout.BaseDPI
	}
	return int(size)
}

// SysColor is one of the system's colors (COLOR_MENUTEXT…), as red, green
// and blue.
func SysColor(index int) (r, g, b byte) {
	color, _, _ := procGetSysColor.Call(uintptr(index))
	return byte(color), byte(color >> 8), byte(color >> 16)
}

// BitmapBits is a bitmap's width, height and bits per pixel; false when
// the handle is no bitmap (freed, for one).
func BitmapBits(bitmap windows.Handle) (width, height, bits int, ok bool) {
	var object bitmapObject
	size, _, _ := procGetObjectW.Call(uintptr(bitmap), unsafe.Sizeof(object), uintptr(unsafe.Pointer(&object)))
	if size == 0 {
		return 0, 0, 0, false
	}
	return int(object.Width), int(object.Height), int(object.BitsPixel), true
}

// DeleteBitmap frees a bitmap a DIB released.
func DeleteBitmap(bitmap windows.Handle) {
	if bitmap != 0 {
		procDeleteObject.Call(uintptr(bitmap))
	}
}
