package unit_test

// EventBridge Rule related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to an eb-rule, and checking:
//   - Right column visible with RELATED header
//   - Correct label (IAM Role)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on count=0 row does NOT emit RelatedNavigateMsg
//
// Demo fixture: nightly-db-backup
// Demo results: role→1 (acme-ci-deploy-role)

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ebRuleSmokeDetail builds a DetailModel for "eb-rule" using the demo fixture.
func ebRuleSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "nightly-db-backup",
		Name: "nightly-db-backup",
		Fields: map[string]string{
			"name":  "nightly-db-backup",
			"state": "ENABLED",
		},
		RawStruct: eventbridgetypes.Rule{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "eb-rule", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverEbRuleRelatedResult delivers a RelatedCheckResultMsg for "eb-rule".
func deliverEbRuleRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "eb-rule",
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
// EbRule-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestEbRule_Smoke_S01_RightColVisible(t *testing.T) {
	d := ebRuleSmokeDetail(t, 120, 30)
	view := d.View()

	if !strings.Contains(view, "RELATED") {
		t.Fatal("EbRule-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(view, "│") {
		t.Fatal("EbRule-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// EbRule-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestEbRule_Smoke_S02_CorrectLabels(t *testing.T) {
	d := ebRuleSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("EbRule-S02: right column not visible; skipping label check")
	}

	if !strings.Contains(plain, "IAM Role") {
		t.Errorf("EbRule-S02: expected label \"IAM Role\" in right column; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// EbRule-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestEbRule_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := ebRuleSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EbRule-S03: right column not visible")
	}

	// Deliver demo-equivalent results: role=1
	d = deliverEbRuleRelatedResult(d, "role", 1, "acme-ci-deploy-role")

	plain := stripAnsi(d.View())

	// role should show (1)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("EbRule-S03: expected '(1)' count in right column after delivering role result; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// EbRule-S04: Tab focuses right column; Enter on role row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestEbRule_Smoke_S04_EnterOnRoleRowNavigates(t *testing.T) {
	d := ebRuleSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EbRule-S04: right column not visible")
	}

	d = deliverEbRuleRelatedResult(d, "role", 1, "acme-ci-deploy-role")

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "role"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("EbRule-S04: Enter on role row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("EbRule-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "role" {
		t.Errorf("EbRule-S04: RelatedNavigateMsg.TargetType = %q, want \"role\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// EbRule-S05: Enter on role row (count=0) must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestEbRule_Smoke_S05_EnterOnZeroCountRowNoNav(t *testing.T) {
	d := ebRuleSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EbRule-S05: right column not visible")
	}

	// Deliver count=0 so no navigation should occur
	d = deliverEbRuleRelatedResult(d, "role", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("EbRule-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// EbRule-S06: role has a real checker; demo checker returns role result
// ---------------------------------------------------------------------------

func TestEbRule_Smoke_S06_RealCheckerAndDemoResult(t *testing.T) {
	defs := resource.GetRelated("eb-rule")
	var roleDef *resource.RelatedDef
	for i := range defs {
		if defs[i].TargetType == "role" {
			roleDef = &defs[i]
			break
		}
	}
	if roleDef == nil {
		t.Fatal("EbRule-S06: role related def not registered")
	}
	if roleDef.Checker == nil {
		t.Fatal("EbRule-S06: role Checker must not be nil; got nil — implementation missing?")
	}

	// Demo checker must return a result for role
	checker := resource.GetRelatedDemo("eb-rule")
	if checker == nil {
		t.Fatal("EbRule-S06: no demo checker registered for eb-rule")
	}
	results := checker(resource.Resource{ID: "nightly-db-backup"})
	var roleResult *resource.RelatedCheckResult
	for i := range results {
		if results[i].TargetType == "role" {
			roleResult = &results[i]
			break
		}
	}
	if roleResult == nil {
		t.Fatal("EbRule-S06: demo checker did not return a result for role target type")
	}
	if roleResult.Count != 1 {
		t.Errorf("EbRule-S06: demo role result Count = %d, want 1", roleResult.Count)
	}
	if len(roleResult.ResourceIDs) == 0 || roleResult.ResourceIDs[0] != "acme-ci-deploy-role" {
		t.Errorf("EbRule-S06: demo role result ResourceIDs = %v, want [\"acme-ci-deploy-role\"]", roleResult.ResourceIDs)
	}
}
