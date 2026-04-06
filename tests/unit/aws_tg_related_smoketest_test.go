package unit_test

// TG related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to a Target Group, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (Load Balancers, ECS Services, Auto Scaling Groups, CW Alarms)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on count=0 row does NOT emit RelatedNavigateMsg
//
// Demo fixture: my-tg
// Demo results: elb→1, ecs-svc→1, asg→0, alarm→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// tgSmokeDetail builds a DetailModel for "tg" using the demo fixture.
func tgSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "my-tg",
		Name: "my-tg",
		Fields: map[string]string{
			"target_group_name": "my-tg",
			"target_group_arn":  "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abc123",
			"vpc_id":            "vpc-abc123",
			"target_type":       "instance",
		},
		RawStruct: elbv2types.TargetGroup{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "tg", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverTGRelatedResult delivers a RelatedCheckResultMsg for "tg".
func deliverTGRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "tg",
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
// TG-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestTG_Smoke_S01_RightColVisible(t *testing.T) {
	d := tgSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("TG-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("TG-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// TG-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestTG_Smoke_S02_CorrectLabels(t *testing.T) {
	d := tgSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("TG-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"Load Balancers", "ECS Services", "Auto Scaling Groups", "CW Alarms"} {
		if !strings.Contains(plain, label) {
			t.Errorf("TG-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// TG-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestTG_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := tgSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("TG-S03: right column not visible")
	}

	// Deliver demo-equivalent results
	d = deliverTGRelatedResult(d, "elb", 1, "prod-alb")
	d = deliverTGRelatedResult(d, "ecs-svc", 1, "api-gateway")
	d = deliverTGRelatedResult(d, "asg", 0)
	d = deliverTGRelatedResult(d, "alarm", 0)

	plain := stripAnsi(d.View())

	// elb and ecs-svc should show (1); asg and alarm should show (0)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("TG-S03: expected '(1)' count in right column after delivering elb/ecs-svc results; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("TG-S03: expected '(0)' for asg/alarm rows; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// TG-S04: Tab focuses right column; Enter on elb row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestTG_Smoke_S04_EnterOnELBRowNavigates(t *testing.T) {
	d := tgSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("TG-S04: right column not visible")
	}

	d = deliverTGRelatedResult(d, "elb", 1, "prod-alb")
	d = deliverTGRelatedResult(d, "ecs-svc", 1, "api-gateway")
	d = deliverTGRelatedResult(d, "asg", 0)
	d = deliverTGRelatedResult(d, "alarm", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "elb"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("TG-S04: Enter on elb row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("TG-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "elb" {
		t.Errorf("TG-S04: RelatedNavigateMsg.TargetType = %q, want \"elb\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// TG-S05: Enter on all-count=0 right column must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestTG_Smoke_S05_EnterOnStubRowNoNav(t *testing.T) {
	d := tgSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("TG-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any row
	d = deliverTGRelatedResult(d, "elb", 0)
	d = deliverTGRelatedResult(d, "ecs-svc", 0)
	d = deliverTGRelatedResult(d, "asg", 0)
	d = deliverTGRelatedResult(d, "alarm", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("TG-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TG-S06: elb, ecs-svc, asg, alarm checkers are non-nil; cfn def no longer exists.
// Demo checker is registered and returns results for all 4 remaining targets.
// ---------------------------------------------------------------------------

func TestTG_Smoke_S06_CheckerRegistration(t *testing.T) {
	defs := resource.GetRelated("tg")

	// Verify non-nil checkers for elb, ecs-svc, asg, alarm
	nonNilCheckerTypes := []string{"elb", "ecs-svc", "asg", "alarm"}
	for _, targetType := range nonNilCheckerTypes {
		var found *resource.RelatedDef
		for i := range defs {
			if defs[i].TargetType == targetType {
				found = &defs[i]
				break
			}
		}
		if found == nil {
			t.Errorf("TG-S06: %s related def not registered", targetType)
			continue
		}
		if found.Checker == nil {
			t.Errorf("TG-S06: %s Checker must be non-nil; got nil", targetType)
		}
	}

	// cfn must no longer be registered
	for i := range defs {
		if defs[i].TargetType == "cfn" {
			t.Errorf("TG-S06: cfn related def must not be registered (removed); found unexpected def")
		}
	}

	// Demo checker must be registered and return results for all 4 remaining target types
	checker := resource.GetRelatedDemo("tg")
	if checker == nil {
		t.Fatal("TG-S06: no demo checker registered for tg")
	}
	results := checker(resource.Resource{ID: "tg-demo"})
	expectedTypes := []string{"elb", "ecs-svc", "asg", "alarm"}
	for _, targetType := range expectedTypes {
		var found *resource.RelatedCheckResult
		for i := range results {
			if results[i].TargetType == targetType {
				found = &results[i]
				break
			}
		}
		if found == nil {
			t.Errorf("TG-S06: demo checker did not return a result for %s target type", targetType)
		}
	}
}
