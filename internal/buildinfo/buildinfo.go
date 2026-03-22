package buildinfo

import "runtime/debug"

// ResolveVersion returns the version string. If v is set by ldflags (anything
// other than "dev" or ""), it is returned as-is. Otherwise, falls back to the
// module version embedded by go install.
func ResolveVersion(v string) string {
	if v != "dev" && v != "" {
		return v
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	if v == "" {
		return "dev"
	}
	return v
}
