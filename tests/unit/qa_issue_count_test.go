package unit

// qa_issue_count_test.go — T010: IssueCount() accessor tests.
//
// These tests verify that IssueCount() on ResourceListModel counts ONLY
// red/yellow statuses (as determined by styles.IsIssueRowColor), not green
// (running) or dim (terminated/unknown) rows.
//
// NOTE: IssueCount() and the underlying issueCount field do not exist yet.
// These tests will compile and pass once the refactor adds the accessor.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// issueCountEC2TypeDef returns a minimal EC2 ResourceTypeDef for issue count tests.
func issueCountEC2TypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
}

// buildIssueCountList constructs a ResourceListModel from the given resources.
func buildIssueCountList(t *testing.T, resources []resource.Resource) views.ResourceListModel {
	t.Helper()
	return views.NewResourceListFromCache(
		issueCountEC2TypeDef(),
		nil,
		keys.Default(),
		resources,
		nil,
		"",
		views.SortColNone,
		true,
		0,
		0,
		false,
	)
}

// ---------------------------------------------------------------------------
// TestResourceListIssueCountOnlyRedYellow
// ---------------------------------------------------------------------------

// TestResourceListIssueCountOnlyRedYellow verifies that IssueCount() returns
// the count of red + yellow status rows only, excluding green (running) and
// dim (terminated) rows.
//
// Resources: 5 running (green), 3 stopped (red), 2 pending (yellow), 1 terminated (dim)
// Expected: IssueCount() == 5  (3 red + 2 yellow)
func TestResourceListIssueCountOnlyRedYellow(t *testing.T) {
	resources := []resource.Resource{
		// Green — running (not an issue)
		{ID: "i-001", Name: "web-01", Fields: map[string]string{"status": "running"}},
		{ID: "i-002", Name: "web-02", Fields: map[string]string{"status": "running"}},
		{ID: "i-003", Name: "web-03", Fields: map[string]string{"status": "running"}},
		{ID: "i-004", Name: "web-04", Fields: map[string]string{"status": "running"}},
		{ID: "i-005", Name: "web-05", Fields: map[string]string{"status": "running"}},
		// Red — stopped (issue)
		{ID: "i-006", Name: "db-01", Fields: map[string]string{"status": "stopped"}},
		{ID: "i-007", Name: "db-02", Fields: map[string]string{"status": "stopped"}},
		{ID: "i-008", Name: "db-03", Fields: map[string]string{"status": "stopped"}},
		// Yellow — pending (issue)
		{ID: "i-009", Name: "cache-01", Fields: map[string]string{"status": "pending"}},
		{ID: "i-010", Name: "cache-02", Fields: map[string]string{"status": "pending"}},
		// Dim — terminated (not an issue)
		{ID: "i-011", Name: "old-01", Fields: map[string]string{"status": "terminated"}},
	}

	m := buildIssueCountList(t, resources)
	got := m.IssueCount()
	want := 5 // 3 stopped + 2 pending
	if got != want {
		t.Errorf("IssueCount() = %d, want %d (3 stopped + 2 pending)", got, want)
	}
}

// ---------------------------------------------------------------------------
// TestResourceListIssueCountZeroWhenAllHealthy
// ---------------------------------------------------------------------------

// TestResourceListIssueCountZeroWhenAllHealthy verifies that IssueCount()
// returns 0 when all resources have a healthy (green) status.
func TestResourceListIssueCountZeroWhenAllHealthy(t *testing.T) {
	resources := []resource.Resource{
		{ID: "i-001", Name: "web-01", Fields: map[string]string{"status": "running"}},
		{ID: "i-002", Name: "web-02", Fields: map[string]string{"status": "running"}},
		{ID: "i-003", Name: "web-03", Fields: map[string]string{"status": "running"}},
	}

	m := buildIssueCountList(t, resources)
	got := m.IssueCount()
	if got != 0 {
		t.Errorf("IssueCount() = %d, want 0 for all-running resources", got)
	}
}

// ---------------------------------------------------------------------------
// TestResourceListIssueCountAllIssues
// ---------------------------------------------------------------------------

// TestResourceListIssueCountAllIssues verifies that IssueCount() equals the
// total resource count when all resources have red or yellow statuses.
func TestResourceListIssueCountAllIssues(t *testing.T) {
	resources := []resource.Resource{
		{ID: "i-001", Name: "db-01", Fields: map[string]string{"status": "stopped"}},
		{ID: "i-002", Name: "db-02", Fields: map[string]string{"status": "failed"}},
		{ID: "i-003", Name: "db-03", Fields: map[string]string{"status": "error"}},
		{ID: "i-004", Name: "cache-01", Fields: map[string]string{"status": "pending"}},
		{ID: "i-005", Name: "cache-02", Fields: map[string]string{"status": "creating"}},
	}

	m := buildIssueCountList(t, resources)
	got := m.IssueCount()
	want := len(resources)
	if got != want {
		t.Errorf("IssueCount() = %d, want %d (all resources are issues)", got, want)
	}
}

// ---------------------------------------------------------------------------
// TestResourceListIssueCountEmptyList
// ---------------------------------------------------------------------------

// TestResourceListIssueCountEmptyList verifies that IssueCount() returns 0
// for an empty resource list.
func TestResourceListIssueCountEmptyList(t *testing.T) {
	m := buildIssueCountList(t, []resource.Resource{})
	got := m.IssueCount()
	if got != 0 {
		t.Errorf("IssueCount() = %d, want 0 for empty list", got)
	}
}
