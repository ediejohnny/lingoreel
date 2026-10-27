package watch

import (
	"slices"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// SyncShortcut moves one subtitle line earlier or later.
type SyncShortcut struct {
	Line  session.Line
	Later bool
}

// SyncShortcuts are the four sync shortcuts, in the order their presses
// refer to: the line on top earlier and later, then the one below.
var SyncShortcuts = []SyncShortcut{
	{session.LinePrimary, false}, {session.LinePrimary, true},
	{session.LineTranslation, false}, {session.LineTranslation, true},
}

// syncCommands are the commands of SyncShortcuts, in the same order.
var syncCommands = []shortcut.Command{
	shortcut.SyncPrimaryEarlier, shortcut.SyncPrimaryLater,
	shortcut.SyncTranslationEarlier, shortcut.SyncTranslationLater,
}

// globalCommands are the commands registered system-wide while the player is
// in the foreground: the sync steps and the passage marks, which must work
// with a player that owns its window (PotPlayer) as well as with the app's.
var globalCommands = append(slices.Clone(syncCommands), shortcut.PassageMarkStart, shortcut.PassageMarkEnd)

// GlobalCommands lists the system-wide commands, in the order their presses
// refer to.
func GlobalCommands() []shortcut.Command {
	return slices.Clone(globalCommands)
}

// GlobalChords are the chords of GlobalCommands in a set of shortcuts.
func GlobalChords(keys shortcut.Map) []shortcut.Chord {
	chords := make([]shortcut.Chord, len(globalCommands))
	for i, command := range globalCommands {
		chords[i] = keys.Of(command)
	}
	return chords
}

// Press is one press of GlobalCommands()[Index]; Repeat tells the key is
// held.
type Press struct {
	Index  int
	Repeat bool
}

// Shortcuts delivers the presses of the global shortcuts while the player is
// in the foreground. The Windows hotkeys adapter implements it.
type Shortcuts interface {
	Presses() <-chan Press
	Close() error
}

// ShortcutTarget is the player the sync shortcuts follow: the app's window
// it draws into, or else (Window zero) any window of its process.
type ShortcutTarget struct {
	ProcessID int
	Window    uintptr
}

// listenShortcuts runs the presses of one set of global shortcuts until it
// is closed: at the end of the session, or when the chords change. A held
// key repeats only the commands made to repeat: holding Ctrl+F10 saves one
// passage, not one per repeat.
func (s *Session) listenShortcuts(shortcuts Shortcuts) {
	for press := range shortcuts.Presses() {
		if press.Index < 0 || press.Index >= len(globalCommands) {
			continue
		}
		command := globalCommands[press.Index]
		if press.Repeat && !command.Repeats() {
			continue
		}
		_ = s.Run(command, press.Repeat) // told on the video when it goes wrong
	}
}

// nudge moves a line one step and tells it with the track the step moved,
// read in step with the offset: a switch of the line's subtitle never puts
// one file's offset under the other's number.
func (s *Session) nudge(line session.Line, later, repeat bool) (subtitle.Offset, bool) {
	s.lineMu.Lock()
	offset, ok := s.sync.Nudge(line, later, repeat)
	track := s.Tracks().Number(line)
	s.lineMu.Unlock()
	if ok {
		s.synced(line, track, offset)
	}
	return offset, ok
}

func (s *Session) synced(line session.Line, track uint64, offset subtitle.Offset) {
	if notice := *s.notice.Load(); s.display != nil && notice != nil && !s.quiet.Load() {
		_ = s.display.Notify(notice(line, offset)) // a lost notice is cosmetic
	}
	if s.listener != nil {
		s.listener.Synced(line, track, offset)
	}
}

// Nudge moves a line one step, as its sync shortcut does.
func (s *Session) Nudge(line session.Line, later bool) (subtitle.Offset, bool) {
	return s.nudge(line, later, false)
}

// ResetSync puts a line back at its file's timing.
func (s *Session) ResetSync(line session.Line) {
	s.lineMu.Lock()
	s.sync.Reset(line)
	offset, track := s.subtitles.Offset(line), s.Tracks().Number(line)
	s.lineMu.Unlock()
	s.synced(line, track, offset)
}

// Offset is a line's own offset, the user's.
func (s *Session) Offset(line session.Line) subtitle.Offset {
	return s.subtitles.Offset(line)
}

// VoiceShift is what keeping the study line on the voice adds to its own
// offset now; zero when nothing.
func (s *Session) VoiceShift() time.Duration {
	if track := s.subtitles.Track(session.LinePrimary); track != nil {
		return track.Voice
	}
	return 0
}
