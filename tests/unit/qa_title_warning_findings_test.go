// qa_title_warning_findings_test.go — RED regression test for the S1
// aggregation bug in Controller.listIssueCount (internal/app/list_body.go).
//
// Per docs/attention-signals.md §Visualization Surfaces (S1) and §S1 — list
// frame title issue count: "N uses the same aggregation as the menu badge:
// Wave 1 issue-colored rows plus Wave 2 `!`-severity findings for the
// resources in the list. `~` findings do not bump." "~" findings are
// glyphToSeverity("~") == domain.SevWarn (internal/aws/snapshot_cross_ref.go);
// "!" findings are glyphToSeverity("!") == domain.SevBroken
// (internal/aws/issue_enrichment.go setWave2Finding docstring).
//
// Root cause: listIssueCount's Wave-2 fallback branch only checks map
// membership —
//
//	} else if _, hasFinding := findings[r.ID]; hasFinding {
//	    ic++
//	}
//
// — with no severity gate. It counts ANY Wave-2 finding (SevWarn "~" included)
// as an issue, inflating both the list frame title's " !N" suffix and
// Controller.GetListIssueCount(). This file pins the correct behavior: only
// SevBroken ("!") Wave-2 findings on an otherwise-Healthy, no-Wave-1-finding
// resource should count.
//
// Harness style follows qa_controller_frame_title_issue_badge_test.go: drives
// the Controller directly via newListController + ApplyResourcesLoaded +
// ApplyEnrichmentState (the Wave-2 enrichment seam), asserting on
// Controller.ListFrameTitle() and Controller.GetListIssueCount().
package unit_test

import (
	"strconv"
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
)

// warnOnlyFindings builds a Wave-2 findings map where every resource ID in
// ids carries a SevWarn ("~", informational) finding — never SevBroken/SevOK.
func warnOnlyFindings(ids []string) map[string][]domain.Finding {
	findings := make(map[string][]domain.Finding, len(ids))
	for _, id := range ids {
		findings[id] = []domain.Finding{{
			Code:     "ec2.test.tildeOnly",
			Phrase:   "informational only",
			Severity: domain.SevWarn,
			Source:   "wave2:ec2",
		}}
	}
	return findings
}

// TestController_ListIssueCount_TildeOnlyFindings_NoSuffix is the RED test for
// case (a): a list whose rows are all Healthy (state="running", no Wave-1
// Findings) with ONLY "~"-severity (SevWarn) Wave-2 findings applied to every
// row. Per S1, "~" findings do not bump the count — the title must carry NO
// " !" suffix at all, and GetListIssueCount() must be 0.
//
// Currently RED: listIssueCount's Wave-2 fallback branch counts any map
// membership regardless of severity, so all 5 rows are (wrongly) counted as
// issues, producing " !5".
func TestController_ListIssueCount_TildeOnlyFindings_NoSuffix(t *testing.T) {
	c := newListController(t, "ec2")

	resources := controllerIssueResourcesWithState(5, 0, "running", "stopped")
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	ids := make([]string, len(resources))
	for i, r := range resources {
		ids[i] = r.ID
	}
	c.ApplyEnrichmentState("ec2", 0, false, warnOnlyFindings(ids), nil)

	if gotCount := c.GetListIssueCount(); gotCount != 0 {
		t.Errorf("GetListIssueCount() = %d, want 0 — SevWarn (\"~\") Wave-2 findings must NOT bump the issue count per docs/attention-signals.md S1", gotCount)
	}

	wantTitle := "ec2(5)"
	got := c.ListFrameTitle()
	if got != wantTitle {
		t.Errorf("ListFrameTitle() = %q, want %q — an all-\"~\"-findings list must render NO \" !N\" suffix", got, wantTitle)
	}
}

// TestController_ListIssueCount_MixedSeverityFindings_CountsBangOnly is the
// RED test for case (b): 2 rows carry a "!"-severity (SevBroken) Wave-2
// finding, 3 rows carry a "~"-severity (SevWarn) Wave-2 finding. All 5 rows
// are otherwise Healthy (state="running", no Wave-1 Findings). Only the 2
// "!"-finding rows should count — the suffix must be exactly " !2", not " !5".
func TestController_ListIssueCount_MixedSeverityFindings_CountsBangOnly(t *testing.T) {
	c := newListController(t, "ec2")

	resources := controllerIssueResourcesWithState(5, 0, "running", "stopped")
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	findings := make(map[string][]domain.Finding, len(resources))
	for i, r := range resources {
		if i < 2 {
			findings[r.ID] = []domain.Finding{{
				Code:     "ec2.test.bang",
				Phrase:   "impaired",
				Severity: domain.SevBroken,
				Source:   "wave2:ec2",
			}}
		} else {
			findings[r.ID] = []domain.Finding{{
				Code:     "ec2.test.tilde",
				Phrase:   "informational only",
				Severity: domain.SevWarn,
				Source:   "wave2:ec2",
			}}
		}
	}
	c.ApplyEnrichmentState("ec2", 2, false, findings, nil)

	if gotCount := c.GetListIssueCount(); gotCount != 2 {
		t.Errorf("GetListIssueCount() = %d, want 2 — only SevBroken (\"!\") Wave-2 findings should count; the 3 SevWarn (\"~\") findings must not", gotCount)
	}

	wantTitle := "ec2(5) !2"
	got := c.ListFrameTitle()
	if got != wantTitle {
		t.Errorf("ListFrameTitle() = %q, want %q — currently regresses to \" !5\" because listIssueCount counts any Wave-2 finding regardless of severity", got, wantTitle)
	}
}

// TestController_ListIssueCount_TitleSuffixParity_MatchesMenuAggregation pins
// case (c): the title's " !N" suffix uses the SAME aggregation as
// Controller.GetListIssueCount() — the documented single source of truth for
// both the menu badge and the list-title suffix (see the S1 contract comment
// at internal/app/list_body.go above buildListFrameTitle, and listIssueCount's
// own docstring referencing the menu sync-back). This guards against a future
// fix that repairs the title's inline computation but leaves
// GetListIssueCount (and therefore the menu badge) on the old, wrong
// aggregation, or vice versa.
func TestController_ListIssueCount_TitleSuffixParity_MatchesMenuAggregation(t *testing.T) {
	c := newListController(t, "ec2")

	resources := controllerIssueResourcesWithState(7, 0, "running", "stopped")
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	findings := make(map[string][]domain.Finding, len(resources))
	for i, r := range resources {
		switch {
		case i < 3:
			findings[r.ID] = []domain.Finding{{
				Code:     "ec2.test.bang",
				Phrase:   "impaired",
				Severity: domain.SevBroken,
				Source:   "wave2:ec2",
			}}
		case i < 6:
			findings[r.ID] = []domain.Finding{{
				Code:     "ec2.test.tilde",
				Phrase:   "informational only",
				Severity: domain.SevWarn,
				Source:   "wave2:ec2",
			}}
		}
	}
	c.ApplyEnrichmentState("ec2", 3, false, findings, nil)

	menuN := c.GetListIssueCount()
	wantTitle := "ec2(7) !" + itoaParity(menuN)
	got := c.ListFrameTitle()
	if got != wantTitle {
		t.Errorf("ListFrameTitle() = %q, want %q — the title's \" !N\" suffix must equal GetListIssueCount() (%d), the same aggregation that drives the menu badge", got, wantTitle, menuN)
	}
	if menuN != 3 {
		t.Errorf("GetListIssueCount() = %d, want 3 (only the SevBroken rows) — parity check itself requires the correct aggregation first", menuN)
	}
}

func itoaParity(n int) string {
	return strconv.Itoa(n)
}
