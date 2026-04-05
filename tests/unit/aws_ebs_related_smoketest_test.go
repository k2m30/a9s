package unit_test

// EBS related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to an EBS volume, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (EC2 Instance, EBS Snapshots, KMS Key)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on count=0 row does NOT emit RelatedNavigateMsg
//
// Demo fixture: vol-0a1b2c3d4e5f60001
// Demo results: ec2→1, ebs-snap→1, kms→1

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

func ebsSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "vol-0a1b2c3d4e5f60001",
		Name: "web-prod-01-root",
		Fields: map[string]string{
			"volume_id":   "vol-0a1b2c3d4e5f60001",
			"name":        "web-prod-01-root",
			"state":       "in-use",
			"size":        "50",
			"type":        "gp3",
			"encrypted":   "true",
			"attached_to": "i-0a1b2c3d4e5f60001",
		},
		RawStruct: ec2types.Volume{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ebs", nil, k)
	d.SetSize(width, height)
	return d
}

func deliverEBSRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "ebs",
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
// EBS-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestEBS_Smoke_S01_RightColVisible(t *testing.T) {
	d := ebsSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("EBS-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("EBS-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// EBS-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestEBS_Smoke_S02_CorrectLabels(t *testing.T) {
	d := ebsSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("EBS-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"EC2 Instance", "EBS Snapshots", "KMS Key"} {
		if !strings.Contains(plain, label) {
			t.Errorf("EBS-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// EBS-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestEBS_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := ebsSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EBS-S03: right column not visible")
	}

	d = deliverEBSRelatedResult(d, "ec2", 1, "i-0a1b2c3d4e5f60001")
	d = deliverEBSRelatedResult(d, "ebs-snap", 1, "snap-0a1b2c3d4e5f60001")
	d = deliverEBSRelatedResult(d, "kms", 1, "a1b2c3d4-5678-90ab-cdef-111111111111")

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "(1)") {
		t.Errorf("EBS-S03: expected '(1)' count in right column; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// EBS-S04: Tab focuses right column; Enter on ec2 row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestEBS_Smoke_S04_EnterOnEC2RowNavigates(t *testing.T) {
	d := ebsSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EBS-S04: right column not visible")
	}

	d = deliverEBSRelatedResult(d, "ec2", 1, "i-0a1b2c3d4e5f60001")
	d = deliverEBSRelatedResult(d, "ebs-snap", 1, "snap-0a1b2c3d4e5f60001")
	d = deliverEBSRelatedResult(d, "kms", 1, "a1b2c3d4-5678-90ab-cdef-111111111111")

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("EBS-S04: Enter on ec2 row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("EBS-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "ec2" {
		t.Errorf("EBS-S04: RelatedNavigateMsg.TargetType = %q, want \"ec2\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// EBS-S05: Enter on count=0 row must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestEBS_Smoke_S05_EnterOnZeroRowNoNav(t *testing.T) {
	d := ebsSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("EBS-S05: right column not visible")
	}

	d = deliverEBSRelatedResult(d, "ec2", 0)
	d = deliverEBSRelatedResult(d, "ebs-snap", 0)
	d = deliverEBSRelatedResult(d, "kms", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("EBS-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// EBS-S06: All checkers non-nil; demo checker returns results
// ---------------------------------------------------------------------------

func TestEBS_Smoke_S06_CheckersAndDemoChecker(t *testing.T) {
	defs := resource.GetRelated("ebs")
	for _, def := range defs {
		if def.Checker == nil {
			t.Errorf("EBS-S06: checker for target %q is nil (stub); all ebs checkers should be non-nil", def.TargetType)
		}
	}

	checker := resource.GetRelatedDemo("ebs")
	if checker == nil {
		t.Fatal("EBS-S06: no demo checker registered for ebs")
	}
	results := checker(resource.Resource{ID: "vol-demo"})
	if len(results) == 0 {
		t.Fatal("EBS-S06: demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("EBS-S06: demo result has empty TargetType")
		}
	}
}
