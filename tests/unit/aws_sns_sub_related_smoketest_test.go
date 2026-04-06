package unit_test

// SNS Subscription related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to an SNS Subscription, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (SNS Topic, Lambda Function, SQS Queue)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row (sns, count=1) emits RelatedNavigateMsg with correct TargetType
//   - Enter on all-count=0 right column does NOT emit RelatedNavigateMsg
//
// Demo fixture: arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012
// Demo results (protocol=sqs): sns→1, sqs→1, lambda→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// snsSubSmokeDetail builds a DetailModel for "sns-sub" using the demo fixture (protocol=sqs).
func snsSubSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012",
		Name: "alarm-notifications",
		Fields: map[string]string{
			"topic_arn":        "arn:aws:sns:us-east-1:123456789012:alarm-notifications",
			"protocol":         "sqs",
			"endpoint":         "arn:aws:sqs:us-east-1:123456789012:alarm-queue",
			"subscription_arn": "arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012",
		},
		RawStruct: snstypes.Subscription{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "sns-sub", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverSNSSubRelatedResult delivers a RelatedCheckResultMsg for "sns-sub".
func deliverSNSSubRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "sns-sub",
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
// SNSSub-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestSNSSub_Smoke_S01_RightColVisible(t *testing.T) {
	d := snsSubSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("SNSSub-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("SNSSub-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// SNSSub-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestSNSSub_Smoke_S02_CorrectLabels(t *testing.T) {
	d := snsSubSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("SNSSub-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"SNS Topic", "Lambda Function", "SQS Queue"} {
		if !strings.Contains(plain, label) {
			t.Errorf("SNSSub-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// SNSSub-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestSNSSub_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := snsSubSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("SNSSub-S03: right column not visible")
	}

	// Deliver demo-equivalent results: sns→1, sqs→1, lambda→0
	d = deliverSNSSubRelatedResult(d, "sns", 1, "arn:aws:sns:us-east-1:123456789012:order-events")
	d = deliverSNSSubRelatedResult(d, "sqs", 1, "order-processing-queue")
	d = deliverSNSSubRelatedResult(d, "lambda", 0)

	plain := stripAnsi(d.View())

	// sns and sqs should show (1); lambda should show (0)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("SNSSub-S03: expected '(1)' count in right column after delivering sns/sqs results; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("SNSSub-S03: expected '(0)' for lambda row; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// SNSSub-S04: Tab focuses right column; Enter on first row (sns, count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestSNSSub_Smoke_S04_EnterOnSNSRowNavigates(t *testing.T) {
	d := snsSubSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("SNSSub-S04: right column not visible")
	}

	d = deliverSNSSubRelatedResult(d, "sns", 1, "arn:aws:sns:us-east-1:123456789012:order-events")
	d = deliverSNSSubRelatedResult(d, "sqs", 1, "order-processing-queue")
	d = deliverSNSSubRelatedResult(d, "lambda", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "sns"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("SNSSub-S04: Enter on sns row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("SNSSub-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "sns" {
		t.Errorf("SNSSub-S04: RelatedNavigateMsg.TargetType = %q, want \"sns\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// SNSSub-S05: All count=0; Enter on right column must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestSNSSub_Smoke_S05_EnterOnAllZeroRowNoNav(t *testing.T) {
	d := snsSubSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("SNSSub-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any navigable row
	d = deliverSNSSubRelatedResult(d, "sns", 0)
	d = deliverSNSSubRelatedResult(d, "sqs", 0)
	d = deliverSNSSubRelatedResult(d, "lambda", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("SNSSub-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// SNSSub-S06: All 3 checkers are non-nil (real); demo checker is registered and
// returns results for all 3 targets.
// ---------------------------------------------------------------------------

func TestSNSSub_Smoke_S06_RealCheckersAndDemoChecker(t *testing.T) {
	defs := resource.GetRelated("sns-sub")
	if len(defs) == 0 {
		t.Fatal("SNSSub-S06: no related defs registered for sns-sub")
	}

	// All 3 checkers must be non-nil (real implementations, not stubs)
	for i := range defs {
		if defs[i].Checker == nil {
			t.Errorf("SNSSub-S06: Checker for target %q must be non-nil (real checker); got nil", defs[i].TargetType)
		}
	}

	// Demo checker must be registered
	checker := resource.GetRelatedDemo("sns-sub")
	if checker == nil {
		t.Fatal("SNSSub-S06: no demo checker registered for sns-sub")
	}

	// Demo checker must return results for all 3 target types
	results := checker(resource.Resource{
		ID: "arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012",
	})

	targetTypes := []string{"sns", "lambda", "sqs"}
	for _, tt := range targetTypes {
		found := false
		for i := range results {
			if results[i].TargetType == tt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SNSSub-S06: demo checker did not return a result for target type %q", tt)
		}
	}
}
