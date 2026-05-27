package unit

// qa_enrichment_review_fixes_test.go — regression tests for two review
// findings landed on top of feature 018-enrichment-visibility:
//
//   1. resolveIdentityColumn must run on the full column list (pre-hscroll) so
//      horizontal scrolling cannot make the marker jump to a different semantic
//      column (e.g. State when Name is scrolled off).
//   2. Ctrl+R on a top-level list must clear the active ResourceListModel's
//      findings immediately, not only the root-model copies.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// -----------------------------------------------------------------------------
// Fix 1: resolveIdentityColumn runs on full columns; marker hidden when identity
// column is hscrolled off-screen
// -----------------------------------------------------------------------------

// TestRowMarker_HiddenWhenIdentityColumnScrolledOff asserts that when the user
// scrolls horizontally so the identity column (Name) is not in the visible
// column slice, the row marker is NOT rendered on a different column.
//
// Pre-fix: resolveIdentityColumn ran on the post-hscroll cols, so it cascaded
// to a different column (e.g. one whose path contained "Name") and the dot
// jumped to that column.
//
// Approach: create a list with a narrow terminal so the full column set doesn't
// fit, then send "l" (ScrollRight) key to advance hscroll. After scrolling,
// check that the Name column values are gone from the output AND the dot is
// also gone.
func TestRowMarker_HiddenWhenIdentityColumnScrolledOff(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "Test",
		ShortName: "test",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 30},
			{Key: "state", Title: "State", Width: 12},
			{Key: "type", Title: "Type", Width: 12},
			{Key: "region", Title: "Region", Width: 12},
		},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	// Narrow width: forces at least one column to overflow so ScrollRight is allowed.
	m.SetSize(50, 10)
	m, _ = m.Init()

	resources := []resource.Resource{
		{ID: "r-1", Name: "alpha-instance-with-distinctive-name", Fields: map[string]string{"name": "alpha-instance-with-distinctive-name", "state": "available", "type": "m5", "region": "us-east-1"}},
	}
	m, _ = m.Update(messages.ResourcesLoaded{ResourceType: "test", Resources: resources})

	findings := map[string]domain.Finding{
		"r-1": {Code: "ec2.system.status.impaired", Phrase: "broken", Severity: domain.SevBroken, Source: "wave2:ec2"},
	}
	m.SetEnrichmentState(len(findings), false, findings)

	// Baseline: prefix marker present at hScrollOffset=0.
	baseline := m.View()
	if !strings.Contains(baseline, "! ") {
		t.Fatalf("pre-condition failed: expected '! ' prefix marker in baseline render; output:\n%s", baseline)
	}

	// Scroll right repeatedly until the Name column value is off-screen.
	scrollRight := tea.KeyPressMsg{Code: 'l', Text: "l"}
	var scrolled string
	for i := 0; i < 3; i++ {
		m, _ = m.Update(scrollRight)
		scrolled = m.View()
		if !strings.Contains(scrolled, "alpha-instance-with-distinctive-name") {
			break
		}
	}

	if strings.Contains(scrolled, "alpha-instance-with-distinctive-name") {
		t.Skip("could not scroll Name column off-screen; terminal width too wide for this test")
	}

	// Key assertion: prefix marker must not render when the identity column is not visible.
	if strings.Contains(scrolled, "! ") {
		t.Errorf("marker must not render when identity column is scrolled off-screen (would jump to wrong column); output:\n%s", scrolled)
	}
}

// -----------------------------------------------------------------------------
// Fix 2: Ctrl+R clears the active ResourceListModel's findings immediately
// -----------------------------------------------------------------------------

// TestCtrlR_ClearsActiveListFindingsImmediately asserts that pressing Ctrl+R on
// a top-level list clears the findingsByID on the active ResourceListModel
// BEFORE the wrapped fetch returns — so stale markers disappear at keypress
// time, not only after a successful rerun.
//
// Pre-fix: the handler cleared the root-model maps but left the active
// ResourceListModel.findingsByID populated until a subsequent SetEnrichmentState.
// If the refresh errored, the stale state persisted indefinitely.
//
// Uses the test helpers defined in qa_enrichment_rerun_overlap_test.go:
// newRootSizedModel, rootApplyMsg, navigateToEC2List, ctrlRKeyMsg.
func TestCtrlR_ClearsActiveListFindingsImmediately(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	// Load real EC2 resources into the list so there's something to mark.
	resources := rerunEC2Resources()
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources,
	})

	// Deliver a successful EnrichmentCheckedMsg to populate findings on the
	// active list via the handler's live-update path.
	m, _ = rootApplyMsg(m, enrichmentCheckedWithFindings(0, 0))

	// Sanity: the "! " prefix marker is visible in the rendered output.
	before := m.View().Content
	if !strings.Contains(before, "! ") {
		t.Fatalf("pre-condition failed: expected '! ' prefix marker in render before Ctrl+R; output:\n%s", before)
	}

	// Dispatch Ctrl+R via the real key path.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Assertion: the prefix marker is gone immediately (no fetch response processed yet).
	// Pre-fix, the marker would persist until a follow-up SetEnrichmentState, which
	// only happens after a successful rerun.
	after := m.View().Content
	if strings.Contains(after, "! ") {
		t.Errorf("Ctrl+R must clear the active list's findings immediately; '! ' prefix marker still present in rendered output:\n%s", after)
	}
}
