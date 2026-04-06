package unit_test

// EIP related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Demo fixture: eipalloc-001 attached to i-001
// Demo results: ec2→1, eni→1, nat→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func eipSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "eipalloc-001",
		Name: "web-server-eip",
		Fields: map[string]string{
			"allocation_id": "eipalloc-001",
			"public_ip":     "54.200.1.1",
			"instance_id":   "i-0a1b2c3d4e5f60001",
		},
		RawStruct: ec2types.Address{
			AllocationId:       aws.String("eipalloc-001"),
			InstanceId:         aws.String("i-0a1b2c3d4e5f60001"),
			NetworkInterfaceId: aws.String("eni-0a1b2c3d4e5f60001"),
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "eip", nil, k)
	d.SetSize(width, height)
	return d
}

func deliverEIPRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "eip",
		Result: resource.RelatedCheckResult{
			TargetType:  targetType,
			Count:       count,
			ResourceIDs: ids,
		},
	}
	updated, _ := d.Update(msg)
	return updated
}

func TestEIP_Smoke_S01_RightColVisible(t *testing.T) {
	d := eipSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("EIP-S01: 'RELATED' header not found at width=120")
	}
}

func TestEIP_Smoke_S02_CorrectLabels(t *testing.T) {
	d := eipSmokeDetail(t, 120, 30)
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("EIP-S02: right column not visible")
	}
	for _, label := range []string{"EC2 Instances", "Network Interfaces", "NAT Gateways"} {
		if !strings.Contains(plain, label) {
			t.Errorf("EIP-S02: expected label %q not found\nview:\n%s", label, plain)
		}
	}
}

func TestEIP_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := eipSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EIP-S03: right column not visible")
	}
	d = deliverEIPRelatedResult(d, "ec2", 1, "i-001")
	d = deliverEIPRelatedResult(d, "eni", 1, "eni-001")
	d = deliverEIPRelatedResult(d, "nat", 0)
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "(1)") {
		t.Errorf("EIP-S03: expected '(1)' not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("EIP-S03: expected '(0)' not found\nview:\n%s", plain)
	}
}

func TestEIP_Smoke_S04_EnterOnEC2Navigates(t *testing.T) {
	d := eipSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EIP-S04: right column not visible")
	}
	d = deliverEIPRelatedResult(d, "ec2", 1, "i-001")
	d = deliverEIPRelatedResult(d, "eni", 1, "eni-001")
	d = deliverEIPRelatedResult(d, "nat", 0)
	d, _ = pressDetailTab(d)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("EIP-S04: Enter must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("EIP-S04: expected RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "ec2" {
		t.Errorf("EIP-S04: TargetType = %q, want \"ec2\"", nav.TargetType)
	}
}

func TestEIP_Smoke_S05_EnterOnAllZeroNoNav(t *testing.T) {
	d := eipSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EIP-S05: right column not visible")
	}
	d = deliverEIPRelatedResult(d, "ec2", 0)
	d = deliverEIPRelatedResult(d, "eni", 0)
	d = deliverEIPRelatedResult(d, "nat", 0)
	d, _ = pressDetailTab(d)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
			t.Errorf("EIP-S05: Enter on all-count=0 must not produce RelatedNavigateMsg")
		}
	}
}

func TestEIP_Smoke_S06_DemoCheckerCoversAllTargets(t *testing.T) {
	checker := resource.GetRelatedDemo("eip")
	if checker == nil {
		t.Fatal("EIP-S06: no demo checker registered for eip")
	}
	results := checker(resource.Resource{ID: "eipalloc-demo"})
	targetTypes := make(map[string]bool)
	for _, r := range results {
		targetTypes[r.TargetType] = true
	}
	for _, expected := range []string{"ec2", "eni", "nat"} {
		if !targetTypes[expected] {
			t.Errorf("EIP-S06: demo checker did not return result for %q", expected)
		}
	}
}
