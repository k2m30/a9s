package unit

// qa_menu_badges_test.go — T027: main menu issue badge data layer tests.
//
// These tests verify the SetIssues() method on MainMenuModel correctly stores
// issue counts, known flags, and truncation flags for per-type badge rendering.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestMainMenuSetIssues verifies that SetIssues stores count, marks type as known,
// and stores truncated=false.
func TestMainMenuSetIssues(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetIssues("ec2", 3, false)

	counts := m.GetIssueCounts()
	known := m.GetIssueKnown()
	truncated := m.GetIssueTruncated()

	if counts["ec2"] != 3 {
		t.Errorf("GetIssueCounts()[ec2] = %d, want 3", counts["ec2"])
	}
	if !known["ec2"] {
		t.Error("GetIssueKnown()[ec2] = false, want true")
	}
	if truncated["ec2"] {
		t.Error("GetIssueTruncated()[ec2] = true, want false")
	}
}

// TestMainMenuSetIssuesTruncated verifies that SetIssues stores truncated=true.
func TestMainMenuSetIssuesTruncated(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetIssues("ec2", 5, true)

	truncated := m.GetIssueTruncated()
	counts := m.GetIssueCounts()
	known := m.GetIssueKnown()

	if !truncated["ec2"] {
		t.Error("GetIssueTruncated()[ec2] = false, want true")
	}
	if counts["ec2"] != 5 {
		t.Errorf("GetIssueCounts()[ec2] = %d, want 5", counts["ec2"])
	}
	if !known["ec2"] {
		t.Error("GetIssueKnown()[ec2] = false, want true")
	}
}

// TestMainMenuSetIssuesZero verifies that SetIssues with count=0 marks the type
// as known with a confirmed zero issue count.
func TestMainMenuSetIssuesZero(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetIssues("ec2", 0, false)

	counts := m.GetIssueCounts()
	known := m.GetIssueKnown()
	truncated := m.GetIssueTruncated()

	if !known["ec2"] {
		t.Error("GetIssueKnown()[ec2] = false, want true (zero count must be marked known)")
	}
	if counts["ec2"] != 0 {
		t.Errorf("GetIssueCounts()[ec2] = %d, want 0", counts["ec2"])
	}
	if truncated["ec2"] {
		t.Error("GetIssueTruncated()[ec2] = true, want false")
	}
}

// TestMainMenuSetIssuesOverwrite verifies that a second SetIssues call overwrites
// the previous value.
func TestMainMenuSetIssuesOverwrite(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetIssues("ec2", 3, false)
	m.SetIssues("ec2", 7, true)

	counts := m.GetIssueCounts()
	truncated := m.GetIssueTruncated()
	known := m.GetIssueKnown()

	if counts["ec2"] != 7 {
		t.Errorf("GetIssueCounts()[ec2] = %d, want 7 after overwrite", counts["ec2"])
	}
	if !truncated["ec2"] {
		t.Error("GetIssueTruncated()[ec2] = false, want true after overwrite")
	}
	if !known["ec2"] {
		t.Error("GetIssueKnown()[ec2] = false, want true after overwrite")
	}
}

// TestMainMenuSetIssuesMultipleTypes verifies independent storage per resource type.
func TestMainMenuSetIssuesMultipleTypes(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetIssues("ec2", 3, false)
	m.SetIssues("rds", 0, false)
	m.SetIssues("s3", 10, true)

	counts := m.GetIssueCounts()
	known := m.GetIssueKnown()
	truncated := m.GetIssueTruncated()

	cases := []struct {
		name        string
		wantCount   int
		wantKnown   bool
		wantTrunc   bool
	}{
		{"ec2", 3, true, false},
		{"rds", 0, true, false},
		{"s3", 10, true, true},
	}
	for _, tc := range cases {
		if counts[tc.name] != tc.wantCount {
			t.Errorf("counts[%s] = %d, want %d", tc.name, counts[tc.name], tc.wantCount)
		}
		if known[tc.name] != tc.wantKnown {
			t.Errorf("known[%s] = %v, want %v", tc.name, known[tc.name], tc.wantKnown)
		}
		if truncated[tc.name] != tc.wantTrunc {
			t.Errorf("truncated[%s] = %v, want %v", tc.name, truncated[tc.name], tc.wantTrunc)
		}
	}
}
