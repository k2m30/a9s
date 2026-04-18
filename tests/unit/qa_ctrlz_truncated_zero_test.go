package unit

// qa_ctrlz_truncated_zero_test.go — Tests that ctrl+z on the main menu shows
// truncated-zero types. Post-AlwaysHealthy-purge (per docs/attention-signals.md,
// every type has at least a Wave 1 or Wave 2 signal), a truncated-zero count
// is a LOWER BOUND — unread pages may carry issues — so the type stays visible
// so the user can drill in.

import (
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func TestCtrlZ_TruncatedZero_Visible(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	defer func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	}()

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	// S3 has 50+ resources, zero issues on page 1, truncated.
	// Per docs/attention-signals.md, S3 has a Wave 2 signal (GetPublicAccessBlock
	// per bucket) — truncated-zero is a lower bound, not confirmed zero.
	m.SetAvailability("s3", 50)
	m.SetTruncated("s3", true)
	m.SetIssues("s3", 0, true)

	m.SetAvailability("ec2", 19)
	m.SetIssues("ec2", 1, false)

	m.Toggle()
	m.SetIssues("ec2", 1, false) // re-trigger applyFilter

	plain := m.View()

	if !strings.Contains(plain, "S3 Buckets") {
		t.Error("S3 Buckets (truncated-zero) should be VISIBLE under ctrl+z — unread pages may carry issues (Wave 2 signal)")
	}
	if !strings.Contains(plain, "EC2 Instances") {
		t.Error("EC2 Instances (has issues) should be visible under ctrl+z")
	}
}

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
