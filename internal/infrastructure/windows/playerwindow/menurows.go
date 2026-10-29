//go:build windows && amd64

package playerwindow

import (
	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// On a dark or a light menu the window draws the rows itself, so a key sits
// on a background of its own in one column clear of the arrows; in the
// system's colors (high contrast) Windows draws the whole menu.

// menuRow is what the window draws of one row of the menu on screen.
type menuRow struct {
	separator      bool
	text, key      string
	icon           windows.Handle
	radio, submenu bool
}

// menuFallbackSize is the menu's text height, in pixels at 100%, when
// Windows cannot tell its menu font: 9 points.
const menuFallbackSize = 12.0

// menuColors are the colors of the rows the window draws on a menu of a
// shade; a key's background is the text's color, faint, so it shows on a
// highlighted row too.
var menuColors = map[menuShade]gdiplus.MenuColors{
	shadeDark: {
		Back: appearance.RGB(0x2B, 0x2B, 0x2B), Hover: appearance.RGB(0x41, 0x41, 0x41),
		Text: menuPalettes[shadeDark].text, Disabled: menuPalettes[shadeDark].disabled,
		KeyBack: appearance.Color{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x1A}, KeyText: appearance.RGB(0xB8, 0xB8, 0xB8),
		Line: appearance.RGB(0x4A, 0x4A, 0x4A),
	},
	shadeLight: {
		Back: appearance.RGB(0xF9, 0xF9, 0xF9), Hover: appearance.RGB(0xE5, 0xE5, 0xE5),
		Text: menuPalettes[shadeLight].text, Disabled: menuPalettes[shadeLight].disabled,
		KeyBack: appearance.Color{A: 0x12}, KeyText: appearance.RGB(0x5A, 0x5A, 0x5A),
		Line: appearance.RGB(0xDA, 0xDA, 0xDA),
	},
}

// drawsRows tells whether the window draws the rows of a menu of a shade.
func drawsRows(shade menuShade) bool {
	_, ours := menuColors[shade]
	return ours
}

// beginRows readies the drawing of a menu's rows: their look at the
// window's scaling, in the menu font Windows uses, and the menu's
// background brush.
func (w *Window) beginRows(shade menuShade) {
	dpi := win32.DpiForWindow(w.host)
	scale := float64(dpi) / layout.BaseDPI
	face, size, ok := win32.MenuFont(dpi)
	if !ok {
		face, size = w.chrome.Bar.FontFamily, menuFallbackSize*scale
	}
	colors := menuColors[shade]
	w.rows = w.rows[:0]
	w.rowLook = gdiplus.MenuRowLook{Font: layout.Font{Family: face, Size: size}, Scale: scale, Colors: colors}
	back := colors.Back
	w.rowBrush = win32.SolidBrush(win32.RGB(back.R, back.G, back.B))
}

// newMenu makes an empty popup menu, on the background of the rows the
// window draws, if it draws them.
func (w *Window) newMenu() (windows.Handle, error) {
	menu, err := win32.CreatePopupMenu()
	if err != nil || w.rowBrush == 0 {
		return menu, err
	}
	if err := win32.SetMenuBackground(menu, w.rowBrush); err != nil {
		win32.DestroyMenu(menu)
		return 0, err
	}
	return menu, nil
}

// endRows forgets the rows of the menu just closed and frees its brush.
func (w *Window) endRows() {
	w.rows = w.rows[:0]
	win32.DeleteObject(w.rowBrush)
	w.rowBrush = 0
}

// addRow keeps a row to draw and returns what the menu item carries to
// find it again: its place, counted from 1.
func (w *Window) addRow(row menuRow) uintptr {
	w.rows = append(w.rows, row)
	return uintptr(len(w.rows))
}

// rowOf is the row a menu item carries; false for an item the window does
// not draw.
func (w *Window) rowOf(kind uint32, data uintptr) (menuRow, bool) {
	if kind != win32.ODT_MENU || data == 0 || data > uintptr(len(w.rows)) {
		return menuRow{}, false
	}
	return w.rows[data-1], true
}

// rowLookOf is a row's look, in a state.
func (w *Window) rowLookOf(row menuRow, state uint32) gdiplus.MenuRowLook {
	look := w.rowLook
	look.Separator, look.Text, look.Key = row.separator, row.text, row.key
	look.Icon, look.IconSide = row.icon, win32.SmallIconSize(win32.DpiForWindow(w.host))
	look.Radio, look.Submenu = row.radio, row.submenu
	look.Checked = state&win32.ODS_CHECKED != 0
	look.Highlighted = state&win32.ODS_SELECTED != 0
	look.Disabled = state&(win32.ODS_GRAYED|win32.ODS_DISABLED) != 0
	return look
}

// measureRow answers WM_MEASUREITEM for a row the window draws; false
// leaves the message to Windows.
func (w *Window) measureRow(lparam uintptr) bool {
	item := win32.MeasureItemAt(lparam)
	row, ours := w.rowOf(item.CtlType, item.ItemData)
	if !ours || w.renderer == nil {
		return false
	}
	width, height := w.renderer.MenuRowSize(w.rowLookOf(row, 0))
	item.ItemWidth, item.ItemHeight = uint32(width), uint32(height)
	win32.AnswerMeasure(lparam, item)
	return true
}

// drawRow answers WM_DRAWITEM for a row the window draws: drawn on a
// picture of its own, then put in place. Windows draws a submenu's arrow
// after the row; the row's place is kept out of that drawing, since the
// row has its own. False leaves the message to Windows.
func (w *Window) drawRow(lparam uintptr) bool {
	item := win32.DrawItemAt(lparam)
	row, ours := w.rowOf(item.CtlType, item.ItemData)
	if !ours || w.renderer == nil {
		return false
	}
	width, height := int(item.Bounds.Right-item.Bounds.Left), int(item.Bounds.Bottom-item.Bounds.Top)
	if width <= 0 || height <= 0 {
		return true
	}
	if w.rowDIB == nil || w.rowDIB.Width != width || w.rowDIB.Height != height {
		w.rowDIB.Close()
		dib, err := win32.NewDIB(width, height)
		if err != nil {
			w.rowDIB = nil
			w.logger.Warn("a menu row's picture failed", "error", err)
			return true
		}
		w.rowDIB = dib
	}
	if err := w.renderer.DrawMenuRow(w.rowDIB, w.rowLookOf(row, item.ItemState)); err != nil {
		w.logger.Warn("a menu row could not be drawn", "error", err)
		return true
	}
	win32.CopyDIB(item.DC, item.Bounds.Left, item.Bounds.Top, w.rowDIB)
	if row.submenu {
		win32.ExcludeClip(item.DC, item.Bounds)
	}
	return true
}

// rowOf is the row an item of the model draws, with its text and key
// worded and its icon.
func rowOf(item contextmenu.Item, text contextmenu.Text, icon windows.Handle) menuRow {
	return menuRow{
		separator: item.Kind == contextmenu.KindSeparator,
		text:      text.Text, key: text.Key, icon: icon,
		radio: item.Radio, submenu: item.Kind == contextmenu.KindSubmenu,
	}
}
