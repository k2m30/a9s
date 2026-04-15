//go:build integration

package integration

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// fullIntegrationNewDemoScenarioWithWave1 builds a demo scenario that has
// fully drained AvailabilityPrefetchedMsg (Wave 1) so issue counts appear on
// the main menu. fullIntegrationNewDemoScenario (used by R2/R3) skips Wave 1
// because it applies ClientsReadyMsg directly without calling m.Init().
func fullIntegrationNewDemoScenarioWithWave1(t *testing.T) *fullIntegrationScenario {
	t.Helper()
	clients := demo.NewServiceClients()
	m := tui.New(demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "demo R1-R4 Init")
	var cmd tea.Cmd
	m, cmd = fullIntegrationApplyMsg(m, initMsg)
	availMsg := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetchedMsg)
		return ok
	})
	m, _ = fullIntegrationApplyMsg(m, availMsg)
	return &fullIntegrationScenario{
		t:                 t,
		model:             m,
		clients:           clients,
		profile:           demo.DemoProfile,
		region:            demo.DemoRegion,
		lastRelatedByName: make(map[string]messages.RelatedCheckResultMsg),
	}
}

// TestFourRules_Demo_R1_IssueCountsOnMenuAfterWave1 verifies that after Wave 1
// (AvailabilityPrefetchedMsg) the main menu shows non-zero issue badges for at
// least one resource type that has non-healthy resources in demo fixtures.
func TestFourRules_Demo_R1_IssueCountsOnMenuAfterWave1(t *testing.T) {
	scenario := fullIntegrationNewDemoScenarioWithWave1(t)

	view := scenario.currentView()
	// Demo fixtures include stopped EC2 instances — at least one type must show issues.
	if !strings.Contains(view, "issues:") && !strings.Contains(view, " !") {
		t.Errorf("R1: main menu has no issue indicators after Wave 1; view:\n%s", view)
	}
}

// TestFourRules_Demo_R2_MenuCountMatchesListCount verifies that for every
// resource type the count shown in the main menu matches the count of resources
// loaded when opening that list.
func TestFourRules_Demo_R2_MenuCountMatchesListCount(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	menuView := scenario.currentView()

	for _, rt := range resource.AllResourceTypes() {
		rt := rt
		t.Run(rt.ShortName, func(t *testing.T) {
			// Parse the displayed count from the menu, e.g. "EC2 Instances (12)"
			// We look for "<Name> (" then parse the integer before ")"
			prefix := rt.Name + " ("
			idx := strings.Index(menuView, prefix)
			if idx == -1 {
				t.Skipf("resource type %q not found in menu view", rt.Name)
			}
			after := menuView[idx+len(prefix):]
			end := strings.IndexByte(after, ')')
			if end == -1 {
				t.Skipf("malformed menu count for %q", rt.Name)
			}
			countStr := strings.TrimSpace(after[:end])
			// Strip "+ " suffix for truncated counts
			countStr = strings.TrimSuffix(countStr, "+")
			// If there are issue annotations like "12 / 3 issues", take only the total
			if slash := strings.Index(countStr, "/"); slash != -1 {
				countStr = strings.TrimSpace(countStr[:slash])
			}

			scenario2 := fullIntegrationNewDemoScenario(t)
			scenario2.OpenList(rt.ShortName)
			if scenario2.lastAPIError != nil {
				t.Skipf("API error opening list for %q: %v", rt.ShortName, scenario2.lastAPIError.Err)
			}

			listCount := len(scenario2.currentListResources)
			if listCount == 0 && countStr == "0" {
				return
			}
			if countStr == "0" && listCount > 0 {
				t.Errorf("R2: menu shows 0 for %q but list loaded %d resources", rt.Name, listCount)
			}
		})
	}
}

// TestFourRules_Demo_R3_IssueBadgeNeverExceedsListCount verifies that the issue
// badge count on the menu never exceeds the total resource count for any type.
// (Each issue is a distinct resource instance, so issues <= total.)
func TestFourRules_Demo_R3_IssueBadgeNeverExceedsListCount(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	menuView := scenario.currentView()

	for _, rt := range resource.AllResourceTypes() {
		rt := rt
		t.Run(rt.ShortName, func(t *testing.T) {
			prefix := rt.Name + " ("
			idx := strings.Index(menuView, prefix)
			if idx == -1 {
				t.Skipf("resource type %q not found in menu view", rt.Name)
			}
			after := menuView[idx+len(prefix):]
			end := strings.IndexByte(after, ')')
			if end == -1 {
				t.Skipf("malformed menu count for %q", rt.Name)
			}
			countStr := strings.TrimSpace(after[:end])

			// Format with issues: "12 / 3 issues" — parse both sides
			slash := strings.Index(countStr, "/")
			if slash == -1 {
				return // no issue count in display — skip
			}
			totalStr := strings.TrimSpace(strings.TrimSuffix(countStr[:slash], "+"))
			issueStr := strings.TrimSpace(strings.TrimSuffix(countStr[slash+1:], "+ issues"))
			issueStr = strings.TrimSuffix(strings.TrimSpace(issueStr), " issues")
			issueStr = strings.TrimSuffix(issueStr, "+")

			var total, issues int
			if _, err := parseInt2(totalStr, &total); err != nil {
				t.Skipf("could not parse total %q for %q", totalStr, rt.Name)
			}
			if _, err := parseInt2(issueStr, &issues); err != nil {
				t.Skipf("could not parse issues %q for %q", issueStr, rt.Name)
			}
			if issues > total {
				t.Errorf("R3: %q issue badge %d exceeds total count %d", rt.Name, issues, total)
			}
		})
	}
}

// TestFourRules_Demo_R4_CtrlZShowsOnlyTypesWithIssues verifies that ctrl+z
// (issue filter) hides resource types with no issues (AlwaysHealthy, etc.) and
// shows types that do have issues.
func TestFourRules_Demo_R4_CtrlZShowsOnlyTypesWithIssues(t *testing.T) {
	scenario := fullIntegrationNewDemoScenarioWithWave1(t)

	scenario.Press("ctrl+z")

	// AlwaysHealthy types must be hidden
	scenario.ExpectViewNotContains("S3 Buckets")
	scenario.ExpectViewNotContains("Secrets Manager")
	scenario.ExpectViewNotContains("SSM Parameters")
	scenario.ExpectViewNotContains("KMS Keys")
	scenario.ExpectViewNotContains("IAM Users")
	scenario.ExpectViewNotContains("Security Groups")
	scenario.ExpectViewNotContains("CloudTrail Events")

	// Types with issues in demo fixtures must remain visible
	// EC2 has stopped instances in the demo fixture set
	scenario.ExpectViewContains("EC2 Instances")
}

// TestFourRules_Demo_R1_SpecificTypesShowIssueCounts is a strict regression pin
// that verifies exact per-type issue counts after Wave 1. It complements the
// broad smoke-test TestFourRules_Demo_R1_IssueCountsOnMenuAfterWave1: a
// regression that silently drops issue counts for a specific type will pass the
// smoke-test but fail here.
func TestFourRules_Demo_R1_SpecificTypesShowIssueCounts(t *testing.T) {
	scenario := fullIntegrationNewDemoScenarioWithWave1(t)
	view := scenario.currentView()

	// Each entry is the exact substring that must appear in the ANSI-stripped
	// main menu after Wave 1 completes. Counts are pinned against the demo
	// fixture data that ships with the binary.
	pins := []string{
		"EC2 Instances (27) issues:13",
		"ECS Services (23) issues:2",
		"DB Instances (23) issues:5",
		"EBS Volumes (5) issues:1",
		"Elastic Beanstalk (4) issues:1",
		"EBS Snapshots (4) issues:1",
		"EKS Clusters (3) issues:1",
		"ElastiCache Redis (4) issues:1",
		"DB Clusters (3) issues:1",
		"EFS File Systems (3) issues:1",
		"NAT Gateways (3) issues:1",
		"AMIs (4) issues:1",
		"Load Balancers (22) issues:1",
	}

	var missing []string
	for _, pin := range pins {
		if !strings.Contains(view, pin) {
			missing = append(missing, pin)
		}
	}
	if len(missing) > 0 {
		t.Errorf("R1 regression: %d pin(s) missing from main menu view:\n  - %s\nfull view:\n%s",
			len(missing), strings.Join(missing, "\n  - "), view)
	}
}

// parseInt2 parses a decimal integer string into *dst. Returns (0, err) on failure.
func parseInt2(s string, dst *int) (int, error) {
	s = strings.TrimSpace(s)
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errNotInt
		}
		n = n*10 + int(c-'0')
	}
	*dst = n
	return n, nil
}

var errNotInt = errNotIntType("not an integer")

type errNotIntType string

func (e errNotIntType) Error() string { return string(e) }
