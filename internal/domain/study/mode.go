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
