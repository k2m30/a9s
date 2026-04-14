package unit

// Tests for internal/resource/lifecycle.go — StandardLifecycleColor.
// Covers the common AWS lifecycle vocabulary and edge cases.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestStandardLifecycleColor(t *testing.T) {
	cases := []struct {
		status string
		want   resource.Color
	}{
		// Healthy — lowercase
		{"active", resource.ColorHealthy},
		{"available", resource.ColorHealthy},
		// Healthy — uppercase
		{"ACTIVE", resource.ColorHealthy},
		{"AVAILABLE", resource.ColorHealthy},

		// Warning / transitioning — lowercase
		{"creating", resource.ColorWarning},
		{"updating", resource.ColorWarning},
		{"deleting", resource.ColorWarning},
		{"modifying", resource.ColorWarning},
		// Warning — uppercase
		{"CREATING", resource.ColorWarning},
		{"UPDATING", resource.ColorWarning},
		{"DELETING", resource.ColorWarning},

		// Broken — lowercase
		{"failed", resource.ColorBroken},
		{"error", resource.ColorBroken},
		// Broken — uppercase
		{"FAILED", resource.ColorBroken},
		{"ERROR", resource.ColorBroken},

		// Dim — lowercase
		{"deleted", resource.ColorDim},
		{"inactive", resource.ColorDim},
		// Dim — uppercase
		{"DELETED", resource.ColorDim},
		{"INACTIVE", resource.ColorDim},

		// Unknown status — falls back to ColorHealthy (function default)
		{"unknown", resource.ColorHealthy},
		{"", resource.ColorHealthy},
		{"running", resource.ColorHealthy},
		{"stopped", resource.ColorHealthy},
		{"PENDING", resource.ColorHealthy},
	}

	for _, tc := range cases {
		got := resource.StandardLifecycleColor(tc.status)
		if got != tc.want {
			t.Errorf("StandardLifecycleColor(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestStandardLifecycleColor_IsIssue(t *testing.T) {
	// ColorWarning and ColorBroken are "issues" (feed attention filter & badge)
	issueCases := []string{"creating", "updating", "deleting", "modifying", "failed", "error",
		"CREATING", "UPDATING", "DELETING", "FAILED", "ERROR"}
	for _, s := range issueCases {
		c := resource.StandardLifecycleColor(s)
		if !c.IsIssue() {
			t.Errorf("StandardLifecycleColor(%q).IsIssue() = false, want true", s)
		}
	}

	// ColorHealthy and ColorDim are NOT issues
	nonIssueCases := []string{"active", "available", "deleted", "inactive", "unknown", ""}
	for _, s := range nonIssueCases {
		c := resource.StandardLifecycleColor(s)
		if c.IsIssue() {
			t.Errorf("StandardLifecycleColor(%q).IsIssue() = true, want false", s)
		}
	}
}

func TestStandardLifecycleColor_CaseSensitivity(t *testing.T) {
	// Mixed case is normalized to lowercase — these return the same color as their
	// lowercase equivalents.
	mixedCases := []struct {
		status string
		want   resource.Color
	}{
		{"Active", resource.ColorHealthy},   // lowercased → "active" → ColorHealthy
		{"Failed", resource.ColorBroken},    // lowercased → "failed" → ColorBroken
		{"Creating", resource.ColorWarning}, // lowercased → "creating" → ColorWarning
		{"Deleted", resource.ColorDim},      // lowercased → "deleted" → ColorDim
	}
	for _, tc := range mixedCases {
		got := resource.StandardLifecycleColor(tc.status)
		if got != tc.want {
			t.Errorf("StandardLifecycleColor(%q) = %v, want %v (mixed case should be normalized)", tc.status, got, tc.want)
		}
	}
}
