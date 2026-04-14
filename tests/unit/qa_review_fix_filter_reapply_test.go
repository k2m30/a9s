package unit

import (
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// parseMenuCount extracts N from "resource-types(N)" or "resource-types(N) [!]".
// Returns -1 if the title does not match the expected pattern.
func parseMenuCount(title string) int {
	start := strings.Index(title, "(")
	end := strings.Index(title, ")")
	if start == -1 || end == -1 || end <= start {
		return -1
	}
	inner := title[start+1 : end]
	// The title may be "N/T" when a text filter is active; ignore that case.
	if strings.Contains(inner, "/") {
		parts := strings.SplitN(inner, "/", 2)
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return -1
		}
		return n
	}
	n, err := strconv.Atoi(inner)
	if err != nil {
		return -1
	}
	return n
}

// TestSetIssuesReappliesFilterWhenCtrlZActive verifies that calling SetIssues()
// after enabling the ctrl+z filter immediately hides resource types that are
// confirmed-zero (count=0, truncated=false).
// This is the fix from issue #196 bug 1: applyFilter() must be called inside SetIssues().
func TestSetIssuesReappliesFilterWhenCtrlZActive(t *testing.T) {
	// Use a large height so View() renders all items.
	menu := views.NewMainMenu(keys.Default())
	menu.SetSize(80, 200)

	// Enable ctrl+z attention filter.
	menu.Toggle()
	if !menu.IsEnabled() {
		t.Fatal("Toggle() did not enable the attention filter")
	}

	// With ctrl+z active and no issue data, all items are in "unknown" state → visible.
	// EC2 Instances should appear in View().
	viewBefore := menu.View()
	if !strings.Contains(viewBefore, "EC2 Instances") {
		t.Fatalf("EC2 Instances not visible in View() before SetIssues — check SetSize or resource registration")
	}

	// Mark "ec2" as confirmed-zero (count=0, truncated=false).
	// Under quad-state: unknown→visible, confirmed-zero→hidden, truncated-zero→visible, has-issues→visible.
	menu.SetIssues("ec2", 0, false)

	// After SetIssues, applyFilter() must have been called — EC2 should now be hidden.
	viewAfter := menu.View()
	if strings.Contains(viewAfter, "EC2 Instances") {
		t.Errorf("SetIssues(ec2, 0, false) with ctrl+z active: EC2 Instances should be hidden but still appears in View()")
	}
}

// TestSetIssuesReappliesFilterWhenCtrlZActive_HasIssuesVisible verifies that a
// resource type with issues > 0 remains visible after SetIssues() when ctrl+z is active.
func TestSetIssuesReappliesFilterWhenCtrlZActive_HasIssuesVisible(t *testing.T) {
	menu := views.NewMainMenu(keys.Default())
	menu.SetSize(80, 200)

	menu.Toggle()

	// EC2 is visible in unknown state.
	if !strings.Contains(menu.View(), "EC2 Instances") {
		t.Fatal("EC2 Instances not visible in View() before SetIssues")
	}

	// Mark ec2 as having issues — must remain visible.
	menu.SetIssues("ec2", 3, false)

	if !strings.Contains(menu.View(), "EC2 Instances") {
		t.Error("SetIssues(ec2, 3, false): ec2 has issues>0 so it must remain visible under ctrl+z")
	}
}

// TestSetIssuesFromCacheReappliesFilter verifies that SetIssuesFromCache() also
// reapplies the filter when ctrl+z is active, mirroring the SetIssues() fix.
func TestSetIssuesFromCacheReappliesFilter(t *testing.T) {
	menu := views.NewMainMenu(keys.Default())
	menu.SetSize(80, 200)

	menu.Toggle()
	if !menu.IsEnabled() {
		t.Fatal("Toggle() did not enable the attention filter")
	}

	// EC2 and RDS instances should be visible in unknown state.
	viewBefore := menu.View()
	if !strings.Contains(viewBefore, "EC2 Instances") {
		t.Fatal("EC2 Instances not visible before SetIssuesFromCache")
	}
	if !strings.Contains(viewBefore, "DB Instances") {
		t.Fatal("DB Instances (dbi) not visible before SetIssuesFromCache")
	}

	// Load cache that marks ec2 and dbi as confirmed-zero.
	// Note: DB Instances uses shortname "dbi", not "rds".
	counts := map[string]int{"ec2": 0, "dbi": 0}
	truncated := map[string]bool{"ec2": false, "dbi": false}
	known := map[string]bool{"ec2": true, "dbi": true}
	menu.SetIssuesFromCache(counts, truncated, known)

	// After SetIssuesFromCache, applyFilter() must have been called — both should be hidden.
	viewAfter := menu.View()
	if strings.Contains(viewAfter, "EC2 Instances") {
		t.Error("SetIssuesFromCache with ec2 confirmed-zero: EC2 Instances should be hidden but still appears in View()")
	}
	if strings.Contains(viewAfter, "DB Instances") {
		t.Error("SetIssuesFromCache with dbi confirmed-zero: DB Instances should be hidden but still appears in View()")
	}
}

// TestSetIssuesFromCacheOnlyHidesKnownEntries verifies that SetIssuesFromCache()
// only hides resource types that are in the known=true map.
func TestSetIssuesFromCacheOnlyHidesKnownEntries(t *testing.T) {
	menu := views.NewMainMenu(keys.Default())
	menu.SetSize(80, 200)

	menu.Toggle()

	// EC2 visible before cache load.
	if !strings.Contains(menu.View(), "EC2 Instances") {
		t.Fatal("EC2 Instances not visible before SetIssuesFromCache")
	}

	// Load cache with ec2 known=false — should not hide ec2 (unknown → visible).
	counts := map[string]int{"ec2": 0}
	truncated := map[string]bool{"ec2": false}
	known := map[string]bool{"ec2": false}
	menu.SetIssuesFromCache(counts, truncated, known)

	// EC2 should remain visible because known=false means it's still unknown state.
	if !strings.Contains(menu.View(), "EC2 Instances") {
		t.Error("SetIssuesFromCache with known=false should NOT hide ec2 (unknown state → visible)")
	}
}
