package unit

// qa_menu_filter_compose_test.go — T041: text search ∩ ctrl+z filter composition tests.
//
// These tests verify the intersection logic of text search and ctrl+z attention
// filter. Both conditions must be satisfied for an item to be visible:
//   - text filter OFF or text matches → passes text gate
//   - ctrl+z OFF or item is visible under issue filter → passes issue gate
//
// A menu item is visible only if it passes BOTH gates.

import (
	"strings"
	"testing"
)

// menuFilterVisible encodes the composed menu filter decision:
//
//   - textFilter: the filter string typed by the user ("" means no text filter)
//   - itemName: the resource type name or short name to match against
//   - ctrlZActive: true when the ctrl+z attention filter is toggled on
//   - known, truncated, count: issue probe state for the item
func menuFilterVisible(textFilter, itemName string, ctrlZActive, known, truncated bool, count int) bool {
	// Text gate: empty filter passes everything; non-empty requires substring match.
	if textFilter != "" {
		if !strings.Contains(strings.ToLower(itemName), strings.ToLower(textFilter)) {
			return false
		}
	}
	// Issue gate: only applied when ctrl+z is active.
	if ctrlZActive {
		if !isVisibleUnderIssueFilter(known, truncated, count) {
			return false
		}
	}
	return true
}

// TestMenuFilterComposition_TextOnly verifies that when only text filter is active
// (ctrl+z off), a matching item is visible regardless of issue state.
func TestMenuFilterComposition_TextOnly(t *testing.T) {
	visible := menuFilterVisible("ec2", "EC2 Instances", false, true, false, 0)
	if !visible {
		t.Error("text matches and ctrl+z is off — item should be visible")
	}
}

// TestMenuFilterComposition_CtrlZOnly verifies that when only ctrl+z is active
// (no text filter), an item with issues is visible.
func TestMenuFilterComposition_CtrlZOnly(t *testing.T) {
	visible := menuFilterVisible("", "EC2 Instances", true, true, false, 3)
	if !visible {
		t.Error("no text filter, ctrl+z on, item has issues — should be visible")
	}
}

// TestMenuFilterComposition_BothActive verifies that when both filters are active
// and both pass, the item is visible.
func TestMenuFilterComposition_BothActive(t *testing.T) {
	visible := menuFilterVisible("ec2", "EC2 Instances", true, true, false, 5)
	if !visible {
		t.Error("text matches and item has issues with ctrl+z on — should be visible")
	}
}

// TestMenuFilterComposition_TextMatchNoIssues verifies that when ctrl+z is active
// and the item has confirmed zero issues, it is hidden even if text matches.
func TestMenuFilterComposition_TextMatchNoIssues(t *testing.T) {
	visible := menuFilterVisible("ec2", "EC2 Instances", true, true, false, 0)
	if visible {
		t.Error("text matches but item has confirmed zero issues with ctrl+z on — should be hidden")
	}
}

// TestMenuFilterComposition_IssuesNoTextMatch verifies that when ctrl+z is active
// and the item has issues but text doesn't match, it is hidden.
func TestMenuFilterComposition_IssuesNoTextMatch(t *testing.T) {
	visible := menuFilterVisible("xyz", "EC2 Instances", true, true, false, 10)
	if visible {
		t.Error("item has issues but text doesn't match — should be hidden")
	}
}

// TestMenuFilterComposition_BothOff verifies that with no filters active, all
// items pass regardless of issue state.
func TestMenuFilterComposition_BothOff(t *testing.T) {
	cases := []struct {
		name      string
		known     bool
		truncated bool
		count     int
	}{
		{"unknown", false, false, 0},
		{"confirmed zero", true, false, 0},
		{"has issues", true, false, 5},
		{"truncated zero", true, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			visible := menuFilterVisible("", "EC2 Instances", false, tc.known, tc.truncated, tc.count)
			if !visible {
				t.Errorf("no filters active — all items should be visible (%s)", tc.name)
			}
		})
	}
}

// TestMenuFilterComposition_TableDriven is a comprehensive table-driven test of
// the filter composition logic across all interesting combinations.
func TestMenuFilterComposition_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		itemName  string
		ctrlZ     bool
		known     bool
		truncated bool
		count     int
		wantVis   bool
	}{
		// Text filter only
		{"text-match/no-ctrlz", "ec2", "EC2 Instances", false, true, false, 0, true},
		{"text-no-match/no-ctrlz", "rds", "EC2 Instances", false, true, false, 0, false},
		// CtrlZ filter only
		{"no-text/ctrlz/has-issues", "", "EC2 Instances", true, true, false, 3, true},
		{"no-text/ctrlz/zero-confirmed", "", "EC2 Instances", true, true, false, 0, false},
		{"no-text/ctrlz/unknown", "", "EC2 Instances", true, false, false, 0, true},
		{"no-text/ctrlz/truncated-zero", "", "EC2 Instances", true, true, true, 0, false},
		// Both active
		{"both/match+issues", "ec2", "EC2 Instances", true, true, false, 5, true},
		{"both/match+no-issues", "ec2", "EC2 Instances", true, true, false, 0, false},
		{"both/no-match+issues", "rds", "EC2 Instances", true, true, false, 5, false},
		{"both/no-match+no-issues", "rds", "EC2 Instances", true, true, false, 0, false},
		// Case insensitive text match
		{"text-case-insensitive", "EC2", "ec2 instances", false, false, false, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := menuFilterVisible(tc.text, tc.itemName, tc.ctrlZ, tc.known, tc.truncated, tc.count)
			if got != tc.wantVis {
				t.Errorf("menuFilterVisible(%q, %q, ctrlZ=%v, known=%v, trunc=%v, count=%d) = %v, want %v",
					tc.text, tc.itemName, tc.ctrlZ, tc.known, tc.truncated, tc.count, got, tc.wantVis)
			}
		})
	}
}
