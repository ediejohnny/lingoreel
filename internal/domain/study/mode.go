// Package study models the learning features, starting with how the
// translation line is shown and revealed.
package study

// Mode decides when the translation line is visible.
type Mode string

const (
	// ModeDual shows both lines; the reveal key can still hide the translation.
	ModeDual Mode = "dual"
	// ModeOnDemand hides the translation until the reveal key is used. It is
	// the recommended mode for active study.
	ModeOnDemand Mode = "on-demand"
	// ModeAlwaysTranslated shows both lines and ignores the reveal key.
	ModeAlwaysTranslated Mode = "always"
)

// Valid reports whether the mode is known.
func (m Mode) Valid() bool {
	return m == ModeDual || m == ModeOnDemand || m == ModeAlwaysTranslated
}

// Modes lists the modes in the order the screens offer them.
func Modes() []Mode {
	return []Mode{ModeDual, ModeOnDemand, ModeAlwaysTranslated}
}

// StartsRevealed tells whether the translation shows when a video opens.
func (m Mode) StartsRevealed() bool {
	return m != ModeOnDemand
}

// Hideable tells whether the translation can be hidden at all: never when
// it is always shown.
func (m Mode) Hideable() bool {
	return m != ModeAlwaysTranslated
}

// Reveal is whether the translation shows after the translation key goes
// down (pressed) or up, given whether it shows now: in dual mode the key
// hides and shows it in turns, on demand it follows the reveal behavior,
// and when always shown the key changes nothing.
func (m Mode) Reveal(behavior RevealBehavior, visible, pressed bool) bool {
	switch m {
	case ModeAlwaysTranslated:
		return true
	case ModeOnDemand:
		return behavior.Revealed(visible, pressed)
	}
	return RevealToggle.Revealed(visible, pressed)
}

// RevealBehavior decides how the reveal key works in on-demand mode.
type RevealBehavior string

const (
	// RevealToggle shows the translation on one press and hides it on the next.
	RevealToggle RevealBehavior = "toggle"
	// RevealHold shows the translation only while the key is held down.
	RevealHold RevealBehavior = "hold"
)

// Valid reports whether the behavior is known.
func (r RevealBehavior) Valid() bool {
	return r == RevealToggle || r == RevealHold
}

// Revealed is whether the translation shows after the reveal key goes down
// (pressed) or up, given whether it shows now. Toggling flips it on each
// press and ignores the release; holding shows it only while pressed.
// Repeats of a held key are not presses.
func (r RevealBehavior) Revealed(visible, pressed bool) bool {
	if r == RevealHold {
		return pressed
	}
	if pressed {
		return !visible
	}
	return visible
}
