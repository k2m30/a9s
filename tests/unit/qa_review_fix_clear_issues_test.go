package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestClearAvailabilityClearsIssueState verifies that ClearAvailability() wipes
// all issue maps so that stale badges from a previous account/region do not survive
// a profile or region switch.
// This is the fix from issue #196 bug 2.
func TestClearAvailabilityClearsIssueState(t *testing.T) {
	menu := views.NewMainMenu(keys.Default())
	menu.SetSize(80, 24)

	// Populate issue state using valid short names.
	menu.SetIssues("ec2", 5, false)
	menu.SetIssues("dbi", 2, true)
	menu.SetIssues("s3", 0, false)

	if menu.GetIssueCounts() == nil {
		t.Fatal("GetIssueCounts() should be non-nil after SetIssues calls")
	}
	if menu.GetIssueKnown() == nil {
		t.Fatal("GetIssueKnown() should be non-nil after SetIssues calls")
	}
	if menu.GetIssueTruncated() == nil {
		t.Fatal("GetIssueTruncated() should be non-nil after SetIssues calls")
	}

	menu.ClearAvailability()

	if counts := menu.GetIssueCounts(); counts != nil {
		t.Errorf("ClearAvailability() should nil issueCounts, got %v", counts)
	}
	if known := menu.GetIssueKnown(); known != nil {
		t.Errorf("ClearAvailability() should nil issueKnown, got %v", known)
	}
	if trunc := menu.GetIssueTruncated(); trunc != nil {
		t.Errorf("ClearAvailability() should nil issueTruncated, got %v", trunc)
	}
}

// TestClearAvailabilityClearsAvailabilityMaps verifies that the availability maps
// are also cleared alongside the issue state.
func TestClearAvailabilityClearsAvailabilityMaps(t *testing.T) {
	menu := views.NewMainMenu(keys.Default())
	menu.SetSize(80, 24)

	menu.SetAvailability("ec2", 10)
	menu.SetAvailability("rds", 3)
	menu.SetTruncated("ec2", true)

	menu.ClearAvailability()

	if av := menu.GetAvailability(); av != nil {
		t.Errorf("ClearAvailability() should nil availability, got %v", av)
	}
	if tr := menu.GetTruncated(); tr != nil {
		t.Errorf("ClearAvailability() should nil truncated, got %v", tr)
	}
}

// TestClearAvailabilityReappliesFilter verifies that after ClearAvailability(),
// resource types that were hidden under ctrl+z become visible again (back to unknown state).
// This is the fix from issue #196 bug 2: applyFilter() must be called inside ClearAvailability().
func TestClearAvailabilityReappliesFilter(t *testing.T) {
	// Use large height so View() renders all items.
	menu := views.NewMainMenu(keys.Default())
	menu.SetSize(80, 200)

	// Enable ctrl+z filter and confirm ec2 and rds as zero — they become hidden.
	menu.Toggle()
	if !menu.IsEnabled() {
		t.Fatal("Toggle() did not enable the attention filter")
	}

	// Mark ec2 and dbi (DB Instances) as confirmed-zero; SetIssues calls applyFilter() (bug 1 fix).
	// Note: DB Instances uses shortname "dbi", not "rds".
	menu.SetIssues("ec2", 0, false)
	menu.SetIssues("dbi", 0, false)

	// Verify that ec2/dbi are now hidden (prerequisite for the clear test).
	viewHidden := stripANSI(menu.View())
	if strings.Contains(viewHidden, "EC2 Instances") {
		t.Fatal("prerequisite failed: EC2 Instances should be hidden after SetIssues(ec2,0,false) with ctrl+z active")
	}
	if strings.Contains(viewHidden, "DB Instances") {
		t.Fatal("prerequisite failed: DB Instances should be hidden after SetIssues(dbi,0,false) with ctrl+z active")
	}

	// ClearAvailability() must restore all items to unknown state → all visible under ctrl+z.
	menu.ClearAvailability()

	viewAfter := stripANSI(menu.View())
	if !strings.Contains(viewAfter, "EC2 Instances") {
		t.Error("after ClearAvailability(), EC2 Instances should be visible again (restored to unknown state)")
	}
	if !strings.Contains(viewAfter, "DB Instances") {
		t.Error("after ClearAvailability(), DB Instances should be visible again (restored to unknown state)")
	}
}

// TestClearAvailabilityIdempotent verifies that calling ClearAvailability() on a
// freshly created menu (no state) does not panic or corrupt state.
func TestClearAvailabilityIdempotent(t *testing.T) {
	menu := views.NewMainMenu(keys.Default())
	menu.SetSize(80, 24)

	// Should not panic.
	menu.ClearAvailability()
	menu.ClearAvailability()

	if menu.GetIssueCounts() != nil {
		t.Error("GetIssueCounts() should be nil after double-clear of empty menu")
	}

	// All items should still be visible.
	count := parseMenuCount(menu.FrameTitle())
	if count <= 0 {
		t.Errorf("expected positive item count after double-clear, got %d (title=%q)", count, menu.FrameTitle())
	}
}
