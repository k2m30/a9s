//go:build integration

package integration

import (
	"strconv"
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

// wipfixTitleBadge returns the issue count a frame title ends with ("… !6"),
// and whether it carries one at all.
func wipfixTitleBadge(t *testing.T, title string) (int, bool) {
	t.Helper()
	i := strings.LastIndex(title, " !")
	if i < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(title[i+2:]), "+"))
	if err != nil {
		t.Fatalf("frame title %q ends with an unparseable issue badge", title)
	}
	return n, true
}

// wipfixVisibleRowCount counts the rendered list rows between the column
// header and the frame's bottom edge.
func wipfixVisibleRowCount(s *fullIntegrationScenario) int {
	n, started := 0, false
	for _, line := range strings.Split(s.currentView(), "\n") {
		switch {
		case strings.Contains(line, "1:Name"):
			started = true
		case started && strings.HasPrefix(strings.TrimSpace(line), "│"):
			if strings.TrimSpace(strings.Trim(line, "│ ")) != "" {
				n++
			}
		}
	}
	return n
}

// TestScenario_CtrlROnRelatedDrill_BadgeCountsTheDrillsOwnRows pins row 38.
// The refresh reaches the drill now, and the drill's count and rows come from
// its own prefiltered set — but the issue badge is still counted over the
// account's whole list, so ten instances built from one AMI are titled with
// the eighteen issues of the forty-one the account has. The badge reads the
// same set the count and the rows read.
func TestScenario_CtrlROnRelatedDrill_BadgeCountsTheDrillsOwnRows(t *testing.T) {
	s := fullIntegrationNewDemoScenario(t)

	s.OpenList("ec2")
	canonicalTitle := wipfixDrillFrameTitle(t, s)
	canonicalBadge, ok := wipfixTitleBadge(t, canonicalTitle)
	if !ok {
		t.Fatalf("precondition: the canonical ec2 list carries no issue badge: %q", canonicalTitle)
	}
	s.Back()

	s.OpenList("ami")
	s.OpenDetailFromCurrentListByID("ami-0a1b2c3d4e5f60002")
	s.FollowRelated("EC2 Instances")

	before := wipfixDrillFrameTitle(t, s)
	beforeBadge, ok := wipfixTitleBadge(t, before)
	if !ok {
		t.Fatalf("precondition: the drill carries no issue badge: %q", before)
	}
	if beforeBadge == canonicalBadge {
		t.Fatalf("precondition: the drill's badge (%d) already equals the account's (%d) — "+
			"this scenario needs them to differ", beforeBadge, canonicalBadge)
	}
	rows := wipfixVisibleRowCount(s)

	s.Press("ctrl+r")

	after := wipfixDrillFrameTitle(t, s)
	afterBadge, ok := wipfixTitleBadge(t, after)
	if !ok {
		t.Fatalf("the drill lost its issue badge after Ctrl+R: %q", after)
	}

	// The refresh re-fetched the same demo fixtures onto the same ten rows, so
	// nothing the badge counts has changed.
	if afterBadge != beforeBadge {
		t.Errorf("after Ctrl+R the drill's badge is !%d, was !%d — the same ten rows are on "+
			"screen and the badge is the only thing that moved\n  before: %q\n  after:  %q",
			afterBadge, beforeBadge, before, after)
	}
	if afterBadge > rows {
		t.Errorf("the drill shows %d rows and titles !%d — a badge counting more issues than "+
			"there are rows is counting the account's list, not the drill's: %q",
			rows, afterBadge, after)
	}
	if !strings.Contains(after, "ec2(10)") || !strings.Contains(after, "ami-0a1b2c3d4e5f60002") {
		t.Errorf("after Ctrl+R the frame is %q, want the drill's own count and parent still there", after)
	}

	// The badge the drill must stop borrowing is still right where it belongs.
	s.Back()
	s.Back()
	s.Back()
	s.OpenList("ec2")
	if got := wipfixDrillFrameTitle(t, s); got != canonicalTitle {
		t.Errorf("the canonical ec2 list now titles %q, was %q — narrowing the drill's badge "+
			"must not narrow the account's", got, canonicalTitle)
	}
}
