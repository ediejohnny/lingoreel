package session

// Screen is what the subtitle display shows at one moment.
type Screen struct {
	Primary     string
	Translation string
	// TranslationVisible hides the translation without freeing its space,
	// so the primary line never moves when it is revealed.
	TranslationVisible bool
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
