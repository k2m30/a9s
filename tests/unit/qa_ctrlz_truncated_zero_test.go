package unit

// qa_ctrlz_truncated_zero_test.go — Tests that ctrl+z on the main menu hides
// truncated-zero types (types with count 50+ but zero issues). Config-only
// resources like S3, ENI, IAM Roles will never have issues regardless of page
// count, so showing them under ctrl+z is noise.

import (
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func TestCtrlZ_TruncatedZero_Hidden(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	defer func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	}()

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	// Simulate: S3 has 50+ resources but zero issues (config-only, no status field)
	m.SetAvailability("s3", 50)
	m.SetTruncated("s3", true)
	m.SetIssues("s3", 0, true) // zero issues, truncated

	// Simulate: EC2 has issues
	m.SetAvailability("ec2", 19)
	m.SetIssues("ec2", 1, false)

	// Enable ctrl+z — Toggle() only flips the bool; we need applyFilter to run.
	// SetIssues calls applyFilter, so set a dummy value to trigger it after toggle.
	m.Toggle()
	// Trigger applyFilter by re-setting an issue count (SetIssues calls applyFilter).
	m.SetIssues("ec2", 1, false)

	plain := m.View()

	// S3 should be HIDDEN — zero issues, even though truncated
	if strings.Contains(plain, "S3 Buckets") {
		t.Error("S3 Buckets (truncated-zero) should be hidden under ctrl+z — config-only type will never have issues")
	}

	// EC2 should be VISIBLE — has issues
	if !strings.Contains(plain, "EC2 Instances") {
		t.Error("EC2 Instances (has issues) should be visible under ctrl+z")
	}
}

// Previously this test asserted ENI truncated-zero hidden. That was wrong —
// ENI is a health-state type (attaching/detaching → Warning), so truncated-zero
// means issues may exist on unread pages. Coverage for ENI-visible moved to
// qa_ctrlz_truncated_zero_health_state_test.go:TestCtrlZ_TruncatedZero_ENI_Visible.

func TestCtrlZ_TruncatedNonzero_Visible(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	defer func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	}()

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	// Truncated with actual issues — should stay visible
	m.SetAvailability("event", 50)
	m.SetTruncated("event", true)
	m.SetIssues("event", 5, true) // 5+ issues

	m.Toggle()

	plain := m.View()
	if !strings.Contains(plain, "CloudTrail Events") {
		t.Error("CloudTrail Events (truncated with issues) should be visible under ctrl+z")
	}
}
