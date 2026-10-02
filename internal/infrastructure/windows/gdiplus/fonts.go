//go:build windows && amd64

package gdiplus

import (
	"slices"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procNewInstalledFontCollection   = dll.NewProc("GdipNewInstalledFontCollection")
	procGetFontCollectionFamilyCount = dll.NewProc("GdipGetFontCollectionFamilyCount")
	procGetFontCollectionFamilyList  = dll.NewProc("GdipGetFontCollectionFamilyList")
	procGetFamilyName                = dll.NewProc("GdipGetFamilyName")
)

// faceNameLength is LF_FACESIZE, the longest family name GDI+ returns.
const faceNameLength = 32

// Families lists the font families installed on the computer, sorted and
// without the vertical ("@") variants, for the font choice of a subtitle.
func (r *Renderer) Families() ([]string, error) {
	// The installed collection belongs to GDI+: it is never deleted.
	var collection uintptr
	if err := call(procNewInstalledFontCollection, uintptr(unsafe.Pointer(&collection))); err != nil {
		return nil, err
	}
	var count int32
	if err := call(procGetFontCollectionFamilyCount, collection, uintptr(unsafe.Pointer(&count))); err != nil {
		return nil, err
	}
	if count <= 0 {
		return []string{}, nil
	}
	families := make([]uintptr, count)
	var found int32
	if err := call(procGetFontCollectionFamilyList, collection, uintptr(count), uintptr(unsafe.Pointer(&families[0])), uintptr(unsafe.Pointer(&found))); err != nil {
		return nil, err
	}
	names := make([]string, 0, found)
	for _, family := range families[:found] {
		var name [faceNameLength]uint16
		if call(procGetFamilyName, family, uintptr(unsafe.Pointer(&name[0])), 0) != nil {
			continue
		}
		if text := windows.UTF16ToString(name[:]); text != "" && text[0] != '@' {
			names = append(names, text)
		}
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}
