package unit_test

// SSM Parameter related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to an SSM Parameter, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (KMS Key, CloudFormation)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row (kms, count=1) emits RelatedNavigateMsg with correct TargetType
//   - Enter on all-count=0 right column does NOT emit RelatedNavigateMsg
//
// Demo fixture: /acme/prod/app/config
// Demo results: kms→1, cfn→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ssmSmokeDetail builds a DetailModel for "ssm" using the demo fixture.
func ssmSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "/acme/prod/app/config",
		Name: "/acme/prod/app/config",
		Fields: map[string]string{
			"name":          "/acme/prod/app/config",
			"type":          "SecureString",
			"version":       "3",
			"last_modified": "2026-01-15 10:30",
			"description":   "Application config",
		},
		RawStruct: ssmtypes.ParameterMetadata{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ssm", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverSSMRelatedResult delivers a RelatedCheckResultMsg for "ssm".
func deliverSSMRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "ssm",
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
// SSM-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestSSM_Smoke_S01_RightColVisible(t *testing.T) {
	d := ssmSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("SSM-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("SSM-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// SSM-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestSSM_Smoke_S02_CorrectLabels(t *testing.T) {
	d := ssmSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("SSM-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"KMS Key", "CloudFormation"} {
		if !strings.Contains(plain, label) {
			t.Errorf("SSM-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// SSM-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestSSM_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := ssmSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("SSM-S03: right column not visible")
	}

	// Deliver demo-equivalent results: kms→1, cfn→0
	d = deliverSSMRelatedResult(d, "kms", 1, "arn:aws:kms:us-east-1:123456789012:key/demo-key-001")
	d = deliverSSMRelatedResult(d, "cfn", 0)

	plain := stripAnsi(d.View())

	// kms should show (1); cfn should show (0)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("SSM-S03: expected '(1)' count in right column after delivering kms result; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("SSM-S03: expected '(0)' for cfn row; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// SSM-S04: Tab focuses right column; Enter on kms row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestSSM_Smoke_S04_EnterOnKMSRowNavigates(t *testing.T) {
	d := ssmSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("SSM-S04: right column not visible")
	}

	d = deliverSSMRelatedResult(d, "kms", 1, "arn:aws:kms:us-east-1:123456789012:key/demo-key-001")
	d = deliverSSMRelatedResult(d, "cfn", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "kms"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("SSM-S04: Enter on kms row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("SSM-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "kms" {
		t.Errorf("SSM-S04: RelatedNavigateMsg.TargetType = %q, want \"kms\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// SSM-S05: All count=0; Enter on right column must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestSSM_Smoke_S05_EnterOnAllZeroRowNoNav(t *testing.T) {
	d := ssmSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("SSM-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any navigable row
	d = deliverSSMRelatedResult(d, "kms", 0)
	d = deliverSSMRelatedResult(d, "cfn", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("SSM-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// SSM-S06: cfn checker is nil (stub); kms checker is non-nil.
// Demo checker is registered and returns results for both targets.
// ---------------------------------------------------------------------------

func TestSSM_Smoke_S06_CheckersAndDemoChecker(t *testing.T) {
	defs := resource.GetRelated("ssm")
	if len(defs) == 0 {
		t.Fatal("SSM-S06: no related defs registered for ssm")
	}

	// cfn must have nil checker; kms must have non-nil checker
	nilCheckers := map[string]bool{"cfn": true}
	nonNilCheckers := map[string]bool{"kms": true}

	for i := range defs {
		tt := defs[i].TargetType
		if nilCheckers[tt] {
			if defs[i].Checker != nil {
				t.Errorf("SSM-S06: Checker for target %q must be nil (stub); got non-nil — implementation changed?", tt)
			}
		}
		if nonNilCheckers[tt] {
			if defs[i].Checker == nil {
				t.Errorf("SSM-S06: Checker for target %q must be non-nil (real checker); got nil", tt)
			}
		}
	}

	// Demo checker must be registered
	checker := resource.GetRelatedDemo("ssm")
	if checker == nil {
		t.Fatal("SSM-S06: no demo checker registered for ssm")
	}

	// Demo checker must return results for both target types
	results := checker(resource.Resource{ID: "/acme/prod/app/config"})

	targetTypes := []string{"kms", "cfn"}
	for _, tt := range targetTypes {
		found := false
		for i := range results {
			if results[i].TargetType == tt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SSM-S06: demo checker did not return a result for target type %q", tt)
		}
	}
}
