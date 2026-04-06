package unit_test

// ECS Service related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to an ECS Service, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (ECS Clusters, Target Groups, CloudWatch Alarms, CloudFormation Stacks)
//   - Counts display correctly after results delivered
//   - Tab focuses right column; Enter on count>0 row emits RelatedNavigateMsg
//   - Enter on all-count=0 right column does NOT emit RelatedNavigateMsg
//   - Stub defs have nil Checker; demo checker still returns a result for each target
//
// Demo fixture: api-gateway service in acme-services cluster
// Demo results: ecs→1, tg→1, alarm→1, cfn→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func ecsSvcSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "api-gateway",
		Name: "api-gateway",
		Fields: map[string]string{
			"service_name":  "api-gateway",
			"cluster":       "acme-services",
			"status":        "ACTIVE",
			"desired_count": "3",
			"running_count": "3",
			"launch_type":   "FARGATE",
		},
		RawStruct: ecstypes.Service{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ecs-svc", nil, k)
	d.SetSize(width, height)
	return d
}

func deliverECSSvcRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "ecs-svc",
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
// ECSSvc-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestECSSvc_Smoke_S01_RightColVisible(t *testing.T) {
	d := ecsSvcSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("ECSSvc-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("ECSSvc-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// ECSSvc-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestECSSvc_Smoke_S02_CorrectLabels(t *testing.T) {
	d := ecsSvcSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("ECSSvc-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"ECS Clusters", "Target Groups", "CloudWatch Alarms", "CloudFormation Stacks"} {
		if !strings.Contains(plain, label) {
			t.Errorf("ECSSvc-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// ECSSvc-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestECSSvc_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := ecsSvcSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("ECSSvc-S03: right column not visible")
	}

	d = deliverECSSvcRelatedResult(d, "ecs", 1, "acme-services")
	d = deliverECSSvcRelatedResult(d, "tg", 1, "api-tg")
	d = deliverECSSvcRelatedResult(d, "alarm", 1, "ecs-svc-cpu-high")
	d = deliverECSSvcRelatedResult(d, "cfn", 0)

	plain := stripAnsi(d.View())

	if !strings.Contains(plain, "(1)") {
		t.Errorf("ECSSvc-S03: expected '(1)' count in right column after delivering ecs/tg/alarm results; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("ECSSvc-S03: expected '(0)' for stub cfn row; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// ECSSvc-S04: Tab focuses right column; Enter on ecs row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestECSSvc_Smoke_S04_EnterOnECSRowNavigates(t *testing.T) {
	d := ecsSvcSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("ECSSvc-S04: right column not visible")
	}

	d = deliverECSSvcRelatedResult(d, "ecs", 1, "acme-services")
	d = deliverECSSvcRelatedResult(d, "tg", 1, "api-tg")
	d = deliverECSSvcRelatedResult(d, "alarm", 1, "ecs-svc-cpu-high")
	d = deliverECSSvcRelatedResult(d, "cfn", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("ECSSvc-S04: Enter on ecs row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("ECSSvc-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "ecs" {
		t.Errorf("ECSSvc-S04: RelatedNavigateMsg.TargetType = %q, want \"ecs\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// ECSSvc-S05: Enter on all-count=0 right column must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestECSSvc_Smoke_S05_EnterOnStubRowNoNav(t *testing.T) {
	d := ecsSvcSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("ECSSvc-S05: right column not visible")
	}

	d = deliverECSSvcRelatedResult(d, "ecs", 0)
	d = deliverECSSvcRelatedResult(d, "tg", 0)
	d = deliverECSSvcRelatedResult(d, "alarm", 0)
	d = deliverECSSvcRelatedResult(d, "cfn", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
			t.Errorf("ECSSvc-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
		}
	}
}

// ---------------------------------------------------------------------------
// ECSSvc-S06: Stub rows (cfn, nil checker) — demo checker returns result for all targets
// ---------------------------------------------------------------------------

func TestECSSvc_Smoke_S06_DemoCheckerOverridesNilChecker(t *testing.T) {
	defs := resource.GetRelated("ecs-svc")
	var cfnDef *resource.RelatedDef
	for i := range defs {
		if defs[i].TargetType == "cfn" {
			cfnDef = &defs[i]
			break
		}
	}
	if cfnDef == nil {
		t.Fatal("ECSSvc-S06: cfn related def not registered")
	}
	// CFN has a real checker (not nil) — verify it exists
	if cfnDef.Checker == nil {
		t.Fatal("ECSSvc-S06: cfn Checker must not be nil")
	}

	checker := resource.GetRelatedDemo("ecs-svc")
	if checker == nil {
		t.Fatal("ECSSvc-S06: no demo checker registered for ecs-svc")
	}
	results := checker(resource.Resource{ID: "demo-svc", Fields: map[string]string{"cluster": "demo-cluster"}})
	targetTypes := make(map[string]bool)
	for _, r := range results {
		targetTypes[r.TargetType] = true
	}
	for _, expected := range []string{"ecs", "tg", "alarm", "cfn"} {
		if !targetTypes[expected] {
			t.Errorf("ECSSvc-S06: demo checker did not return a result for %q target type", expected)
		}
	}
}
