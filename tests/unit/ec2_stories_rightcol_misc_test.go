package unit_test

// ec2_stories_rightcol_misc_test.go — EC2 QA stories EC2-018 through EC2-057
// covering right column behavior, layout edge cases, and misc features.
//
// Stories tested here (no spec008 build tag — no FieldCursor() calls):
//   EC2-018: Right column visible by default on wide terminal
//   EC2-019: Right column rows start dim before results arrive
//   EC2-020: Right column rows light up as counts arrive
//   EC2-021: Tab moves focus to right column
//   EC2-022: Tab returns focus to left column
//   EC2-023: r key toggles right column off/on
//   EC2-024: h/l switches focus between columns (FAILS NOW — not implemented)
//   EC2-033: count=0 row — cursor skips over it
//   EC2-043: All count=0 — Enter has no effect in right column
//   EC2-044: Terminal too narrow — app shows "too narrow" message
//   EC2-045: Stacked layout at 80-99 columns (FAILS NOW — not implemented)
//   EC2-047: Esc from detail returns to EC2 list (not main menu)
//   EC2-049: Navigable fields work with right column hidden
//   EC2-050: Copy field value (FAILS NOW — CopyContent returns YAML, not field value)
//   EC2-051: Copy from right column (FAILS NOW — not implemented)
//   EC2-052: Ctrl+R in detail view refreshes and re-checks related (FAILS NOW)
//   EC2-055: Help screen shown via ? key
//   EC2-056: y key emits NavigateMsg to YAML view
//   EC2-057: PageUp/PageDown delegates to viewport

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

// ec2StoryDetail builds a DetailModel for "ec2" at the given width/height.
// If withDefs is true, registers two related defs so the right column auto-shows.
// Returns a cleanup func that unregisters the defs when withDefs is true.
func ec2StoryDetail(t *testing.T, width, height int, withDefs bool) (views.DetailModel, func()) {
	t.Helper()
	res := resource.Resource{
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "web-prod-01",
		Fields: map[string]string{
			"InstanceId":   "i-0a1b2c3d4e5f60001",
			"VpcId":        "vpc-0abc123def456789a",
			"SubnetId":     "subnet-0aaa111111111111a",
			"ImageId":      "ami-0abc123def456789a",
			"InstanceType": "t3.large",
			"State":        "running",
		},
	}
	origEC2Defs := resource.GetRelated("ec2")
	restoreEC2 := func() { resource.SetRelatedForTest("ec2", origEC2Defs) }
	if withDefs {
		resource.SetRelatedForTest("ec2", []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: noopChecker},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: noopChecker},
		})
	} else {
		resource.SetRelatedForTest("ec2", nil)
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(width, height)
	return d, restoreEC2
}

// deliverRelatedResult delivers a RelatedCheckResultMsg to a DetailModel.
func deliverRelatedResult(d views.DetailModel, targetType string, count int) views.DetailModel {
	msg := messages.RelatedCheckResult{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType: targetType,
			Count:      count,
		},
	}
	updated, _ := d.Update(msg)
	return updated
}

// pressDetailKey sends a single character keypress to a DetailModel.
func pressDetailKey(d views.DetailModel, ch string) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: -1, Text: ch})
}

// ---------------------------------------------------------------------------
// EC2-018: Right column visible by default on wide terminal
// ---------------------------------------------------------------------------
