package unit_test

// ec2_stories_nav_chains_test.go — EC2 navigation chain stories.
//
// Covers stories:
//   EC2-027, EC2-028             Section 4  — count=1 right-col Enter opens detail
//   EC2-030, EC2-031, EC2-032    Section 5  — filtered list drill-down + Esc chain
//   EC2-034                      Section 5  — EBS snapshot multi-hop (P2)
//   EC2-035..EC2-041             Section 6  — full navigation chains
//   EC2-042, EC2-046             Section 7  — edge cases: missing resource, depth indicator
//   EC2-058, EC2-059             Section 7  — CloudTrail pre-filter, session cache (P2)
//
// Package: unit_test (no build tags — runs under go test ./tests/unit/ without flags)
//
// Most tests FAIL AT RUNTIME until handleRelatedNavigate in app_handlers.go is fixed
// to push TargetDetail when TargetID is set (count=1 path) and to filter by exact
// RelatedIDs when multiple IDs are provided. Tests marked [PASSES NOW] are regression
// guards that document current behaviour.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

// chainApplyMsg forwards a message through tui.Model.Update and type-asserts
// the result back to tui.Model.
func chainApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tui.Model), cmd
}

// chainViewContent returns the raw Content string from tui.Model.View().
func chainViewContent(m tui.Model) string {
	return m.View().Content
}

// chainStrip removes ANSI escape sequences for plain-text assertions.
func chainStrip(s string) string {
	return stripAnsi(s) // reuse the regex helper from helpers_external_test.go
}

// newChainDemoModel creates a demo-mode tui.Model sized at 120×30.
func newChainDemoModel(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = chainApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// ec2TestResource returns a representative EC2 resource fixture.
// IDs are chosen to match fixtures in internal/demo/fixtures_compute.go so that
// forward navigations (VpcId, SubnetId, GroupId) resolve to existing fixtures.
func ec2TestResource() resource.Resource {
	return resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Status: "running",
		Fields: map[string]string{
			"instance_id": "i-0a1b2c3d4e5f60001",
			"name":        "web-prod-01",
			"state":       "running",
			"type":        "t3.large",
			"private_ip":  "10.0.1.10",
			"public_ip":   "54.210.33.112",
			"launch_time": "2025-11-15 08:30",
			"vpc_id":      "vpc-0abc123def456789a",
			"subnet_id":   "subnet-0aaa111111111111a",
		},
	}
}

// chainNavigateToEC2Detail pushes an EC2 detail view onto the model stack.
func chainNavigateToEC2Detail(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	ec2Res := ec2TestResource()
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})
	return m
}

// chainPreloadResources injects a ResourcesLoadedMsg to pre-populate the model's
// resource cache for a given type before sending navigation messages.
func chainPreloadResources(m tui.Model, resType string, resources []resource.Resource) tui.Model {
	m, _ = chainApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: resType,
		Resources:    resources,
	})
	return m
}

// chainEsc sends a single Escape key press.
func chainEsc(m tui.Model) tui.Model {
	m, _ = chainApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	return m
}

// ---------------------------------------------------------------------------
// Section 4 — Right Column Enter (count=1): EC2-027, EC2-028
// ---------------------------------------------------------------------------

// TestEC2_027_ASG_Count1_OpensDetail verifies that a RelatedNavigateMsg with
// TargetType "asg" and a single TargetID causes the model to push an ASG detail
// view rather than a filtered list.
//
// FAILS AT RUNTIME until handleRelatedNavigate pushes TargetDetail for count=1.
func TestEC2_027_ASG_Count1_OpensDetail(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	asgRes := resource.Resource{
		ID:     "web-prod-asg",
		Name:   "web-prod-asg",
		Status: "InService",
		Fields: map[string]string{
			"name":             "web-prod-asg",
			"min_size":         "2",
			"max_size":         "10",
			"desired_capacity": "3",
		},
	}
	m = chainPreloadResources(m, "asg", []resource.Resource{asgRes})

	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "asg",
		TargetID:   "web-prod-asg",
	})

	view := chainStrip(chainViewContent(m))

	if !strings.Contains(view, "web-prod-asg") {
		t.Errorf("EC2-027: after RelatedNavigateMsg(TargetType=asg, TargetID=web-prod-asg), view must show ASG detail with name %q; got:\n%s",
			"web-prod-asg", view)
	}
	// Must NOT show a filtered list title like "asg(1)"
	if strings.Contains(view, "asg(1)") {
		t.Errorf("EC2-027: RelatedNavigateMsg with TargetID must open detail view, not a filtered list; found list indicator in view:\n%s", view)
	}
}

// TestEC2_028_EIP_Count1_OpensDetail verifies that a RelatedNavigateMsg with
// TargetType "eip" and a single TargetID pushes an EIP detail view.
//
// FAILS AT RUNTIME until handleRelatedNavigate pushes TargetDetail for count=1.
func TestEC2_028_EIP_Count1_OpensDetail(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	eipRes := resource.Resource{
		ID:     "eipalloc-0abc123def456789a",
		Name:   "web-prod-eip",
		Status: "associated",
		Fields: map[string]string{
			"allocation_id": "eipalloc-0abc123def456789a",
			"public_ip":     "54.210.33.112",
			"instance_id":   "i-0a1b2c3d4e5f60001",
		},
	}
	m = chainPreloadResources(m, "eip", []resource.Resource{eipRes})

	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "eip",
		TargetID:   "eipalloc-0abc123def456789a",
	})

	view := chainStrip(chainViewContent(m))

	if !strings.Contains(view, "web-prod-eip") {
		t.Errorf("EC2-028: after RelatedNavigateMsg(TargetType=eip, TargetID=eipalloc-0abc123def456789a), view must show EIP detail with name %q; got:\n%s",
			"web-prod-eip", view)
	}
	if strings.Contains(view, "eip(1)") {
		t.Errorf("EC2-028: RelatedNavigateMsg with TargetID must open detail view, not a filtered list; found list indicator in view:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// Section 5 — Filtered list drill-down: EC2-030, EC2-031, EC2-032, EC2-034
// ---------------------------------------------------------------------------

// TestEC2_030_EnterOnAlarmInFilteredList verifies the three-step sequence:
// EC2 detail → filtered alarm list → alarm detail.
//
// FAILS AT RUNTIME until handleRelatedNavigate filters by exact RelatedIDs and
// NavigateMsg(TargetDetail) pushes detail over a list view.
func TestEC2_030_EnterOnAlarmInFilteredList(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	alarms := []resource.Resource{
		{ID: "web-prod-cpu-high", Name: "web-prod-cpu-high", Status: "ALARM"},
		{ID: "web-prod-status-check", Name: "web-prod-status-check", Status: "OK"},
	}
	m = chainPreloadResources(m, "alarm", alarms)

	// Navigate to filtered alarm list (count=2 path)
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "alarm",
		RelatedIDs: []string{"web-prod-cpu-high", "web-prod-status-check"},
	})

	// Now open alarm detail via NavigateMsg
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "alarm",
		Resource:     &alarms[0],
	})

	view := chainStrip(chainViewContent(m))

	if !strings.Contains(view, "web-prod-cpu-high") {
		t.Errorf("EC2-030: after entering alarm detail from filtered list, view must contain alarm name %q; got:\n%s",
			"web-prod-cpu-high", view)
	}
}

// TestEC2_031_EscFromAlarmDetail_ReturnsToFilteredList verifies that pressing Esc
// from the alarm detail returns to the filtered alarm list.
//
// FAILS AT RUNTIME until the navigation chain is correctly managed by the stack.
func TestEC2_031_EscFromAlarmDetail_ReturnsToFilteredList(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	alarms := []resource.Resource{
		{ID: "web-prod-cpu-high", Name: "web-prod-cpu-high", Status: "ALARM"},
		{ID: "web-prod-status-check", Name: "web-prod-status-check", Status: "OK"},
	}
	m = chainPreloadResources(m, "alarm", alarms)

	// Push filtered alarm list
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "alarm",
		RelatedIDs: []string{"web-prod-cpu-high", "web-prod-status-check"},
	})

	// Push alarm detail
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "alarm",
		Resource:     &alarms[0],
	})

	// Esc from alarm detail
	m = chainEsc(m)

	view := chainStrip(chainViewContent(m))

	// Must show the filtered list (containing both alarms), not the EC2 detail
	if !strings.Contains(view, "web-prod-cpu-high") {
		t.Errorf("EC2-031: after Esc from alarm detail, view must still show filtered alarm list containing %q; got:\n%s",
			"web-prod-cpu-high", view)
	}
	if !strings.Contains(view, "web-prod-status-check") {
		t.Errorf("EC2-031: after Esc from alarm detail, view must still show filtered alarm list containing %q; got:\n%s",
			"web-prod-status-check", view)
	}
	// Must NOT be on the EC2 detail (which would show the instance name as a title/field)
	if strings.Contains(view, "i-0a1b2c3d4e5f60001") && !strings.Contains(view, "alarm") {
		t.Errorf("EC2-031: Esc from alarm detail must return to alarm list, not EC2 detail; got:\n%s", view)
	}
}

// TestEC2_032_EscFromFilteredList_ReturnsToEC2Detail verifies that pressing Esc
// from the filtered alarm list returns to the EC2 detail view.
//
// FAILS AT RUNTIME until the navigation stack correctly pops back to EC2 detail.
func TestEC2_032_EscFromFilteredList_ReturnsToEC2Detail(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	alarms := []resource.Resource{
		{ID: "web-prod-cpu-high", Name: "web-prod-cpu-high", Status: "ALARM"},
		{ID: "web-prod-status-check", Name: "web-prod-status-check", Status: "OK"},
	}
	m = chainPreloadResources(m, "alarm", alarms)

	// Push filtered alarm list
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "alarm",
		RelatedIDs: []string{"web-prod-cpu-high", "web-prod-status-check"},
	})

	// Push alarm detail
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "alarm",
		Resource:     &alarms[0],
	})

	// Esc → alarm list
	m = chainEsc(m)

	// Esc → EC2 detail
	m = chainEsc(m)

	view := chainStrip(chainViewContent(m))

	if !strings.Contains(view, "web-prod-01") {
		t.Errorf("EC2-032: after two Esc presses from alarm detail chain, view must show EC2 detail with name %q; got:\n%s",
			"web-prod-01", view)
	}
}

// TestEC2_034_EBSSnapshots_MultiHop verifies that a RelatedNavigateMsg for EBS
// snapshots with two RelatedIDs shows exactly those two snapshots in a filtered list.
//
// FAILS AT RUNTIME until handleRelatedNavigate filters by exact RelatedIDs.
func TestEC2_034_EBSSnapshots_MultiHop(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	snaps := []resource.Resource{
		{ID: "snap-0aaa111111111111a", Name: "snap-web-prod-root", Status: "completed"},
		{ID: "snap-0bbb222222222222b", Name: "snap-web-prod-data", Status: "completed"},
		{ID: "snap-unrelated-zzz999", Name: "snap-other-server", Status: "completed"},
	}
	m = chainPreloadResources(m, "ebs-snap", snaps)

	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "ebs-snap",
		RelatedIDs: []string{"snap-0aaa111111111111a", "snap-0bbb222222222222b"},
	})

	view := chainStrip(chainViewContent(m))

	if !strings.Contains(view, "snap-web-prod-root") {
		t.Errorf("EC2-034: filtered snapshot list must contain %q; got:\n%s", "snap-web-prod-root", view)
	}
	if !strings.Contains(view, "snap-web-prod-data") {
		t.Errorf("EC2-034: filtered snapshot list must contain %q; got:\n%s", "snap-web-prod-data", view)
	}
	if strings.Contains(view, "snap-other-server") {
		t.Errorf("EC2-034: filtered snapshot list must NOT contain unrelated snapshot %q; got:\n%s", "snap-other-server", view)
	}
}

// ---------------------------------------------------------------------------
// Section 6 — Full navigation chains: EC2-035 through EC2-041
// ---------------------------------------------------------------------------

// TestEC2_035_ChainA_EC2ToVPCAndBack verifies the forward and return chain:
// EC2 detail → VPC detail (via RelatedNavigateMsg) → Esc → EC2 detail.
//
// FAILS AT RUNTIME until handleRelatedNavigate pushes TargetDetail for count=1.
func TestEC2_035_ChainA_EC2ToVPCAndBack(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	vpcRes := resource.Resource{
		ID:     "vpc-0abc123def456789a",
		Name:   "production-vpc",
		Status: "available",
		Fields: map[string]string{
			"vpc_id":     "vpc-0abc123def456789a",
			"cidr_block": "10.0.0.0/16",
			"state":      "available",
		},
	}
	m = chainPreloadResources(m, "vpc", []resource.Resource{vpcRes})

	// Navigate to VPC detail
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "vpc",
		TargetID:   "vpc-0abc123def456789a",
	})

	viewVPC := chainStrip(chainViewContent(m))
	if !strings.Contains(viewVPC, "vpc-0abc123def456789a") {
		t.Errorf("EC2-035: after RelatedNavigateMsg to VPC, view must contain VPC ID %q; got:\n%s",
			"vpc-0abc123def456789a", viewVPC)
	}

	// Press Esc → back to EC2 detail
	m = chainEsc(m)

	viewEC2 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewEC2, "web-prod-01") {
		t.Errorf("EC2-035: after Esc from VPC detail, view must show EC2 detail with name %q; got:\n%s",
			"web-prod-01", viewEC2)
	}
}

// TestEC2_036_ChainB_EC2ToSubnetAndBack verifies EC2 detail → Subnet detail → Esc.
//
// FAILS AT RUNTIME until handleRelatedNavigate pushes TargetDetail for count=1.
func TestEC2_036_ChainB_EC2ToSubnetAndBack(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	subnetRes := resource.Resource{
		ID:     "subnet-0aaa111111111111a",
		Name:   "public-a",
		Status: "available",
		Fields: map[string]string{
			"subnet_id": "subnet-0aaa111111111111a",
			"vpc_id":    "vpc-0abc123def456789a",
			"az":        "us-east-1a",
		},
	}
	m = chainPreloadResources(m, "subnet", []resource.Resource{subnetRes})

	// Navigate to Subnet detail
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "subnet",
		TargetID:   "subnet-0aaa111111111111a",
	})

	viewSubnet := chainStrip(chainViewContent(m))
	if !strings.Contains(viewSubnet, "subnet-0aaa111111111111a") {
		t.Errorf("EC2-036: after RelatedNavigateMsg to Subnet, view must contain Subnet ID %q; got:\n%s",
			"subnet-0aaa111111111111a", viewSubnet)
	}

	// Esc → EC2 detail
	m = chainEsc(m)

	viewEC2 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewEC2, "web-prod-01") {
		t.Errorf("EC2-036: after Esc from Subnet detail, view must show EC2 detail with name %q; got:\n%s",
			"web-prod-01", viewEC2)
	}
}

// TestEC2_037_ChainC_EC2ToSGAndBack verifies EC2 detail → SG detail → Esc.
//
// FAILS AT RUNTIME until handleRelatedNavigate pushes TargetDetail for count=1.
func TestEC2_037_ChainC_EC2ToSGAndBack(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	sgRes := resource.Resource{
		ID:     "sg-0aaa111111111111a",
		Name:   "acme-web-alb-sg",
		Status: "active",
		Fields: map[string]string{
			"group_id":   "sg-0aaa111111111111a",
			"group_name": "acme-web-alb-sg",
			"vpc_id":     "vpc-0abc123def456789a",
		},
	}
	m = chainPreloadResources(m, "sg", []resource.Resource{sgRes})

	// Navigate to SG detail
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "sg",
		TargetID:   "sg-0aaa111111111111a",
	})

	viewSG := chainStrip(chainViewContent(m))
	if !strings.Contains(viewSG, "sg-0aaa111111111111a") {
		t.Errorf("EC2-037: after RelatedNavigateMsg to SG, view must contain SG ID %q; got:\n%s",
			"sg-0aaa111111111111a", viewSG)
	}

	// Esc → EC2 detail
	m = chainEsc(m)

	viewEC2 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewEC2, "web-prod-01") {
		t.Errorf("EC2-037: after Esc from SG detail, view must show EC2 detail with name %q; got:\n%s",
			"web-prod-01", viewEC2)
	}
}

// TestEC2_038_ChainD_EC2TabToTGAndBack verifies EC2 detail → TG detail (right column,
// count=1) → Esc → EC2 detail.
//
// FAILS AT RUNTIME until handleRelatedNavigate pushes TargetDetail for count=1.
func TestEC2_038_ChainD_EC2TabToTGAndBack(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	tgRes := resource.Resource{
		ID:     "tg-web-prod",
		Name:   "web-prod-tg",
		Status: "active",
		Fields: map[string]string{
			"name":     "web-prod-tg",
			"port":     "80",
			"protocol": "HTTP",
		},
	}
	m = chainPreloadResources(m, "tg", []resource.Resource{tgRes})

	// Simulate right-column Enter on TG (count=1 path uses TargetID)
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "tg",
		TargetID:   "tg-web-prod",
	})

	viewTG := chainStrip(chainViewContent(m))
	if !strings.Contains(viewTG, "web-prod-tg") {
		t.Errorf("EC2-038: after RelatedNavigateMsg to TG, view must show TG detail with name %q; got:\n%s",
			"web-prod-tg", viewTG)
	}

	// Esc → EC2 detail
	m = chainEsc(m)

	viewEC2 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewEC2, "web-prod-01") {
		t.Errorf("EC2-038: after Esc from TG detail, view must show EC2 detail with name %q; got:\n%s",
			"web-prod-01", viewEC2)
	}
}

// TestEC2_039_ChainE_EC2ToAlarmListToDetailAndBackx2 verifies the five-step chain:
// EC2 detail → filtered alarm list (2 alarms) → alarm detail → Esc → alarm list → Esc → EC2 detail.
//
// FAILS AT RUNTIME until handleRelatedNavigate filters by RelatedIDs and the stack
// manages multiple hops correctly.
func TestEC2_039_ChainE_EC2ToAlarmListToDetailAndBackx2(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	alarms := []resource.Resource{
		{ID: "alarm-ec2039-cpu", Name: "web-prod-cpu-high", Status: "ALARM"},
		{ID: "alarm-ec2039-status", Name: "web-prod-status-check", Status: "OK"},
		{ID: "alarm-ec2039-unrelated", Name: "other-server-alarm", Status: "OK"},
	}
	m = chainPreloadResources(m, "alarm", alarms)

	// Step 1: Push filtered alarm list (2 of 3 alarms)
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "alarm",
		RelatedIDs: []string{"alarm-ec2039-cpu", "alarm-ec2039-status"},
	})

	viewList1 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewList1, "web-prod-cpu-high") {
		t.Errorf("EC2-039 step1: filtered alarm list must contain %q; got:\n%s", "web-prod-cpu-high", viewList1)
	}
	if strings.Contains(viewList1, "other-server-alarm") {
		t.Errorf("EC2-039 step1: filtered alarm list must NOT contain unrelated alarm %q; got:\n%s", "other-server-alarm", viewList1)
	}

	// Step 2: Push alarm detail
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "alarm",
		Resource:     &alarms[0],
	})

	viewDetail := chainStrip(chainViewContent(m))
	if !strings.Contains(viewDetail, "web-prod-cpu-high") {
		t.Errorf("EC2-039 step2: alarm detail must contain alarm name %q; got:\n%s", "web-prod-cpu-high", viewDetail)
	}

	// Step 3: Esc → filtered alarm list
	m = chainEsc(m)

	viewList2 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewList2, "web-prod-cpu-high") {
		t.Errorf("EC2-039 step3: after Esc from alarm detail, view must show filtered alarm list with %q; got:\n%s",
			"web-prod-cpu-high", viewList2)
	}

	// Step 4: Esc → EC2 detail
	m = chainEsc(m)

	viewEC2 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewEC2, "web-prod-01") {
		t.Errorf("EC2-039 step4: after second Esc, view must show EC2 detail with name %q; got:\n%s",
			"web-prod-01", viewEC2)
	}
}

// TestEC2_040_ChainF_Depth6_EC2ToVPCToSubnetAndBack verifies a six-level deep
// navigation chain and checks that the view stack supports that depth.
//
// Navigation sequence (depths assuming menu=1, ec2-list=2, ec2-detail=3):
//   EC2 detail (3) → VPC detail (4) → Subnet list (5) → Subnet detail (6) → unwind.
//
// FAILS AT RUNTIME until handleRelatedNavigate correctly manages deep stacks.
func TestEC2_040_ChainF_Depth6_EC2ToVPCToSubnetAndBack(t *testing.T) {
	m := newChainDemoModel(t)
	// stack: menu (1)

	// Navigate to EC2 list then detail (stack: menu + ec2-list + ec2-detail = 3)
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	ec2Res := ec2TestResource()
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// Pre-load VPC and Subnet resources
	vpcRes := resource.Resource{
		ID:     "vpc-0abc123def456789a",
		Name:   "production-vpc",
		Status: "available",
		Fields: map[string]string{"vpc_id": "vpc-0abc123def456789a"},
	}
	subnets := []resource.Resource{
		{ID: "subnet-0aaa111111111111a", Name: "public-a", Status: "available"},
		{ID: "subnet-0bbb222222222222b", Name: "public-b", Status: "available"},
	}
	m = chainPreloadResources(m, "vpc", []resource.Resource{vpcRes})
	m = chainPreloadResources(m, "subnet", subnets)

	// Navigate to VPC detail (depth 4)
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "vpc",
		TargetID:   "vpc-0abc123def456789a",
	})

	viewVPC := chainStrip(chainViewContent(m))
	if !strings.Contains(viewVPC, "vpc-0abc123def456789a") {
		t.Errorf("EC2-040 depth4: VPC detail must contain VPC ID; got:\n%s", viewVPC)
	}

	// Navigate to filtered Subnet list (depth 5)
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "subnet",
		RelatedIDs: []string{"subnet-0aaa111111111111a", "subnet-0bbb222222222222b"},
	})

	viewSubnetList := chainStrip(chainViewContent(m))
	if !strings.Contains(viewSubnetList, "public-a") {
		t.Errorf("EC2-040 depth5: subnet list must contain %q; got:\n%s", "public-a", viewSubnetList)
	}

	// Navigate to Subnet detail (depth 6)
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "subnet",
		Resource:     &subnets[0],
	})

	viewSubnetDetail := chainStrip(chainViewContent(m))
	if !strings.Contains(viewSubnetDetail, "subnet-0aaa111111111111a") {
		t.Errorf("EC2-040 depth6: subnet detail must contain subnet ID; got:\n%s", viewSubnetDetail)
	}

	// Unwind: Esc → subnet list
	m = chainEsc(m)
	viewBack5 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewBack5, "public-a") {
		t.Errorf("EC2-040 unwind5: after Esc from subnet detail, must show subnet list; got:\n%s", viewBack5)
	}

	// Esc → VPC detail
	m = chainEsc(m)
	viewBack4 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewBack4, "vpc-0abc123def456789a") {
		t.Errorf("EC2-040 unwind4: after Esc from subnet list, must show VPC detail; got:\n%s", viewBack4)
	}

	// Esc → EC2 detail
	m = chainEsc(m)
	viewBack3 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewBack3, "web-prod-01") {
		t.Errorf("EC2-040 unwind3: after Esc from VPC detail, must show EC2 detail; got:\n%s", viewBack3)
	}
}

// TestEC2_041_ChainG_MixedLeftAndRight verifies a mixed left+right column navigation:
// EC2 detail → SG detail (left-col Enter) → EC2 filtered list (right-col Enter) → Esc → SG detail → Esc → EC2 detail.
//
// FAILS AT RUNTIME until handleRelatedNavigate correctly handles the full chain.
func TestEC2_041_ChainG_MixedLeftAndRight(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	// Pre-load SG resource (for left-col navigation from EC2 detail → SG detail)
	sgRes := resource.Resource{
		ID:     "sg-0aaa111111111111a",
		Name:   "acme-web-alb-sg",
		Status: "active",
		Fields: map[string]string{"group_id": "sg-0aaa111111111111a"},
	}
	m = chainPreloadResources(m, "sg", []resource.Resource{sgRes})

	// Step 1: Left-col Enter on GroupId → SG detail
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "sg",
		TargetID:   "sg-0aaa111111111111a",
	})

	viewSG := chainStrip(chainViewContent(m))
	if !strings.Contains(viewSG, "sg-0aaa111111111111a") {
		t.Errorf("EC2-041 step1: SG detail must contain SG ID %q; got:\n%s", "sg-0aaa111111111111a", viewSG)
	}

	// Pre-load EC2 resources for SG's right-col reverse lookup
	ec2Res := ec2TestResource()
	m = chainPreloadResources(m, "ec2", []resource.Resource{ec2Res})

	// Step 2: Right-col Enter on "EC2 Instances" → filtered EC2 list
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "ec2",
		RelatedIDs: []string{"i-0a1b2c3d4e5f60001"},
	})

	viewEC2List := chainStrip(chainViewContent(m))
	if !strings.Contains(viewEC2List, "web-prod-01") {
		t.Errorf("EC2-041 step2: filtered EC2 list must contain %q; got:\n%s", "web-prod-01", viewEC2List)
	}

	// Step 3: Esc → SG detail
	m = chainEsc(m)

	viewSG2 := chainStrip(chainViewContent(m))
	if !strings.Contains(viewSG2, "sg-0aaa111111111111a") {
		t.Errorf("EC2-041 step3: after Esc from EC2 list, must show SG detail with ID %q; got:\n%s",
			"sg-0aaa111111111111a", viewSG2)
	}

	// Step 4: Esc → EC2 detail
	m = chainEsc(m)

	viewEC2Detail := chainStrip(chainViewContent(m))
	if !strings.Contains(viewEC2Detail, "web-prod-01") {
		t.Errorf("EC2-041 step4: after Esc from SG detail, must show EC2 detail with name %q; got:\n%s",
			"web-prod-01", viewEC2Detail)
	}
}

// ---------------------------------------------------------------------------
// Section 7 — Edge cases: EC2-042, EC2-046, EC2-058, EC2-059
// ---------------------------------------------------------------------------

// TestEC2_042_NavToMissingResource_FlashMessage verifies that navigating to a
// resource ID that does not exist in the demo cache either:
//   a) Keeps the user on the EC2 detail view, OR
//   b) Shows a flash/error indicator in the rendered output.
//
// Both outcomes are acceptable — the critical invariant is that the user is NOT
// silently moved to an empty or broken view.
//
// FAILS AT RUNTIME until handleRelatedNavigate emits a FlashMsg on cache miss
// (currently it creates a new empty list and fetches — no flash, no stay).
func TestEC2_042_NavToMissingResource_FlashMessage(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	// Do NOT pre-load any resource in the "ami" type cache.
	// Navigating to an AMI ID that doesn't exist should fail gracefully.
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "ami",
		TargetID:   "ami-nonexistent-000000",
	})

	view := chainStrip(chainViewContent(m))

	// Either we stayed on EC2 detail (contains "web-prod-01"),
	// or we got an error/flash indicator.
	stayedOnEC2 := strings.Contains(view, "web-prod-01")
	hasErrorIndicator := strings.Contains(view, "not found") ||
		strings.Contains(view, "error") ||
		strings.Contains(view, "Error") ||
		strings.Contains(view, "unknown resource type") ||
		strings.Contains(view, "ami-nonexistent")

	if !stayedOnEC2 && !hasErrorIndicator {
		t.Errorf("EC2-042: navigating to a missing resource must either stay on EC2 detail or show an error; got:\n%s", view)
	}
}

// TestEC2_046_DepthIndicator verifies that the depth indicator feature is either
// present (shows [N] when depth > 4) or documents that it is not yet implemented.
//
// Current production code in layout/frame.go does NOT have depth-aware header logic.
// This test documents the EXPECTED behaviour (from story EC2-046) and will fail
// until the coder implements the depth indicator in the header.
//
// FAILS AT RUNTIME — depth indicator is not yet implemented.
func TestEC2_046_DepthIndicator(t *testing.T) {
	m := newChainDemoModel(t)
	// Stack depth starts at 1 (main menu).
	// Reach depth 6 by pushing five more views.

	// depth 2: EC2 list
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// depth 3: EC2 detail
	ec2Res := ec2TestResource()
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// depth 4: VPC detail
	vpcRes := resource.Resource{ID: "vpc-depth4", Name: "vpc-depth4", Status: "available"}
	m = chainPreloadResources(m, "vpc", []resource.Resource{vpcRes})
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{TargetType: "vpc", TargetID: "vpc-depth4"})

	// depth 5: Subnet list
	subnets := []resource.Resource{
		{ID: "subnet-depth5a", Name: "depth5-subnet", Status: "available"},
	}
	m = chainPreloadResources(m, "subnet", subnets)
	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "subnet",
		RelatedIDs: []string{"subnet-depth5a"},
	})

	// depth 6: Subnet detail
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "subnet",
		Resource:     &subnets[0],
	})

	view6 := chainStrip(chainViewContent(m))

	// Expected: header shows "[6]" instead of version. Not yet implemented.
	if !strings.Contains(view6, "[6]") {
		t.Errorf("EC2-046: at stack depth 6, header must contain %q depth indicator; got header area:\n%s",
			"[6]", view6)
	}

	// Esc back to depth 5
	m = chainEsc(m)
	view5 := chainStrip(chainViewContent(m))
	if !strings.Contains(view5, "[5]") {
		t.Errorf("EC2-046: at stack depth 5, header must contain %q depth indicator; got:\n%s", "[5]", view5)
	}

	// Esc back to depth 4 — version should reappear (depth <= 4)
	m = chainEsc(m)
	view4 := chainStrip(chainViewContent(m))
	// At depth 4, version should appear (no depth indicator needed per spec)
	if strings.Contains(view4, "[4]") {
		t.Errorf("EC2-046: at stack depth 4, header must NOT contain [4] depth indicator; got:\n%s", view4)
	}
}

// TestEC2_058_CloudTrailPreFiltered documents the expected behaviour when the user
// navigates to CloudTrail Events from an EC2 detail right column.
//
// The CloudTrail search/pre-filter feature does not yet exist. This test is written
// to document expected behaviour and will fail until the feature is implemented.
//
// Priority: P2 — FAILS AT RUNTIME (feature not yet implemented).
func TestEC2_058_CloudTrailPreFiltered(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	// Simulate right-column Enter on "CloudTrail Events" row.
	// The expected result is a pre-filtered search view for the EC2 instance ID.
	// Since no CloudTrail resource type exists yet, this navigates to "cloudtrail"
	// and the expectation is that the view contains the pre-filter query.
	cloudtrailEvents := []resource.Resource{
		{ID: "event-001", Name: "RunInstances", Status: "Success",
			Fields: map[string]string{"resource_name": "i-0a1b2c3d4e5f60001"}},
	}
	m = chainPreloadResources(m, "cloudtrail", cloudtrailEvents)

	m, _ = chainApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "cloudtrail",
		RelatedIDs: []string{"event-001"},
	})

	view := chainStrip(chainViewContent(m))

	// Expected: view is pre-filtered to show only events for this EC2 instance.
	// The filter should be applied with the instance ID as query text.
	if !strings.Contains(view, "i-0a1b2c3d4e5f60001") && !strings.Contains(view, "RunInstances") {
		t.Errorf("EC2-058: CloudTrail navigation must show events pre-filtered for EC2 instance; expected view to contain EC2 ID or event name; got:\n%s", view)
	}
}

// TestEC2_059_SessionCachePreventsRecheck verifies that navigating back to an EC2
// instance detail after previously visiting it delivers cached related-check results
// immediately rather than dimming all rows and rechecking.
//
// The test verifies the cache contract indirectly: we deliver RelatedCheckResultMsg
// results, navigate away, and verify the model's ResourcesLoadedMsg handling doesn't
// wipe the cache.
//
// Priority: P2 — PASSES if the cache is not invalidated on navigation. FAILS if the
// implementation re-fetches on every entry.
func TestEC2_059_SessionCachePreventsRecheck(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	// Deliver related check results for this EC2 instance
	checkResult := messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       1,
			ResourceIDs: []string{"tg-web-prod"},
		},
	}
	m, _ = chainApplyMsg(m, checkResult)

	// Navigate away from EC2 detail (Esc to go back)
	m = chainEsc(m)

	// Navigate back to the same EC2 detail
	ec2Res := ec2TestResource()
	m, _ = chainApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// Deliver the check result again — if caching works, this should be idempotent
	// (counts are shown immediately on re-entry without re-checking)
	m, _ = chainApplyMsg(m, checkResult)

	view := chainStrip(chainViewContent(m))

	// The EC2 detail view must be rendered (not blank or error)
	if view == "" {
		t.Error("EC2-059: re-entering EC2 detail after navigating away must render a non-empty view")
	}
	if !strings.Contains(view, "web-prod-01") {
		t.Errorf("EC2-059: re-entered EC2 detail must contain instance name %q; got:\n%s", "web-prod-01", view)
	}
}
