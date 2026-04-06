package unit_test

// SQS Queue related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to an SQS Queue, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (SNS Subscriptions, CloudWatch Alarms, Lambda Functions)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row (sns-sub, count=1) emits RelatedNavigateMsg with correct TargetType
//   - Enter on all-count=0 right column does NOT emit RelatedNavigateMsg
//
// Demo fixture: payment-processing
// Demo results: sns-sub→1, alarm→1, lambda→0

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	internalaws "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// sqsSmokeDetail builds a DetailModel for "sqs" using the demo fixture.
func sqsSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "payment-processing",
		Name: "payment-processing",
		Fields: map[string]string{
			"queue_name":         "payment-processing",
			"queue_url":          "https://sqs.us-east-1.amazonaws.com/123456789012/payment-processing",
			"approx_messages":    "42",
			"approx_not_visible": "3",
			"delay_seconds":      "0",
		},
		RawStruct: internalaws.SQSQueueAttributesRow{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "sqs", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverSQSRelatedResult delivers a RelatedCheckResultMsg for "sqs".
func deliverSQSRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "sqs",
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
// SQS-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestSQS_Smoke_S01_RightColVisible(t *testing.T) {
	d := sqsSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("SQS-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("SQS-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// SQS-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestSQS_Smoke_S02_CorrectLabels(t *testing.T) {
	d := sqsSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("SQS-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"SNS Subscriptions", "CloudWatch Alarms", "Lambda Functions"} {
		if !strings.Contains(plain, label) {
			t.Errorf("SQS-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// SQS-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestSQS_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := sqsSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("SQS-S03: right column not visible")
	}

	// Deliver demo-equivalent results: sns-sub→1, alarm→1, lambda→0
	d = deliverSQSRelatedResult(d, "sns-sub", 1, "arn:aws:sns:us-east-1:123456789012:payment-events:sub-001")
	d = deliverSQSRelatedResult(d, "alarm", 1, "payment-queue-depth-alarm")
	d = deliverSQSRelatedResult(d, "lambda", 0)

	plain := stripAnsi(d.View())

	// sns-sub and alarm should show (1); lambda should show (0)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("SQS-S03: expected '(1)' count in right column after delivering sns-sub/alarm results; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("SQS-S03: expected '(0)' for lambda row; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// SQS-S04: Tab focuses right column; Enter on first row (sns-sub, count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestSQS_Smoke_S04_EnterOnSNSSubRowNavigates(t *testing.T) {
	d := sqsSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("SQS-S04: right column not visible")
	}

	d = deliverSQSRelatedResult(d, "sns-sub", 1, "arn:aws:sns:us-east-1:123456789012:payment-events:sub-001")
	d = deliverSQSRelatedResult(d, "alarm", 1, "payment-queue-depth-alarm")
	d = deliverSQSRelatedResult(d, "lambda", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "sns-sub"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("SQS-S04: Enter on sns-sub row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("SQS-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "sns-sub" {
		t.Errorf("SQS-S04: RelatedNavigateMsg.TargetType = %q, want \"sns-sub\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// SQS-S05: All count=0; Enter on right column must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestSQS_Smoke_S05_EnterOnAllZeroRowNoNav(t *testing.T) {
	d := sqsSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("SQS-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any navigable row
	d = deliverSQSRelatedResult(d, "sns-sub", 0)
	d = deliverSQSRelatedResult(d, "alarm", 0)
	d = deliverSQSRelatedResult(d, "lambda", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("SQS-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// SQS-S06: sns-sub, alarm, and lambda checkers are non-nil (real checkers).
// cfn def no longer exists.
// Demo checker is registered and returns results for all 3 targets.
// ---------------------------------------------------------------------------

func TestSQS_Smoke_S06_CheckersAndDemoChecker(t *testing.T) {
	defs := resource.GetRelated("sqs")
	if len(defs) == 0 {
		t.Fatal("SQS-S06: no related defs registered for sqs")
	}

	// sns-sub, alarm, and lambda must all have non-nil checkers
	nonNilCheckers := map[string]bool{"sns-sub": true, "alarm": true, "lambda": true}

	for i := range defs {
		tt := defs[i].TargetType
		if nonNilCheckers[tt] {
			if defs[i].Checker == nil {
				t.Errorf("SQS-S06: Checker for target %q must be non-nil (real checker); got nil", tt)
			}
		}
		// cfn must no longer be registered
		if tt == "cfn" {
			t.Errorf("SQS-S06: cfn related def must not be registered (removed); found unexpected def")
		}
	}

	// Demo checker must be registered
	checker := resource.GetRelatedDemo("sqs")
	if checker == nil {
		t.Fatal("SQS-S06: no demo checker registered for sqs")
	}

	// Demo checker must return results for all 3 target types
	results := checker(resource.Resource{ID: "payment-processing"})

	targetTypes := []string{"sns-sub", "alarm", "lambda"}
	for _, tt := range targetTypes {
		found := false
		for i := range results {
			if results[i].TargetType == tt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SQS-S06: demo checker did not return a result for target type %q", tt)
		}
	}
}
