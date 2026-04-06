package unit_test

// TGW related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to a Transit Gateway, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (VPCs, Route Tables, CloudFormation)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on rtb row (count=1) emits RelatedNavigateMsg with correct TargetType
//   - Enter on all count=0 rows does NOT emit RelatedNavigateMsg
//
// Demo fixture: tgw-0a1b2c3d4e5f67890
// Demo results: rtb→1, vpc→0 (nil checker), cfn→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// tgwSmokeDetail builds a DetailModel for "tgw" using the demo fixture.
func tgwSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "tgw-0a1b2c3d4e5f67890",
		Name: "prod-tgw",
		Fields: map[string]string{
			"tgw_id": "tgw-0a1b2c3d4e5f67890",
			"name":   "prod-tgw",
			"state":  "available",
		},
		RawStruct: ec2types.TransitGateway{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "tgw", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverTGWRelatedResult delivers a RelatedCheckResultMsg for "tgw".
func deliverTGWRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "tgw",
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
// TGW-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestTGW_Smoke_S01_RightColVisible(t *testing.T) {
	d := tgwSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("TGW-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("TGW-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// TGW-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestTGW_Smoke_S02_CorrectLabels(t *testing.T) {
	d := tgwSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("TGW-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"VPCs", "Route Tables", "CloudFormation"} {
		if !strings.Contains(plain, label) {
			t.Errorf("TGW-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// TGW-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestTGW_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := tgwSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("TGW-S03: right column not visible")
	}

	// Deliver demo-equivalent results
	d = deliverTGWRelatedResult(d, "rtb", 1, "rtb-0aaa111111111111a")
	d = deliverTGWRelatedResult(d, "vpc", 0)
	d = deliverTGWRelatedResult(d, "cfn", 0)

	plain := stripAnsi(d.View())

	// rtb should show (1); vpc and cfn should show (0)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("TGW-S03: expected '(1)' count in right column after delivering rtb result; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("TGW-S03: expected '(0)' for vpc/cfn stub rows; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// TGW-S04: Tab focuses right column; Enter on rtb row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestTGW_Smoke_S04_EnterOnRTBRowNavigates(t *testing.T) {
	d := tgwSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("TGW-S04: right column not visible")
	}

	d = deliverTGWRelatedResult(d, "rtb", 1, "rtb-0aaa111111111111a")
	d = deliverTGWRelatedResult(d, "vpc", 0)
	d = deliverTGWRelatedResult(d, "cfn", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "rtb"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("TGW-S04: Enter on rtb row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("TGW-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "rtb" {
		t.Errorf("TGW-S04: RelatedNavigateMsg.TargetType = %q, want \"rtb\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// TGW-S05: All count=0 rows → Enter must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestTGW_Smoke_S05_EnterOnAllZeroRowsNoNav(t *testing.T) {
	d := tgwSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("TGW-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any navigable row
	d = deliverTGWRelatedResult(d, "rtb", 0)
	d = deliverTGWRelatedResult(d, "vpc", 0)
	d = deliverTGWRelatedResult(d, "cfn", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("TGW-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TGW-S06: vpc checker nil; rtb and cfn non-nil; demo checker registered
// ---------------------------------------------------------------------------

func TestTGW_Smoke_S06_CheckerRegistration(t *testing.T) {
	defs := resource.GetRelated("tgw")

	type defCheck struct {
		targetType     string
		wantNilChecker bool
	}
	checks := []defCheck{
		{"vpc", true},
		{"cfn", false},
		{"rtb", false},
	}

	defMap := make(map[string]*resource.RelatedDef)
	for i := range defs {
		defMap[defs[i].TargetType] = &defs[i]
	}

	for _, c := range checks {
		def, found := defMap[c.targetType]
		if !found {
			t.Errorf("TGW-S06: related def for %q not registered", c.targetType)
			continue
		}
		if c.wantNilChecker && def.Checker != nil {
			t.Errorf("TGW-S06: %q Checker must be nil (stub); got non-nil — implementation changed?", c.targetType)
		}
		if !c.wantNilChecker && def.Checker == nil {
			t.Errorf("TGW-S06: %q Checker must be non-nil; got nil", c.targetType)
		}
	}

	// Demo checker must still return a result for all target types
	checker := resource.GetRelatedDemo("tgw")
	if checker == nil {
		t.Fatal("TGW-S06: no demo checker registered for tgw")
	}
	results := checker(resource.Resource{ID: "tgw-0a1b2c3d4e5f67890"})

	resultMap := make(map[string]*resource.RelatedCheckResult)
	for i := range results {
		resultMap[results[i].TargetType] = &results[i]
	}

	for _, targetType := range []string{"vpc", "rtb", "cfn"} {
		if _, found := resultMap[targetType]; !found {
			t.Errorf("TGW-S06: demo checker did not return a result for %q target type", targetType)
		}
	}

	// rtb should have count=1 for the demo fixture ID
	if rtbResult, ok := resultMap["rtb"]; ok {
		if rtbResult.Count != 1 {
			t.Errorf("TGW-S06: demo rtb result count = %d, want 1", rtbResult.Count)
		}
	}
}
