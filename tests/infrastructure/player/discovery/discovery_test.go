package discovery_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/potplayer"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/vlc"
)

// fakeEnvironment is an in-memory machine.
type fakeEnvironment struct {
	values  map[string]string // "root|path|name" -> value
	subkeys map[string][]string
	files   map[string]bool // lower-case paths
	env     map[string]string
	path    map[string]string
}

func newEnvironment() *fakeEnvironment {
	return &fakeEnvironment{
		values:  map[string]string{},
		subkeys: map[string][]string{},
		files:   map[string]bool{},
		env:     map[string]string{"ProgramFiles": `C:\Program Files`, "ProgramFiles(x86)": `C:\Program Files (x86)`, "LOCALAPPDATA": `C:\Users\u\AppData\Local`},
		path:    map[string]string{},
	}
}

func valueKey(root discovery.Root, path, name string) string {
	return string(rune('0'+root)) + "|" + strings.ToLower(path) + "|" + strings.ToLower(name)
}

func (f *fakeEnvironment) setValue(root discovery.Root, path, name, value string) {
	f.values[valueKey(root, path, name)] = value
}

func (f *fakeEnvironment) addFile(path string) { f.files[strings.ToLower(path)] = true }

func (f *fakeEnvironment) RegistryString(root discovery.Root, path, name string) (string, bool) {
	value, found := f.values[valueKey(root, path, name)]
	return value, found
}

func (f *fakeEnvironment) RegistrySubkeys(root discovery.Root, path string) []string {
	return f.subkeys[string(rune('0'+root))+"|"+strings.ToLower(path)]
}

func (f *fakeEnvironment) FileExists(path string) bool { return f.files[strings.ToLower(path)] }
func (f *fakeEnvironment) Getenv(name string) string   { return f.env[name] }

func (f *fakeEnvironment) LookPath(file string) (string, bool) {
	path, found := f.path[strings.ToLower(file)]
	return path, found
}

func (f *fakeEnvironment) Glob(pattern string) []string {
	var matches []string
	for file := range f.files {
		if ok, _ := filepath.Match(strings.ToLower(pattern), file); ok {
			matches = append(matches, file)
		}
	}
	return matches
}

// allHints are the hints of every known player: VLC's from its plugin and
// the others' from the discovery list.
func allHints() []discovery.Hints {
	withPlugins := []discovery.Hints{vlc.Plugin{}.Hints(), potplayer.Plugin{}.Hints()}
	return append(withPlugins, discovery.Known()...)
}

func found(t *testing.T, installations []player.Installation) map[player.ID]string {
	t.Helper()
	paths := map[player.ID]string{}
	for _, installation := range installations {
		if _, duplicated := paths[installation.ID]; duplicated {
			t.Fatalf("player %s reported twice", installation.ID)
		}
		paths[installation.ID] = strings.ToLower(installation.Path)
	}
	return paths
}

func TestDetectFindsNothingOnACleanMachine(t *testing.T) {
	t.Parallel()
	if got := discovery.Detect(newEnvironment(), allHints(), nil); len(got) != 0 {
		t.Fatalf("Detect on an empty machine = %+v", got)
	}
}

func TestDetectUsesEachSource(t *testing.T) {
	t.Parallel()
	env := newEnvironment()
	// VLC through its own registry key holding the install folder.
	env.setValue(discovery.LocalMachine, `SOFTWARE\VideoLAN\VLC`, "InstallDir", `D:\Apps\VLC`)
	env.addFile(`D:\Apps\VLC\vlc.exe`)
	// mpv through the winget links folder.
	env.addFile(`C:\Users\u\AppData\Local\Microsoft\WinGet\Links\mpv.exe`)
	// PotPlayer through Program Files.
	env.addFile(`C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe`)
	// MPC-HC through an uninstall entry whose icon carries an index and quotes.
	env.subkeys["0|"+strings.ToLower(`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`)] = []string{"MPC-HC_is1"}
	env.setValue(discovery.LocalMachine, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\MPC-HC_is1`, "DisplayName", "MPC-HC 2.8.2 (64-bit)")
	env.setValue(discovery.LocalMachine, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\MPC-HC_is1`, "DisplayIcon", `"E:\Media\MPC-HC\mpc-hc64.exe",0`)
	env.addFile(`E:\Media\MPC-HC\mpc-hc64.exe`)
	// MPC-BE through the PATH.
	env.path["mpc-be64.exe"] = `F:\Tools\mpc-be64.exe`
	env.addFile(`F:\Tools\mpc-be64.exe`)

	paths := found(t, discovery.Detect(env, allHints(), nil))
	want := map[player.ID]string{
		player.VLC:       `d:\apps\vlc\vlc.exe`,
		player.MPV:       `c:\users\u\appdata\local\microsoft\winget\links\mpv.exe`,
		player.PotPlayer: `c:\program files\daum\potplayer\potplayermini64.exe`,
		player.MPCHC:     `e:\media\mpc-hc\mpc-hc64.exe`,
		player.MPCBE:     `f:\tools\mpc-be64.exe`,
	}
	if len(paths) != len(want) {
		t.Fatalf("Detect found %v, want %v", paths, want)
	}
	for id, path := range want {
		if paths[id] != path {
			t.Fatalf("%s found at %q, want %q", id, paths[id], path)
		}
	}
}

func TestConfiguredPathWinsWhenItExists(t *testing.T) {
	t.Parallel()
	env := newEnvironment()
	env.addFile(`C:\Program Files\VideoLAN\VLC\vlc.exe`)
	env.addFile(`D:\Portable\VLC\vlc.exe`)

	paths := found(t, discovery.Detect(env, allHints(), map[player.ID]string{player.VLC: `D:\Portable\VLC\vlc.exe`}))
	if paths[player.VLC] != `d:\portable\vlc\vlc.exe` {
		t.Fatalf("configured path ignored: %q", paths[player.VLC])
	}

	stale := found(t, discovery.Detect(env, allHints(), map[player.ID]string{player.VLC: `D:\Removed\vlc.exe`}))
	if stale[player.VLC] != `c:\program files\videolan\vlc\vlc.exe` {
		t.Fatalf("a configured path that no longer exists must fall back to detection, got %q", stale[player.VLC])
	}
}

func TestDetectRejectsPathsThatAreNotThePlayer(t *testing.T) {
	t.Parallel()
	env := newEnvironment()
	// A registry value pointing to another program, and a relative PATH hit.
	env.setValue(discovery.LocalMachine, `SOFTWARE\VideoLAN\VLC`, "", `C:\Windows\System32\cmd.exe`)
	env.addFile(`C:\Windows\System32\cmd.exe`)
	env.path["mpv.exe"] = `mpv.exe`
	env.addFile(`mpv.exe`)
	// A value pointing to the right name that does not exist on disk.
	env.setValue(discovery.CurrentUser, `SOFTWARE\DAUM\PotPlayer64`, "ProgramPath", `C:\Gone\PotPlayerMini64.exe`)

	configured := map[player.ID]string{player.MPCBE: `C:\Tools\notepad.exe`}
	env.addFile(`C:\Tools\notepad.exe`)

	if got := discovery.Detect(env, allHints(), configured); len(got) != 0 {
		t.Fatalf("Detect accepted foreign or missing programs: %+v", got)
	}
}

func TestDetectFollowsTheHintsItIsGiven(t *testing.T) {
	t.Parallel()
	env := newEnvironment()
	env.addFile(`C:\Program Files\VideoLAN\VLC\vlc.exe`)
	env.addFile(`C:\Program Files\mpv\mpv.exe`)
	// Only the players whose hints were given are looked for.
	if got := found(t, discovery.Detect(env, discovery.Known(), nil)); len(got) != 1 || got[player.MPV] == "" {
		t.Fatalf("Detect with the list without plugins = %v, want only mpv", got)
	}
	// Hints listed twice are looked for once; incomplete hints are skipped.
	hints := []discovery.Hints{vlc.Plugin{}.Hints(), vlc.Plugin{}.Hints(), {ID: "empty"}, {Executables: []string{"x.exe"}}}
	if got := discovery.Detect(env, hints, nil); len(got) != 1 || got[0].ID != player.VLC {
		t.Fatalf("Detect = %+v, want VLC once", got)
	}
	// No uninstall pattern: that source is skipped, not a crash.
	bare := discovery.Hints{ID: "bare", Executables: []string{"bare.exe"}}
	env.subkeys["0|"+strings.ToLower(`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`)] = []string{"Bare"}
	if got := discovery.Detect(env, []discovery.Hints{bare}, nil); len(got) != 0 {
		t.Fatalf("Detect = %+v, want nothing", got)
	}
	if known := discovery.Known(); len(known) == 0 || &known[0] == &discovery.Known()[0] {
		t.Fatal("Known must return its own copy")
	}
}

func TestDetectOnThisMachineReturnsOnlyExistingExecutables(t *testing.T) {
	t.Parallel()
	installations := discovery.Detect(discovery.SystemEnvironment{}, allHints(), nil)
	for _, installation := range installations {
		info, err := os.Stat(installation.Path)
		if err != nil || !info.Mode().IsRegular() || !filepath.IsAbs(installation.Path) {
			t.Fatalf("%s reported at %q, which is not an existing absolute file", installation.ID, installation.Path)
		}
		t.Logf("found %s at %s", installation.ID.DisplayName(), installation.Path)
	}
	if os.Getenv("LINGOREEL_VLC_TEST") == "1" {
		if _, ok := found(t, installations)[player.VLC]; !ok {
			t.Fatal("LINGOREEL_VLC_TEST is set but VLC was not detected")
		}
	}
}
