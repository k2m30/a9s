package unit_test

// Subnet related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to a Subnet, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (EC2 Instances, Network Interfaces, NAT Gateways, Load Balancers, Route Tables, CloudFormation)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on count=0 row does NOT emit RelatedNavigateMsg
//
// Demo fixture: subnet-0a1b2c3d4e5f60001
// Demo results: ec2→3, eni→1, nat→1, elb→2, rtb→0, cfn→0

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

// subnetSmokeDetail builds a DetailModel for "subnet" using the demo fixture.
func subnetSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "subnet-0a1b2c3d4e5f60001",
		Name: "prod-public-a",
		Fields: map[string]string{
			"subnet_id":         "subnet-0a1b2c3d4e5f60001",
			"name":              "prod-public-a",
			"vpc_id":            "vpc-0a1b2c3d4e5f60001",
			"cidr_block":        "10.0.1.0/24",
			"availability_zone": "us-east-1a",
			"state":             "available",
			"available_ips":     "251",
		},
		RawStruct: ec2types.Subnet{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "subnet", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverSubnetRelatedResult delivers a RelatedCheckResultMsg for "subnet".
func deliverSubnetRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "subnet",
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
// Subnet-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestSubnet_Smoke_S01_RightColVisible(t *testing.T) {
	d := subnetSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("Subnet-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("Subnet-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// Subnet-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestSubnet_Smoke_S02_CorrectLabels(t *testing.T) {
	d := subnetSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("Subnet-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"EC2 Instances", "Network Interfaces", "NAT Gateways", "Load Balancers", "Route Tables", "CloudFormation"} {
		if !strings.Contains(plain, label) {
			t.Errorf("Subnet-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// Subnet-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestSubnet_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := subnetSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("Subnet-S03: right column not visible")
	}

	// Deliver demo-equivalent results
	d = deliverSubnetRelatedResult(d, "ec2", 3, "i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002", "i-0a1b2c3d4e5f60003")
	d = deliverSubnetRelatedResult(d, "eni", 1, "eni-0a1b2c3d4e5f60001")
	d = deliverSubnetRelatedResult(d, "nat", 1, "nat-0a1b2c3d4e5f60001")
	d = deliverSubnetRelatedResult(d, "elb", 2, "elb-0a1b2c3d4e5f60001", "elb-0a1b2c3d4e5f60002")
	d = deliverSubnetRelatedResult(d, "rtb", 0)
	d = deliverSubnetRelatedResult(d, "cfn", 0)

	plain := stripAnsi(d.View())

	// ec2 shows (3), elb shows (2), eni/nat show (1)
	if !strings.Contains(plain, "(3)") && !strings.Contains(plain, "(2)") && !strings.Contains(plain, "(1)") {
		t.Errorf("Subnet-S03: expected at least one of '(3)', '(2)', '(1)' counts in right column after delivering results; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("Subnet-S03: expected '(0)' for rtb/cfn rows; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Subnet-S04: Tab focuses right column; Enter on ec2 row (count=3) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestSubnet_Smoke_S04_EnterOnEC2RowNavigates(t *testing.T) {
	d := subnetSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("Subnet-S04: right column not visible")
	}

	d = deliverSubnetRelatedResult(d, "ec2", 3, "i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002", "i-0a1b2c3d4e5f60003")
	d = deliverSubnetRelatedResult(d, "eni", 1, "eni-0a1b2c3d4e5f60001")
	d = deliverSubnetRelatedResult(d, "nat", 1, "nat-0a1b2c3d4e5f60001")
	d = deliverSubnetRelatedResult(d, "elb", 2, "elb-0a1b2c3d4e5f60001", "elb-0a1b2c3d4e5f60002")
	d = deliverSubnetRelatedResult(d, "rtb", 0)
	d = deliverSubnetRelatedResult(d, "cfn", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "ec2"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Subnet-S04: Enter on ec2 row (count=3) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("Subnet-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "ec2" {
		t.Errorf("Subnet-S04: RelatedNavigateMsg.TargetType = %q, want \"ec2\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// Subnet-S05: Enter on all count=0 rows must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestSubnet_Smoke_S05_EnterOnZeroCountRowNoNav(t *testing.T) {
	d := subnetSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("Subnet-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any row
	d = deliverSubnetRelatedResult(d, "ec2", 0)
	d = deliverSubnetRelatedResult(d, "eni", 0)
	d = deliverSubnetRelatedResult(d, "nat", 0)
	d = deliverSubnetRelatedResult(d, "elb", 0)
	d = deliverSubnetRelatedResult(d, "rtb", 0)
	d = deliverSubnetRelatedResult(d, "cfn", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("Subnet-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Subnet-S06: rtb and cfn checkers are nil; ec2, eni, nat, elb checkers are non-nil
// Demo checker registered and returns results for all target types
// ---------------------------------------------------------------------------

func TestSubnet_Smoke_S06_CheckerNilAndDemoRegistered(t *testing.T) {
	defs := resource.GetRelated("subnet")

	nilCheckers := map[string]bool{"rtb": true, "cfn": true}
	nonNilCheckers := map[string]bool{"ec2": true, "eni": true, "nat": true, "elb": true}

	for i := range defs {
		tt := defs[i].TargetType
		if nilCheckers[tt] {
			if defs[i].Checker != nil {
				t.Errorf("Subnet-S06: %s Checker must be nil (stub); got non-nil — implementation changed?", tt)
			}
		}
		if nonNilCheckers[tt] {
			if defs[i].Checker == nil {
				t.Errorf("Subnet-S06: %s Checker must be non-nil; got nil", tt)
			}
		}
	}

	checker := resource.GetRelatedDemo("subnet")
	if checker == nil {
		t.Fatal("Subnet-S06: no demo checker registered for subnet")
	}
	results := checker(resource.Resource{ID: "subnet-demo"})

	wantTypes := []string{"ec2", "eni", "nat", "elb", "rtb", "cfn"}
	for _, wantType := range wantTypes {
		found := false
		for i := range results {
			if results[i].TargetType == wantType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Subnet-S06: demo checker did not return a result for target type %q", wantType)
		}
	}
}
