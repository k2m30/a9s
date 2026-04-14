package unit

// qa_enrichment_review_fixes_test.go — regression tests for the four review
// findings landed on top of feature 018-enrichment-visibility:
//
//   1. SetEnrichmentState must recompute visibleFindingCount so the banner's
//      long/short text reflects current findings after a live update.
//   2. View() must reserve a row for the enrichment banner so the last data
//      row / load-more hint isn't clipped.
//   3. resolveIdentityColumn must run on the full column list (pre-hscroll) so
//      horizontal scrolling cannot make the marker jump to a different semantic
//      column (e.g. State when Name is scrolled off).
//   4. Ctrl+R on a top-level list must clear the active ResourceListModel's
//      findings immediately, not only the root-model copies.
//
// Assertions target observable behavior (banner text variant, number of
// rendered data rows, dot presence per column, dot absence after Ctrl+R).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// -----------------------------------------------------------------------------
// Fix 1: SetEnrichmentState recomputes visibleFindingCount
// -----------------------------------------------------------------------------

// TestSetEnrichmentState_RecomputesVisibleFindingCountOnLiveUpdate asserts that
// applying findings AFTER the list has already been loaded yields the short-form
// banner (no "— not visible on this page" suffix) when at least one finding is
// for a currently visible row.
//
// Pre-fix behavior: visibleFindingCount was only computed in applyFilter, which
// runs on ResourcesLoadedMsg. A subsequent SetEnrichmentState call left
// visibleFindingCount at its prior value (0 on cold start), so renderEnrichmentBanner
// used the long form even when marked rows were visible.
func TestSetEnrichmentState_RecomputesVisibleFindingCountOnLiveUpdate(t *testing.T) {
	// Build a list that has already loaded healthy rows BEFORE any enrichment.
	m := loadedBannerModel(t, bannerHealthyResources(), nil)

	// Now simulate a live EnrichmentCheckedMsg → SetEnrichmentState arrival:
	// a finding for db-1 (which IS in filteredResources).
	findings := map[string]resource.EnrichmentFinding{
		"db-1": {Severity: "~", Summary: "pending maintenance: minor-version-upgrade"},
	}
	m.SetEnrichmentState(0, false, true, findings)

	output := m.View()
	if !strings.Contains(output, "background checks") {
		t.Fatalf("expected banner to be rendered after live enrichment update; output:\n%s", output)
	}
	if strings.Contains(output, "not visible on this page") {
		t.Errorf("expected SHORT-form banner (visible finding present), got LONG form; this means visibleFindingCount was stale after SetEnrichmentState.\noutput:\n%s", output)
	}
}

// TestSetEnrichmentState_RecomputesVisibleFindingCountOnRecovery asserts the
// inverse: when findings are replaced with a map whose keys are NOT in the
// visible set, the banner switches to long form.
func TestSetEnrichmentState_RecomputesVisibleFindingCountOnRecovery(t *testing.T) {
	// Start with a finding on db-1 (visible).
	findings := map[string]resource.EnrichmentFinding{
		"db-1": {Severity: "~", Summary: "pending maintenance"},
	}
	m := loadedBannerModel(t, bannerHealthyResources(), func(m *views.ResourceListModel) {
		m.SetEnrichmentState(0, false, true, findings)
	})
	if !strings.Contains(m.View(), "background checks") || strings.Contains(m.View(), "not visible") {
		t.Fatalf("pre-condition failed: expected short-form banner before update")
	}

	// Replace with findings for an off-page ID only.
	m.SetEnrichmentState(0, false, true, map[string]resource.EnrichmentFinding{
		"db-offpage-999": {Severity: "~", Summary: "off-page finding"},
	})

	output := m.View()
	if !strings.Contains(output, "not visible on this page") {
		t.Errorf("expected LONG-form banner after replacing with off-page finding; visibleFindingCount was likely stale.\noutput:\n%s", output)
	}
}

// -----------------------------------------------------------------------------
// Fix 2: View() reserves a row for the banner
// -----------------------------------------------------------------------------

// TestBanner_ReservesRowInVisibleWindow asserts that when the banner is shown,
// the number of rendered data rows is reduced by one compared to when it is
// hidden. Pre-fix: the banner was added WITHOUT decrementing visibleRows, so
// the list rendered one row too tall (clipping the last row or load-more hint).
func TestBanner_ReservesRowInVisibleWindow(t *testing.T) {
	td := bannerTypeDef()
	k := keys.Default()
	makeModel := func(withBanner bool) views.ResourceListModel {
		m := views.NewResourceList(td, nil, k)
		m.SetSize(120, 10)
		m, _ = m.Init()

		resources := make([]resource.Resource, 20)
		for i := range resources {
			id := "db-" + string(rune('a'+i))
			resources[i] = resource.Resource{
				ID:     id,
				Name:   "prod-" + id,
				Status: "available",
				Fields: map[string]string{"db_instance_id": id, "status": "available"},
			}
		}
		m, _ = m.Update(messages.ResourcesLoadedMsg{ResourceType: "rds", Resources: resources})

		if withBanner {
			m.SetEnrichmentState(0, false, true, map[string]resource.EnrichmentFinding{
				"off-page-id": {Severity: "~", Summary: "hidden finding"},
			})
		}
		return m
	}

	countDataRows := func(view string) int {
		// The identity column is "db_instance_id" — rendered values are "db-a",
		// "db-b", etc. Count lines that contain "db-" followed by a letter.
		count := 0
		for _, line := range strings.Split(view, "\n") {
			for c := 'a'; c <= 'z'; c++ {
				if strings.Contains(line, "db-"+string(c)) {
					count++
					break
				}
			}
		}
		return count
	}

	noBanner := makeModel(false)
	withBanner := makeModel(true)

	noBannerRows := countDataRows(noBanner.View())
	withBannerRows := countDataRows(withBanner.View())

	// Pre-conditions.
	if !strings.Contains(withBanner.View(), "background checks") {
		t.Fatalf("pre-condition failed: banner not shown in withBanner view")
	}
	if strings.Contains(noBanner.View(), "background checks") {
		t.Fatalf("pre-condition failed: banner rendered in noBanner view")
	}

	// Key assertion: banner-present render shows one fewer data row.
	if withBannerRows != noBannerRows-1 {
		t.Errorf("banner should reserve one row: got %d data rows with banner vs %d without (expected %d with banner)",
			withBannerRows, noBannerRows, noBannerRows-1)
	}
}

// -----------------------------------------------------------------------------
// Fix 3: resolveIdentityColumn runs on full columns; marker hidden when identity
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
		{ID: "r-1", Name: "alpha-instance-with-distinctive-name", Status: "available", Fields: map[string]string{"name": "alpha-instance-with-distinctive-name", "state": "available", "type": "m5", "region": "us-east-1"}},
	}
	m, _ = m.Update(messages.ResourcesLoadedMsg{ResourceType: "test", Resources: resources})

	findings := map[string]resource.EnrichmentFinding{
		"r-1": {Severity: "!", Summary: "broken"},
	}
	m.SetEnrichmentState(0, false, true, findings)

	// Baseline: dot present at hScrollOffset=0.
	baseline := m.View()
	if !strings.Contains(baseline, "\u00b7") {
		t.Fatalf("pre-condition failed: expected dot marker in baseline render; output:\n%s", baseline)
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

	// Key assertion: dot must not render when the identity column is not visible.
	if strings.Contains(scrolled, "\u00b7") {
		t.Errorf("marker must not render when identity column is scrolled off-screen (would jump to wrong column); output:\n%s", scrolled)
	}
}

// -----------------------------------------------------------------------------
// Fix 4: Ctrl+R clears the active ResourceListModel's findings immediately
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    resources,
	})

	// Deliver a successful EnrichmentCheckedMsg to populate findings on the
	// active list via the handler's live-update path.
	m, _ = rootApplyMsg(m, enrichmentCheckedWithFindings(0, 0))

	// Sanity: the dot marker is visible in the rendered output.
	before := m.View().Content
	if !strings.Contains(before, "\u00b7") {
		t.Fatalf("pre-condition failed: expected finding dot in render before Ctrl+R; output:\n%s", before)
	}

	// Dispatch Ctrl+R via the real key path.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Assertion: the dot is gone immediately (no fetch response processed yet).
	// Pre-fix, the dot would persist until a follow-up SetEnrichmentState, which
	// only happens after a successful rerun.
	after := m.View().Content
	if strings.Contains(after, "\u00b7") {
		t.Errorf("Ctrl+R must clear the active list's findings immediately; dot marker still present in rendered output:\n%s", after)
	}
}
