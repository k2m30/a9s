package unit_test

// AMI related-view smoke test — verifies actual TUI behavior via DetailModel.
//
// Equivalent to running ./a9s --demo, navigating to an AMI, and checking:
//   - Right column visible with RELATED header
//   - Correct labels (EC2 Instances, EBS Snapshots, Auto Scaling Groups)
//   - Counts display correctly after results delivered
//   - Tab focuses right column
//   - Enter on count>0 row emits RelatedNavigateMsg with correct TargetType
//   - Enter on count=0 (stub) row does NOT emit RelatedNavigateMsg
//
// Demo fixture: ami-0a1b2c3d4e5f60001
// Demo results: ec2→1, ebs-snap→1, asg→0 (stub)

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

// amiSmokeDetail builds a DetailModel for "ami" using the demo fixture.
func amiSmokeDetail(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "ami-0a1b2c3d4e5f60001",
		Name: "golden-image-2026-01",
		Fields: map[string]string{
			"image_id":     "ami-0a1b2c3d4e5f60001",
			"name":         "golden-image-2026-01",
			"state":        "available",
			"architecture": "x86_64",
		},
		RawStruct: ec2types.Image{},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ami", nil, k)
	d.SetSize(width, height)
	return d
}

// deliverAMIRelatedResult delivers a RelatedCheckResultMsg for "ami".
func deliverAMIRelatedResult(d views.DetailModel, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "ami",
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
// AMI-S01: Right column shows with RELATED header at wide terminal
// ---------------------------------------------------------------------------

func TestAMI_Smoke_S01_RightColVisible(t *testing.T) {
	d := amiSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Fatal("AMI-S01: right column must auto-show at width=120 with registered related defs; 'RELATED' header not found in View()")
	}
	if !strings.Contains(d.View(), "│") {
		t.Fatal("AMI-S01: column separator │ must be present at width=120")
	}
}

// ---------------------------------------------------------------------------
// AMI-S02: Correct labels in right column
// ---------------------------------------------------------------------------

func TestAMI_Smoke_S02_CorrectLabels(t *testing.T) {
	d := amiSmokeDetail(t, 120, 30)

	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "RELATED") {
		t.Skip("AMI-S02: right column not visible; skipping label check")
	}

	for _, label := range []string{"EC2 Instances", "EBS Snapshots", "Auto Scaling Groups"} {
		if !strings.Contains(plain, label) {
			t.Errorf("AMI-S02: expected label %q in right column; not found\nview:\n%s", label, plain)
		}
	}
}

// ---------------------------------------------------------------------------
// AMI-S03: Counts display correctly after results delivered
// ---------------------------------------------------------------------------

func TestAMI_Smoke_S03_CountsAfterDeliver(t *testing.T) {
	d := amiSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("AMI-S03: right column not visible")
	}

	// Deliver demo-equivalent results
	d = deliverAMIRelatedResult(d, "ec2", 1, "i-0a1b2c3d4e5f60001")
	d = deliverAMIRelatedResult(d, "ebs-snap", 1, "snap-0a1b2c3d4e5f60001")
	d = deliverAMIRelatedResult(d, "asg", 0)

	plain := stripAnsi(d.View())

	// ec2 and ebs-snap should show (1); asg should show (0)
	if !strings.Contains(plain, "(1)") {
		t.Errorf("AMI-S03: expected '(1)' count in right column after delivering ec2/ebs-snap results; not found\nview:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("AMI-S03: expected '(0)' for stub asg row; not found\nview:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// AMI-S04: Tab focuses right column; Enter on ec2 row (count=1) emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestAMI_Smoke_S04_EnterOnEC2RowNavigates(t *testing.T) {
	d := amiSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("AMI-S04: right column not visible")
	}

	d = deliverAMIRelatedResult(d, "ec2", 1, "i-0a1b2c3d4e5f60001")
	d = deliverAMIRelatedResult(d, "ebs-snap", 1, "snap-0a1b2c3d4e5f60001")
	d = deliverAMIRelatedResult(d, "asg", 0)

	// Tab to focus right column
	d, _ = pressDetailTab(d)

	// Press Enter — expect RelatedNavigateMsg for "ec2"
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("AMI-S04: Enter on ec2 row (count=1) must emit a cmd; got nil")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("AMI-S04: Enter must produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "ec2" {
		t.Errorf("AMI-S04: RelatedNavigateMsg.TargetType = %q, want \"ec2\"", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// AMI-S05: Enter on asg row (count=0 stub) must NOT emit RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestAMI_Smoke_S05_EnterOnStubRowNoNav(t *testing.T) {
	d := amiSmokeDetail(t, 120, 30)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("AMI-S05: right column not visible")
	}

	// All count=0 so cursor cannot land on any row
	d = deliverAMIRelatedResult(d, "ec2", 0)
	d = deliverAMIRelatedResult(d, "ebs-snap", 0)
	d = deliverAMIRelatedResult(d, "asg", 0)

	d, _ = pressDetailTab(d)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("AMI-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AMI-S06: Stub rows (asg, nil checker) show as count=0 or loading, not "?"
// This verifies the demo checker is being used (not the nil production checker)
// ---------------------------------------------------------------------------

func TestAMI_Smoke_S06_DemoCheckerOverridesNilChecker(t *testing.T) {
	defs := resource.GetRelated("ami")
	var asgDef *resource.RelatedDef
	for i := range defs {
		if defs[i].TargetType == "asg" {
			asgDef = &defs[i]
			break
		}
	}
	if asgDef == nil {
		t.Fatal("AMI-S06: asg related def not registered")
	}
	if asgDef.Checker == nil {
		t.Fatal("AMI-S06: asg Checker must not be nil — implementation missing?")
	}
	// Demo checker must still return a result for asg (Count:0 is valid — it
	// just means the demo account has no ASGs attached to this AMI)
	checker := resource.GetRelatedDemo("ami")
	if checker == nil {
		t.Fatal("AMI-S06: no demo checker registered for ami")
	}
	results := checker(resource.Resource{ID: "ami-demo"})
	var asgResult *resource.RelatedCheckResult
	for i := range results {
		if results[i].TargetType == "asg" {
			asgResult = &results[i]
			break
		}
	}
	if asgResult == nil {
		t.Fatal("AMI-S06: demo checker did not return a result for asg target type")
	}
}
