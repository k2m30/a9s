package unit_test

// EFS related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Demo fixture: fs-001 encrypted EFS
// Demo results: kms→1, cfn→0, lambda→0 (stub)

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func efsSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	encrypted := true
	res := resource.Resource{
		ID:   "fs-001",
		Name: "shared-data",
		Fields: map[string]string{
			"file_system_id":   "fs-001",
			"name":             "shared-data",
			"life_cycle_state": "available",
			"encrypted":        "true",
		},
		RawStruct: efstypes.FileSystemDescription{
			FileSystemId: aws.String("fs-001"),
			Name:         aws.String("shared-data"),
			Encrypted:    &encrypted,
			KmsKeyId:     aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "efs", nil, k)
	d.SetSize(width, height)
	return d
}

func deliverEFSRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "efs",
		Result: resource.RelatedCheckResult{
			TargetType:  targetType,
			Count:       count,
			ResourceIDs: ids,
		},
	}
	updated, _ := d.Update(msg)
	return updated
}

func TestEFS_Smoke_S01_RightColVisible(t *testing.T) {
	d := efsSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("EFS-S01: right column must auto-show at width=120; 'RELATED' not found")
	}
}

func TestEFS_Smoke_S02_CorrectLabels(t *testing.T) {
	d := efsSmokeDetail(t, 120, 30)
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("EFS-S02: right column not visible")
	}
	for _, label := range []string{"KMS Keys", "CloudFormation Stacks", "Lambda Functions"} {
		if !strings.Contains(plain, label) {
			t.Errorf("EFS-S02: expected label %q not found\nview:\n%s", label, plain)
		}
	}
}

func TestEFS_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := efsSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EFS-S03: right column not visible")
	}
	d = deliverEFSRelatedResult(d, "kms", 1, "a1b2c3d4")
	d = deliverEFSRelatedResult(d, "cfn", 0)
	d = deliverEFSRelatedResult(d, "lambda", 0)
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "(1)") {
		t.Errorf("EFS-S03: expected '(1)' count not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("EFS-S03: expected '(0)' count not found\nview:\n%s", plain)
	}
}

func TestEFS_Smoke_S04_EnterOnKMSNavigates(t *testing.T) {
	d := efsSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EFS-S04: right column not visible")
	}
	d = deliverEFSRelatedResult(d, "kms", 1, "a1b2c3d4")
	d = deliverEFSRelatedResult(d, "cfn", 0)
	d = deliverEFSRelatedResult(d, "lambda", 0)
	d, _ = pressDetailTab(d)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("EFS-S04: Enter on kms row must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("EFS-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "kms" {
		t.Errorf("EFS-S04: TargetType = %q, want \"kms\"", nav.TargetType)
	}
}

func TestEFS_Smoke_S05_EnterOnAllZeroNoNav(t *testing.T) {
	d := efsSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EFS-S05: right column not visible")
	}
	d = deliverEFSRelatedResult(d, "kms", 0)
	d = deliverEFSRelatedResult(d, "cfn", 0)
	d = deliverEFSRelatedResult(d, "lambda", 0)
	d, _ = pressDetailTab(d)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
			t.Errorf("EFS-S05: Enter on all-count=0 must not produce RelatedNavigateMsg")
		}
	}
}

func TestEFS_Smoke_S06_DemoCheckerCoversAllTargets(t *testing.T) {
	checker := resource.GetRelatedDemo("efs")
	if checker == nil {
		t.Fatal("EFS-S06: no demo checker registered for efs")
	}
	results := checker(resource.Resource{ID: "fs-demo"})
	targetTypes := make(map[string]bool)
	for _, r := range results {
		targetTypes[r.TargetType] = true
	}
	for _, expected := range []string{"kms", "cfn", "lambda"} {
		if !targetTypes[expected] {
			t.Errorf("EFS-S06: demo checker did not return result for %q", expected)
		}
	}
}
