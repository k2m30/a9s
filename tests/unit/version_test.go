package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/buildinfo"
)

func TestResolveVersion_LdflagsSet(t *testing.T) {
	// When ldflags set a real version, it should be returned as-is
	got := buildinfo.ResolveVersion("1.2.3")
	if got != "1.2.3" {
		t.Errorf("ResolveVersion(\"1.2.3\") = %q, want \"1.2.3\"", got)
	}
}

func TestResolveVersion_DevFallsBackToBuildInfo(t *testing.T) {
	// When version is "dev", it should fall back to debug.BuildInfo
	// In test context BuildInfo returns "(devel)" so it stays "dev"
	got := buildinfo.ResolveVersion("dev")
	// In test binary, module version is "(devel)", so fallback can't help — stays "dev"
	if got == "" {
		t.Error("ResolveVersion(\"dev\") returned empty string")
	}
}

func TestResolveVersion_EmptyInput(t *testing.T) {
	got := buildinfo.ResolveVersion("")
	if got == "" {
		t.Error("ResolveVersion(\"\") returned empty string")
	}
}
