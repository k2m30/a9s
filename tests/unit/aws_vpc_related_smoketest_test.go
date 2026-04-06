package unit_test

// VPC related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to a VPC, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (Subnets, Security Groups, EC2 Instances, Load Balancers,
//     NAT Gateways, Internet Gateways, Route Tables, VPC Endpoints, CloudFormation)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on count=0 (stub) row does NOT emit RelatedNavigateMsg
//
// Demo fixture: vpc-0abc123def456789a
// Demo results: subnet→4, sg→4, ec2→2, elb→1, nat→2, igw→1, rtb→3, vpce→1, cfn→0

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

// vpcSmokeDetail builds a DetailModel for "vpc" using the demo fixture.
func vpcSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "vpc-0abc123def456789a",
		Name: "prod-vpc",
		Fields: map[string]string{
			"vpc_id":     "vpc-0abc123def456789a",
			"name":       "prod-vpc",
			"cidr_block": "10.0.0.0/16",
			"state":      "available",
			"is_default": "false",
		},
		RawStruct: ec2types.Vpc{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "vpc", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverVPCRelatedResult delivers a RelatedCheckResultMsg for "vpc".
func deliverVPCRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "vpc",
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
// VPC-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestVPC_Smoke_S01_RightColVisible(t *testing.T) {
	d := vpcSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("VPC-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("VPC-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// VPC-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestVPC_Smoke_S02_CorrectLabels(t *testing.T) {
	d := vpcSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("VPC-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{
		"Subnets",
		"Security Groups",
		"EC2 Instances",
		"Load Balancers",
		"NAT Gateways",
		"Internet Gateways",
		"Route Tables",
		"VPC Endpoints",
		"CloudFormation",
	} {
		if !strings.Contains(plain, label) {
			t.Errorf("VPC-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// VPC-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestVPC_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := vpcSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("VPC-S03: right column not visible")
	}

	// Deliver demo-equivalent results
	d = deliverVPCRelatedResult(d, "subnet", 4, "subnet-1", "subnet-2", "subnet-3", "subnet-4")
	d = deliverVPCRelatedResult(d, "sg", 4, "sg-1", "sg-2", "sg-3", "sg-4")
	d = deliverVPCRelatedResult(d, "ec2", 2, "i-1", "i-2")
	d = deliverVPCRelatedResult(d, "elb", 1, "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-elb/1234")
	d = deliverVPCRelatedResult(d, "nat", 2, "nat-1", "nat-2")
	d = deliverVPCRelatedResult(d, "igw", 1, "igw-1")
	d = deliverVPCRelatedResult(d, "rtb", 3, "rtb-1", "rtb-2", "rtb-3")
	d = deliverVPCRelatedResult(d, "vpce", 1, "vpce-1")
	d = deliverVPCRelatedResult(d, "cfn", 0)

	plain := stripAnsi(d.View())

	// subnet, sg have (4); ec2, nat have (2); elb, igw, vpce have (1); rtb has (3); cfn has (0)
	hasPositiveCount := strings.Contains(plain, "(4)") || strings.Contains(plain, "(3)") ||
		strings.Contains(plain, "(2)") || strings.Contains(plain, "(1)")
	if !hasPositiveCount {
		t.Errorf("VPC-S03: expected positive count like '(4)', '(3)', '(2)', or '(1)' in right column after delivering results; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("VPC-S03: expected '(0)' for cfn stub row; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// VPC-S04: Tab focuses right column; Enter on subnet row (count=4) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestVPC_Smoke_S04_EnterOnSubnetRowNavigates(t *testing.T) {
	d := vpcSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("VPC-S04: right column not visible")
	}

	d = deliverVPCRelatedResult(d, "subnet", 4, "subnet-1", "subnet-2", "subnet-3", "subnet-4")
	d = deliverVPCRelatedResult(d, "sg", 4, "sg-1", "sg-2", "sg-3", "sg-4")
	d = deliverVPCRelatedResult(d, "ec2", 2, "i-1", "i-2")
	d = deliverVPCRelatedResult(d, "elb", 1, "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-elb/1234")
	d = deliverVPCRelatedResult(d, "nat", 2, "nat-1", "nat-2")
	d = deliverVPCRelatedResult(d, "igw", 1, "igw-1")
	d = deliverVPCRelatedResult(d, "rtb", 3, "rtb-1", "rtb-2", "rtb-3")
	d = deliverVPCRelatedResult(d, "vpce", 1, "vpce-1")
	d = deliverVPCRelatedResult(d, "cfn", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "subnet" (first row, count=4)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("VPC-S04: Enter on subnet row (count=4) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("VPC-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "subnet" {
		t.Errorf("VPC-S04: RelatedNavigateMsg.TargetType = %q, want \"subnet\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// VPC-S05: Enter on all-count=0 right column must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestVPC_Smoke_S05_EnterOnAllZeroRowsNoNav(t *testing.T) {
	d := vpcSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("VPC-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any navigable row
	d = deliverVPCRelatedResult(d, "subnet", 0)
	d = deliverVPCRelatedResult(d, "sg", 0)
	d = deliverVPCRelatedResult(d, "ec2", 0)
	d = deliverVPCRelatedResult(d, "elb", 0)
	d = deliverVPCRelatedResult(d, "nat", 0)
	d = deliverVPCRelatedResult(d, "igw", 0)
	d = deliverVPCRelatedResult(d, "rtb", 0)
	d = deliverVPCRelatedResult(d, "vpce", 0)
	d = deliverVPCRelatedResult(d, "cfn", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("VPC-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// VPC-S06: cfn checker nil; all others non-nil; demo checker registered
// ---------------------------------------------------------------------------

func TestVPC_Smoke_S06_CheckerNilAndDemoRegistered(t *testing.T) {
	defs := resource.GetRelated("vpc")

	// Verify non-nil checkers for the 8 real implementations
	realCheckers := []string{"subnet", "sg", "ec2", "elb", "nat", "igw", "rtb", "vpce"}
	for _, targetType := range realCheckers {
		var found *resource.RelatedDef
		for i := range defs {
			if defs[i].TargetType == targetType {
				found = &defs[i]
				break
			}
		}
		if found == nil {
			t.Errorf("VPC-S06: related def for %q not registered", targetType)
			continue
		}
		if found.Checker == nil {
			t.Errorf("VPC-S06: Checker for %q must be non-nil; got nil", targetType)
		}
	}

	// Verify cfn checker is nil (stub)
	var cfnDef *resource.RelatedDef
	for i := range defs {
		if defs[i].TargetType == "cfn" {
			cfnDef = &defs[i]
			break
		}
	}
	if cfnDef == nil {
		t.Fatal("VPC-S06: cfn related def not registered")
	}
	if cfnDef.Checker != nil {
		t.Fatal("VPC-S06: cfn Checker must be nil (stub); got non-nil — implementation changed?")
	}

	// Demo checker must return a result for cfn (Count:0 is valid)
	checker := resource.GetRelatedDemo("vpc")
	if checker == nil {
		t.Fatal("VPC-S06: no demo checker registered for vpc")
	}
	results := checker(resource.Resource{ID: "vpc-demo"})
	var cfnResult *resource.RelatedCheckResult
	for i := range results {
		if results[i].TargetType == "cfn" {
			cfnResult = &results[i]
			break
		}
	}
	if cfnResult == nil {
		t.Fatal("VPC-S06: demo checker did not return a result for cfn target type")
	}
}
