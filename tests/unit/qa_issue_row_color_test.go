package unit

import (
	"os"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

func TestIsIssueRowColor_GreenStatusesFalse(t *testing.T) {
	greenStatuses := []string{
		"running", "available", "active", "in-use", "succeeded",
		"healthy", "ok", "issued", "deployed", "enabled",
		"green", "success", "completed",
	}
	for _, s := range greenStatuses {
		t.Run(s, func(t *testing.T) {
			if styles.IsIssueRowColor(s) {
				t.Errorf("IsIssueRowColor(%q) = true, want false (green/healthy)", s)
			}
		})
	}
}

func TestIsIssueRowColor_RedStatusesTrue(t *testing.T) {
	redStatuses := []string{
		"stopped", "failed", "error", "deleting", "deleted",
		"timed_out", "unhealthy", "unavailable", "alarm",
		"expired", "revoked", "rejected", "pendingdeletion",
		"rollback_complete", "import_rollback_complete", "red",
		"deregistered", "impaired",
	}
	for _, s := range redStatuses {
		t.Run(s, func(t *testing.T) {
			if !styles.IsIssueRowColor(s) {
				t.Errorf("IsIssueRowColor(%q) = false, want true (red/error)", s)
			}
		})
	}
}

func TestIsIssueRowColor_YellowStatusesTrue(t *testing.T) {
	yellowStatuses := []string{
		"pending", "stopping", "creating", "modifying", "updating",
		"draining", "initial", "insufficient_data",
		"pending_validation", "inprogress", "healing",
		"rebooting_broker", "maintenance", "rebooting", "resizing",
		"pendingimport", "pendingacceptance", "yellow",
		"temporary_failure", "recovering", "recoverable",
		"initializing", "pending_redrive",
	}
	for _, s := range yellowStatuses {
		t.Run(s, func(t *testing.T) {
			if !styles.IsIssueRowColor(s) {
				t.Errorf("IsIssueRowColor(%q) = false, want true (yellow/warning)", s)
			}
		})
	}
}

func TestIsIssueRowColor_DimStatusesFalse(t *testing.T) {
	dimStatuses := []string{
		"terminated", "shutting-down", "aborted",
		"unused", "disabled", "inactive", "grey",
		"not_started", "paused",
	}
	for _, s := range dimStatuses {
		t.Run(s, func(t *testing.T) {
			if styles.IsIssueRowColor(s) {
				t.Errorf("IsIssueRowColor(%q) = true, want false (dim/terminated)", s)
			}
		})
	}
}

func TestIsIssueRowColor_UnknownEmptyFalse(t *testing.T) {
	for _, s := range []string{"", "some_unknown_status", "NONEXISTENT"} {
		t.Run(s, func(t *testing.T) {
			if styles.IsIssueRowColor(s) {
				t.Errorf("IsIssueRowColor(%q) = true, want false (unknown)", s)
			}
		})
	}
}

func TestIsIssueRowColor_SuffixMatching(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"create_failed", true},
		{"update_in_progress", true},
		{"delete_complete", false},
	}
	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			got := styles.IsIssueRowColor(c.status)
			if got != c.want {
				t.Errorf("IsIssueRowColor(%q) = %v, want %v", c.status, got, c.want)
			}
		})
	}
}

func TestIsIssueRowColor_CaseInsensitive(t *testing.T) {
	cases := []string{"STOPPED", "Stopped", "PENDING", "Pending"}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			if !styles.IsIssueRowColor(s) {
				t.Errorf("IsIssueRowColor(%q) = false, want true (case insensitive)", s)
			}
		})
	}
}

func TestIsIssueRowColor_NoColorStillCountsIssues(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	defer func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	}()

	// Under NO_COLOR, issue counts must still be accurate — IsIssueRowColor is
	// color-independent and uses issueStatusSet, not rowColorCache.
	issueStatuses := []string{"stopped", "failed", "pending", "alarm", "impaired"}
	for _, s := range issueStatuses {
		t.Run(s, func(t *testing.T) {
			if !styles.IsIssueRowColor(s) {
				t.Errorf("IsIssueRowColor(%q) = false under NO_COLOR, want true", s)
			}
		})
	}

	healthyStatuses := []string{"running", "available", "active"}
	for _, s := range healthyStatuses {
		t.Run(s+"_healthy", func(t *testing.T) {
			if styles.IsIssueRowColor(s) {
				t.Errorf("IsIssueRowColor(%q) = true under NO_COLOR, want false", s)
			}
		})
	}
}

func TestIsIssueRowColor_CloudTrailSeverity(t *testing.T) {
	// CloudTrail severity levels are NOT resource issues — they are event
	// classifications. They should render colored in event lists but NOT
	// count as issues on the main menu badge.
	cases := []struct {
		status string
		want   bool
	}{
		{"ct-info", false},      // dim — routine reads
		{"ct-attention", false}, // yellow in event list, but NOT a resource issue
		{"ct-danger", false},    // red in event list, but NOT a resource issue
	}
	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			got := styles.IsIssueRowColor(c.status)
			if got != c.want {
				t.Errorf("IsIssueRowColor(%q) = %v, want %v", c.status, got, c.want)
			}
		})
	}
}
