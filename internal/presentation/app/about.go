package app

import (
	"github.com/ediejohnny/lingoreel/internal/presentation/notices"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// AboutView is what the About card of the settings' Help page tells of the app.
type AboutView struct {
	Version string `json:"version"`
}

// About tells the app's version, for the About card.
func (a *API) About() AboutView {
	return AboutView{Version: cmpOr(a.deps.About.Version, ptbr.T("diagnostics.unknownVersion"))}
}

// ThirdPartyNotices is the licenses of the code from others built into the
// app, which the About card shows as their authors ask. They are long, so the
// card asks for them only when they are to be read.
func (a *API) ThirdPartyNotices() string {
	return notices.ThirdParty()
}
