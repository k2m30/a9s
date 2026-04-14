package unit

// qa_enrichment_banner_test.go — T029–T032: US1 enrichment banner behavioral tests.
//
// Tests verify what the user observes (banner text, presence/absence) when
// SetEnrichmentState is called on ResourceListModel under various conditions.
// All assertions are behavioral string-contains checks against View() output.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// bannerTypeDef returns a ResourceTypeDef with all-healthy default statuses.
func bannerTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "RDS Instances",
		ShortName: "rds",
		Columns: []resource.Column{
			{Key: "db_instance_id", Title: "DB Instance ID", Width: 22},
			{Key: "status", Title: "Status", Width: 12},
		},
	}
}

// bannerHealthyResources returns three running RDS instances (no issue-row color).
func bannerHealthyResources() []resource.Resource {
	return []resource.Resource{
		{ID: "db-1", Name: "prod-db-1", Status: "available", Fields: map[string]string{"db_instance_id": "db-1", "status": "available"}},
		{ID: "db-2", Name: "prod-db-2", Status: "available", Fields: map[string]string{"db_instance_id": "db-2", "status": "available"}},
		{ID: "db-3", Name: "prod-db-3", Status: "available", Fields: map[string]string{"db_instance_id": "db-3", "status": "available"}},
	}
}

// bannerIssueResources returns resources some of which have issue-colored status.
func bannerIssueResources() []resource.Resource {
	return []resource.Resource{
		{ID: "db-1", Name: "prod-db-1", Status: "available", Fields: map[string]string{"db_instance_id": "db-1", "status": "available"}},
		{ID: "db-2", Name: "prod-db-2", Status: "failed", Fields: map[string]string{"db_instance_id": "db-2", "status": "failed"}},
	}
}

// loadedBannerModel builds a ResourceListModel loaded with the given resources.
// setEnrichment, if non-nil, is called after loading so that applySortAndFilter
// (triggered by ResourcesLoadedMsg) computes visibleFindingCount from the current
// findingsByID.
func loadedBannerModel(
	t *testing.T,
	resources []resource.Resource,
	setEnrichment func(*views.ResourceListModel),
) views.ResourceListModel {
	t.Helper()
	td := bannerTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Apply enrichment state BEFORE loading resources so that the subsequent
	// ResourcesLoadedMsg → applySortAndFilter correctly computes visibleFindingCount
	// from the already-set findingsByID.
	if setEnrichment != nil {
		setEnrichment(&m)
	}

	// ResourcesLoadedMsg triggers applySortAndFilter, which reads findingsByID
	// to compute visibleFindingCount.
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "rds",
		Resources:    resources,
	})
	return m
}

// ---------------------------------------------------------------------------
// T029-a: Banner hidden when findingsByID is empty (cold start)
// ---------------------------------------------------------------------------

// TestEnrichmentBanner_HiddenWhenNoFindings asserts that when there are no
// findings (findingsByID is empty), no banner is shown regardless of other
// enrichment state.
func TestEnrichmentBanner_HiddenWhenNoFindings(t *testing.T) {
	m := loadedBannerModel(t, bannerHealthyResources(), func(m *views.ResourceListModel) {
		// enrichmentRan=true but zero findings — banner must be hidden.
		m.SetEnrichmentState(0, false, true, map[string]resource.EnrichmentFinding{})
	})

	output := m.View()
	if strings.Contains(output, "background checks") {
		t.Error("banner must be hidden when findingsByID is empty, but banner text was found in output")
	}
}

// ---------------------------------------------------------------------------
// T029-b: Banner hidden when enrichmentRanThisSession == false
// ---------------------------------------------------------------------------

// TestEnrichmentBanner_HiddenWhenEnrichmentNotRan asserts that when findings
// are populated but Wave 2 has not run this session (enrichmentRanThisSession=false),
// the banner is not shown. This is the "only cached counts exist" scenario.
func TestEnrichmentBanner_HiddenWhenEnrichmentNotRan(t *testing.T) {
	findings := map[string]resource.EnrichmentFinding{
		"db-1": {Severity: "~", Summary: "pending maintenance: system-update"},
		"db-2": {Severity: "~", Summary: "pending maintenance: minor-version-upgrade"},
	}

	m := loadedBannerModel(t, bannerHealthyResources(), func(m *views.ResourceListModel) {
		// ran=false — Wave 2 has not completed this session.
		m.SetEnrichmentState(0, false, false, findings)
	})

	output := m.View()
	if strings.Contains(output, "background checks") {
		t.Error("banner must be hidden when enrichmentRanThisSession=false, but banner text was found in output")
	}
}

// ---------------------------------------------------------------------------
// T029-c: Banner hidden when at least one issue-colored row is visible
// ---------------------------------------------------------------------------

// TestEnrichmentBanner_HiddenWhenIssueRowVisible asserts that when at least one
// visible row has an issue-colored status (e.g., "failed"), the banner is hidden
// even if findings exist and enrichment ran.
func TestEnrichmentBanner_HiddenWhenIssueRowVisible(t *testing.T) {
	findings := map[string]resource.EnrichmentFinding{
		"db-2": {Severity: "!", Summary: "latest build FAILED"},
	}

	// bannerIssueResources has "db-2" with status "failed" — an issue-colored row.
	m := loadedBannerModel(t, bannerIssueResources(), func(m *views.ResourceListModel) {
		m.SetEnrichmentState(1, false, true, findings)
	})

	output := m.View()
	if strings.Contains(output, "background checks") {
		t.Error("banner must be hidden when issue-colored rows are visible, but banner text was found in output")
	}
}

// ---------------------------------------------------------------------------
// T030-a: Banner long form when no visible row has a finding
// ---------------------------------------------------------------------------

// TestEnrichmentBanner_ShownLongForm_WhenNoVisibleMarkedRow asserts that when
// findings exist and Wave 2 ran, and NO visible row's ID is in findingsByID,
// the banner shows the long form: "⚠ N issues detected by background checks —
// not visible on this page".
func TestEnrichmentBanner_ShownLongForm_WhenNoVisibleMarkedRow(t *testing.T) {
	// Findings are for resources NOT in the current list (off-page findings).
	findings := map[string]resource.EnrichmentFinding{
		"db-99":  {Severity: "~", Summary: "pending maintenance: system-update"},
		"db-100": {Severity: "~", Summary: "pending maintenance: minor-version-upgrade"},
	}

	m := loadedBannerModel(t, bannerHealthyResources(), func(m *views.ResourceListModel) {
		// issueCount=0 (~ findings don't count for menu badge), ran=true.
		m.SetEnrichmentState(0, false, true, findings)
	})

	output := m.View()

	if !strings.Contains(output, "background checks") {
		t.Error("banner must be shown when findings exist, ran=true, and no issue-colored rows visible")
	}
	if !strings.Contains(output, "not visible on this page") {
		t.Errorf("expected long-form banner with '— not visible on this page', got:\n%s", output)
	}
	// Verify count: 2 findings.
	if !strings.Contains(output, "2 issues") {
		t.Errorf("expected '2 issues' in banner, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T030-b: Banner short form when at least one visible row has a finding
// ---------------------------------------------------------------------------

// TestEnrichmentBanner_ShownShortForm_WhenVisibleMarkedRow asserts that when
// at least one visible row's ID is in findingsByID (the row is on this page),
// the banner shows the short form without the "— not visible on this page" suffix.
func TestEnrichmentBanner_ShownShortForm_WhenVisibleMarkedRow(t *testing.T) {
	// db-1 is in bannerHealthyResources AND in findings → visibleFindingCount > 0.
	findings := map[string]resource.EnrichmentFinding{
		"db-1": {Severity: "~", Summary: "pending maintenance: system-update"},
	}

	m := loadedBannerModel(t, bannerHealthyResources(), func(m *views.ResourceListModel) {
		m.SetEnrichmentState(0, false, true, findings)
	})

	output := m.View()

	if !strings.Contains(output, "background checks") {
		t.Error("banner must be shown when findings exist, ran=true, and no issue-colored rows visible")
	}
	if strings.Contains(output, "not visible on this page") {
		t.Errorf("expected SHORT-form banner (no suffix), but got long form:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T030-c: Banner shows N+ when enrichmentTruncated == true
// ---------------------------------------------------------------------------

// TestEnrichmentBanner_TruncatedShowsNPlus asserts that when enrichmentTruncated
// is true, the banner displays "N+" instead of "N" for the issue count.
func TestEnrichmentBanner_TruncatedShowsNPlus(t *testing.T) {
	findings := map[string]resource.EnrichmentFinding{
		"db-99": {Severity: "~", Summary: "pending maintenance: system-update"},
	}

	m := loadedBannerModel(t, bannerHealthyResources(), func(m *views.ResourceListModel) {
		// truncated=true — enrichment count is a lower bound.
		m.SetEnrichmentState(0, true, true, findings)
	})

	output := m.View()

	if !strings.Contains(output, "background checks") {
		t.Error("banner must be shown when findings exist, ran=true, and no issue-colored rows visible")
	}
	// "1+" must appear (not just "1") when truncated.
	if !strings.Contains(output, "1+") {
		t.Errorf("expected '1+' in truncated banner, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T031: Tilde-only findings still trigger the banner (RDS case)
// ---------------------------------------------------------------------------

// TestEnrichmentBanner_TildeOnlyFindingsStillTrigger asserts that findings with
// severity "~" (informational, excluded from menu badge) STILL trigger the banner,
// because the banner keys off len(findingsByID) — NOT the menu-badge IssueCount.
// This is the RDS pending maintenance scenario where IssueCount=0 but findings exist.
func TestEnrichmentBanner_TildeOnlyFindingsStillTrigger(t *testing.T) {
	// All three findings are severity "~" — none contribute to menu IssueCount.
	findings := map[string]resource.EnrichmentFinding{
		"db-99":  {Severity: "~", Summary: "pending maintenance: system-update (New OS patch)"},
		"db-100": {Severity: "~", Summary: "pending maintenance: minor-version-upgrade"},
		"db-101": {Severity: "~", Summary: "pending maintenance: os-upgrade"},
	}

	m := loadedBannerModel(t, bannerHealthyResources(), func(m *views.ResourceListModel) {
		// issueCount=0 because ~ findings don't count; ran=true because Wave 2 completed.
		m.SetEnrichmentState(0, false, true, findings)
	})

	output := m.View()

	if !strings.Contains(output, "background checks") {
		t.Errorf("banner must fire for ~-only findings (severity-agnostic count), but output was:\n%s", output)
	}
	if !strings.Contains(output, "3 issues") {
		t.Errorf("expected '3 issues' in banner, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T032: Text filter hides marked rows → banner uses long form
// ---------------------------------------------------------------------------

// TestEnrichmentBanner_TextFilterHidingMarkedRowsUsesLongForm asserts that when
// a text filter hides all rows that have findings (visibleFindingCount becomes 0),
// the banner uses the LONG form "— not visible on this page", even though the
// underlying allResources contains the finding's resource.
func TestEnrichmentBanner_TextFilterHidingMarkedRowsUsesLongForm(t *testing.T) {
	// Three healthy resources: db-1, db-2, db-3.
	// Finding is for "db-1". We apply a filter "db-2" which hides db-1 and db-3.
	// After filtering: db-2 is visible, db-1 (which has a finding) is hidden.
	findings := map[string]resource.EnrichmentFinding{
		"db-1": {Severity: "~", Summary: "pending maintenance: system-update"},
	}

	// Use NewResourceListFromCache with filterText="db-2" so the filter is applied
	// during construction (before SetEnrichmentState). Then SetEnrichmentState
	// and trigger re-filter by sending a ResourcesLoadedMsg.
	td := bannerTypeDef()
	k := keys.Default()

	// Build with filter pre-applied via NewResourceListFromCache.
	m := views.NewResourceListFromCache(
		td, nil, k,
		bannerHealthyResources(), nil,
		"db-2", // filterText hides db-1 and db-3
		views.SortColNone, true, 0, 0, false,
	)
	m.SetSize(120, 20)

	// Set findings for db-1, Wave 2 ran. Since filter hides db-1,
	// visibleFindingCount will be 0 → long form.
	m.SetEnrichmentState(0, false, true, findings)

	// Trigger applySortAndFilter (re-recomputes visibleFindingCount with current findingsByID).
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "rds",
		Resources:    bannerHealthyResources(),
	})

	output := m.View()

	if !strings.Contains(output, "background checks") {
		t.Errorf("banner must fire even when finding's row is filtered out, but output was:\n%s", output)
	}
	if !strings.Contains(output, "not visible on this page") {
		t.Errorf("expected long form '— not visible on this page' when all marked rows are hidden by filter, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T032-b: Filter reveals issue-colored row → banner disappears
// ---------------------------------------------------------------------------

// TestEnrichmentBanner_IssueRowRevealedByRemovingFilter asserts that when a
// filter hides the issue-colored row (db-2/failed), the banner appears because
// visibleIssueCount==0. This tests the filter-interaction rule: banner fires
// when filtering hides all issue-colored rows.
func TestEnrichmentBanner_IssueRowRevealedByRemovingFilter(t *testing.T) {
	findings := map[string]resource.EnrichmentFinding{
		"db-2": {Severity: "!", Summary: "instance status impaired"},
	}

	// Apply filter "db-1" so only db-1 (available) is visible; db-2 (failed) is hidden.
	// With no issue-colored rows visible, banner should appear.
	td := bannerTypeDef()
	k := keys.Default()

	m := views.NewResourceListFromCache(
		td, nil, k,
		bannerIssueResources(), nil,
		"db-1", // filter hides db-2 (failed)
		views.SortColNone, true, 0, 0, false,
	)
	m.SetSize(120, 20)
	m.SetEnrichmentState(1, false, true, findings)

	// Trigger applySortAndFilter to recompute visibleFindingCount from findingsByID.
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "rds",
		Resources:    bannerIssueResources(),
	})

	// With filter "db-1": only db-1 (available) is visible — no issue rows,
	// and db-2 (which has a finding) is hidden. Banner should fire (long form).
	output := m.View()
	if !strings.Contains(output, "background checks") {
		t.Errorf("banner must show when issue-colored row (db-2/failed) is filtered out and findings exist, got:\n%s", output)
	}
}
