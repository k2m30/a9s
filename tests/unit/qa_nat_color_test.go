package unit

// qa_nat_color_test.go — Regression tests for NAT Gateway Color state mapping.
//
// Pins each state in the NAT Gateway Color switch so that regressions in
// state-to-Color mappings are caught. Tests fail if any state is remapped.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func natResource(state string) resource.Resource {
	return resource.Resource{
		ID:     "nat-0abc1234def56789a",
		Name:   "my-nat",
		Fields: map[string]string{"state": state},
	}
}

// TestNATColor_Available_IsColorHealthy verifies "available" → ColorHealthy.
func TestNATColor_Available_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("nat")
	if got := td.Color(natResource("available")); got != resource.ColorHealthy {
		t.Errorf("nat Color state=available = %v, want ColorHealthy", got)
	}
}

// TestNATColor_Pending_IsColorWarning verifies "pending" → ColorWarning.
func TestNATColor_Pending_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("nat")
	if got := td.Color(natResource("pending")); got != resource.ColorWarning {
		t.Errorf("nat Color state=pending = %v, want ColorWarning", got)
	}
}

// TestNATColor_Deleting_IsColorWarning verifies "deleting" → ColorWarning.
func TestNATColor_Deleting_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("nat")
	if got := td.Color(natResource("deleting")); got != resource.ColorWarning {
		t.Errorf("nat Color state=deleting = %v, want ColorWarning", got)
	}
}

// TestNATColor_Failed_IsColorBroken verifies "failed" → ColorBroken.
func TestNATColor_Failed_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("nat")
	if got := td.Color(natResource("failed")); got != resource.ColorBroken {
		t.Errorf("nat Color state=failed = %v, want ColorBroken", got)
	}
}

// TestNATColor_Deleted_IsColorDim verifies "deleted" → ColorDim.
func TestNATColor_Deleted_IsColorDim(t *testing.T) {
	td := resource.FindResourceType("nat")
	if got := td.Color(natResource("deleted")); got != resource.ColorDim {
		t.Errorf("nat Color state=deleted = %v, want ColorDim", got)
	}
}

// TestNATColor_Unknown_IsColorHealthy verifies unknown state → ColorHealthy (default).
func TestNATColor_Unknown_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("nat")
	if got := td.Color(natResource("unknown-state")); got != resource.ColorHealthy {
		t.Errorf("nat Color state=unknown-state = %v, want ColorHealthy (default)", got)
	}
}
