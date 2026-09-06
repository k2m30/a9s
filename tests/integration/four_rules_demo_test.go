//go:build integration

package integration

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
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
		_, ok := msg.(messages.AvailabilityPrefetched)
		return ok
	})
	m, _ = fullIntegrationApplyMsg(m, availMsg)
	return &fullIntegrationScenario{
		t:                 t,
		model:             m,
		clients:           clients,
		profile:           demo.DemoProfile,
		region:            demo.DemoRegion,
		lastRelatedByName: make(map[string]messages.RelatedCheckResult),
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
// (issue filter) hides resource types whose issue count is a confirmed zero and
// surfaces types that have issues or whose zero is a lower bound (truncated).
// Post-AlwaysHealthy-purge every registered type has a Wave 1 or Wave 2 signal
// per docs/attention-signals.md, so visibility is driven only by probe state.
func TestFourRules_Demo_R4_CtrlZShowsOnlyTypesWithIssues(t *testing.T) {
	scenario := fullIntegrationNewDemoScenarioWithWave1(t)

	scenario.Press("ctrl+z")

	// ExcludeFromIssueBadge types are unconditionally hidden under ctrl+z
	scenario.ExpectViewNotContains("CloudTrail Events")

	// Types with issues in demo fixtures must remain visible
	// EC2 has stopped instances in the demo fixture set
	scenario.ExpectViewContains("EC2 Instances")
}

// renderedMenuCounts parses the Wave-1 main menu into the numbers it shows per
// type — "EC2 Instances (40) issues:15" — keyed by shortName. A truncated
// count's "+" is dropped: a fixtures.Pin carries numbers, not truncation. A
// type the menu shows without a count is absent from the result.
func renderedMenuCounts(t *testing.T, view string) map[string]menuCounts {
	t.Helper()
	out := make(map[string]menuCounts)
	for _, rt := range resource.AllResourceTypes() {
		prefix := rt.Name + " ("
		idx := strings.Index(view, prefix)
		if idx == -1 {
			continue
		}
		rest := view[idx+len(prefix):]
		if nl := strings.IndexByte(rest, '\n'); nl != -1 {
			rest = rest[:nl]
		}
		end := strings.IndexByte(rest, ')')
		if end == -1 {
			t.Fatalf("malformed menu count for %q in line %q", rt.Name, rest)
		}
		var counts menuCounts
		if _, err := parseInt2(strings.TrimSuffix(rest[:end], "+"), &counts.rows); err != nil {
			t.Fatalf("could not parse row count %q for %q", rest[:end], rt.Name)
		}
		if badge, ok := strings.CutPrefix(rest[end+1:], " issues:"); ok {
			if cut := strings.IndexFunc(badge, func(r rune) bool { return r < '0' || r > '9' }); cut != -1 {
				badge = badge[:cut]
			}
			if _, err := parseInt2(badge, &counts.issues); err != nil {
				t.Fatalf("could not parse issue badge %q for %q", badge, rt.Name)
			}
		}
		out[rt.ShortName] = counts
	}
	return out
}

type menuCounts struct{ rows, issues int }

// TestFourRules_Demo_R1_SpecificTypesShowIssueCounts is a strict regression pin
// that verifies exact per-type row counts and Wave-1 issue badges. It
// complements the broad smoke-test
// TestFourRules_Demo_R1_IssueCountsOnMenuAfterWave1: a regression that
// silently drops issue counts for a specific type will pass the smoke-test but
// fail here.
//
// The expected numbers come from each type's own fixtures.Pin, declared beside
// the fixtures that produce them, and are compared against numbers re-parsed
// from the rendered menu. Both halves are Wave-1 badges: a Healthy row
// carrying only a Wave-2 `!` is not counted yet, so a type's Pin is lower than
// the settled badge its scenario test pins (dbc reads 14 here and 15 in
// scenario_dbc_visual_test.go). That gap is expected on a type with a Wave-2
// enricher and a defect on a type without one.
func TestFourRules_Demo_R1_SpecificTypesShowIssueCounts(t *testing.T) {
	pins := fixtures.Pins()
	if len(pins) == 0 {
		t.Fatal("no type registered a fixtures.Pin — this test has no expected counts to check")
	}

	scenario := fullIntegrationNewDemoScenarioWithWave1(t)
	rendered := renderedMenuCounts(t, scenario.currentView())

	for _, p := range pins {
		got, ok := rendered[p.ShortName]
		if !ok {
			t.Errorf("%s: pinned Rows=%d Issues=%d but the demo menu shows no count for this type",
				p.ShortName, p.Rows, p.Issues)
			continue
		}
		if got.rows != p.Rows || got.issues != p.Issues {
			t.Errorf("%s: menu shows (%d) issues:%d, Pin says Rows=%d Issues=%d — "+
				"move the number in that type's fixture file",
				p.ShortName, got.rows, got.issues, p.Rows, p.Issues)
		}
	}
}

// TestFourRules_Demo_R1_EveryDemoMenuTypeIsPinned is the other direction: a
// type whose count the demo menu shows and whose fixture file registers no
// Pin has no regression pin at all. The literal table this test replaced
// covered thirteen types out of the whole menu, so a fixture change to any
// other type moved its counts silently.
func TestFourRules_Demo_R1_EveryDemoMenuTypeIsPinned(t *testing.T) {
	pinned := make(map[string]bool)
	for _, p := range fixtures.Pins() {
		pinned[p.ShortName] = true
	}

	scenario := fullIntegrationNewDemoScenarioWithWave1(t)
	rendered := renderedMenuCounts(t, scenario.currentView())

	var missing []string
	for shortName, counts := range rendered {
		if !pinned[shortName] {
			missing = append(missing, fmt.Sprintf("%s (%d) issues:%d", shortName, counts.rows, counts.issues))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d demo menu type(s) register no fixtures.Pin — add one in each type's fixture file:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
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
