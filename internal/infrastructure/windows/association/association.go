//go:build windows

// Package association lets Windows open videos with the app, when the user
// asks for it: the app appears in "Open with" and the videos' context menu
// gets an entry to watch them with it, worded in the interface's language.
// Everything is written for the current user only, and no other program
// loses its place as the default.
package association

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	// ProgID names the app's kind of file in the registry.
	ProgID = "LingoReel.Video"
	// verb is the context menu entry's key.
	verb = "LingoReel"
)

// Names word what the Explorer shows: the kind of file ("Video (LingoReel)") and the context menu entry ("Watch with LingoReel").
type Names struct {
	Type, Menu string
}

// defaultNames are used when the registrar is given no words.
var defaultNames = Names{Type: "Video (LingoReel)", Menu: "Watch with LingoReel"}

// Registrar writes and removes the association under Base, a key of the
// current user: `Software\Classes` for the app, another one in tests.
type Registrar struct {
	Base       string
	Executable string
	// Extensions are the files to offer, such as ".mkv".
	Extensions []string
	// Names words the entries in the interface's language, read on each
	// Enable; nil, or an empty word, keeps the English default.
	Names func() Names
}

// names are the words to write now.
func (r Registrar) names() Names {
	names := defaultNames
	if r.Names != nil {
		given := r.Names()
		if given.Type != "" {
			names.Type = given.Type
		}
		if given.Menu != "" {
			names.Menu = given.Menu
		}
	}
	return names
}

// command is how Windows starts the app with a file.
func (r Registrar) command() string {
	return fmt.Sprintf(`"%s" "%%1"`, r.Executable)
}

// Enabled tells whether the association is there and starts this program.
func (r Registrar) Enabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, r.Base+`\`+ProgID+`\shell\open\command`, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading the association: %w", err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue("")
	if err != nil {
		return false, nil
	}
	return strings.EqualFold(value, r.command()), nil
}

// Enable writes the association and tells the Explorer.
func (r Registrar) Enable() error {
	if r.Executable == "" || len(r.Extensions) == 0 {
		return errors.New("association: no program or no extensions")
	}
	names := r.names()
	values := []struct {
		path, name, value string
	}{
		{ProgID, "", names.Type},
		{ProgID + `\DefaultIcon`, "", fmt.Sprintf(`"%s",0`, r.Executable)},
		{ProgID + `\shell\open`, "", names.Menu},
		{ProgID + `\shell\open\command`, "", r.command()},
	}
	for _, extension := range r.Extensions {
		menu := `SystemFileAssociations\` + extension + `\shell\` + verb
		values = append(values,
			struct{ path, name, value string }{extension + `\OpenWithProgids`, ProgID, ""},
			struct{ path, name, value string }{menu, "", names.Menu},
			struct{ path, name, value string }{menu, "Icon", fmt.Sprintf(`"%s",0`, r.Executable)},
			struct{ path, name, value string }{menu + `\command`, "", r.command()},
		)
	}
	for _, v := range values {
		if err := r.set(v.path, v.name, v.value); err != nil {
			return errors.Join(err, r.Disable())
		}
	}
	notifyExplorer()
	return nil
}

// Disable removes everything Enable wrote, and only that.
func (r Registrar) Disable() error {
	var errs []error
	errs = append(errs, deleteTree(r.Base+`\`+ProgID))
	for _, extension := range r.Extensions {
		errs = append(errs, deleteTree(r.Base+`\SystemFileAssociations\`+extension+`\shell\`+verb))
		errs = append(errs, deleteValue(r.Base+`\`+extension+`\OpenWithProgids`, ProgID))
	}
	notifyExplorer()
	return errors.Join(errs...)
}

func (r Registrar) set(path, name, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, r.Base+`\`+path, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer key.Close()
	if err := key.SetStringValue(name, value); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// deleteTree removes a key and everything under it; a key that is not
// there is fine.
func deleteTree(path string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	children, err := key.ReadSubKeyNames(-1)
	key.Close()
	if err != nil {
		return fmt.Errorf("listing %s: %w", path, err)
	}
	for _, child := range children {
		if err := deleteTree(path + `\` + child); err != nil {
			return err
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	return nil
}

func deleteValue(path, name string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer key.Close()
	if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("removing %s from %s: %w", name, path, err)
	}
	return nil
}

// notifyExplorer makes the Explorer read the associations again.
func notifyExplorer() {
	const shcneAssocChanged, shcnfIDList = 0x08000000, 0
	windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify").Call(shcneAssocChanged, shcnfIDList, 0, 0)
}
