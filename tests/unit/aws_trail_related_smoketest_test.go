package unit_test

// CloudTrail trail related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to a CloudTrail trail, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (S3 Bucket, Log Groups, SNS Topic, KMS Key)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on count=0 row does NOT emit RelatedNavigateMsg
//
// Demo fixture: acme-management-trail
// Demo results: s3→1, logs→1, sns→0, kms→1

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// trailSmokeDetail builds a DetailModel for "trail" using the demo fixture.
func trailSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "my-trail",
		Name: "my-trail",
		Fields: map[string]string{
			"trail_name": "my-trail",
			"s3_bucket":  "my-audit-bucket",
		},
		RawStruct: cloudtrailtypes.Trail{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "trail", nil, k)
	d.SetSize(width, height)
	return d
}

// trailSmokeDetailWithID builds a DetailModel for "trail" using a specific demo fixture ID.
func trailSmokeDetailWithID(t *testing.T, id, name string, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   id,
		Name: name,
		Fields: map[string]string{
			"trail_name": name,
			"s3_bucket":  "my-audit-bucket",
		},
		RawStruct: cloudtrailtypes.Trail{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "trail", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverTrailRelatedResult delivers a RelatedCheckResultMsg for "trail".
func deliverTrailRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "trail",
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
// Trail-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestTrail_Smoke_S01_RightColVisible(t *testing.T) {
	d := trailSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("Trail-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("Trail-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// Trail-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestTrail_Smoke_S02_CorrectLabels(t *testing.T) {
	d := trailSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("Trail-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"S3 Bucket", "Log Groups", "SNS Topic", "KMS Key"} {
		if !strings.Contains(plain, label) {
			t.Errorf("Trail-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// Trail-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestTrail_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	// Use acme-management-trail which has demo results: s3→1, logs→1, sns→0, kms→1
	d := trailSmokeDetailWithID(t, "acme-management-trail", "acme-management-trail", 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("Trail-S03: right column not visible")
	}

	// Deliver demo-equivalent results
	d = deliverTrailRelatedResult(d, "s3", 1, "my-audit-bucket")
	d = deliverTrailRelatedResult(d, "logs", 1, "/aws/cloudtrail/management")
	d = deliverTrailRelatedResult(d, "sns", 0)
	d = deliverTrailRelatedResult(d, "kms", 1, "arn:aws:kms:us-east-1:123456789012:key/trail-key-id")

	plain := stripAnsi(d.View())

	// s3, logs, and kms should show (1); sns should show (0)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("Trail-S03: expected '(1)' count in right column after delivering s3/logs/kms results; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("Trail-S03: expected '(0)' for sns row; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Trail-S04: Tab focuses right column; Enter on s3 row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestTrail_Smoke_S04_EnterOnS3RowNavigates(t *testing.T) {
	d := trailSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("Trail-S04: right column not visible")
	}

	d = deliverTrailRelatedResult(d, "s3", 1, "my-audit-bucket")
	d = deliverTrailRelatedResult(d, "logs", 1, "/aws/cloudtrail/management")
	d = deliverTrailRelatedResult(d, "sns", 0)
	d = deliverTrailRelatedResult(d, "kms", 1, "arn:aws:kms:us-east-1:123456789012:key/trail-key-id")

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "s3"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Trail-S04: Enter on s3 row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("Trail-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "s3" {
		t.Errorf("Trail-S04: RelatedNavigateMsg.TargetType = %q, want \"s3\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// Trail-S05: Enter with all count=0 must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestTrail_Smoke_S05_EnterOnAllZeroRowsNoNav(t *testing.T) {
	d := trailSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("Trail-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any row
	d = deliverTrailRelatedResult(d, "s3", 0)
	d = deliverTrailRelatedResult(d, "logs", 0)
	d = deliverTrailRelatedResult(d, "sns", 0)
	d = deliverTrailRelatedResult(d, "kms", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("Trail-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Trail-S06: All 4 checkers are non-nil (real). Demo checker registered and returns all 4 targets.
// ---------------------------------------------------------------------------

func TestTrail_Smoke_S06_AllCheckersNonNilAndDemoRegistered(t *testing.T) {
	defs := resource.GetRelated("trail")
	if len(defs) == 0 {
		t.Fatal("Trail-S06: no related defs registered for trail")
	}

	expectedTargets := []string{"s3", "logs", "sns", "kms"}

	// Verify all 4 checkers are non-nil (real implementations)
	for _, targetType := range expectedTargets {
		var found *resource.RelatedDef
		for i := range defs {
			if defs[i].TargetType == targetType {
				found = &defs[i]
				break
			}
		}
		if found == nil {
			t.Errorf("Trail-S06: related def for target %q not registered", targetType)
			continue
		}
		if found.Checker == nil {
			t.Errorf("Trail-S06: Checker for target %q must be non-nil (real implementation); got nil", targetType)
		}
	}

	// Demo checker must be registered and return results for all 4 target types
	checker := resource.GetRelatedDemo("trail")
	if checker == nil {
		t.Fatal("Trail-S06: no demo checker registered for trail")
	}

	results := checker(resource.Resource{ID: "acme-management-trail"})
	for _, targetType := range expectedTargets {
		var found *resource.RelatedCheckResult
		for i := range results {
			if results[i].TargetType == targetType {
				found = &results[i]
				break
			}
		}
		if found == nil {
			t.Errorf("Trail-S06: demo checker did not return a result for target type %q", targetType)
		}
	}
}
