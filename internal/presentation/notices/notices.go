// Package notices carries the licenses of the code from others built into the
// app: Go, its modules, the screens' packages and the voice model. Their
// authors ask for them to go with every copy, and the app ships as a single
// file, so they go inside it. scripts\update-notices.ps1 writes them, and the
// check refuses them when they no longer name what the app is built from.
package notices

import _ "embed"

//go:embed third-party.txt
var thirdParty string

// ThirdParty is the notices' text: each license as its authors wrote it,
// under the name and version of what it covers.
func ThirdParty() string {
	return thirdParty
}
