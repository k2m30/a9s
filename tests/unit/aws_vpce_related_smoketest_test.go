package unit_test

// VPCE related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to a VPC Endpoint, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (Subnets, Security Groups, Route Tables, Network Interfaces)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on all count=0 does NOT emit RelatedNavigateMsg
//
// Demo fixture: vpce-0aaa111111111111a (S3 Gateway endpoint)
// Demo results: subnet→2, sg→1, rtb→2, eni→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// vpceSmokeDetail builds a DetailModel for "vpce" using the demo fixture.
func vpceSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "vpce-0aaa111111111111a",
		Name: "com.amazonaws.us-east-1.s3",
		Fields: map[string]string{
			"vpce_id":      "vpce-0aaa111111111111a",
			"service_name": "com.amazonaws.us-east-1.s3",
			"type":         "Gateway",
			"state":        "available",
			"vpc_id":       "vpc-abc123",
		},
		RawStruct: ec2types.VpcEndpoint{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "vpce", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverVPCERelatedResult delivers a RelatedCheckResultMsg for "vpce".
func deliverVPCERelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "vpce",
		Result: resource.RelatedCheckResult{
			TargetType:  targetType,
			Count:       count,
			ResourceIDs: ids,
		},
	}
	updated, _ := d.Update(msg)
	return updated
}

// ---------------------------------------------------------------------------
// VPCE-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestVPCE_Smoke_S01_RightColVisible(t *testing.T) {
	d := vpceSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("VPCE-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("VPCE-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// VPCE-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestVPCE_Smoke_S02_CorrectLabels(t *testing.T) {
	d := vpceSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("VPCE-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"Subnets", "Security Groups", "Route Tables", "Network Interfaces"} {
		if !strings.Contains(plain, label) {
			t.Errorf("VPCE-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// VPCE-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestVPCE_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := vpceSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("VPCE-S03: right column not visible")
	}

	// Deliver demo-equivalent results for vpce-0aaa111111111111a (S3 Gateway endpoint):
	// subnet→2, sg→1, rtb→2, eni→0
	d = deliverVPCERelatedResult(d, "subnet", 2, "subnet-vpce1", "subnet-vpce2")
	d = deliverVPCERelatedResult(d, "sg", 1, "sg-vpce1")
	d = deliverVPCERelatedResult(d, "rtb", 2, "rtb-vpce1", "rtb-vpce2")
	d = deliverVPCERelatedResult(d, "eni", 0)

	plain := stripAnsi(d.View())

	// subnet(2), sg(1), rtb(2) should show non-zero counts
	if !strings.Contains(plain, "(2)") {
		t.Errorf("VPCE-S03: expected '(2)' count in right column after delivering subnet/rtb results; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(1)") {
		t.Errorf("VPCE-S03: expected '(1)' count in right column after delivering sg result; not found\nview:\n%s", plain)
	}
	// eni should show (0)
	if !strings.Contains(plain, "(0)") {
		t.Errorf("VPCE-S03: expected '(0)' for eni row (count=0); not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// VPCE-S04: Tab focuses right column; Enter on subnet row (count=2) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestVPCE_Smoke_S04_EnterOnSubnetRowNavigates(t *testing.T) {
	d := vpceSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("VPCE-S04: right column not visible")
	}

	d = deliverVPCERelatedResult(d, "subnet", 2, "subnet-vpce1", "subnet-vpce2")
	d = deliverVPCERelatedResult(d, "sg", 1, "sg-vpce1")
	d = deliverVPCERelatedResult(d, "rtb", 2, "rtb-vpce1", "rtb-vpce2")
	d = deliverVPCERelatedResult(d, "eni", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "subnet" (first row, count=2)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("VPCE-S04: Enter on subnet row (count=2) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("VPCE-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "subnet" {
		t.Errorf("VPCE-S04: RelatedNavigateMsg.TargetType = %q, want \"subnet\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// VPCE-S05: Enter on all count=0 rows must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestVPCE_Smoke_S05_EnterOnAllZeroRowsNoNav(t *testing.T) {
	d := vpceSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("VPCE-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any navigable row
	d = deliverVPCERelatedResult(d, "subnet", 0)
	d = deliverVPCERelatedResult(d, "sg", 0)
	d = deliverVPCERelatedResult(d, "rtb", 0)
	d = deliverVPCERelatedResult(d, "eni", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("VPCE-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// VPCE-S06: All 4 checkers non-nil. Demo checker registered.
// This verifies all related defs have real checker implementations.
// ---------------------------------------------------------------------------

func TestVPCE_Smoke_S06_AllCheckersNonNilAndDemoRegistered(t *testing.T) {
	defs := resource.GetRelated("vpce")
	if len(defs) != 4 {
		t.Fatalf("VPCE-S06: expected 4 related defs for vpce, got %d", len(defs))
	}

	expectedTargets := []string{"subnet", "sg", "rtb", "eni"}
	for _, target := range expectedTargets {
		var found *resource.RelatedDef
		for i := range defs {
			if defs[i].TargetType == target {
				found = &defs[i]
				break
			}
		}
		if found == nil {
			t.Errorf("VPCE-S06: related def for target %q not registered", target)
			continue
		}
		if found.Checker == nil {
			t.Errorf("VPCE-S06: Checker for target %q must be non-nil (real implementation); got nil", target)
		}
	}

	// Demo checker must be registered for vpce
	checker := resource.GetRelatedDemo("vpce")
	if checker == nil {
		t.Fatal("VPCE-S06: no demo checker registered for vpce")
	}

	// Demo checker must return results for each expected target type
	results := checker(resource.Resource{ID: "vpce-0aaa111111111111a"})
	for _, target := range expectedTargets {
		var found *resource.RelatedCheckResult
		for i := range results {
			if results[i].TargetType == target {
				found = &results[i]
				break
			}
		}
		if found == nil {
			t.Errorf("VPCE-S06: demo checker did not return a result for target type %q", target)
		}
	}
}
