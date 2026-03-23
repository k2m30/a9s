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

// ResolveCommit returns the commit hash. If c is already set by ldflags
// (not "none" or ""), returns it. Otherwise reads vcs.revision from build info.
func ResolveCommit(c string) string {
	if c != "none" && c != "" {
		return c
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				if len(s.Value) > 12 {
					return s.Value[:12]
				}
				return s.Value
			}
		}
	}
	return c
}

// ResolveDate returns the build date. If d is already set by ldflags
// (not "unknown" or ""), returns it. Otherwise reads vcs.time from build info.
func ResolveDate(d string) string {
	if d != "unknown" && d != "" {
		return d
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.time" && s.Value != "" {
				return s.Value
			}
		}
	}
	return d
}
