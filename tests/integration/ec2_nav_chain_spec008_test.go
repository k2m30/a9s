package integration

// ec2_nav_chain_spec008_test.go — Spec-008: EC2 full navigation chain tests.
//
// These tests verify the complete navigation chain in demo mode:
//   Menu → EC2 list → EC2 detail → (related nav) → Target resource
//
// TestEC2_008_NavChain_FieldCursorGetterExists and
// TestEC2_008_NavChain_JMovesFieldCursor deleted (round 4, specs/022-codebase-
// cleanup): both drove DetailModel.FieldCursor()/.Update()/.View() directly
// (dead legacy API). Covered by tests/unit's live controller path:
// detail_livepath_migration_test.go's TestDetailController_MoveDown_
// SkipsSectionHeadersAndSpacers (ActionMoveDown repeatedly advances
// Body.Detail.FieldCursor, always landing on a real field row) and
// detail_ports_test.go's TestWave3_DetailController_MoveDown_
// ClampsAtLastField / MoveUp_ClampsAtFirstField (boundary clamp).

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ---------------------------------------------------------------------------
// Local helpers (cannot use unit package helpers from integration package)
// ---------------------------------------------------------------------------

var navAnsiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func navStripANSI(s string) string {
	return navAnsiRe.ReplaceAllString(s, "")
}

func navApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tui.Model), cmd
}

func navViewContent(m tui.Model) string {
	return m.View().Content
}

func newNavDemoModel(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = navApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// firstEC2Resource returns the first EC2 resource from demo fixtures.
func firstEC2Resource(t *testing.T) resource.Resource {
	t.Helper()
	// The first demo EC2 instance is i-0a1b2c3d4e5f60001 / web-prod-01
	// We construct it directly to avoid import cycle with demo package.
	return resource.Resource{
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "web-prod-01",
		Fields: map[string]string{
			"instance_id": "i-0a1b2c3d4e5f60001",
			"name":        "web-prod-01",
			"state":       "running",
			"status":      "running",
			"type":        "t3.large",
			"private_ip":  "10.0.1.10",
			"public_ip":   "54.210.33.112",
			"launch_time": "2025-11-15 08:30",
			"vpc_id":      "vpc-0abc123def456789a",
			"subnet_id":   "subnet-0aaa111111111111a",
		},
	}
}

// ---------------------------------------------------------------------------
// TestEC2_008_NavChain_RightCol_Count1_OpensDetail
// ---------------------------------------------------------------------------

// TestEC2_008_NavChain_RightCol_Count1_OpensDrillTarget verifies that when a
// RelatedNavigateMsg with TargetID arrives, the model mirrors manual Enter
// on the target row. tg has Children[Key="enter"] → tg_health so the fast
// path must enter that child view rather than push generic detail (rule
// 2026-04-24: count-1 drill does exactly what Enter would do).
func TestEC2_008_NavChain_RightCol_Count1_OpensDrillTarget(t *testing.T) {
	m := newNavDemoModel(t)

	ec2Res := firstEC2Resource(t)
	m, _ = navApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// Pre-populate TG cache
	tgRes := resource.Resource{
		ID:   "tg-ec2chain-001",
		Name: "prod-api-tg",
		Fields: map[string]string{
			"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/prod-api-tg/abc123",
			"status":           "active",
		},
	}
	m, _ = navApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "tg",
		Resources:    []resource.Resource{tgRes},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	// Deliver RelatedNavigateMsg with TargetID (count=1 path).
	// Rule (user, 2026-07-06, supersedes 2026-04-24): a related drill that
	// narrows to exactly one resource opens that resource's DETAIL view for
	// every target type; enter-keyed child views stay reachable only via
	// Enter inside the target's own list.
	m, cmd := navApplyMsg(m, messages.RelatedNavigate{
		TargetType: "tg",
		TargetID:   "tg-ec2chain-001",
	})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = navApplyMsg(m, msg)
		}
	}

	view := navStripANSI(navViewContent(m))

	if !strings.Contains(view, "detail -- tg-ec2chain-001") {
		t.Errorf("after RelatedNavigateMsg(TargetID) on tg, view must open the tg detail; got:\n%s", view)
	}
	if strings.Contains(view, "tg(1)") {
		t.Errorf("RelatedNavigateMsg(TargetID) must not open a filtered list; got:\n%s", view)
	}
	if strings.Contains(view, "tg_health") {
		t.Errorf("RelatedNavigateMsg(TargetID) must not enter the tg_health child view; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestEC2_008_NavChain_RightCol_CountN_ShowsFilteredList
// ---------------------------------------------------------------------------

// TestEC2_008_NavChain_RightCol_CountN_ShowsFilteredList verifies that when a
// RelatedNavigateMsg with RelatedIDs arrives, only the listed resources are shown.
//
// FAILS AT RUNTIME until handleRelatedNavigate filters by exact IDs.
func TestEC2_008_NavChain_RightCol_CountN_ShowsFilteredList(t *testing.T) {
	m := newNavDemoModel(t)

	ec2Res := firstEC2Resource(t)
	m, _ = navApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// Pre-populate alarm cache with 3 alarms
	alarmResources := []resource.Resource{
		{ID: "alarm-chain-1", Name: "high-cpu", Fields: map[string]string{"status": "alarm"}},
		{ID: "alarm-chain-2", Name: "status-check", Fields: map[string]string{"status": "ok"}},
		{ID: "alarm-chain-3", Name: "irrelevant-alarm", Fields: map[string]string{"status": "ok"}},
	}
	m, _ = navApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "alarm",
		Resources:    alarmResources,
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	// Deliver RelatedNavigateMsg with RelatedIDs (count>1 path)
	m, _ = navApplyMsg(m, messages.RelatedNavigate{
		TargetType: "alarm",
		RelatedIDs: []string{"alarm-chain-1", "alarm-chain-2"},
	})

	view := navStripANSI(navViewContent(m))

	if !strings.Contains(view, "high-cpu") {
		t.Errorf("view must contain filtered alarm %q; got:\n%s", "high-cpu", view)
	}
	if !strings.Contains(view, "status-check") {
		t.Errorf("view must contain filtered alarm %q; got:\n%s", "status-check", view)
	}
	if strings.Contains(view, "irrelevant-alarm") {
		t.Errorf("view must NOT contain unfiltered alarm %q when RelatedIDs excludes it; got:\n%s", "irrelevant-alarm", view)
	}
}

// ---------------------------------------------------------------------------
// TestEC2_008_NavChain_EscReturnsToEC2Detail
// ---------------------------------------------------------------------------

// TestEC2_008_NavChain_EscReturnsToEC2Detail verifies that pressing Esc after
// a RelatedNavigateMsg navigation returns to the EC2 detail view.
//
// FAILS AT RUNTIME until handleRelatedNavigate pushes TargetDetail for single IDs
// (regression guard — once the above tests pass, Esc must also work).
func TestEC2_008_NavChain_EscReturnsToEC2Detail(t *testing.T) {
	m := newNavDemoModel(t)

	ec2Res := firstEC2Resource(t)
	m, _ = navApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	vpcRes := resource.Resource{ID: "vpc-0abc123def456789a", Name: "prod-vpc", Fields: map[string]string{"status": "available"}}
	m, _ = navApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "vpc",
		Resources:    []resource.Resource{vpcRes},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	// Navigate to VPC detail via RelatedNavigateMsg
	m, _ = navApplyMsg(m, messages.RelatedNavigate{
		TargetType: "vpc",
		TargetID:   "vpc-0abc123def456789a",
	})

	// Press Esc to go back
	m, _ = navApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	view := navStripANSI(navViewContent(m))

	// After Esc, we should be back on the EC2 detail (frame title = "web-prod-01")
	if !strings.Contains(view, "web-prod-01") {
		t.Errorf("after Esc from related navigation, view must show EC2 detail with name %q; got:\n%s", "web-prod-01", view)
	}
}
