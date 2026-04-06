package unit_test

// WAF Web ACL related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to a WAF Web ACL, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (Load Balancers, API Gateways, CloudFront)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on count=0 (stub) row does NOT emit RelatedNavigateMsg
//
// Demo fixture: my-waf-id
// Demo results: elb→1, apigw→0, cf→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// wafSmokeDetail builds a DetailModel for "waf" using the demo fixture.
func wafSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "my-waf-id",
		Name: "my-waf",
		Fields: map[string]string{
			"name":        "my-waf",
			"id":          "my-waf-id",
			"description": "Test WAF",
		},
		RawStruct: wafv2types.WebACLSummary{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "waf", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverWAFRelatedResult delivers a RelatedCheckResultMsg for "waf".
func deliverWAFRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "waf",
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
// WAF-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestWAF_Smoke_S01_RightColVisible(t *testing.T) {
	d := wafSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("WAF-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("WAF-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// WAF-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestWAF_Smoke_S02_CorrectLabels(t *testing.T) {
	d := wafSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("WAF-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"Load Balancers", "API Gateways", "CloudFront"} {
		if !strings.Contains(plain, label) {
			t.Errorf("WAF-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// WAF-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestWAF_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := wafSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("WAF-S03: right column not visible")
	}

	// Deliver demo-equivalent results
	d = deliverWAFRelatedResult(d, "elb", 1, "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123")
	d = deliverWAFRelatedResult(d, "apigw", 0)
	d = deliverWAFRelatedResult(d, "cf", 0)

	plain := stripAnsi(d.View())

	// elb should show (1); apigw and cf should show (0)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("WAF-S03: expected '(1)' count in right column after delivering elb result; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("WAF-S03: expected '(0)' for stub apigw/cf rows; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// WAF-S04: Tab focuses right column; Enter on elb row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestWAF_Smoke_S04_EnterOnELBRowNavigates(t *testing.T) {
	d := wafSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("WAF-S04: right column not visible")
	}

	d = deliverWAFRelatedResult(d, "elb", 1, "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123")
	d = deliverWAFRelatedResult(d, "apigw", 0)
	d = deliverWAFRelatedResult(d, "cf", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "elb"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("WAF-S04: Enter on elb row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("WAF-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "elb" {
		t.Errorf("WAF-S04: RelatedNavigateMsg.TargetType = %q, want \"elb\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// WAF-S05: Enter on all count=0 rows must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestWAF_Smoke_S05_EnterOnStubRowNoNav(t *testing.T) {
	d := wafSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("WAF-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any row
	d = deliverWAFRelatedResult(d, "elb", 0)
	d = deliverWAFRelatedResult(d, "apigw", 0)
	d = deliverWAFRelatedResult(d, "cf", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("WAF-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// WAF-S06: All 3 checkers nil (stubs). Demo checker registered and returns all 3 targets.
// ---------------------------------------------------------------------------

func TestWAF_Smoke_S06_DemoCheckerOverridesNilChecker(t *testing.T) {
	defs := resource.GetRelated("waf")

	nilTargets := []string{"elb", "apigw", "cf"}
	for _, targetType := range nilTargets {
		var def *resource.RelatedDef
		for i := range defs {
			if defs[i].TargetType == targetType {
				def = &defs[i]
				break
			}
		}
		if def == nil {
			t.Fatalf("WAF-S06: %s related def not registered", targetType)
		}
		if def.Checker != nil {
			t.Fatalf("WAF-S06: %s Checker must be nil (stub); got non-nil — implementation changed?", targetType)
		}
	}

	// Demo checker must still return a result for all 3 target types
	checker := resource.GetRelatedDemo("waf")
	if checker == nil {
		t.Fatal("WAF-S06: no demo checker registered for waf")
	}
	results := checker(resource.Resource{ID: "my-waf-id"})

	for _, targetType := range nilTargets {
		var result *resource.RelatedCheckResult
		for i := range results {
			if results[i].TargetType == targetType {
				result = &results[i]
				break
			}
		}
		if result == nil {
			t.Fatalf("WAF-S06: demo checker did not return a result for %s target type", targetType)
		}
	}
}
