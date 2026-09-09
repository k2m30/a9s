package unit

// qa_unified_issue_count_test.go — Tests for the unified issue count contract:
//
//  1. The menu count for a type after EnrichmentCheckedMsg equals the list's
//     FrameTitle count.
//  2. Wave-1/Wave-2 dedup and "~"-severity exclusion (unifiedIssueCount /
//     Controller.GetListIssueCount) are covered by
//     qa_issue_count_invariant_test.go (direct Controller assertions) and by
//     TestUnifiedIssueCount_IgnoresTildeSeverityFindings below (live
//     EnrichmentChecked → menu badge path).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: Menu count == list count after Wave 2 (EnrichmentCheckedMsg)
// ─────────────────────────────────────────────────────────────────────────────

// TestMenuCount_MatchesListCount_AfterWave2 verifies that after handleEnrichmentChecked
// processes an EnrichmentCheckedMsg, the issue count shown in the active
// ResourceListModel FrameTitle is consistent with what the menu shows for that type.
//
// This is R2: menu count == list count after Wave 2.
func TestMenuCount_MatchesListCount_AfterWave2(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	// Load EC2 resources: 2 running (no Wave-1 issues).
	resources := []resource.Resource{
		{ID: "i-0abc1111aaa111111", Name: "web-server-1",
			Fields: map[string]string{"state": "running"}},
		{ID: "i-0abc2222bbb222222", Name: "web-server-2",
			Fields: map[string]string{"state": "running"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources, Provenance: messages.FetchProvenanceCanonicalList,
	})

	// Deliver Wave-2 enrichment: 1 finding for the first instance.
	// Gen=0 and TypeGen=0 match a fresh model's initial generation counters.

	m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Truncated:    false,
		Findings: map[string][]domain.Finding{
			"i-0abc1111aaa111111": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
		Gen:     0,
		TypeGen: 0,
	})

	// The list FrameTitle should reflect enrichmentIssueCount=1 (Wave-2 count).
	// A bare Contains(listContent, "1") would pass even with the count
	// missing entirely — both loaded resource IDs already contain "1"
	// ("i-0abc1111aaa111111", "i-0abc2222bbb222222"). Assert the exact
	// frame-title token buildListFrameTitle produces for this input
	// (total=2, issueCount=1, no filter/attention/truncation): "ec2(2) !1".
	listContent := stripANSI(m.View().Content)
	if !strings.Contains(listContent, "ec2(2) !1") {
		t.Errorf("list view does not contain the exact frame title token \"ec2(2) !1\" after EnrichmentCheckedMsg; output:\n%s", listContent)
	}

	// Navigate back to the main menu via the actual back key (esc, not "q" —
	// "q" is bound to Quit in keys.Default(), so the previous "q" press left
	// the model on the SAME list screen; menuContent was byte-identical to
	// listContent, and the old Contains(menuContent, "1") check passed
	// vacuously against the still-visible list, never exercising the menu at
	// all).
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	menuContent := stripANSI(m.View().Content)
	if strings.Contains(menuContent, "ec2(2) !1") {
		t.Fatalf("precondition: still on the list screen after esc; menu content unexpectedly matches the list frame title:\n%s", menuContent)
	}

	// The menu badge for ec2 must show exactly "issues:1" on the EC2
	// Instances row, not just a bare "1" (which the row's alias/availability
	// digits could satisfy even with a wrong or missing count).
	ec2Line := findLineContaining(menuContent, "EC2 Instances")
	if !strings.Contains(ec2Line, "issues:1") {
		t.Errorf("EC2 Instances menu row does not show \"issues:1\"; got line:\n%s\nfull menu:\n%s", ec2Line, menuContent)
	}
	if strings.Contains(ec2Line, "issues:2") || strings.Contains(ec2Line, "issues:0") {
		t.Errorf("EC2 Instances menu row shows a wrong issue count, want exactly 1; got line:\n%s", ec2Line)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Target #4 — unifiedIssueCount must not count "~"-severity findings
//
// Only "!" findings bump the badge; "~" (informational) findings never
// contribute to the issue-ID set.
// ─────────────────────────────────────────────────────────────────────────────

// tildeSeverityEC2Instances returns 3 EC2 resources whose Color is Healthy
// (running state → ColorHealthy → !IsIssue). Used as Wave-1 baseline
// so any badge count must come from Wave-2 findings only.
func tildeSeverityEC2Instances() []resource.Resource {
	return []resource.Resource{
		{ID: "i-aaa", Name: "server-a",
			Fields: map[string]string{"name": "server-a", "state": "running"}},
		{ID: "i-bbb", Name: "server-b",
			Fields: map[string]string{"name": "server-b", "state": "running"}},
		{ID: "i-ccc", Name: "server-c",
			Fields: map[string]string{"name": "server-c", "state": "running"}},
	}
}

// TestUnifiedIssueCount_IgnoresTildeSeverityFindings verifies three cases:
//  1. One "!" finding + two "~" findings → unified count = 1 (only "!" counts).
//  2. Three "~" findings, no "!" → unified count = 0 (informational only, no badge).
//  3. One Wave-1 broken resource + one "~" finding on its ID → count = 1
//     (broken comes from Wave-1 IsIssue; "~" must not double-count or bump).
//
// Regression pin: unifiedIssueCount must ignore findings where Severity != "!".
// The menu issue badge (format " issues:N") must reflect only "!" findings.
//
// Navigation pattern: AvailabilityCheckedMsg seeds probeResources so that
// unifiedIssueCount has wave1Resources to work with; NavigateMsg pops back to
// the menu so that m.View() renders the menu (not the resource list).
func TestUnifiedIssueCount_IgnoresTildeSeverityFindings(t *testing.T) {
	tui.Version = "test"

	t.Run("one ! finding + two ~ findings → count=1 (only ! counts)", func(t *testing.T) {
		m := newRootSizedModel()

		// Use AvailabilityCheckedMsg stamped with the live AvailabilityGen
		// (session.New seeds it to 1) to seed probeResources["ec2"]
		// so unifiedIssueCount has wave1Resources.
		// All three resources are running → Wave-1 contributes 0 to issue IDs.
		resources := tildeSeverityEC2Instances()
		m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
			ResourceType: "ec2",
			Count:        3,
			Resources:    resources,
			Issues:       0,
			Gen:          m.Core().Session().AvailabilityGen,
		})

		m = navigateToEC2List(m)

		// EnrichmentCheckedMsg: unifiedIssueCount re-derives from Findings.
		// Bug (before fix): all 3 findings counted → issues:3.
		// Correct (after fix): only "!" findings → issues:1.
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: "ec2",
			Truncated:    false,
			Findings: map[string][]domain.Finding{
				"i-aaa": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
				"i-bbb": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance", Severity: domain.SevWarn, Source: "wave2:ec2"}},
				"i-ccc": {{Code: "ec2.instance.quota", Phrase: "quota 80%+ used", Severity: domain.SevWarn, Source: "wave2:ec2"}},
			},
			Gen:     0,
			TypeGen: 0,
		})

		// Pop back to the menu so m.View() renders the main menu.
		m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetMainMenu})
		menuContent := stripANSI(m.View().Content)

		// The menu badge format is " issues:N". Bug produces " issues:3".
		// After fix: " issues:1" (only the "!" finding counts).
		// A missing badge entirely (e.g. a regression that drops the "!"
		// finding too) must also fail — checking only the wrong-count
		// negatives would let that pass silently.
		if !strings.Contains(menuContent, " issues:1") {
			t.Errorf("menu does not show issues:1 — the single SevBroken finding must still count; output:\n%s", menuContent)
		}
		if strings.Contains(menuContent, " issues:3") {
			t.Errorf("menu shows issues:3, want issues:1 — ~ severity must not count; output:\n%s", menuContent)
		}
		if strings.Contains(menuContent, " issues:2") {
			t.Errorf("menu shows issues:2, want issues:1 — ~ severity must not count; output:\n%s", menuContent)
		}
	})

	t.Run("three ~ findings only → count=0 (no badge)", func(t *testing.T) {
		m := newRootSizedModel()

		resources := tildeSeverityEC2Instances()
		m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
			ResourceType: "ec2",
			Count:        3,
			Resources:    resources,
			Issues:       0,
			Gen:          m.Core().Session().AvailabilityGen,
		})

		m = navigateToEC2List(m)

		// All three findings are "~" (informational). unifiedIssueCount must return 0.
		// Bug (before fix): counts all 3 → issues:3.
		// Correct (after fix): no "!" findings → no badge.
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: "ec2",
			Truncated:    false,
			Findings: map[string][]domain.Finding{
				"i-aaa": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance", Severity: domain.SevWarn, Source: "wave2:ec2"}},
				"i-bbb": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance", Severity: domain.SevWarn, Source: "wave2:ec2"}},
				"i-ccc": {{Code: "ec2.instance.quota", Phrase: "quota 80%+ used", Severity: domain.SevWarn, Source: "wave2:ec2"}},
			},
			Gen:     0,
			TypeGen: 0,
		})

		m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetMainMenu})
		menuContent := stripANSI(m.View().Content)

		// No issue badge: " issues:" must not appear at all for ec2.
		// Bug produces " issues:3"; correct result has no badge (issueBadge returns "").
		if strings.Contains(menuContent, " issues:") {
			t.Errorf("menu shows issue badge, want none — all findings are ~ severity; output:\n%s", menuContent)
		}
	})

	t.Run("one Wave-1 broken + ~ finding on same ID → count=1 (no double-count)", func(t *testing.T) {
		m := newRootSizedModel()

		// colorEC2 is colorFromAnyFinding-only (core/aws/catalog_compute.go,
		// since the color-findings-conformance wave) — a raw "state":"stopped"
		// field alone no longer makes ResolveColor return ColorBroken; the
		// resource needs its own attached Wave-1 Finding (Source: "wave1") to
		// be genuinely issue-colored. Wave-1 contributes 1 to the issue count.
		brokenResource := resource.Resource{
			ID:     "i-stopped",
			Name:   "stopped-server",
			Fields: map[string]string{"name": "stopped-server", "state": "stopped"},
			Findings: []domain.Finding{
				{Code: "ec2.state.stopped", Phrase: "stopped", Severity: domain.SevBroken, Source: "wave1"},
			},
		}
		m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
			ResourceType: "ec2",
			Count:        1,
			Resources:    []resource.Resource{brokenResource},
			Issues:       1, // Wave-1 issue
			Gen:          m.Core().Session().AvailabilityGen,
		})

		m = navigateToEC2List(m)

		// Wave-2: a "~" finding on the same ID. Wave-1 already contributes count=1.
		// unifiedIssueCount must return 1 (dedup + no ~ bump).
		// Bug (before fix): "~" is counted, dedup collapses to 1 anyway — this subtest
		// catches the case where a DIFFERENT id has a "~" finding that inflates count.
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: "ec2",
			Truncated:    false,
			Findings: map[string][]domain.Finding{
				"i-stopped": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance", Severity: domain.SevWarn, Source: "wave2:ec2"}},
			},
			Gen:     0,
			TypeGen: 0,
		})

		m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetMainMenu})
		menuContent := stripANSI(m.View().Content)

		// Wave-1 broken resource contributes issues:1. ~ on same ID must not bump to 2.
		if !strings.Contains(menuContent, " issues:1") {
			t.Errorf("menu does not show issues:1 — the Wave-1 broken resource must still count; output:\n%s", menuContent)
		}
		if strings.Contains(menuContent, " issues:2") {
			t.Errorf("menu shows issues:2, want issues:1 — ~ on broken resource must not double-count; output:\n%s", menuContent)
		}
	})
}
