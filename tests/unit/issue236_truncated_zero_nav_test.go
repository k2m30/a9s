package unit

// Tests for issue #236: a resource type with Count=0 AND Truncated=true (page 1
// returned no items but more pages exist) must NOT be treated as confirmed-empty.
//
// These tests FAIL with the current code and PASS after the fix.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ---------------------------------------------------------------------------
// Test 1: cursor can land on a truncated-zero row
// ---------------------------------------------------------------------------

func TestIssue236_CursorCanLandOnTruncatedZeroRow(t *testing.T) {
	// Business rule: a row where the probe fetched page 1 and got 0 items, but
	// the page was truncated (more pages exist), is NOT confirmed empty.
	// The cursor must be able to land on it.
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 3 {
		t.Skip("need at least 3 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)

	// allTypes[1] = count 0 + truncated (uncertain — more pages may have resources).
	m.SetAvailability(allTypes[1].ShortName, 0)
	m.SetTruncated(allTypes[1].ShortName, true)

	// Cursor starts at allTypes[0]. Press Down once.
	m, _ = m.Update(menuKeyDown())

	selected := m.SelectedItem()
	if selected.ShortName != allTypes[1].ShortName {
		t.Errorf("cursor should land on truncated-zero item %q (it is NOT confirmed empty), got %q",
			allTypes[1].ShortName, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Enter navigates on a truncated-zero row
// ---------------------------------------------------------------------------

func TestIssue236_EnterNavigatesOnTruncatedZeroRow(t *testing.T) {
	// Business rule: the user must be able to open a resource list to check if
	// later pages have resources. Enter on a truncated-zero row must emit a
	// NavigateMsg to the resource list, not be blocked.
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Skip("no resource types registered")
	}

	m := newSizedMainMenu(t, 80, 200)

	// allTypes[0] (cursor starts here) = count 0 + truncated.
	m.SetAvailability(allTypes[0].ShortName, 0)
	m.SetTruncated(allTypes[0].ShortName, true)

	// Press Enter on the truncated-zero row.
	_, cmd := m.Update(menuKeyEnter())

	if cmd == nil {
		t.Fatalf("Enter on truncated-zero row %q must return a non-nil cmd (NavigateMsg expected)",
			allTypes[0].ShortName)
	}

	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("Enter on truncated-zero row must produce a messages.Navigate, got %T", msg)
	}
	if nav.ResourceType != allTypes[0].ShortName {
		t.Errorf("NavigateMsg.ResourceType = %q, want %q", nav.ResourceType, allTypes[0].ShortName)
	}
	if nav.Target != messages.TargetResourceList {
		t.Errorf("NavigateMsg.Target = %v, want messages.TargetResourceList", nav.Target)
	}
}

// ---------------------------------------------------------------------------
// Test 3: display shows "(0+)" for truncated-zero, not bare "(0)"
// ---------------------------------------------------------------------------

func TestIssue236_DisplayShowsZeroPlusForTruncatedZero(t *testing.T) {
	// Business rule: the user needs to know the count is uncertain, not confirmed
	// zero. A truncated-zero row must display "(0+)" not bare "(0)".
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 0)
	m.SetTruncated("ec2", true)

	output := m.View()

	// Must contain "(0+)" — the plus indicates more resources may exist.
	if !strings.Contains(output, "(0+)") {
		t.Errorf("View() should contain '(0+)' for ec2 with count=0 AND truncated=true, output:\n%s", output)
	}

	// After removing all "(0+)" occurrences, "(0)" must not appear — bare zero
	// would falsely tell the user the type is confirmed empty.
	withoutZeroPlus := strings.ReplaceAll(output, "(0+)", "")
	if strings.Contains(withoutZeroPlus, "(0)") {
		t.Errorf("View() must NOT contain bare '(0)' for truncated-zero ec2 (only '(0+)' is correct), output:\n%s", output)
	}
}
