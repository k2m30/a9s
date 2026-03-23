package buildinfo

import (
	"runtime/debug"
	"strings"
)

// ResolveVersion returns the version string without a leading "v" prefix.
// The header renderer adds its own "v" prefix, so the version must be bare.
// If v is set by ldflags (anything other than "dev" or ""), it is returned
// (stripped of leading "v"). Otherwise, falls back to the module version
// embedded by go install.
func ResolveVersion(v string) string {
	if v != "dev" && v != "" {
		return strings.TrimPrefix(v, "v")
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	if v == "" {
		return "dev"
	}
	return v
}
