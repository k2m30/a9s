package unit

// qa_issue_truncated_preserve_test.go — Tests that Wave 2 enrichment does NOT
// clear Wave 1's resource-count truncation signal when computing issueTruncated.
//
// Invariant: if Wave 1 set resource-count Truncated=true for a type, then
// issueTruncated for that type must remain true after Wave 2 completes, even
// when Wave 2 itself emits Truncated=false.
//
// Reference: app_handlers_navigate.go — handleEnrichmentChecked, lines:
//   "If resource count is already a lower bound (Wave 1 truncated), the
//    issue count is also a lower bound — preserve that signal even when
//    Wave 2 itself did not truncate."

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// newTestModelSized creates a fresh tui.Model with explicit terminal dimensions.
func newTestModelSized(t *testing.T, w, h int) tui.Model {
	t.Helper()
	m := tui.New("", "")
	m2, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	if tm, ok := m2.(tui.Model); ok {
		return tm
	}
	return m
}

// injectAvailabilityChecked sends an AvailabilityCheckedMsg to the model and
// returns the updated model. Gen=0 is accepted unconditionally (test-injection
// bypass, same pattern as handleAvailabilityChecked).
func injectAvailabilityChecked(m tui.Model, shortName string, count int, truncated bool, issues int) tui.Model {
	return injectAvailabilityCheckedWithResources(m, shortName, count, truncated, issues, nil)
}

// injectAvailabilityCheckedWithResources is like injectAvailabilityChecked but
// allows the test to supply a Resources slice. This is required when Wave 2
// will emit Findings keyed by IDs that must be recognised by unifiedIssueCount
// (which only counts findings whose ID matches a Wave 1 resource).
func injectAvailabilityCheckedWithResources(m tui.Model, shortName string, count int, truncated bool, issues int, resources []resource.Resource) tui.Model {
	msg := messages.AvailabilityCheckedMsg{
		ResourceType: shortName,
		HasResources: count > 0,
		Count:        count,
		Truncated:    truncated,
		Issues:       issues,
		Gen:          0, // zero = accepted unconditionally
		Resources:    resources,
	}
	m2, _ := m.Update(msg)
	if tm, ok := m2.(tui.Model); ok {
		return tm
	}
	return m
}

// injectEnrichmentChecked sends an EnrichmentCheckedMsg to the model and
// returns the updated model. Gen=0, TypeGen=0 are accepted unconditionally.
func injectEnrichmentChecked(m tui.Model, shortName string, issues int, truncated bool) tui.Model {
	return injectEnrichmentCheckedWithFindings(m, shortName, issues, truncated, map[string]resource.EnrichmentFinding{})
}

// injectEnrichmentCheckedWithFindings is like injectEnrichmentChecked but
// allows the test to supply Findings (needed for scenarios where the
// handler's "truncation-without-findings is spurious" guard would otherwise
// skip promotion).
func injectEnrichmentCheckedWithFindings(m tui.Model, shortName string, issues int, truncated bool, findings map[string]resource.EnrichmentFinding) tui.Model {
	msg := messages.EnrichmentCheckedMsg{
		ResourceType: shortName,
		Issues:       issues,
		Truncated:    truncated,
		Findings:     findings,
		Gen:          0,
		TypeGen:      0,
	}
	m2, _ := m.Update(msg)
	if tm, ok := m2.(tui.Model); ok {
		return tm
	}
	return m
}

// getMenuFromModel extracts the MainMenuModel from the first stack entry by
// updating with a no-op to trigger a View() cycle, then reads issueTruncated
// via the menu's GetIssueTruncated accessor.
//
// Because tui.Model does not expose the menu directly, we send a deliberate
// AvailabilityCheckedMsg with Gen=0 (accepted unconditionally) that does NOT
// change any state (unknown shortName) and then read the view output.
//
// For truncation state we use the public menu accessor via a menu-typed cast.
// Since tui.Model.Update returns tea.Model we pattern-match back to tui.Model,
// then call the exported View() to verify rendered output rather than inspecting
// internal state. To check issueTruncated we have a helper below.
func menuIssueTruncated(m tui.Model, shortName string) bool {
	// Send a no-op update to retrieve a typed model reference. We use
	// WindowSizeMsg because it is always handled without side effects when size
	// is unchanged.
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if tm, ok := m2.(tui.Model); ok {
		m = tm
	}
	// Render view and look for "issues:N+" which indicates issueTruncated=true
	// for any type. For the targeted shortName we need to check the menu badge.
	// We use a targeted AvailabilityCheckedMsg to sniff: inject an issue count
	// of 1 with truncated, then read the view for "issues:1+".
	// However, that would mutate state. Instead we query via the menu output.
	_ = m
	// This is a pure behavior test — we read the rendered view and assert on
	// the "issues:N+" rendered badge, which is what the user sees.
	return false // placeholder; replaced by view-based assertions in each test
}

// renderMenuView renders the full tui.Model view and returns the string
// representation (stripped of tea.View wrapper for plain comparison).
func renderMenuView(m tui.Model) string {
	v := m.View()
	return v.Content
}

// TestIssueTruncated_Wave2DoesNotOverrideWave1ResourceTruncation verifies that
// when Wave 1 emits Truncated=true for a resource type, that truncation signal
// is preserved in issueTruncated even after Wave 2 emits Truncated=false.
//
// Scenario: probe reports count=36, truncated=true, issues=1 (badge shows "issues:1+").
// Then Wave 2 enricher runs and emits Truncated=false.
// Expected: badge must still show "issues:1+" — the "+" must not disappear.
func TestIssueTruncated_Wave2DoesNotOverrideWave1ResourceTruncation(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	m := newTestModelSized(t, 120, 40)

	// Wave 1: probe result with resource-count truncated=true, 1 issue.
	// Seed Resources with an issue-colored EC2 row so that when Wave 2 below
	// recomputes via unifiedIssueCount, the 1-issue count is preserved (the
	// recomputation iterates wave1Resources and requires at least one IsIssue
	// row to match msg.Issues=1).
	wave1 := []resource.Resource{
		{
			ID:     "i-aaa0000000000001a",
			Name:   "impaired-ec2",
			Status: "stopped",
			Fields: map[string]string{"system_status": "impaired"},
		},
	}
	m = injectAvailabilityCheckedWithResources(m, "ec2", 36, true, 1, wave1)

	// Wave 2: enricher emits Truncated=false (no enricher-level truncation).
	// The resource-count truncation from Wave 1 must override this.
	m = injectEnrichmentChecked(m, "ec2", 1, false)

	// The rendered menu must show "issues:1+" — the "+" preserves the signal
	// that the resource count is a lower bound (hence issue count is too).
	view := renderMenuView(m)

	if !strings.Contains(view, "issues:1+") {
		t.Errorf("expected badge 'issues:1+' to appear in menu after Wave 2 with resource-truncated=true,"+
			" but view does not contain it.\nView:\n%s", view)
	}
}

// TestIssueTruncated_Wave2OverridesOnlyWhenWave1NotTruncated verifies the
// symmetric case: when Wave 1 resource-count was NOT truncated, Wave 2's
// own truncation still sets issueTruncated=true (the "+" appears) — provided
// Wave 2 actually produced a finding. A spurious Truncated=true with no
// Findings is deliberately NOT promoted (existing guard in the handler:
// "truncation signals count completeness, not hidden issues — if Wave 2 had
// seen one, it would have produced a Finding").
//
// Scenario: Wave 1 Truncated=false, issues=0.
// Wave 2 Truncated=true + 1 real finding.
// Expected: badge shows "issues:1+" (Wave 2 truncation respected).
func TestIssueTruncated_Wave2OverridesOnlyWhenWave1NotTruncated(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	m := newTestModelSized(t, 120, 40)

	// Wave 1: complete probe, not truncated, no issues. Seed Resources so that
	// unifiedIssueCount can match the Wave 2 finding below against a known ID.
	findingID := "i-aaa0000000000001a"
	wave1 := []resource.Resource{
		{ID: findingID, Name: findingID, Fields: map[string]string{}},
	}
	m = injectAvailabilityCheckedWithResources(m, "ec2", 5, false, 0, wave1)

	// Wave 2: enricher found 1 real finding AND was itself truncated.
	// A realistic scenario — enricher scans a capped window, emits findings for
	// what it saw, and flags more may exist beyond the window.
	findings := map[string]resource.EnrichmentFinding{
		findingID: {Severity: "!", Summary: "Impaired", Rows: nil},
	}
	m = injectEnrichmentCheckedWithFindings(m, "ec2", 1, true, findings)

	view := renderMenuView(m)

	// Wave 2 truncation must be respected: badge shows "issues:1+".
	if !strings.Contains(view, "issues:1+") {
		t.Errorf("expected badge 'issues:1+' when Wave 2 itself truncated,"+
			" got view:\n%s", view)
	}
}

// TestIssueBadge_ShowsPlusWhenResourceTruncated is a direct MainMenuModel test
// verifying that SetTruncated + SetIssues(n, true) renders "issues:N+" in View().
//
// This tests the rendering path independently of the tui.Model routing,
// confirming the menu's own issueBadge() method honours issueTruncated.
func TestIssueBadge_ShowsPlusWhenResourceTruncated(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Skip("no resource types registered")
	}
	shortName := allTypes[0].ShortName

	m := newSizedMainMenu(t, 120, 40)

	// Simulate: resource count is a lower bound (Wave 1 truncated).
	m.SetTruncated(shortName, true)
	// Simulate: SetIssues with truncated=true signals the count is a lower bound.
	m.SetIssues(shortName, 1, true)

	view := m.View()
	if !strings.Contains(view, "issues:1+") {
		t.Errorf("menu.View() must contain 'issues:1+' for %q with SetTruncated=true and SetIssues(1,true),"+
			" got:\n%s", shortName, view)
	}
}
