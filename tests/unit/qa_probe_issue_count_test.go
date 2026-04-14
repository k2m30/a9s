package unit

// qa_probe_issue_count_test.go — T011: issue counting logic for availability probe.
//
// Tests the counting logic that will be used inside probeResourceAvailability
// to populate AvailabilityCheckedMsg.Issues. Since the probe itself makes AWS
// calls, we test the counting logic directly via styles.IsIssueRowColor.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// countIssueRowsInResources counts resources where IsIssueRowColor returns true.
// This mirrors the logic that will be used inside probeResourceAvailability
// to populate the Issues field of AvailabilityCheckedMsg.
func countIssueRowsInResources(resources []resource.Resource) int {
	count := 0
	for _, r := range resources {
		if styles.IsIssueRowColor(r.Status) {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// TestCountIssueRowsInResources
// ---------------------------------------------------------------------------

// TestCountIssueRowsInResources verifies the counting logic across mixed
// resource statuses, matching what the probe will compute.
func TestCountIssueRowsInResources(t *testing.T) {
	tests := []struct {
		name      string
		resources []resource.Resource
		want      int
	}{
		{
			name: "mixed statuses: only red and yellow counted",
			resources: []resource.Resource{
				{ID: "r-001", Status: "running"},    // green — not issue
				{ID: "r-002", Status: "stopped"},    // red — issue
				{ID: "r-003", Status: "pending"},    // yellow — issue
				{ID: "r-004", Status: "terminated"}, // dim — not issue
				{ID: "r-005", Status: "failed"},     // red — issue
			},
			want: 3, // stopped + pending + failed
		},
		{
			name: "all healthy (running)",
			resources: []resource.Resource{
				{ID: "r-001", Status: "running"},
				{ID: "r-002", Status: "running"},
			},
			want: 0,
		},
		{
			name: "all red (stopped)",
			resources: []resource.Resource{
				{ID: "r-001", Status: "stopped"},
				{ID: "r-002", Status: "stopped"},
				{ID: "r-003", Status: "stopped"},
			},
			want: 3,
		},
		{
			name: "yellow statuses (creating, updating, draining)",
			resources: []resource.Resource{
				{ID: "r-001", Status: "creating"},
				{ID: "r-002", Status: "updating"},
				{ID: "r-003", Status: "draining"},
			},
			want: 3,
		},
		{
			name: "suffix-matched issue statuses",
			resources: []resource.Resource{
				{ID: "r-001", Status: "deploy_failed"},
				{ID: "r-002", Status: "update_in_progress"},
				{ID: "r-003", Status: "running"}, // not an issue
			},
			want: 2, // deploy_failed + update_in_progress
		},
		{
			name: "empty list",
			resources: []resource.Resource{},
			want:      0,
		},
		{
			name: "single healthy resource",
			resources: []resource.Resource{
				{ID: "r-001", Status: "running"},
			},
			want: 0,
		},
		{
			name: "single issue resource",
			resources: []resource.Resource{
				{ID: "r-001", Status: "stopped"},
			},
			want: 1,
		},
		{
			name: "terminated and deleted are red (issues)",
			resources: []resource.Resource{
				{ID: "r-001", Status: "terminated"}, // dim — not issue
				{ID: "r-002", Status: "deleted"},    // red — issue
			},
			want: 1, // only deleted
		},
		{
			name: "error status is an issue",
			resources: []resource.Resource{
				{ID: "r-001", Status: "error"},
				{ID: "r-002", Status: "available"}, // green-ish — not issue
			},
			want: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := countIssueRowsInResources(tc.resources)
			if got != tc.want {
				t.Errorf("countIssueRowsInResources() = %d, want %d", got, tc.want)
			}
		})
	}
}
