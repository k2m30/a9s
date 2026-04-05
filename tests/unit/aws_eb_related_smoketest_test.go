package unit_test

// EB related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to an EB environment, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (CloudFormation Stack, Log Groups, Auto Scaling Groups)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on count=0 row does NOT emit RelatedNavigateMsg
//
// Demo fixture: e-acmeprodapi
// Demo results: cfn→1, logs→0, asg→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ebSmokeDetail builds a DetailModel for "eb" using the demo fixture.
func ebSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "e-acmeprodapi",
		Name: "acme-prod-api",
		Fields: map[string]string{
			"environment_id":   "e-acmeprodapi",
			"environment_name": "acme-prod-api",
			"status":           "Ready",
		},
		RawStruct: ebtypes.EnvironmentDescription{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "eb", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverEBRelatedResult delivers a RelatedCheckResultMsg for "eb".
func deliverEBRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "eb",
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
// EB-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestEB_Smoke_S01_RightColVisible(t *testing.T) {
	d := ebSmokeDetail(t, 120, 30)
	v := d.View()

	if !strings.Contains(v, "RELATED") {
		t.Fatal("EB-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(v, "│") {
		t.Fatal("EB-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// EB-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestEB_Smoke_S02_CorrectLabels(t *testing.T) {
	d := ebSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("EB-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"CloudFormation Stack", "Log Groups", "Auto Scaling Groups"} {
		if !strings.Contains(plain, label) {
			t.Errorf("EB-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// EB-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestEB_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := ebSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EB-S03: right column not visible")
	}

	// Deliver demo-equivalent results: cfn=1, logs=0, asg=0
	d = deliverEBRelatedResult(d, "cfn", 1, "awseb-e-acmeprodapi-stack")
	d = deliverEBRelatedResult(d, "logs", 0)
	d = deliverEBRelatedResult(d, "asg", 0)

	plain := stripAnsi(d.View())

	// cfn should show (1); logs and asg should show (0)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("EB-S03: expected '(1)' count in right column after delivering cfn result; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("EB-S03: expected '(0)' for logs/asg rows; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// EB-S04: Tab focuses right column; Enter on cfn row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestEB_Smoke_S04_EnterOnCFNRowNavigates(t *testing.T) {
	d := ebSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EB-S04: right column not visible")
	}

	d = deliverEBRelatedResult(d, "cfn", 1, "awseb-e-acmeprodapi-stack")
	d = deliverEBRelatedResult(d, "logs", 0)
	d = deliverEBRelatedResult(d, "asg", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "cfn"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("EB-S04: Enter on cfn row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("EB-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "cfn" {
		t.Errorf("EB-S04: RelatedNavigateMsg.TargetType = %q, want \"cfn\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// EB-S05: Enter on all-count=0 rows must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestEB_Smoke_S05_EnterOnZeroRowsNoNav(t *testing.T) {
	d := ebSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EB-S05: right column not visible")
	}

	// All count=0
	d = deliverEBRelatedResult(d, "cfn", 0)
	d = deliverEBRelatedResult(d, "logs", 0)
	d = deliverEBRelatedResult(d, "asg", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("EB-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// EB-S06: All checkers are real; demo checker returns cfn result
// ---------------------------------------------------------------------------

func TestEB_Smoke_S06_DemoCheckerReturnsCFNResult(t *testing.T) {
	defs := resource.GetRelated("eb")

	// Verify all 3 have real checkers (not nil)
	for _, target := range []string{"cfn", "logs", "asg"} {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker == nil {
					t.Errorf("EB-S06: %q Checker must not be nil — expected real checker", target)
				}
				break
			}
		}
		if !found {
			t.Errorf("EB-S06: related def for target %q not registered", target)
		}
	}

	// Demo checker must return a result with cfn TargetType
	checker := resource.GetRelatedDemo("eb")
	if checker == nil {
		t.Fatal("EB-S06: no demo checker registered for eb")
	}
	results := checker(resource.Resource{ID: "e-acmeprodapi"})
	var cfnResult *resource.RelatedCheckResult
	for i := range results {
		if results[i].TargetType == "cfn" {
			cfnResult = &results[i]
			break
		}
	}
	if cfnResult == nil {
		t.Fatal("EB-S06: demo checker did not return a result for cfn target type")
	}
}
