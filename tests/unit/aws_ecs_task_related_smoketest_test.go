package unit_test

// ECS Task related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Demo fixture: task abc123 in acme-services cluster, service:api-gateway group
// Demo results: ecs-svc→1, ecs→1

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func ecsTaskSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "abc123def456",
		Name: "abc123def456",
		Fields: map[string]string{
			"task_id":         "abc123def456",
			"cluster":         "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services",
			"status":          "RUNNING",
			"task_definition": "arn:aws:ecs:us-east-1:123456789012:task-definition/api:5",
			"launch_type":     "FARGATE",
			"cpu":             "256",
			"memory":          "512",
		},
		RawStruct: ecstypes.Task{
			Group:      aws.String("service:api-gateway"),
			ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"),
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ecs-task", nil, k)
	d.SetSize(width, height)
	return d
}

func deliverECSTaskRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "ecs-task",
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
// ECSTask-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestECSTask_Smoke_S01_RightColVisible(t *testing.T) {
	d := ecsTaskSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("ECSTask-S01: right column must auto-show at width=120; 'RELATED' header not found")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("ECSTask-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// ECSTask-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestECSTask_Smoke_S02_CorrectLabels(t *testing.T) {
	d := ecsTaskSmokeDetail(t, 120, 30)
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("ECSTask-S02: right column not visible")
	}
	for _, label := range []string{"ECS Services", "ECS Clusters"} {
		if !strings.Contains(plain, label) {
			t.Errorf("ECSTask-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// ECSTask-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestECSTask_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := ecsTaskSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("ECSTask-S03: right column not visible")
	}
	d = deliverECSTaskRelatedResult(d, "ecs-svc", 1, "api-gateway")
	d = deliverECSTaskRelatedResult(d, "ecs", 1, "acme-services")
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "(1)") {
		t.Errorf("ECSTask-S03: expected '(1)' count; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// ECSTask-S04: Tab focuses right column; Enter on first row emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestECSTask_Smoke_S04_EnterNavigates(t *testing.T) {
	d := ecsTaskSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("ECSTask-S04: right column not visible")
	}
	d = deliverECSTaskRelatedResult(d, "ecs-svc", 1, "api-gateway")
	d = deliverECSTaskRelatedResult(d, "ecs", 1, "acme-services")
	d, _ = pressDetailTab(d)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("ECSTask-S04: Enter on count=1 row must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("ECSTask-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "ecs-svc" {
		t.Errorf("ECSTask-S04: RelatedNavigateMsg.TargetType = %q, want \"ecs-svc\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// ECSTask-S05: Enter on all-count=0 must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestECSTask_Smoke_S05_EnterOnStubRowNoNav(t *testing.T) {
	d := ecsTaskSmokeDetail(t, 120, 30)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("ECSTask-S05: right column not visible")
	}
	d = deliverECSTaskRelatedResult(d, "ecs-svc", 0)
	d = deliverECSTaskRelatedResult(d, "ecs", 0)
	d, _ = pressDetailTab(d)
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
			t.Errorf("ECSTask-S05: Enter on all-count=0 must not produce RelatedNavigateMsg")
		}
	}
}

// ---------------------------------------------------------------------------
// ECSTask-S06: Demo checker returns results for all targets
// ---------------------------------------------------------------------------

func TestECSTask_Smoke_S06_DemoCheckerReturnsAllTargets(t *testing.T) {
	checker := resource.GetRelatedDemo("ecs-task")
	if checker == nil {
		t.Fatal("ECSTask-S06: no demo checker registered for ecs-task")
	}
	results := checker(resource.Resource{ID: "demo-task", Fields: map[string]string{"cluster": "demo-cluster"}})
	targetTypes := make(map[string]bool)
	for _, r := range results {
		targetTypes[r.TargetType] = true
	}
	for _, expected := range []string{"ecs-svc", "ecs"} {
		if !targetTypes[expected] {
			t.Errorf("ECSTask-S06: demo checker did not return a result for %q target type", expected)
		}
	}
}
