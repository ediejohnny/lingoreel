package session

import (
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// Screen is what the subtitle display shows at one moment.
type Screen struct {
	Primary     string
	Translation string
	// TranslationVisible hides the translation without freeing its space,
	// so the primary line never moves when it is revealed.
	TranslationVisible bool
	// PrimaryHidden hides the line on top, as the ladder of "Estudar
	// trecho" does in its last laps.
	PrimaryHidden bool
	// Cue is set when the primary line is shown before its time.
	Cue *CueScreen
	// Fill is the share of the primary line's characters already spoken.
	Fill float64
	// Highlighted marks the Highlight line as the one being edited, with a
	// discreet outline.
	Highlighted bool
	Highlight   Line
}

// CueScreen is how a preview shows the time left.
type CueScreen struct {
	Indicator study.Indicator
	// Progress goes from 1 when the preview begins to 0 at the line.
	Progress float64
	// Seconds is the countdown, in whole seconds of real time.
	Seconds int
}

// Look is how a display draws the subtitles and the marks around them.
type Look struct {
	Layout      appearance.Layout
	Primary     appearance.SubtitleStyle
	Translation appearance.SubtitleStyle
	// PreviewColor and FillColor are the Modo Karaokê's colors: a line
	// shown before its time, and the part of a line already spoken. Cue is
	// how the time left before such a line shows.
	PreviewColor, FillColor appearance.Color
	Cue                     appearance.CueStyle
	// Notice is how a notice looks, and NoticeDuration how long it stays.
	Notice         appearance.NoticeLook
	NoticeDuration time.Duration
	// Selection frames the line being corrected.
	Selection appearance.SelectionLook
}

// Restyler is a display that can change its look while it shows, so a
// change in the settings reaches the video at once.
type Restyler interface {
	Restyle(look Look) error
}

// Display draws the subtitles over the video. The native overlay implements
// it; the session only decides what to show.
type Display interface {
	// Show replaces what is on screen without waiting for the drawing.
	Show(screen Screen) error
	// Notify shows a short notice in a corner of the video for a moment,
	// such as which subtitle a sync shortcut moved; a new notice replaces
	// the one on screen. It does not wait for the drawing.
	Notify(text string) error
	// Done is closed when the player's window goes away.
	Done() <-chan struct{}
	Close() error
}
