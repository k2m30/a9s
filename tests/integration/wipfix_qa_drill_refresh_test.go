//go:build integration

package integration

import (
	"strings"
	"testing"
)

// wipfix_qa_drill_refresh_test.go drives row 29 through the terminal: Ctrl+R
// pressed on a client-side related drill. The drill holds the subset the
// related check resolved, not the type's whole population, so a refresh
// dispatched on the canonical lane either lands nowhere or replaces the
// drill's rows with the full list — both of which the operator reads as the
// drill having lied about what it found.

// wipfixDrillFrameTitle returns the framed title line of the current screen.
func wipfixDrillFrameTitle(t *testing.T, s *fullIntegrationScenario) string {
	t.Helper()
	for _, line := range strings.Split(s.currentView(), "\n") {
		if strings.Contains(line, "──") && strings.Contains(line, "(") {
			return strings.TrimSpace(strings.Trim(line, "┌┐│─ "))
		}
	}
	t.Fatalf("no framed title line in:\n%s", s.currentView())
	return ""
}

// TestScenario_CtrlROnRelatedDrill_KeepsTheDrillsOwnRows pins that a refresh
// on an AMI's EC2 drill leaves the drill showing the instances built from
// that AMI, never the account's whole EC2 list.
func TestScenario_CtrlROnRelatedDrill_KeepsTheDrillsOwnRows(t *testing.T) {
	s := fullIntegrationNewDemoScenario(t)

	// The canonical population, for comparison: whatever the drill shows must
	// not silently become this.
	s.OpenList("ec2")
	canonical := len(s.currentListResources)
	if canonical == 0 {
		t.Fatal("precondition: the demo ec2 list is empty")
	}
	s.Back()

	s.OpenList("ami")
	s.OpenDetailFromCurrentListByID("ami-0a1b2c3d4e5f60002")
	related, ok := s.lastRelatedByName["EC2 Instances"]
	if !ok {
		t.Fatal("precondition: the AMI detail has no EC2 Instances related row")
	}
	drillCount := related.Result.Count()
	if drillCount == 0 || drillCount == canonical {
		t.Fatalf("precondition: the drill holds %d rows and the canonical list %d — "+
			"this scenario needs them to differ", drillCount, canonical)
	}

	s.FollowRelated("EC2 Instances")
	before := wipfixDrillFrameTitle(t, s)
	if !strings.Contains(before, "ami-0a1b2c3d4e5f60002") {
		t.Fatalf("precondition: the drill frame does not name its parent AMI: %q", before)
	}

	s.Press("ctrl+r")
	after := wipfixDrillFrameTitle(t, s)

	if !strings.Contains(after, "ami-0a1b2c3d4e5f60002") {
		t.Errorf("after Ctrl+R the frame is %q — the refresh left the drill's own screen", after)
	}
	if !strings.Contains(after, "ec2(") {
		t.Errorf("after Ctrl+R the frame is %q, want an ec2 list", after)
	}
	if !strings.Contains(after, "ec2(10)") {
		t.Errorf("after Ctrl+R the drill titles %q — it held 10 instances built from this AMI "+
			"before the refresh, and the canonical ec2 population is %d", after, canonical)
	}
}
