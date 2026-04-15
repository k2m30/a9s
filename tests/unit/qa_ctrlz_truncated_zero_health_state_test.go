package unit

// qa_ctrlz_truncated_zero_health_state_test.go — pins the distinction between:
//   - Always-healthy types (S3, IAM, etc.): truncated-zero is CONFIRMED zero → hide under ctrl+z
//   - Health-state types (EC2, ENI, RDS, etc.): truncated-zero is a LOWER BOUND
//     (issues may exist on pages not yet loaded) → show under ctrl+z so the user
//     can drill in.
//
// See CodeRabbit review on PR #273 for context. The rule lives in
// ResourceTypeDef.AlwaysHealthy; isVisibleUnderIssueFilter consults it.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestCtrlZ_TruncatedZero_EC2_Visible — health-state type (EC2) with truncated
// zero issues must remain visible under ctrl+z, because page 2+ may have issues.
func TestCtrlZ_TruncatedZero_EC2_Visible(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	m.SetAvailability("ec2", 50)
	m.SetTruncated("ec2", true)
	m.SetIssues("ec2", 0, true) // zero issues on page 1, but truncated

	m.Toggle()
	m.SetIssues("ec2", 0, true) // re-trigger applyFilter after toggle

	plain := m.View()
	if !strings.Contains(plain, "EC2 Instances") {
		t.Errorf("EC2 Instances (health-state type, truncated-zero) must be visible under ctrl+z — page 2+ may have issues; got:\n%s", plain)
	}
}

// TestCtrlZ_TruncatedZero_ENI_Visible — same rule as EC2; ENI has state-based
// Color (attaching/detaching → Warning).
func TestCtrlZ_TruncatedZero_ENI_Visible(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	m.SetAvailability("eni", 35)
	m.SetTruncated("eni", true)
	m.SetIssues("eni", 0, true)

	m.Toggle()
	m.SetIssues("eni", 0, true)

	plain := m.View()
	if !strings.Contains(plain, "Network Interfaces") {
		t.Errorf("ENI (health-state type, truncated-zero) must be visible under ctrl+z; got:\n%s", plain)
	}
}

// TestCtrlZ_TruncatedZero_S3_Hidden — always-healthy type (S3) with truncated
// zero issues must stay hidden under ctrl+z; the count is CONFIRMED zero.
func TestCtrlZ_TruncatedZero_S3_Hidden(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	m.SetAvailability("s3", 50)
	m.SetTruncated("s3", true)
	m.SetIssues("s3", 0, true)

	// Seed EC2 so menu is non-empty under ctrl+z.
	m.SetAvailability("ec2", 5)
	m.SetIssues("ec2", 1, false)

	m.Toggle()
	m.SetIssues("ec2", 1, false) // re-trigger applyFilter

	plain := m.View()
	if strings.Contains(plain, "S3 Buckets") {
		t.Errorf("S3 (always-healthy type, truncated-zero) must be hidden under ctrl+z — count is confirmed zero; got:\n%s", plain)
	}
}

// TestResourceTypeDef_AlwaysHealthy_Registered — every type whose Color func
// never returns an issue color (Warning/Broken) must have AlwaysHealthy set.
// ColorDim is not an issue color; a type returning only Healthy+Dim qualifies.
// Guard against forgetting to flag new always-healthy types.
func TestResourceTypeDef_AlwaysHealthy_Registered(t *testing.T) {
	// AlwaysHealthy means the Color func never returns Warning or Broken.
	// ColorDim (e.g. "scheduled for deletion") is not an issue — types that
	// return only ColorHealthy and/or ColorDim also qualify. This list is
	// derived by reading the Color function bodies in internal/resource/types_*.go.
	expected := map[string]bool{
		"backup": true, "ses": true, "ecr": true,
		"codeartifact": true, "athena": true, "s3": true,
		"opensearch": true, "redshift": true, "rds-snap": true, "r53": true,
		"apigw": true, "sqs": true, "sns": true, "sns-sub": true, "kinesis": true,
		"msk": true, "logs": true, "trail": true, "sg": true,
		"rtb": true, "igw": true, "eip": true, "ssm": true,
		"role": true, "policy": true, "iam-user": true, "iam-group": true, "waf": true,
		"secrets": true,
	}
	for _, td := range resource.AllResourceTypes() {
		if expected[td.ShortName] && !td.AlwaysHealthy {
			t.Errorf("type %q has a trivial always-healthy Color func but AlwaysHealthy=false", td.ShortName)
		}
		if !expected[td.ShortName] && td.AlwaysHealthy {
			t.Errorf("type %q marked AlwaysHealthy=true but its Color func is non-trivial (expected state-based)", td.ShortName)
		}
	}
}
