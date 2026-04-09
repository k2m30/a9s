package integration

// ec2_nav_chain_spec008_test.go — Spec-008: EC2 full navigation chain tests.
//
// These tests verify the complete navigation chain in demo mode:
//   Menu → EC2 list → EC2 detail → (related nav) → Target resource
//
// COMPILE FAILURE (until coder adds this):
//   1. d.FieldCursor()   — requires exported getter on DetailModel
//
// All tests in this file fail to compile under //go:build spec008 until
// that getter lands.

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
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
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = navApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// firstEC2Resource returns the first EC2 resource from demo fixtures.
func firstEC2Resource(t *testing.T) resource.Resource {
	t.Helper()
	// The first demo EC2 instance is i-0a1b2c3d4e5f60001 / web-prod-01
	// We construct it directly to avoid import cycle with demo package.
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

// ---------------------------------------------------------------------------
// TestEC2_008_NavChain_FieldCursorGetterExists
// ---------------------------------------------------------------------------

// TestEC2_008_NavChain_FieldCursorGetterExists verifies that DetailModel exposes
// FieldCursor() and it returns a non-negative value after SetSize.
//
// FAILS TO COMPILE until the coder adds FieldCursor() to DetailModel.
func TestEC2_008_NavChain_FieldCursorGetterExists(t *testing.T) {
	m := newNavDemoModel(t)

	ec2Res := firstEC2Resource(t)
	m, _ = navApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// Extract the active view as DetailModel — this validates the getter compiles.
	// The method FieldCursor() does not exist yet; compile error IS the failing test.
	// The test below will only run once the getter exists.
	view := navStripANSI(navViewContent(m))
	if view == "" {
		t.Error("EC2 detail view must not be empty")
	}

	// Attempt to use FieldCursor() via a detached detail model (compile gate).
	// This will fail to compile until the coder adds the getter.
	detailM := views.NewDetail(ec2Res, "ec2", nil, keys.Default())
	detailM.SetSize(120, 30)
	cursor := detailM.FieldCursor()
	if cursor < 0 {
		t.Errorf("FieldCursor() returned negative value %d after construction", cursor)
	}
}

// ---------------------------------------------------------------------------
// TestEC2_008_NavChain_JMovesFieldCursor
// ---------------------------------------------------------------------------

// TestEC2_008_NavChain_JMovesFieldCursor verifies that pressing j 3 times in an
// EC2 detail view moves the field cursor from 0 to 3.
//
// FAILS TO COMPILE until FieldCursor() is added.
// FAILS AT RUNTIME until j key handling is added to DetailModel.Update().
func TestEC2_008_NavChain_JMovesFieldCursor(t *testing.T) {
	ec2Res := firstEC2Resource(t)

	detailM := views.NewDetail(ec2Res, "ec2", nil, keys.Default())
	detailM.SetSize(120, 30)

	// Press j 3 times
	for range 3 {
		detailM, _ = detailM.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	}

	cursor := detailM.FieldCursor()
	if cursor != 3 {
		t.Errorf("after 3 j presses, FieldCursor() must be 3, got %d", cursor)
	}

	// View must not panic
	v := detailM.View()
	if v == "" {
		t.Error("View() must not return empty string after j presses")
	}
}

// ---------------------------------------------------------------------------
// TestEC2_008_NavChain_RightCol_Count1_OpensDetail
// ---------------------------------------------------------------------------

// TestEC2_008_NavChain_RightCol_Count1_OpensDetail verifies that when a
// RelatedNavigateMsg with TargetID arrives, the model pushes a detail view.
//
// FAILS AT RUNTIME until handleRelatedNavigate pushes TargetDetail for single IDs.
func TestEC2_008_NavChain_RightCol_Count1_OpensDetail(t *testing.T) {
	m := newNavDemoModel(t)

	ec2Res := firstEC2Resource(t)
	m, _ = navApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// Pre-populate TG cache
	tgRes := resource.Resource{ID: "tg-ec2chain-001", Name: "prod-api-tg", Status: "active"}
	m, _ = navApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "tg",
		Resources:    []resource.Resource{tgRes},
	})

	// Deliver RelatedNavigateMsg with TargetID (count=1 path)
	m, _ = navApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "tg",
		TargetID:   "tg-ec2chain-001",
	})

	view := navStripANSI(navViewContent(m))

	if !strings.Contains(view, "prod-api-tg") {
		t.Errorf("after RelatedNavigateMsg(TargetID), view must show TG detail with name %q; got:\n%s", "prod-api-tg", view)
	}
	if strings.Contains(view, "tg(1)") {
		t.Errorf("RelatedNavigateMsg(TargetID) must open DETAIL not list; found list indicator in view:\n%s", view)
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
	m, _ = navApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// Pre-populate alarm cache with 3 alarms
	alarmResources := []resource.Resource{
		{ID: "alarm-chain-1", Name: "high-cpu", Status: "alarm"},
		{ID: "alarm-chain-2", Name: "status-check", Status: "ok"},
		{ID: "alarm-chain-3", Name: "irrelevant-alarm", Status: "ok"},
	}
	m, _ = navApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "alarm",
		Resources:    alarmResources,
	})

	// Deliver RelatedNavigateMsg with RelatedIDs (count>1 path)
	m, _ = navApplyMsg(m, messages.RelatedNavigateMsg{
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
	m, _ = navApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	vpcRes := resource.Resource{ID: "vpc-0abc123def456789a", Name: "prod-vpc", Status: "available"}
	m, _ = navApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "vpc",
		Resources:    []resource.Resource{vpcRes},
	})

	// Navigate to VPC detail via RelatedNavigateMsg
	m, _ = navApplyMsg(m, messages.RelatedNavigateMsg{
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
