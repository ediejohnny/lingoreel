package spdev

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/infrastructure/windows/hotkeys"
	"lingoreel/internal/infrastructure/windows/win32"
)

// syncShortcut moves one line earlier or later.
type syncShortcut struct {
	binding hotkeys.Binding
	line    session.Line
	later   bool
}

// syncShortcuts are the default sync shortcuts: Shift for the line on top
// (the primary), Ctrl for the one below (the translation); comma moves it
// earlier and period later, matching "<" and ">" on the same keys.
var syncShortcuts = []syncShortcut{
	{hotkeys.Binding{Modifiers: win32.MOD_SHIFT, Key: win32.VK_OEM_COMMA}, session.LinePrimary, false},
	{hotkeys.Binding{Modifiers: win32.MOD_SHIFT, Key: win32.VK_OEM_PERIOD}, session.LinePrimary, true},
	{hotkeys.Binding{Modifiers: win32.MOD_CONTROL, Key: win32.VK_OEM_COMMA}, session.LineTranslation, false},
	{hotkeys.Binding{Modifiers: win32.MOD_CONTROL, Key: win32.VK_OEM_PERIOD}, session.LineTranslation, true},
}

// languageNames are the languages named in the notices.
var languageNames = map[string]string{
	"en": "inglês", "pt": "português", "es": "espanhol", "fr": "francês",
	"de": "alemão", "it": "italiano", "ja": "japonês", "ko": "coreano", "zh": "chinês",
}

// lineNames describe each line by its place on screen and its language,
// as in "Legenda de cima (inglês)".
func lineNames(info media.Info, primary, translation uint64) [2]string {
	name := func(place string, number uint64) string {
		track, found := info.Track(number)
		if !found || track.Language == "" {
			return place
		}
		language, known := languageNames[track.Language.Base()]
		if !known {
			language = string(track.Language)
		}
		return fmt.Sprintf("%s (%s)", place, language)
	}
	return [2]string{name("Legenda de cima", primary), name("Legenda de baixo", translation)}
}

// formatOffset shows an offset as "+0,150 s" or "−0,150 s".
func formatOffset(offset subtitle.Offset) string {
	text := strings.Replace(fmt.Sprintf("%+.3f s", offset.Duration().Seconds()), ".", ",", 1)
	return strings.Replace(text, "-", "−", 1)
}

// listenSync moves the lines with the sync shortcuts while the player is in
// the foreground, and with "z" + Enter on the console, which puts both back
// at their files' timing. Each change is printed and noticed on the video.
// It returns when the context ends; stop releases the shortcuts.
func listenSync(ctx context.Context, processID int, sync *session.Sync, names [2]string, screen session.Display, in io.Reader, out *printer) (stop func() error, err error) {
	bindings := make([]hotkeys.Binding, len(syncShortcuts))
	for i, shortcut := range syncShortcuts {
		bindings[i] = shortcut.binding
	}
	keys, err := hotkeys.Start(hotkeys.Options{
		ProcessID: processID, Bindings: bindings,
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	})
	if err != nil {
		return nil, err
	}
	report := func(line session.Line, offset subtitle.Offset) {
		text := fmt.Sprintf("%s: %s", names[line], formatOffset(offset))
		out.line("Sincronia: %s", text)
		if screen != nil {
			if err := screen.Notify(text); err != nil {
				out.line("Aviso: o overlay não mostrou o aviso: %v", err)
			}
		}
	}
	go func() {
		for press := range keys.Presses() {
			shortcut := syncShortcuts[press.Index]
			if offset, ok := sync.Nudge(shortcut.line, shortcut.later, press.Repeat); ok {
				report(shortcut.line, offset)
			}
		}
	}()
	if in != nil {
		go readConsole(ctx, in, func(command string) {
			if command == resetCommand {
				for _, line := range []session.Line{session.LinePrimary, session.LineTranslation} {
					sync.Reset(line)
					report(line, 0)
				}
			}
		})
	}
	return keys.Close, nil
}

// resetCommand on the console puts both lines back at their files' timing.
const resetCommand = "z"

// readConsole calls run with each line typed on the console, trimmed and in
// lower case, until the context ends or the input closes. A pending read of
// the console cannot be interrupted: the goroutine ends with the process.
func readConsole(ctx context.Context, in io.Reader, run func(string)) {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		run(strings.ToLower(strings.TrimSpace(scanner.Text())))
	}
}
