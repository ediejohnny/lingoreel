package shortcut_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"lingoreel/internal/domain/shortcut"
)

func TestParseAndStringRoundTrip(t *testing.T) {
	cases := []struct {
		text, canonical string
		want            shortcut.Chord
	}{
		{"Space", "Space", shortcut.Chord{Key: shortcut.KeySpace}},
		{"Shift+Comma", "Shift+Comma", shortcut.Chord{Modifiers: shortcut.Shift, Key: shortcut.KeyComma}},
		// Modifiers in any order come back in the fixed one.
		{"Shift+Ctrl+KeyP", "Ctrl+Shift+KeyP", shortcut.Chord{Modifiers: shortcut.Ctrl | shortcut.Shift, Key: "KeyP"}},
		{"Alt+Ctrl+Digit1", "Ctrl+Alt+Digit1", shortcut.Chord{Modifiers: shortcut.Ctrl | shortcut.Alt, Key: "Digit1"}},
		{"  Ctrl+F5 ", "Ctrl+F5", shortcut.Chord{Modifiers: shortcut.Ctrl, Key: "F5"}},
		{"AltRight", "AltRight", shortcut.Chord{Key: shortcut.KeyAltRight}},
		{"NumpadEnter", "NumpadEnter", shortcut.Chord{Key: "NumpadEnter"}},
	}
	for _, c := range cases {
		got, err := shortcut.Parse(c.text)
		if err != nil {
			t.Fatalf("Parse(%q) = %v", c.text, err)
		}
		if got != c.want || got.String() != c.canonical {
			t.Fatalf("Parse(%q) = %+v %q, want %+v %q", c.text, got, got.String(), c.want, c.canonical)
		}
	}
}

func TestParseRefusesWhatCannotBeAShortcut(t *testing.T) {
	cases := []struct {
		text string
		want error
	}{
		{"", shortcut.ErrNoKey},
		{"Ctrl+", shortcut.ErrNoKey},
		{"+", shortcut.ErrUnknownKey},
		{"Ctrl", shortcut.ErrUnknownKey},        // a modifier alone is not a key
		{"ControlLeft", shortcut.ErrUnknownKey}, // nor is its own key
		{"Win+KeyE", shortcut.ErrUnknownKey},
		{"ctrl+KeyP", shortcut.ErrUnknownKey}, // names are exact
		{"Ctrl+Ctrl+KeyP", shortcut.ErrUnknownKey},
		{"KeyAA", shortcut.ErrUnknownKey},
		{"F13", shortcut.ErrUnknownKey},
		{"Key", shortcut.ErrUnknownKey},
		{"Ctrl+Alt+Shift+KeyP", shortcut.ErrTooManyKeys},
		{"Alt+Shift+KeyP", shortcut.ErrAltShift},
		{"Shift+Alt+Space", shortcut.ErrAltShift},
		{"Ctrl+AltRight", shortcut.ErrAloneKey},
		{"Shift+AltRight", shortcut.ErrAloneKey},
		{"Alt+F4", shortcut.ErrReserved},
		{"Alt+Tab", shortcut.ErrReserved},
		{"Alt+Escape", shortcut.ErrReserved},
		{"Alt+Space", shortcut.ErrReserved},
		{"Ctrl+Escape", shortcut.ErrReserved},
		{"F12", shortcut.ErrReserved},
		{"Ctrl+Space\x00", shortcut.ErrUnknownKey},
		{strings.Repeat("Ctrl+", 1000) + "KeyP", shortcut.ErrUnknownKey},
	}
	for _, c := range cases {
		if _, err := shortcut.Parse(c.text); !errors.Is(err, c.want) {
			t.Errorf("Parse(%q) = %v, want %v", c.text, err, c.want)
		}
	}
	// The neighbors of the reserved ones are fine.
	for _, text := range []string{"Ctrl+F4", "Shift+F12", "Ctrl+F12", "Ctrl+Tab", "Shift+Escape", "Ctrl+Space", "F11", "Escape"} {
		if _, err := shortcut.Parse(text); err != nil {
			t.Errorf("Parse(%q) = %v, want it accepted", text, err)
		}
	}
}

func TestEveryLetterDigitAndFunctionKeyIsKnown(t *testing.T) {
	var names []string
	for c := 'A'; c <= 'Z'; c++ {
		names = append(names, "Key"+string(c))
	}
	for c := '0'; c <= '9'; c++ {
		names = append(names, "Digit"+string(c), "Numpad"+string(c))
	}
	for i := 1; i <= 12; i++ {
		names = append(names, "F"+strconv.Itoa(i))
	}
	for _, name := range names {
		if !shortcut.Key(name).Known() {
			t.Errorf("%s unknown", name)
		}
	}
}

func TestChordJSONRefusesInvalidText(t *testing.T) {
	var chords map[string]shortcut.Chord
	if err := json.Unmarshal([]byte(`{"a":"Ctrl+Comma"}`), &chords); err != nil || chords["a"] != (shortcut.Chord{Modifiers: shortcut.Ctrl, Key: shortcut.KeyComma}) {
		t.Fatalf("decode = %v %+v", err, chords)
	}
	for _, bad := range []string{`{"a":"Alt+Shift+KeyP"}`, `{"a":" "}`, `{"a":"Ctrl+"}`, `{"a":"Alt+F4"}`, `{"a":3}`} {
		if err := json.Unmarshal([]byte(bad), &chords); err == nil {
			t.Errorf("decoded %s", bad)
		}
	}
	if _, err := json.Marshal(shortcut.Chord{Key: "Nope"}); err == nil {
		t.Fatal("an invalid chord was written")
	}
	if _, err := json.Marshal(shortcut.Chord{Modifiers: shortcut.Ctrl}); err == nil {
		t.Fatal("modifiers without a key were written")
	}
}

func TestNoShortcutIsTheEmptyChord(t *testing.T) {
	// Written and read back as an empty text.
	text, err := json.Marshal(map[string]shortcut.Chord{"a": {}})
	if err != nil || string(text) != `{"a":""}` {
		t.Fatalf("Marshal = %s %v", text, err)
	}
	chords := map[string]shortcut.Chord{"a": {Key: shortcut.KeyM}}
	if err := json.Unmarshal([]byte(`{"a":""}`), &chords); err != nil || !chords["a"].IsZero() {
		t.Fatalf("Unmarshal = %+v %v", chords, err)
	}
	if (shortcut.Chord{Key: shortcut.KeyM}).IsZero() || (shortcut.Chord{Modifiers: shortcut.Ctrl}).IsZero() {
		t.Fatal("a chord with a key or a modifier counted as none")
	}
	// The chord itself still needs a key; only a map takes none.
	if err := (shortcut.Chord{}).Validate(); !errors.Is(err, shortcut.ErrNoKey) {
		t.Fatalf("Validate() = %v", err)
	}

	// Two commands without a shortcut share nothing, and nothing finds them.
	m := shortcut.Map{shortcut.Mute: {}, shortcut.Fullscreen: {}}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate = %v", err)
	}
	if !m.Of(shortcut.Mute).IsZero() || !m.Resolved()[shortcut.Mute].IsZero() {
		t.Fatal("the empty chord fell back to the default")
	}
	if _, found := m.Find(shortcut.Chord{}); found {
		t.Fatal("Find(none) found a command")
	}
	if _, found := m.Find(shortcut.Chord{Key: shortcut.KeyM}); found {
		t.Fatal("M still runs the command that has no shortcut")
	}
	// Its old key is free for another command.
	if err := (shortcut.Map{shortcut.Mute: {}, shortcut.PlayPause: {Key: shortcut.KeyM}}).Validate(); err != nil {
		t.Fatalf("the freed key = %v", err)
	}
}

func TestJumpsRewindsAndRepeats(t *testing.T) {
	jumps := map[shortcut.Command]time.Duration{
		shortcut.SeekBack: -5 * time.Second, shortcut.SeekForward: 5 * time.Second,
		shortcut.SeekBack10: -10 * time.Second, shortcut.SeekForward10: 10 * time.Second,
		shortcut.SeekBack30: -30 * time.Second, shortcut.SeekForward30: 30 * time.Second,
	}
	rewinds := map[shortcut.Command]time.Duration{
		shortcut.RepeatLast3: 3 * time.Second, shortcut.RepeatLast5: 5 * time.Second, shortcut.RepeatLast10: 10 * time.Second,
	}
	// Holding moves on through the video, the volume and the sync; nothing
	// that switches, replays or changes the speed repeats.
	repeats := map[shortcut.Command]bool{
		shortcut.PrecisionBack: true, shortcut.PrecisionForward: true, shortcut.FrameBack: true, shortcut.FrameForward: true,
		shortcut.VolumeUp: true, shortcut.VolumeDown: true,
		shortcut.SyncPrimaryEarlier: true, shortcut.SyncPrimaryLater: true,
		shortcut.SyncTranslationEarlier: true, shortcut.SyncTranslationLater: true,
	}
	for command := range jumps {
		repeats[command] = true
	}
	for _, command := range shortcut.Commands() {
		jump, isJump := command.Jump()
		if want, is := jumps[command]; isJump != is || jump != want {
			t.Errorf("%s.Jump() = %v %v, want %v %v", command, jump, isJump, want, is)
		}
		rewind, isRewind := command.Rewind()
		if want, is := rewinds[command]; isRewind != is || rewind != want {
			t.Errorf("%s.Rewind() = %v %v, want %v %v", command, rewind, isRewind, want, is)
		}
		if command.Repeats() != repeats[command] {
			t.Errorf("%s.Repeats() = %v", command, command.Repeats())
		}
	}
	if _, jumps := shortcut.Command("nope").Jump(); jumps {
		t.Fatal("an unknown command jumps")
	}
}

func TestDefaultsCoverEveryCommandWithoutConflicts(t *testing.T) {
	defaults := shortcut.Defaults()
	if err := defaults.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(defaults) != len(shortcut.Commands()) {
		t.Fatalf("%d defaults for %d commands", len(defaults), len(shortcut.Commands()))
	}
	// The consolidated map of the plan; the explicit translation choices
	// start without a shortcut.
	want := map[shortcut.Command]string{
		shortcut.PlayPause: "Space", shortcut.SeekBack: "ArrowLeft", shortcut.SeekForward: "ArrowRight",
		shortcut.VolumeUp: "ArrowUp", shortcut.VolumeDown: "ArrowDown", shortcut.Mute: "KeyM",
		shortcut.Fullscreen: "KeyF", shortcut.ExitFullscreen: "Escape", shortcut.ReplayLine: "Backspace",
		shortcut.Translation:        "AltRight",
		shortcut.SyncPrimaryEarlier: "Shift+Comma", shortcut.SyncPrimaryLater: "Shift+Period",
		shortcut.SyncTranslationEarlier: "Ctrl+Comma", shortcut.SyncTranslationLater: "Ctrl+Period",
		shortcut.SeekBack10: "Shift+ArrowLeft", shortcut.SeekForward10: "Shift+ArrowRight",
		shortcut.SeekBack30: "Ctrl+ArrowLeft", shortcut.SeekForward30: "Ctrl+ArrowRight",
		shortcut.PrecisionBack: "Alt+ArrowLeft", shortcut.PrecisionForward: "Alt+ArrowRight",
		shortcut.FrameBack: "Comma", shortcut.FrameForward: "Period",
		shortcut.SpeedDown: "Ctrl+ArrowDown", shortcut.SpeedUp: "Ctrl+ArrowUp", shortcut.SpeedNormal: "Ctrl+Digit0",
		shortcut.RepeatPhrase: "F9", shortcut.RepeatLast3: "Digit3", shortcut.RepeatLast5: "Digit5", shortcut.RepeatLast10: "Digit0",
		shortcut.ShowBoth: "", shortcut.PrimaryOnly: "",
	}
	if len(want) != len(shortcut.Commands()) {
		t.Fatalf("the test knows %d commands of %d", len(want), len(shortcut.Commands()))
	}
	for command, text := range want {
		if got := defaults[command].String(); got != text {
			t.Errorf("default of %s = %s, want %s", command, got, text)
		}
		if command.Category() == "" {
			t.Errorf("%s has no category", command)
		}
	}
	// A copy: changing one map does not change the next defaults.
	defaults[shortcut.PlayPause] = shortcut.Chord{Key: "KeyP"}
	if shortcut.Defaults()[shortcut.PlayPause].Key != shortcut.KeySpace {
		t.Fatal("Defaults shares its map")
	}
}

func TestConflictsAreFoundAgainstTheDefaultsToo(t *testing.T) {
	// Play/Pause moved onto the default of the first sync shortcut: the
	// map holds one entry, the conflict is with a default.
	m := shortcut.Map{shortcut.PlayPause: {Modifiers: shortcut.Shift, Key: shortcut.KeyComma}}
	var conflict *shortcut.ConflictError
	if err := m.Validate(); !errors.As(err, &conflict) {
		t.Fatalf("Validate = %v, want a conflict", err)
	}
	if conflict.First != shortcut.PlayPause || conflict.Second != shortcut.SyncPrimaryEarlier || conflict.Chord.String() != "Shift+Comma" {
		t.Fatalf("conflict = %+v", conflict)
	}
	// Swapping two shortcuts is no conflict.
	swapped := shortcut.Map{shortcut.Mute: {Key: shortcut.KeyF}, shortcut.Fullscreen: {Key: shortcut.KeyM}}
	if err := swapped.Validate(); err != nil {
		t.Fatalf("a swap = %v", err)
	}
	if command, found := swapped.Find(shortcut.Chord{Key: shortcut.KeyF}); !found || command != shortcut.Mute {
		t.Fatalf("Find(F) = %s %v", command, found)
	}
	if _, found := swapped.Find(shortcut.Chord{Key: "KeyQ"}); found {
		t.Fatal("Find(Q) found a command")
	}
}

func TestValidateRefusesUnknownCommandsAndBadChords(t *testing.T) {
	if err := (shortcut.Map{"rewind-everything": {Key: "KeyR"}}).Validate(); !errors.Is(err, shortcut.ErrUnknownCommand) {
		t.Fatalf("unknown command = %v", err)
	}
	if err := (shortcut.Map{shortcut.Mute: {Modifiers: shortcut.Alt | shortcut.Shift, Key: "KeyM"}}).Validate(); !errors.Is(err, shortcut.ErrAltShift) {
		t.Fatalf("Alt+Shift in a map = %v", err)
	}
	if err := (shortcut.Map{shortcut.Mute: {Modifiers: shortcut.Ctrl}}).Validate(); !errors.Is(err, shortcut.ErrNoKey) {
		t.Fatalf("modifiers without a key in a map = %v", err)
	}
}

func TestResolvedFillsTheMissingCommandsAndDropsUnknownOnes(t *testing.T) {
	m := shortcut.Map{shortcut.Mute: {Key: "KeyU"}, "gone": {Key: "KeyG"}}
	resolved := m.Resolved()
	if resolved[shortcut.Mute].Key != "KeyU" || resolved[shortcut.PlayPause].Key != shortcut.KeySpace {
		t.Fatalf("resolved = %+v", resolved)
	}
	if _, found := resolved["gone"]; found || len(resolved) != len(shortcut.Commands()) {
		t.Fatalf("resolved has %d entries: %+v", len(resolved), resolved)
	}
	if (shortcut.Map(nil)).Of(shortcut.ReplayLine).Key != shortcut.KeyBackspace {
		t.Fatal("a nil map has no defaults")
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"Ctrl+Shift+Comma", "AltRight", "Alt+F4", "+", "", "Shift+Alt+KeyA"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		chord, err := shortcut.Parse(text)
		if err != nil {
			return
		}
		// Whatever is accepted is valid and reads back the same.
		if chord.Validate() != nil {
			t.Fatalf("Parse(%q) accepted an invalid chord", text)
		}
		again, err := shortcut.Parse(chord.String())
		if err != nil || again != chord {
			t.Fatalf("round trip of %q: %+v %v", text, again, err)
		}
	})
}
