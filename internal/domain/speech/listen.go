package speech

// listenWarmUp is how many frames the Listener hears, after it starts or a
// seek, before it can be sure: the model needs a few chunks of the new
// sound to forget the old.
const listenWarmUp = 10

// Listener tells, frame by frame, whether someone is speaking, while the
// sound is only heard as it plays.
type Listener struct {
	threshold float32
	on        bool
	heard     int
}

// NewListener hears with a sensitivity.
func NewListener(sensitivity Sensitivity) *Listener {
	return &Listener{threshold: sensitivity.threshold()}
}

// Reset forgets the sound heard, which says nothing about what comes after
// a seek.
func (l *Listener) Reset() {
	l.on, l.heard = false, 0
}

// Hear takes the next frame. Voice is whether someone speaks in it; sure,
// whether enough was heard to tell (when not, the estimate stays).
func (l *Listener) Hear(frame Frame) (voice, sure bool) {
	l.on = hears(frame, l.threshold, l.on)
	if l.heard < listenWarmUp {
		l.heard++
		return false, false
	}
	return l.on, true
}
