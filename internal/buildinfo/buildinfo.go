// Package buildinfo exposes metadata injected at build time.
package buildinfo

// Version is the semantic version of the build. Release builds set it with
// -ldflags "-X lingoreel/internal/buildinfo.Version=<version>".
var Version = "0.0.0-dev"
