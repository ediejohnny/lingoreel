//go:build windows

package association_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/association"
)

// testBase is a key of the current user that only these tests use; it is
// removed at the end, so the real associations are never touched.
func testBase(t *testing.T) string {
	t.Helper()
	base := fmt.Sprintf(`Software\LingoReelTests\association-%d`, time.Now().UnixNano())
	t.Cleanup(func() { removeTree(t, `Software\LingoReelTests`) })
	return base
}

func removeTree(t *testing.T, path string) {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return
	}
	if err != nil {
		t.Errorf("opening %s: %v", path, err)
		return
	}
	children, _ := key.ReadSubKeyNames(-1)
	key.Close()
	for _, child := range children {
		removeTree(t, path+`\`+child)
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
		t.Errorf("removing %s: %v", path, err)
	}
}

func read(t *testing.T, path, name string) (string, bool) {
	t.Helper()
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	return value, err == nil
}

func write(t *testing.T, path, name, value string) {
	t.Helper()
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	if err := key.SetStringValue(name, value); err != nil {
		t.Fatal(err)
	}
}

func TestTheAssociationComesAndGoesWithoutTouchingOthers(t *testing.T) {
	base := testBase(t)
	const exe = `C:\Program Files\LingoReel\LingoReel.exe`
	r := association.Registrar{Base: base, Executable: exe, Extensions: []string{".mkv", ".mp4"}}
	// Another program already owns .mkv and offers itself in "Abrir com".
	write(t, base+`\.mkv`, "", "VLC.mkv")
	write(t, base+`\.mkv\OpenWithProgids`, "VLC.mkv", "")
	write(t, base+`\SystemFileAssociations\.mkv\shell\enqueue`, "", "Adicionar à lista do VLC")

	if on, err := r.Enabled(); on || err != nil {
		t.Fatalf("Enabled before = %v, %v", on, err)
	}
	if err := r.Enable(); err != nil {
		t.Fatal(err)
	}
	command := `"` + exe + `" "%1"`
	want := []struct{ path, name, value string }{
		{base + `\` + association.ProgID + `\shell\open\command`, "", command},
		{base + `\.mkv\OpenWithProgids`, association.ProgID, ""},
		{base + `\.mp4\OpenWithProgids`, association.ProgID, ""},
		{base + `\SystemFileAssociations\.mp4\shell\LingoReel`, "", "Assistir com o LingoReel"},
		{base + `\SystemFileAssociations\.mkv\shell\LingoReel\command`, "", command},
	}
	for _, w := range want {
		if got, found := read(t, w.path, w.name); !found || got != w.value {
			t.Errorf("%s [%s] = %q (%v), want %q", w.path, w.name, got, found, w.value)
		}
	}
	if on, err := r.Enabled(); !on || err != nil {
		t.Fatalf("Enabled after = %v, %v", on, err)
	}
	// The default program is still the other one.
	if got, _ := read(t, base+`\.mkv`, ""); got != "VLC.mkv" {
		t.Fatalf("the default of .mkv became %q", got)
	}
	// Another copy of the app (moved elsewhere) is not this one.
	moved := r
	moved.Executable = `D:\Outro\LingoReel.exe`
	if on, _ := moved.Enabled(); on {
		t.Fatal("the association of another copy counted as this one's")
	}

	if err := r.Disable(); err != nil {
		t.Fatal(err)
	}
	if on, _ := r.Enabled(); on {
		t.Fatal("still enabled after Disable")
	}
	for _, gone := range []struct{ path, name string }{
		{base + `\` + association.ProgID, ""},
		{base + `\.mkv\OpenWithProgids`, association.ProgID},
		{base + `\SystemFileAssociations\.mkv\shell\LingoReel`, ""},
	} {
		if _, found := read(t, gone.path, gone.name); found {
			t.Errorf("%s [%s] is still there", gone.path, gone.name)
		}
	}
	// What belongs to the other program stays.
	for _, kept := range []struct{ path, name, value string }{
		{base + `\.mkv`, "", "VLC.mkv"},
		{base + `\.mkv\OpenWithProgids`, "VLC.mkv", ""},
		{base + `\SystemFileAssociations\.mkv\shell\enqueue`, "", "Adicionar à lista do VLC"},
	} {
		if got, found := read(t, kept.path, kept.name); !found || got != kept.value {
			t.Errorf("the other program's %s [%s] = %q (%v)", kept.path, kept.name, got, found)
		}
	}
	// Disabling again is harmless.
	if err := r.Disable(); err != nil {
		t.Fatalf("a second Disable = %v", err)
	}
}

func TestEnableNeedsAProgramAndExtensions(t *testing.T) {
	base := testBase(t)
	for _, r := range []association.Registrar{
		{Base: base, Extensions: []string{".mkv"}},
		{Base: base, Executable: `C:\a.exe`},
	} {
		if err := r.Enable(); err == nil {
			t.Errorf("Enable(%+v) succeeded", r)
		}
	}
}
