package unit_test

// EKS related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Demo fixture: acme-prod cluster
// Demo results: ng→2, alarm→0, cfn→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func eksSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "acme-prod",
		Name: "acme-prod",
		Fields: map[string]string{
			"cluster_name": "acme-prod",
			"version":      "1.29",
			"status":       "ACTIVE",
		},
		RawStruct: &ekstypes.Cluster{
			Name:    aws.String("acme-prod"),
			Version: aws.String("1.29"),
			Status:  ekstypes.ClusterStatusActive,
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:                  aws.String("vpc-0abc123def456789a"),
				ClusterSecurityGroupId: aws.String("sg-0ccc333333333333c"),
			},
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "eks", nil, k)
	d.SetSize(width, height)
	return d
}

func deliverEKSRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "eks",
		Result: resource.RelatedCheckResult{
			TargetType:  targetType,
			Count:       count,
			ResourceIDs: ids,
		},
	}
	updated, _ := d.Update(msg)
	return updated
}

func TestEKS_Smoke_S01_RightColVisible(t *testing.T) {
	d := eksSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("EKS-S01: 'RELATED' header not found at width=120")
	}
}

func TestEKS_Smoke_S02_CorrectLabels(t *testing.T) {
	d := eksSmokeDetail(t, 120, 30)
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("EKS-S02: right column not visible")
	}
	for _, label := range []string{"Node Groups", "CloudWatch Alarms", "CloudFormation Stacks"} {
		if !strings.Contains(plain, label) {
			t.Errorf("EKS-S02: expected label %q not found\nview:\n%s", label, plain)
		}
	}
}

func TestEKS_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := eksSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EKS-S03: right column not visible")
	}
	d = deliverEKSRelatedResult(d, "ng", 2, "general-pool", "gpu-pool")
	d = deliverEKSRelatedResult(d, "alarm", 0)
	d = deliverEKSRelatedResult(d, "cfn", 0)
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "(2)") {
		t.Errorf("EKS-S03: expected '(2)' for ng count\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("EKS-S03: expected '(0)' for alarm/cfn\nview:\n%s", plain)
	}
}

func TestEKS_Smoke_S04_EnterOnNGNavigates(t *testing.T) {
	d := eksSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EKS-S04: right column not visible")
	}
	d = deliverEKSRelatedResult(d, "ng", 2, "general-pool", "gpu-pool")
	d = deliverEKSRelatedResult(d, "alarm", 0)
	d = deliverEKSRelatedResult(d, "cfn", 0)
	d, _ = pressDetailTab(d)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("EKS-S04: Enter must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("EKS-S04: expected RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "ng" {
		t.Errorf("EKS-S04: TargetType = %q, want \"ng\"", nav.TargetType)
	}
}

func TestEKS_Smoke_S05_EnterOnAllZeroNoNav(t *testing.T) {
	d := eksSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EKS-S05: right column not visible")
	}
	d = deliverEKSRelatedResult(d, "ng", 0)
	d = deliverEKSRelatedResult(d, "alarm", 0)
	d = deliverEKSRelatedResult(d, "cfn", 0)
	d, _ = pressDetailTab(d)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
			t.Errorf("EKS-S05: Enter on all-count=0 must not produce RelatedNavigateMsg")
		}
	}
}

func TestEKS_Smoke_S06_DemoCheckerCoversAllTargets(t *testing.T) {
	checker := resource.GetRelatedDemo("eks")
	if checker == nil {
		t.Fatal("EKS-S06: no demo checker registered for eks")
	}
	results := checker(resource.Resource{ID: "demo-cluster"})
	targetTypes := make(map[string]bool)
	for _, r := range results {
		targetTypes[r.TargetType] = true
	}
	for _, expected := range []string{"ng", "alarm", "cfn"} {
		if !targetTypes[expected] {
			t.Errorf("EKS-S06: demo checker did not return result for %q", expected)
		}
	}
}
