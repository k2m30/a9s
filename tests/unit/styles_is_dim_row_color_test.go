package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// TestIsDimRowColor verifies styles.IsDimRowColor(status) per §7.1 of
// docs/design/ct-event-list-v2.md.
//
// A row is "dim" when RowColorStyle(status) resolves to ColTerminated or
// ColHeaderFg (the neutral fall-through).  All other colors carry severity
// signal and must return false.
func TestIsDimRowColor(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		// ct-events severity ladder (§1.1) — added by P2 coder to rowColorCache
		{"ct-info", true},       // ColTerminated (dim)
		{"ct-attention", false}, // ColPending (yellow)
		{"ct-danger", false},    // ColStopped (red)

		// ec2 lifecycle
		{"running", false},       // ColRunning (green)
		{"stopped", false},       // ColStopped (red) — visible per §7.4
		{"terminated", true},     // ColTerminated (dim) — hidden per §7.4
		{"shutting-down", true},  // ColTerminated (dim) — hidden per §7.4
		{"pending", false},       // ColPending (yellow)
		{"stopping", false},      // ColPending (yellow)

		// default fall-through (no cache hit, no suffix match)
		{"", true},               // ColHeaderFg (neutral) — dim
		{"unknown-status", true}, // ColHeaderFg (neutral) — dim

		// CloudFormation suffix patterns (handled via HasSuffix branches)
		{"CREATE_IN_PROGRESS", false}, // _in_progress → ColPending (yellow)
		{"CREATE_COMPLETE", false},    // _complete → ColRunning (green)
		{"UPDATE_FAILED", false},      // _failed → ColStopped (red)
	}

	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			got := styles.IsDimRowColor(c.status)
			if got != c.want {
				t.Errorf("IsDimRowColor(%q) = %v, want %v", c.status, got, c.want)
			}
		})
	}
}
