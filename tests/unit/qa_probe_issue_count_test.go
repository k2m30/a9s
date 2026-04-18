package unit

// qa_probe_issue_count_test.go — T011: issue counting logic for availability probe.
//
// Tests the counting logic used inside probeResourceAvailability to populate
// AvailabilityCheckedMsg.Issues. Post-refactor, counting is per-type via
// td.Color(r).IsIssue() (and ExcludeFromIssueBadge for badge-exempt types).

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// countIssueRowsForType counts resources where td.Color(r).IsIssue() returns true,
// excluding types with ExcludeFromIssueBadge set (they don't contribute to badge counts).
// This mirrors the logic inside probeResourceAvailability.
func countIssueRowsForType(td *resource.ResourceTypeDef, resources []resource.Resource) int {
	if td == nil || td.Color == nil || td.ExcludeFromIssueBadge {
		return 0
	}
	count := 0
	for _, r := range resources {
		if td.Color(r).IsIssue() {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// TestCountIssueRowsForType_EC2
// ---------------------------------------------------------------------------

// TestCountIssueRowsForType_EC2 verifies the counting logic for EC2 instances.
// EC2 issue classification reads Fields["state"] and status fields, not Resource.Status.
func TestCountIssueRowsForType_EC2(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found")
	}

	tests := []struct {
		name      string
		resources []resource.Resource
		want      int
	}{
		{
			name: "mixed ec2 statuses: only stopped/pending counted",
			resources: []resource.Resource{
				{ID: "i-001", Fields: map[string]string{"state": "running"}},    // Healthy
				{ID: "i-002", Fields: map[string]string{"state": "stopped"}},    // Broken — issue
				{ID: "i-003", Fields: map[string]string{"state": "pending"}},    // Warning — issue
				{ID: "i-004", Fields: map[string]string{"state": "terminated"}}, // Dim
				{ID: "i-005", Fields: map[string]string{"state": "stopping"}},   // Broken — issue
			},
			want: 3, // stopped + pending + stopping
		},
		{
			name: "all healthy running",
			resources: []resource.Resource{
				{ID: "i-001", Fields: map[string]string{"state": "running"}},
				{ID: "i-002", Fields: map[string]string{"state": "running"}},
			},
			want: 0,
		},
		{
			name: "all stopped",
			resources: []resource.Resource{
				{ID: "i-001", Fields: map[string]string{"state": "stopped"}},
				{ID: "i-002", Fields: map[string]string{"state": "stopped"}},
				{ID: "i-003", Fields: map[string]string{"state": "stopped"}},
			},
			want: 3,
		},
		{
			name: "impaired running instance (system_status=impaired)",
			resources: []resource.Resource{
				{ID: "i-001", Fields: map[string]string{"state": "running", "system_status": "impaired"}},
				{ID: "i-002", Fields: map[string]string{"state": "running", "system_status": "ok"}},
			},
			want: 1,
		},
		{
			name: "initializing instance (instance_status=initializing)",
			resources: []resource.Resource{
				{ID: "i-001", Fields: map[string]string{"state": "running", "instance_status": "initializing"}},
				{ID: "i-002", Fields: map[string]string{"state": "running"}},
			},
			want: 1,
		},
		{
			name: "empty list",
			resources: []resource.Resource{},
			want:      0,
		},
		{
			name: "terminated is dim, shutting-down is warning (issue)",
			resources: []resource.Resource{
				{ID: "i-001", Fields: map[string]string{"state": "terminated"}},
				{ID: "i-002", Fields: map[string]string{"state": "shutting-down"}},
			},
			want: 1, // shutting-down → Warning (transitional issue), terminated → Dim (not issue)
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := countIssueRowsForType(td, tc.resources)
			if got != tc.want {
				t.Errorf("countIssueRowsForType(ec2, ...) = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestCountIssueRowsForType_ExcludeFromIssueBadge verifies that types with
// ExcludeFromIssueBadge set (e.g. ct-events) always return 0 from the counter,
// even when they have Broken/Warning rows.
func TestCountIssueRowsForType_ExcludeFromIssueBadge(t *testing.T) {
	td := resource.FindResourceType("ct-events")
	if td == nil {
		t.Fatal("ct-events resource type not found")
	}
	if !td.ExcludeFromIssueBadge {
		t.Fatal("ct-events: ExcludeFromIssueBadge must be true for this test to be meaningful")
	}

	resources := []resource.Resource{
		{ID: "evt-0001", Status: "ct-danger"},
		{ID: "evt-0002", Status: "ct-attention"},
		{ID: "evt-0003", Status: "ct-info"},
	}

	got := countIssueRowsForType(td, resources)
	if got != 0 {
		t.Errorf("countIssueRowsForType(ct-events, ...) = %d, want 0 (ExcludeFromIssueBadge=true)", got)
	}
}

// TestCountIssueRowsForType_NilTypeDef guards against nil type def.
func TestCountIssueRowsForType_NilTypeDef(t *testing.T) {
	resources := []resource.Resource{
		{ID: "r-001", Status: "stopped"},
	}
	got := countIssueRowsForType(nil, resources)
	if got != 0 {
		t.Errorf("countIssueRowsForType(nil, ...) = %d, want 0", got)
	}
}
