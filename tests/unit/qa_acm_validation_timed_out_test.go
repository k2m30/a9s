package unit

// qa_acm_validation_timed_out_test.go — Regression: ACM VALIDATION_TIMED_OUT → ColorBroken.
//
// Bug: VALIDATION_TIMED_OUT was not included in the ColorBroken case group,
// causing expired/timed-out certificates to appear as healthy (default return).
// Fix: VALIDATION_TIMED_OUT added to the case "EXPIRED", "REVOKED", "FAILED", "VALIDATION_TIMED_OUT".
//
// Tests fail if the fix is reverted: VALIDATION_TIMED_OUT would fall through to
// the default return (ColorHealthy) instead of ColorBroken.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func acmResource(status string) resource.Resource {
	return resource.Resource{
		ID:     "arn:aws:acm:us-east-1:123456789012:certificate/abc-123",
		Name:   "example.com",
		Fields: map[string]string{"status": status},
	}
}

// TestACMColor_ValidationTimedOut_IsColorBroken verifies VALIDATION_TIMED_OUT → ColorBroken.
// Regresses if the case is removed and status falls through to ColorHealthy default.
func TestACMColor_ValidationTimedOut_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("acm")
	if got := td.Color(acmResource("VALIDATION_TIMED_OUT")); got != resource.ColorBroken {
		t.Errorf("acm Color status=VALIDATION_TIMED_OUT = %v, want ColorBroken — was the VALIDATION_TIMED_OUT case removed?", got)
	}
}

// TestACMColor_Issued_IsColorHealthy verifies ISSUED → ColorHealthy.
func TestACMColor_Issued_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("acm")
	if got := td.Color(acmResource("ISSUED")); got != resource.ColorHealthy {
		t.Errorf("acm Color status=ISSUED = %v, want ColorHealthy", got)
	}
}

// TestACMColor_PendingValidation_IsColorWarning verifies PENDING_VALIDATION → ColorWarning.
func TestACMColor_PendingValidation_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("acm")
	if got := td.Color(acmResource("PENDING_VALIDATION")); got != resource.ColorWarning {
		t.Errorf("acm Color status=PENDING_VALIDATION = %v, want ColorWarning", got)
	}
}

// TestACMColor_Expired_IsColorBroken verifies EXPIRED → ColorBroken.
func TestACMColor_Expired_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("acm")
	if got := td.Color(acmResource("EXPIRED")); got != resource.ColorBroken {
		t.Errorf("acm Color status=EXPIRED = %v, want ColorBroken", got)
	}
}

// TestACMColor_Revoked_IsColorBroken verifies REVOKED → ColorBroken.
func TestACMColor_Revoked_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("acm")
	if got := td.Color(acmResource("REVOKED")); got != resource.ColorBroken {
		t.Errorf("acm Color status=REVOKED = %v, want ColorBroken", got)
	}
}

// TestACMColor_Failed_IsColorBroken verifies FAILED → ColorBroken.
func TestACMColor_Failed_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("acm")
	if got := td.Color(acmResource("FAILED")); got != resource.ColorBroken {
		t.Errorf("acm Color status=FAILED = %v, want ColorBroken", got)
	}
}

// TestACMColor_Inactive_IsColorDim verifies INACTIVE → ColorDim.
func TestACMColor_Inactive_IsColorDim(t *testing.T) {
	td := resource.FindResourceType("acm")
	if got := td.Color(acmResource("INACTIVE")); got != resource.ColorDim {
		t.Errorf("acm Color status=INACTIVE = %v, want ColorDim", got)
	}
}
