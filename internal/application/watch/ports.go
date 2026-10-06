package watch

// The pieces a session works with: what the window drives, the outside
// parts it is given, what it is asked to watch, and who hears it.

import (
	"context"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/players"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Remote is what the controls on the player window drive: the same
// session the app's panel drives, so both always agree.
type Remote interface {
	State() playback.Sample
	Controls() *session.Controls
	HasTranslation() bool
	TranslationVisible() bool
	SetTranslationVisible(visible bool)
	// Mode is the study mode; when it always shows the translation, the
	// translation cannot be hidden.
	Mode() study.Mode
	// Run carries out a shortcut's command; repeat tells the key is held.
	Run(command shortcut.Command, repeat bool) error
	// Reveal is the translation key going down (pressed) or up; a held
	// key's repeats are not presses.
	Reveal(pressed bool)
	// Select picks a line on screen for editing; the video must be paused.
	Select(line session.Line) (Selection, error)
	// Edit changes the selected line, as the box over the video corrects
	// its text; the video must be paused.
	Edit(change EditChange) error

	// What the bar's menus show and change: the passage being marked or
	// studied and its options, the saved passages, the Modo Karaokê, the
	// study mode and the sync.
	MarkedStart() (time.Duration, bool)
	Passage() session.PracticeStatus
	PassageOptions() (study.PracticePlan, bool)
	SetPassageOptions(plan study.PracticePlan, snap bool) error
	SavedPassages() []SavedPassage
	StudySavedAt(index int) error
	Karaoke() session.Karaoke
	SetKaraokeSettings(karaoke session.Karaoke) error
	Indicator() study.Indicator
	SetIndicator(indicator study.Indicator) error
	LineRepeats() int
	SetLineRepeats(times int) error
	SetMode(mode study.Mode) error
	HasPrimary() bool
	ResetSync(line session.Line)
	// OpenEditor pauses, picks the line on screen and brings the app's
	// panel, where the editor is, to the front.
	OpenEditor(line session.Line) error

	// What the menu over the video shows besides: the lines on screen (the
	// one under a click is copied), each line's offset, what can be undone,
	// and the tracks to switch between.
	Shown() session.Visible
	Offset(line session.Line) subtitle.Offset
	CanUndo() bool
	CanRedo() bool
	Tracks() Tracks
}

// Window is the app's own window a player draws its video into. It shows
// the subtitles (it is the session's display) and its own controls over the
// video, and closing it by any means ends the session.
type Window interface {
	session.Display
	// Handle is the window the player draws into.
	Handle() uintptr
	// Closed is closed when the user closes the window.
	Closed() <-chan struct{}
	// Attach hands the window the session its controls drive, once the
	// player shows the video.
	Attach(remote Remote)
	// SetShortcuts changes the keys the window takes.
	SetShortcuts(keys shortcut.Map)
}

// WindowRequest describes the player window to open.
type WindowRequest struct {
	// Title names the video, usually its file name.
	Title     string
	Placement player.Placement
	Info      media.Info
}

// Ports are the outside pieces a session uses; nil ones are left out.
type Ports struct {
	Service  *analysis.Service
	Registry *players.Registry
	// OpenWindow opens the app's window for players that draw into one
	// (player.Descriptor.DrawsInHost); nil lets every player open its own.
	OpenWindow func(ctx context.Context, request WindowRequest) (Window, error)
	// OpenDisplay puts the subtitle display over a player's own window; nil
	// shows nothing (the development tool prints instead).
	OpenDisplay func(ctx context.Context, running players.Running, info media.Info) (session.Display, error)
	// StartShortcuts listens to GlobalCommands, on the given chords, while
	// the player is in the foreground; nil leaves the shortcuts off.
	StartShortcuts func(target ShortcutTarget, chords []shortcut.Chord) (Shortcuts, error)
	// WindowHearsKeys tells that the window OpenWindow opens takes every key
	// itself, sync ones too (it is part of the app's window, whose panel
	// takes them as well): its sessions start no system-wide shortcuts,
	// which would take "<" from the panel's fields. A player in a window of
	// its own keeps them.
	WindowHearsKeys bool
	// PracticeNotice words what happens to a passage being studied for a
	// notice on the video; nil shows none.
	PracticeNotice func(session.PracticeEvent, session.PracticeStatus) string
	// MarkNotice words what a passage mark did for a notice on the video;
	// nil shows none.
	MarkNotice func(MarkEvent, study.Segment) string
	// VolumeNotice words the volume after a volume key or mute, with the
	// player's ceiling, for a notice on the video; nil shows none.
	VolumeNotice func(volume, ceiling playback.Volume, muted bool) string
	// ShowPanel brings the app's panel to the front, where the line editor
	// is; nil leaves it where it is.
	ShowPanel func()
	// Tap hears the player's sound, so the Modo Karaokê paints each line
	// along the actor's voice; nil leaves the estimate from its letters.
	Tap voice.Tap
	// FindDecoder looks for the user's FFmpeg, to decode the voice ahead of
	// the video; nil, or a nil answer, hears the player instead.
	FindDecoder func(ctx context.Context) voice.Decoder
	Now         func() time.Time
}

// Request is what to watch and how.
type Request struct {
	Analysis analysis.Analysis
	// Tracks are the chosen audio, primary and translation tracks; zero
	// means none (or the player's choice for audio).
	Tracks media.Selection
	Player player.Installation
	Start  time.Duration
	Paused bool
	// Window is where the player's window opens; zero leaves it to the player.
	Window player.Placement
	Steps  subtitle.NudgeSteps
	// Mode is the study mode, which decides whether the translation starts
	// hidden; empty is ModeDual.
	Mode study.Mode
	// PrecisionStep is the jump of the precision shortcuts; zero is
	// DefaultPrecisionStep.
	PrecisionStep time.Duration
	// Saved is what the project keeps for each line: its offset and the
	// user's edits, applied when they still fit the file.
	Saved map[session.Line]project.Track
	// Karaoke is the Modo Karaokê to start with, its indicator and how many
	// times each line plays again (0 is off).
	Karaoke     session.Karaoke
	Indicator   study.Indicator
	LineRepeats int
	// Speed is the playback speed to start with; zero keeps the player's.
	Speed playback.Speed
	// Shortcuts are the keys of the commands; Reveal is how the translation
	// key works.
	Shortcuts shortcut.Map
	Reveal    study.RevealBehavior
	// NoSyncNotice leaves out the notice of a sync change on the video.
	NoSyncNotice bool
	// Passage, when set, is studied as soon as the player shows the video:
	// the session opens at its start. Start is then ignored.
	Passage *PassageStart
	// PassagePlan is how a passage closed by the mark shortcut is studied,
	// until the user picks other options; the zero plan pauses at the end.
	PassagePlan study.PracticePlan
}

// PassageStart is a passage to study from the start of a session.
type PassageStart struct {
	Segment study.Segment
	Plan    study.PracticePlan
}

// Listener hears what happens in a session. Its methods run on the
// session's goroutines and must return quickly.
type Listener interface {
	// SubtitlesChanged reports new lines on screen at a playback position.
	SubtitlesChanged(position time.Duration, visible session.Visible)
	// Synced reports a line's new offset after a sync shortcut or a reset,
	// with the track the line shows (its subtitle may be switched meanwhile).
	Synced(line session.Line, track uint64, offset subtitle.Offset)
	// Edited reports a line's edits, as the project keeps them, after an
	// edit, an undo or a redo, with the track they were made on.
	Edited(line session.Line, track uint64, edits map[string]project.Edit)
	// PassageMarked reports a passage closed by the mark shortcut, already
	// being studied with plan, so it can be saved.
	PassageMarked(segment study.Segment, plan study.PracticePlan)
}
