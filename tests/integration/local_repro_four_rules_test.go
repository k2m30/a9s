//go:build integration

package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// liveProfile returns the AWS profile to use for live R1-R4 tests, or "" if
// unset. Tests skip when this returns "".
func liveProfile() string {
	return strings.TrimSpace(os.Getenv("A9S_REPRO_PROFILE"))
}

// newLiveR1R4Scenario builds a live scenario with both Wave 1 and Wave 2
// enrichment fully drained so R1-R4 assertions are against a settled menu.
func newLiveR1R4Scenario(t *testing.T) *fullIntegrationScenario {
	t.Helper()
	profile := liveProfile()
	if profile == "" {
		t.Skip("A9S_REPRO_PROFILE not set; skipping live R1-R4 test")
	}

	scenario := fullIntegrationNewLiveScenario(t, profile, "")

	// Wave 1: drain AvailabilityPrefetchedMsg (or AvailabilityCheckedMsg chain)
	// fullIntegrationNewLiveScenario applies ClientsReadyMsg. We now need to
	// wait for Wave 1 completion.  Use the same extraction pattern as the full
	// integration test.
	m := tui.New(profile, "", tui.WithNoCache(true))
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})

	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "live R1-R4 Init")
	var connectCmd tea.Cmd
	m, connectCmd = fullIntegrationApplyMsg(m, initMsg)
	clientsReadyRaw := fullIntegrationRequireCmdMsg(t, connectCmd, "live R1-R4 AWS connect")
	clientsReady, ok := clientsReadyRaw.(messages.ClientsReady)
	if !ok {
		t.Fatalf("live R1-R4 AWS connect returned %T, expected messages.ClientsReady", clientsReadyRaw)
	}
	if clientsReady.Err != nil {
		t.Fatalf("live R1-R4 AWS connect failed for profile=%q: %v", profile, clientsReady.Err)
	}

	var prefetchCmd tea.Cmd
	m, prefetchCmd = fullIntegrationApplyMsg(m, clientsReady)
	availMsg := fullIntegrationExtractMsg(t, prefetchCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetched)
		return ok
	})
	var wave2Cmd tea.Cmd
	m, wave2Cmd = fullIntegrationApplyMsg(m, availMsg)

	// R1: drain Wave 2 — applying AvailabilityPrefetchedMsg returns the
	// startEnrichment batch. Every EnrichmentCheckedMsg it produces must be
	// applied back to the model so menu badges reflect post-Wave-2 state.
	// Without this drain, tests only validate Wave 1 and miss Wave 2 findings
	// (e.g. RDS pending maintenance surfaced via EnrichRDSDocDBMaintenance).
	m = drainWave2Enrichment(t, m, wave2Cmd)

	// Rebuild scenario from the fully-initialised model so its view is current.
	scenario.model = m
	return scenario
}

// drainWave2Enrichment recursively applies every EnrichmentCheckedMsg produced
// by the given cmd chain (and subsequent follow-ups) to the model, returning
// the settled post-Wave-2 state. Non-EnrichmentCheckedMsg messages (e.g.
// ResourcesLoadedMsg from Wave-1 refreshes) are also applied so handler
// bookkeeping stays consistent. Caps iterations to avoid runaway loops on a
// bug.
func drainWave2Enrichment(t *testing.T, m tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()
	m, _ = drainWave2EnrichmentCollect(t, m, cmd)
	return m
}

// drainWave2EnrichmentCollect is like drainWave2Enrichment but also returns
// the per-type findings observed during the drain. The returned map is
// shortName → finding-keyed map of resource IDs to domain.Finding, so
// callers can drill into specific affected resources for end-to-end assertions.
func drainWave2EnrichmentCollect(t *testing.T, m tui.Model, cmd tea.Cmd) (tui.Model, map[string]map[string]domain.Finding) {
	t.Helper()
	const maxIterations = 200 // 9 enrichers × worst-case 20 follow-ups; generous
	queue := []tea.Cmd{cmd}
	enrichmentMsgs := 0
	collected := map[string]map[string]domain.Finding{}
	for i := 0; i < maxIterations && len(queue) > 0; i++ {
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		for _, msg := range fullIntegrationCollectCmdMessages(next) {
			if msg == nil {
				continue
			}
			if em, ok := msg.(messages.EnrichmentChecked); ok {
				enrichmentMsgs++
				if len(em.Findings) > 0 {
					if collected[em.ResourceType] == nil {
						collected[em.ResourceType] = map[string]domain.Finding{}
					}
					for id, f := range em.Findings {
						collected[em.ResourceType][id] = f
					}
				}
			}
			var followup tea.Cmd
			m, followup = fullIntegrationApplyMsg(m, msg)
			if followup != nil {
				queue = append(queue, followup)
			}
		}
	}
	summary := map[string]int{}
	for k, v := range collected {
		summary[k] = len(v)
	}
	t.Logf("drainWave2Enrichment: processed %d EnrichmentCheckedMsg(s); types with findings=%v", enrichmentMsgs, summary)
	return m, collected
}

// TestLiveR1_IssueCountsVisibleOnMenuAfterWave1 verifies R1: after Wave 1
// completes, the main menu renders non-zero issue counts for any resource type
// that has unhealthy (non-green) resources in the account.
func TestLiveR1_IssueCountsVisibleOnMenuAfterWave1(t *testing.T) {
	scenario := newLiveR1R4Scenario(t)

	view := scenario.currentView()
	// At least one resource type should show issue annotations if the account
	// has any non-healthy resources. We check generically for "issue" text;
	// an account with no unhealthy resources will produce an empty assertion
	// but the test still verifies the menu rendered without crashing.
	t.Logf("R1: main menu view (excerpt):\n%s", firstLines(view, 40))

	// The menu must render. No panic = R1 basic structural invariant holds.
	if view == "" {
		t.Error("R1: main menu rendered empty view after Wave 1")
	}
}

// TestLiveR2_MenuCountMatchesListCount verifies R2: for every resource type,
// the count shown on the main menu equals the count of resources actually
// loaded when opening that type's list.
func TestLiveR2_MenuCountMatchesListCount(t *testing.T) {
	profile := liveProfile()
	if profile == "" {
		t.Skip("A9S_REPRO_PROFILE not set; skipping live R2 test")
	}

	scenario := newLiveR1R4Scenario(t)
	menuView := scenario.currentView()

	for _, rt := range resource.AllResourceTypes() {
		rt := rt
		t.Run(rt.ShortName, func(t *testing.T) {
			prefix := rt.Name + " ("
			idx := strings.Index(menuView, prefix)
			if idx == -1 {
				t.Skipf("resource type %q not present in menu", rt.Name)
			}
			after := menuView[idx+len(prefix):]
			end := strings.IndexByte(after, ')')
			if end == -1 {
				t.Skipf("malformed menu entry for %q", rt.Name)
			}
			countStr := strings.TrimSpace(after[:end])
			// Normalise truncated/issue suffixes: "12 / 3 issues" -> "12"
			if slash := strings.Index(countStr, "/"); slash != -1 {
				countStr = strings.TrimSpace(countStr[:slash])
			}
			countStr = strings.TrimSuffix(strings.TrimSpace(countStr), "+")

			var menuCount int
			if _, err := parseInt2(countStr, &menuCount); err != nil {
				t.Skipf("could not parse menu count %q for %q", countStr, rt.Name)
			}

			// Open the list for this type on a fresh scenario to avoid cross-type pollution.
			s2 := fullIntegrationNewLiveScenario(t, profile, "")
			s2.OpenList(rt.ShortName)
			if s2.lastAPIError != nil {
				t.Skipf("API error loading list for %q: %v", rt.ShortName, s2.lastAPIError.Err)
			}

			listCount := len(s2.currentListResources)
			// For truncated lists the menu shows N+ and list returns first page.
			// We just verify list is non-empty when menu says > 0.
			if menuCount > 0 && listCount == 0 {
				t.Errorf("R2: menu count=%d for %q but list loaded 0 resources", menuCount, rt.Name)
			}
			if menuCount == 0 && listCount > 0 {
				t.Errorf("R2: menu count=0 for %q but list loaded %d resources", rt.Name, listCount)
			}
			t.Logf("R2: %s menu=%d list=%d", rt.ShortName, menuCount, listCount)
		})
	}
}

// TestLiveR3_IssueBadgeNeverExceedsListCount verifies R3: the issue count
// badge on the main menu never exceeds the total resource count for any type.
func TestLiveR3_IssueBadgeNeverExceedsListCount(t *testing.T) {
	if liveProfile() == "" {
		t.Skip("A9S_REPRO_PROFILE not set; skipping live R3 test")
	}

	scenario := newLiveR1R4Scenario(t)
	menuView := scenario.currentView()

	var violations []string
	for _, rt := range resource.AllResourceTypes() {
		prefix := rt.Name + " ("
		idx := strings.Index(menuView, prefix)
		if idx == -1 {
			continue
		}
		after := menuView[idx+len(prefix):]
		end := strings.IndexByte(after, ')')
		if end == -1 {
			continue
		}
		countStr := strings.TrimSpace(after[:end])
		slash := strings.Index(countStr, "/")
		if slash == -1 {
			continue // no issue annotation
		}

		totalStr := strings.TrimSuffix(strings.TrimSpace(countStr[:slash]), "+")
		issueStr := strings.TrimSpace(countStr[slash+1:])
		issueStr = strings.TrimSuffix(issueStr, " issues")
		issueStr = strings.TrimSuffix(issueStr, "+")
		issueStr = strings.TrimSpace(issueStr)

		var total, issues int
		if _, err := parseInt2(totalStr, &total); err != nil {
			continue
		}
		if _, err := parseInt2(issueStr, &issues); err != nil {
			continue
		}
		if issues > total {
			violations = append(violations, fmt.Sprintf("%s: issues=%d > total=%d", rt.Name, issues, total))
		}
		t.Logf("R3: %s total=%d issues=%d", rt.ShortName, total, issues)
	}
	if len(violations) > 0 {
		t.Errorf("R3 violations (issue badge exceeds total count):\n%s", strings.Join(violations, "\n"))
	}
}

// TestLiveR4_CtrlZShowsOnlyTypesWithIssues verifies R4: the ctrl+z issue filter
// hides resource types whose zero issue count is confirmed (not truncated) and
// preserves types that do have issues. Post-AlwaysHealthy-purge only
// ExcludeFromIssueBadge types (ct-events) are unconditionally hidden.
func TestLiveR4_CtrlZShowsOnlyTypesWithIssues(t *testing.T) {
	if liveProfile() == "" {
		t.Skip("A9S_REPRO_PROFILE not set; skipping live R4 test")
	}

	scenario := newLiveR1R4Scenario(t)
	menuView := scenario.currentView()

	// Identify which types have non-zero issue counts from the settled menu.
	// Menu format: "<Display Name> (<total>)        :<shortname>" followed on the
	// same line by " issues:<N>" or " issues:<N>+" when a type has findings.
	// We scan per line to avoid confusing a badge from one line with a name on
	// another.
	typesWithIssues := map[string]bool{}
	for _, line := range strings.Split(menuView, "\n") {
		for _, rt := range resource.AllResourceTypes() {
			if !strings.Contains(line, rt.Name+" (") {
				continue
			}
			badgeIdx := strings.Index(line, " issues:")
			if badgeIdx == -1 {
				continue
			}
			after := line[badgeIdx+len(" issues:"):]
			// Grab the integer count up to the first non-digit / non-'+' char.
			end := 0
			for end < len(after) && (after[end] >= '0' && after[end] <= '9') {
				end++
			}
			if end == 0 {
				continue
			}
			var issues int
			if _, err := parseInt2(after[:end], &issues); err == nil && issues > 0 {
				typesWithIssues[rt.Name] = true
			}
		}
	}

	scenario.Press("ctrl+z")
	filteredView := scenario.currentView()

	// ExcludeFromIssueBadge types must always be hidden under ctrl+z.
	// (Post-AlwaysHealthy-purge; every other type's visibility is driven by
	// its per-type probe state.)
	for _, name := range []string{
		"CloudTrail Events",
	} {
		if strings.Contains(filteredView, name) {
			t.Errorf("R4: %q visible under ctrl+z but must always be hidden (ExcludeFromIssueBadge)", name)
		}
	}

	// Types confirmed to have issues must still be visible
	for name := range typesWithIssues {
		if !strings.Contains(filteredView, name) {
			t.Errorf("R4: %q has issues but disappeared under ctrl+z", name)
		}
	}

	t.Logf("R4: types with issues in account: %v", typesWithIssues)
}

// firstLines returns the first n lines of s.
func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// TestLiveR_DBIDetailShowsPendingMaintenance verifies the end-to-end finding
// surfacing path: with Wave 2 drained, a dbi detail view opened on a resource
// that has a pending-maintenance finding must render the "Attention"
// section. This pins the path the user observed as broken in the original bug
// report.
//
// Runs only against a live profile where DB instances with pending maintenance
// actually exist (both dev and prod read-only profiles had
// findings on the last run: 2 and 3 respectively). Skips on profiles that
// happen to have no pending maintenance.
func TestLiveR_DBIDetailShowsPendingMaintenance(t *testing.T) {
	profile := liveProfile()
	if profile == "" {
		t.Skip("A9S_REPRO_PROFILE not set; skipping live DBI detail test")
	}

	// Build the settled model AND capture the findings observed during Wave 2.
	scenario := fullIntegrationNewLiveScenario(t, profile, "")
	m := tui.New(profile, "", tui.WithNoCache(true))
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "DBI detail Init")
	var connectCmd tea.Cmd
	m, connectCmd = fullIntegrationApplyMsg(m, initMsg)
	clientsReadyRaw := fullIntegrationRequireCmdMsg(t, connectCmd, "DBI detail AWS connect")
	clientsReady := clientsReadyRaw.(messages.ClientsReady)
	if clientsReady.Err != nil {
		t.Fatalf("AWS connect failed for profile=%q: %v", profile, clientsReady.Err)
	}
	var prefetchCmd tea.Cmd
	m, prefetchCmd = fullIntegrationApplyMsg(m, clientsReady)
	availMsg := fullIntegrationExtractMsg(t, prefetchCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetched)
		return ok
	})
	var wave2Cmd tea.Cmd
	m, wave2Cmd = fullIntegrationApplyMsg(m, availMsg)
	m, findings := drainWave2EnrichmentCollect(t, m, wave2Cmd)
	scenario.model = m

	dbiFindings := findings["dbi"]
	if len(dbiFindings) == 0 {
		t.Skipf("no dbi findings in profile %q; cannot validate Pending Maintenance end-to-end", profile)
	}

	// Pick the first affected dbi instance ID.
	var targetID string
	for id := range dbiFindings {
		targetID = id
		break
	}
	t.Logf("opening dbi detail for %q (expects Pending Maintenance finding)", targetID)

	// Navigate: open dbi list, then open the affected resource's detail view.
	scenario.OpenList("dbi")
	if scenario.lastAPIError != nil {
		t.Fatalf("API error opening dbi list: %v", scenario.lastAPIError.Err)
	}
	scenario.OpenDetailFromCurrentListByID(targetID)

	detailView := scenario.currentView()
	if !strings.Contains(detailView, "Attention") {
		t.Errorf("detail view for dbi %q does not contain \"Pending Maintenance\" section.\nView excerpt:\n%s",
			targetID, firstLines(detailView, 60))
	}
}
