package unit

// qa_menu_issue_maps_test.go — T028: SetIssuesFromCache and getter tests.
//
// These tests verify bulk-loading issue state from cache maps and that the getter
// methods return the correct data (or nil when the menu is freshly created).

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestMainMenuGettersReturnNilWhenEmpty verifies that a freshly created menu
// has nil issue maps — no issue data has been loaded yet.
func TestMainMenuGettersReturnNilWhenEmpty(t *testing.T) {
	m := views.NewMainMenu(keys.Default())

	if m.GetIssueCounts() != nil {
		t.Error("GetIssueCounts() should be nil for a fresh menu")
	}
	if m.GetIssueKnown() != nil {
		t.Error("GetIssueKnown() should be nil for a fresh menu")
	}
	if m.GetIssueTruncated() != nil {
		t.Error("GetIssueTruncated() should be nil for a fresh menu")
	}
}

// TestMainMenuSetIssuesFromCacheRespectsKnown verifies that only entries where
// known=true are stored; entries with known=false are ignored.
func TestMainMenuSetIssuesFromCacheRespectsKnown(t *testing.T) {
	m := views.NewMainMenu(keys.Default())

	counts := map[string]int{
		"ec2": 5,
		"rds": 2,
	}
	truncated := map[string]bool{
		"ec2": false,
		"rds": false,
	}
	known := map[string]bool{
		"ec2": true,
		"rds": false, // should NOT be stored
	}

	m.SetIssuesFromCache(counts, truncated, known)

	gotCounts := m.GetIssueCounts()
	gotKnown := m.GetIssueKnown()

	// ec2 must be stored
	if gotCounts["ec2"] != 5 {
		t.Errorf("counts[ec2] = %d, want 5", gotCounts["ec2"])
	}
	if !gotKnown["ec2"] {
		t.Error("known[ec2] should be true")
	}

	// rds must NOT be stored (known=false means unknown, not probed)
	if gotKnown["rds"] {
		t.Error("known[rds] should not be set — entry was known=false in cache")
	}
	if gotCounts["rds"] != 0 {
		t.Errorf("counts[rds] = %d, want 0 (not stored)", gotCounts["rds"])
	}
}

// TestMainMenuSetIssuesFromCacheEmptyMaps verifies that calling SetIssuesFromCache
// with empty maps initializes the internal maps without panicking.
func TestMainMenuSetIssuesFromCacheEmptyMaps(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetIssuesFromCache(
		map[string]int{},
		map[string]bool{},
		map[string]bool{},
	)

	// Maps should be initialized but empty (not nil).
	if m.GetIssueCounts() == nil {
		t.Error("GetIssueCounts() should not be nil after SetIssuesFromCache")
	}
	if m.GetIssueKnown() == nil {
		t.Error("GetIssueKnown() should not be nil after SetIssuesFromCache")
	}
	if m.GetIssueTruncated() == nil {
		t.Error("GetIssueTruncated() should not be nil after SetIssuesFromCache")
	}
}

// TestMainMenuSetIssuesFromCacheThenSetIssues verifies that SetIssues after
// SetIssuesFromCache correctly overwrites only the specified entry.
func TestMainMenuSetIssuesFromCacheThenSetIssues(t *testing.T) {
	m := views.NewMainMenu(keys.Default())

	m.SetIssuesFromCache(
		map[string]int{"ec2": 1, "rds": 2},
		map[string]bool{"ec2": false, "rds": false},
		map[string]bool{"ec2": true, "rds": true},
	)
	// Override ec2
	m.SetIssues("ec2", 99, true)

	if m.GetIssueCounts()["ec2"] != 99 {
		t.Errorf("counts[ec2] = %d, want 99 after SetIssues overwrite", m.GetIssueCounts()["ec2"])
	}
	if !m.GetIssueTruncated()["ec2"] {
		t.Error("truncated[ec2] should be true after SetIssues overwrite")
	}
	// rds must be unchanged
	if m.GetIssueCounts()["rds"] != 2 {
		t.Errorf("counts[rds] = %d, want 2 (unchanged)", m.GetIssueCounts()["rds"])
	}
}
