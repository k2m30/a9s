package unit

// qa_prefetch_issue_count_test.go — T012: issue count fields on availability messages.
//
// Tests that AvailabilityPrefetchedMsg carries IssueCounts map[string]int and
// IssueTruncated map[string]bool fields, and that AvailabilityCheckedMsg carries
// an Issues int field.
//
// NOTE: These fields do not exist yet on the message structs. These tests will
// compile and pass once the refactor adds the fields.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ---------------------------------------------------------------------------
// TestAvailabilityPrefetchedMsgIssueCountFields
// ---------------------------------------------------------------------------

// TestAvailabilityPrefetchedMsgIssueCountFields verifies that
// AvailabilityPrefetchedMsg carries IssueCounts and IssueTruncated fields
// and that they hold assigned values correctly.
func TestAvailabilityPrefetchedMsgIssueCountFields(t *testing.T) {
	msg := messages.AvailabilityPrefetchedMsg{
		Entries: map[string]int{
			"ec2": 10,
			"rds": 3,
		},
		Truncated: map[string]bool{
			"ec2": false,
			"rds": true,
		},
		IssueCounts: map[string]int{
			"ec2": 2,
			"rds": 1,
		},
		IssueTruncated: map[string]bool{
			"ec2": false,
			"rds": true,
		},
	}

	t.Run("IssueCounts field holds ec2 count", func(t *testing.T) {
		got := msg.IssueCounts["ec2"]
		if got != 2 {
			t.Errorf("IssueCounts[ec2] = %d, want 2", got)
		}
	})

	t.Run("IssueCounts field holds rds count", func(t *testing.T) {
		got := msg.IssueCounts["rds"]
		if got != 1 {
			t.Errorf("IssueCounts[rds] = %d, want 1", got)
		}
	})

	t.Run("IssueTruncated field reflects rds truncation", func(t *testing.T) {
		got := msg.IssueTruncated["rds"]
		if !got {
			t.Error("IssueTruncated[rds] = false, want true")
		}
	})

	t.Run("IssueTruncated field reflects ec2 non-truncation", func(t *testing.T) {
		got := msg.IssueTruncated["ec2"]
		if got {
			t.Error("IssueTruncated[ec2] = true, want false")
		}
	})

	t.Run("nil IssueCounts is safe to construct without issue fields", func(t *testing.T) {
		bare := messages.AvailabilityPrefetchedMsg{
			Entries:   map[string]int{"ec2": 5},
			Truncated: map[string]bool{"ec2": false},
		}
		// Verify existing fields are accessible (prevent unusedwrite).
		if bare.Entries["ec2"] != 5 {
			t.Errorf("Entries[ec2] = %d, want 5", bare.Entries["ec2"])
		}
		if bare.Truncated["ec2"] {
			t.Error("Truncated[ec2] = true, want false")
		}
		// IssueCounts and IssueTruncated absent → zero values (nil maps)
		if bare.IssueCounts != nil {
			t.Errorf("IssueCounts expected nil, got %v", bare.IssueCounts)
		}
		if bare.IssueTruncated != nil {
			t.Errorf("IssueTruncated expected nil, got %v", bare.IssueTruncated)
		}
	})
}

// ---------------------------------------------------------------------------
// TestAvailabilityCheckedMsgIssueField
// ---------------------------------------------------------------------------

// TestAvailabilityCheckedMsgIssueField verifies that AvailabilityCheckedMsg
// carries an Issues int field and that it holds the assigned value correctly.
func TestAvailabilityCheckedMsgIssueField(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		hasResources bool
		count        int
		issues       int
		truncated    bool
	}{
		{
			name:         "healthy resources — zero issues",
			resourceType: "ec2",
			hasResources: true,
			count:        10,
			issues:       0,
			truncated:    false,
		},
		{
			name:         "some issues found",
			resourceType: "rds",
			hasResources: true,
			count:        5,
			issues:       2,
			truncated:    false,
		},
		{
			name:         "all issues on truncated page",
			resourceType: "eks",
			hasResources: true,
			count:        50,
			issues:       50,
			truncated:    true,
		},
		{
			name:         "no resources — no issues",
			resourceType: "s3",
			hasResources: false,
			count:        0,
			issues:       0,
			truncated:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := messages.AvailabilityCheckedMsg{
				ResourceType: tc.resourceType,
				HasResources: tc.hasResources,
				Count:        tc.count,
				Issues:       tc.issues,
				Truncated:    tc.truncated,
			}
			if msg.Issues != tc.issues {
				t.Errorf("Issues = %d, want %d", msg.Issues, tc.issues)
			}
			if msg.Count != tc.count {
				t.Errorf("Count = %d, want %d", msg.Count, tc.count)
			}
			if msg.ResourceType != tc.resourceType {
				t.Errorf("ResourceType = %q, want %q", msg.ResourceType, tc.resourceType)
			}
			if msg.HasResources != tc.hasResources {
				t.Errorf("HasResources = %v, want %v", msg.HasResources, tc.hasResources)
			}
			if msg.Truncated != tc.truncated {
				t.Errorf("Truncated = %v, want %v", msg.Truncated, tc.truncated)
			}
		})
	}
}
