package unit

// qa_menu_attention_filter_test.go — T040: visibility logic tests for ctrl+z
// on the main menu.
//
// The decision tree: unknown→visible, zero (any)→hidden, nonzero→visible.
// Truncated-zero is treated as hidden because config-only resource types
// (S3, ENI, IAM, etc.) will never have issues regardless of page count.

import (
	"testing"
)

// isVisibleUnderIssueFilter encodes the visibility rule for a single
// resource type when the ctrl+z attention filter is active:
//
//   - unknown (not yet probed) → visible (conservative: don't hide if unsure)
//   - known + count > 0        → visible (has issues)
//   - known + count == 0       → hidden (regardless of truncation)
func isVisibleUnderIssueFilter(known, _ bool, count int) bool {
	if !known {
		return true // unknown → visible
	}
	if count > 0 {
		return true // has issues → visible
	}
	return false // zero issues → hidden
}

// TestMainMenuQuadStateVisibility_Unknown verifies that a type with no probed
// data is visible (conservative default — don't hide unknown types).
func TestMainMenuQuadStateVisibility_Unknown(t *testing.T) {
	visible := isVisibleUnderIssueFilter(false, false, 0)
	if !visible {
		t.Error("unknown type should be visible under issue filter")
	}
}

// TestMainMenuQuadStateVisibility_ZeroConfirmed verifies that a type with
// confirmed zero issues is hidden.
func TestMainMenuQuadStateVisibility_ZeroConfirmed(t *testing.T) {
	visible := isVisibleUnderIssueFilter(true, false, 0)
	if visible {
		t.Error("confirmed zero-issue type should be hidden under issue filter")
	}
}

// TestMainMenuQuadStateVisibility_ZeroTruncated verifies that a type with
// zero issues is hidden even when truncated — config-only types never have issues.
func TestMainMenuQuadStateVisibility_ZeroTruncated(t *testing.T) {
	visible := isVisibleUnderIssueFilter(true, true, 0)
	if visible {
		t.Error("truncated zero count should be hidden — config-only types never have issues")
	}
}

// TestMainMenuQuadStateVisibility_Nonzero verifies that a type with at least one
// issue is visible.
func TestMainMenuQuadStateVisibility_Nonzero(t *testing.T) {
	for _, count := range []int{1, 2, 10, 100} {
		visible := isVisibleUnderIssueFilter(true, false, count)
		if !visible {
			t.Errorf("count=%d: type with issues should be visible under issue filter", count)
		}
	}
}

// TestMainMenuQuadStateVisibility_NonzeroTruncated verifies that a type with
// nonzero truncated count is also visible.
func TestMainMenuQuadStateVisibility_NonzeroTruncated(t *testing.T) {
	visible := isVisibleUnderIssueFilter(true, true, 5)
	if !visible {
		t.Error("truncated nonzero count should be visible")
	}
}

// TestMainMenuQuadStateVisibility_TableDriven is a comprehensive table-driven
// test of all four quad-state cases.
func TestMainMenuQuadStateVisibility_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		known     bool
		truncated bool
		count     int
		wantVis   bool
	}{
		{"unknown/zero/not-truncated", false, false, 0, true},
		{"unknown/zero/truncated", false, true, 0, true},
		{"unknown/nonzero", false, false, 3, true},
		{"known/zero/confirmed", true, false, 0, false},
		{"known/zero/truncated", true, true, 0, false},
		{"known/nonzero/not-truncated", true, false, 1, true},
		{"known/nonzero/truncated", true, true, 5, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isVisibleUnderIssueFilter(tc.known, tc.truncated, tc.count)
			if got != tc.wantVis {
				t.Errorf("isVisibleUnderIssueFilter(known=%v, truncated=%v, count=%d) = %v, want %v",
					tc.known, tc.truncated, tc.count, got, tc.wantVis)
			}
		})
	}
}
