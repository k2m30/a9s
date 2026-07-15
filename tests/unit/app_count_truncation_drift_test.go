// app_count_truncation_drift_test.go — RED pin for a live cross-surface
// defect: the s3 menu badge shows "issues:5" (exact) while the s3 list's own
// title suffix shows "!5+" (truncated) after a Wave-2 enrichment result is
// followed by a rows-derived resync.
//
// Root cause: syncMenuIssueCount's truncation-clear arm
// (`case newIssues == curIssues && curIssueTrunc && !newTrunc`) fires
// whenever an equal-count, untruncated observation lands — but
// syncExactTotalToMenu (internal/app/handle.go) computes newTrunc from
// `ls.HasPagination`, which reflects the LIST'S OWN fetch pagination, not
// whether the enrichment scan that produced newIssues (via
// c.listIssueCount, internal/app/list_body.go) covered every row. A list
// can hold an exact (untruncated) page of 55 rows while the type's Wave-2
// enrichment cap (e.g. 50 rows scanned) only confirmed issues among the
// first 50 — the exact row-fetch says "this IS the whole list" but says
// nothing about whether the issue COUNT among those rows is itself
// complete. The clear arm conflates the two and wrongly strips
// IssueTruncated the moment any equal-count, fully-fetched list syncs back,
// even though the enrichment result that seeded curIssues/curIssueTrunc
// never claimed completeness beyond its own cap.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// newCountTruncationDriftController builds a Controller + its backing Core,
// mirroring newEnrichmentMenuBadgeController in
// app_enrichment_menu_badge_test.go — duplicated here per that file's own
// stated precedent (no cross-file coupling to another test file's helper
// lifetime).
func newCountTruncationDriftController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	return c
}

// s3BrokenFindings builds n Wave-1-shaped (non-wave2-sourced), SevBroken
// findings for distinct fake S3 bucket IDs — matching the shape
// listHasBadgeFinding (internal/app/list_body.go) counts unconditionally at
// any issue severity, so each row bumps listIssueCount by exactly one.
func s3BrokenFindings(n int) []domain.Finding {
	out := make([]domain.Finding, n)
	for i := range out {
		out[i] = domain.Finding{
			Code:     "s3.public_access",
			Phrase:   "public access not blocked",
			Severity: domain.SevBroken,
			Source:   "wave1",
		}
	}
	return out
}

// TestSyncExactTotalToMenu_DoesNotClearEnrichmentTruncation_OnEqualExactRows
// pins Pin A: an authoritative, truncated Wave-2 enrichment result (5 issues,
// capped/truncated at 50 rows scanned) must not have its IssueTruncated flag
// cleared merely because a later, untruncated 55-row list fetch reports the
// same 5-issue count via the rows-derived sync lane. The exact list confirms
// the ROW COUNT is complete; it says nothing about whether the enrichment
// scan itself covered every row, so the menu badge must keep showing "!5+"
// until a fresh, wider enrichment pass actually confirms exactly 5.
func TestSyncExactTotalToMenu_DoesNotClearEnrichmentTruncation_OnEqualExactRows(t *testing.T) {
	c := newCountTruncationDriftController(t)

	// Step 1: the authoritative Wave-2 enrichment result lands first — 5
	// issues found, but the enrichment scan itself was capped/truncated
	// (e.g. a 50-row enrichment budget on a much larger bucket list).
	findings := map[string][]domain.Finding{
		"arn:aws:s3:::a9s-test-bucket-a": {{Code: "s3.public_access", Phrase: "public access not blocked", Severity: domain.SevWarn, Source: "wave2:s3"}},
	}
	c.ApplyEnrichmentState("s3", 5, true, findings, map[string]map[domain.FindingCode]domain.AttentionDetail{})

	if got := c.GetMenuIssueCounts()["s3"]; got != 5 {
		t.Fatalf("precondition failed: GetMenuIssueCounts()[s3] = %d, want 5", got)
	}
	if got := c.GetMenuIssueTruncated()["s3"]; !got {
		t.Fatalf("precondition failed: GetMenuIssueTruncated()[s3] = %v, want true", got)
	}

	// Step 2: open the s3 list, then drive the rows-derived sync lane the
	// way handle.go's syncExactTotalToMenu runs on every ResourcesLoaded —
	// 55 rows total, exactly 5 of them carrying a Wave-1-shaped SevBroken
	// finding (so listIssueCount computes newIssues=5, matching curIssues),
	// and NO pagination (Pagination.IsTruncated=false) — an exact list.
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	const totalRows = 55
	const brokenRows = 5
	broken := s3BrokenFindings(brokenRows)
	rows := make([]resource.Resource, totalRows)
	for i := range rows {
		rows[i] = resource.Resource{ID: "bucket-" + itoaTest(i), Type: "s3"}
		if i < brokenRows {
			rows[i].Findings = []domain.Finding{broken[i]}
		}
	}
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Gen:          0,
	})

	if got := c.GetMenuIssueCounts()["s3"]; got != 5 {
		t.Errorf("GetMenuIssueCounts()[s3] = %d, want 5 (unchanged)", got)
	}
	if got := c.GetMenuIssueTruncated()["s3"]; got != true {
		t.Errorf("GetMenuIssueTruncated()[s3] = %v, want true — an exact 55-row list fetch must NOT clear the enrichment cap's truncation flag just because the row-derived issue count happens to equal the prior enrichment count (menu \"issues:5\" vs list \"!5+\" drift)", got)
	}
}

// TestSyncExactTotalToMenu_ClearsPriorRowsDerivedTruncation_OnEqualExactRows
// is the companion green case: when curIssueTrunc was itself seeded by a
// PREVIOUS rows-derived (non-authoritative) sync — not by an authoritative
// Wave-2 enrichment result — an equal-count exact rows-derived resync is
// still allowed to clear the truncation flag. This isolates the defect to
// specifically "authoritative enrichment truncation must survive a
// rows-derived equal-count sync", rather than banning the clear arm outright.
func TestSyncExactTotalToMenu_ClearsPriorRowsDerivedTruncation_OnEqualExactRows(t *testing.T) {
	c := newCountTruncationDriftController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenu{ResourceType: "s3", Issues: 5, Truncated: true},
	})
	if got := c.GetMenuIssueTruncated()["s3"]; !got {
		t.Fatalf("precondition failed: GetMenuIssueTruncated()[s3] = %v, want true", got)
	}

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	const totalRows = 55
	const brokenRows = 5
	broken := s3BrokenFindings(brokenRows)
	rows := make([]resource.Resource, totalRows)
	for i := range rows {
		rows[i] = resource.Resource{ID: "bucket-" + itoaTest(i), Type: "s3"}
		if i < brokenRows {
			rows[i].Findings = []domain.Finding{broken[i]}
		}
	}
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Gen:          0,
	})

	if got := c.GetMenuIssueCounts()["s3"]; got != 5 {
		t.Errorf("GetMenuIssueCounts()[s3] = %d, want 5", got)
	}
	if got := c.GetMenuIssueTruncated()["s3"]; got {
		t.Errorf("GetMenuIssueTruncated()[s3] = %v, want false — a truncation flag seeded by a prior rows-derived sync (not an authoritative enrichment result) may still be cleared by an equal-count exact rows-derived resync", got)
	}
}
