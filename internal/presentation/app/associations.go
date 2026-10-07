package app

import "github.com/ediejohnny/lingoreel/internal/presentation/ptbr"

var (
	errNoAssociations = worded(func() string { return ptbr.T("error.association.unavailable") })
	errAssociation    = worded(func() string { return ptbr.T("error.association.failed") })
)

// FileAssociation tells whether Windows offers the app for videos.
func (a *API) FileAssociation() (bool, error) {
	if a.deps.Associations == nil {
		return false, nil
	}
	on, err := a.deps.Associations.Enabled()
	if err != nil {
		a.logger.Warn("the file association could not be read", "error", err)
		return false, errAssociation
	}
	return on, nil
}

// SetFileAssociation puts the app in "Abrir com" and the videos' context
// menu, or takes it out. Only the current user's settings change, and the
// default program of each kind of video stays.
func (a *API) SetFileAssociation(on bool) error {
	if a.deps.Associations == nil {
		return errNoAssociations
	}
	change := a.deps.Associations.Disable
	if on {
		change = a.deps.Associations.Enable
	}
	if err := change(); err != nil {
		a.logger.Warn("the file association did not change", "on", on, "error", err)
		return a.because(errAssociation, err)
	}
	a.logger.Info("file association changed", "on", on)
	return nil
}
