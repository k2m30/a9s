// An exact (untruncated) list fetch says the row count is complete; it says
// nothing about whether the Wave-2 enrichment scan that produced the issue
// count covered every row. A list can hold an exact page of 55 rows while the
// type's enrichment cap scanned only 50, so an authoritative, truncated
// enrichment count keeps its IssueTruncated flag when an equal-count exact
// rows-derived sync (syncExactTotalToMenu, core/app/handle.go) lands.
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

func newCountTruncationDriftController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return c
}

// s3BrokenFindings builds n Wave-1-shaped (non-wave2-sourced), SevBroken
// findings for distinct fake S3 bucket IDs — matching the shape
// listHasBadgeFinding (core/app/list_body.go) counts unconditionally at
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

func TestSyncExactTotalToMenu_DoesNotClearEnrichmentTruncation_OnEqualExactRows(t *testing.T) {
	c := newCountTruncationDriftController(t)

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
	handlePage(c, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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

// A truncation flag seeded by a previous rows-derived (non-authoritative) sync
// may be cleared by an equal-count exact rows-derived resync.
func TestSyncExactTotalToMenu_ClearsPriorRowsDerivedTruncation_OnEqualExactRows(t *testing.T) {
	c := newCountTruncationDriftController(t)
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

	// Seed the truncation the rows-derived way this case is about: a
	// truncated first page carrying the same five broken rows. Every PatchMenu
	// producer is a live probe or a Wave-2 result, whose truncation is
	// authoritative and which an equal-count rows-derived resync may not
	// clear, so seeding through PatchMenu would not isolate the distinction
	// this case is about.
	handlePage(c, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "next"},
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})
	if got := c.GetMenuIssueTruncated()["s3"]; !got {
		t.Fatalf("precondition failed: GetMenuIssueTruncated()[s3] = %v, want true", got)
	}

	handlePage(c, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})

	if got := c.GetMenuIssueCounts()["s3"]; got != 5 {
		t.Errorf("GetMenuIssueCounts()[s3] = %d, want 5", got)
	}
	if got := c.GetMenuIssueTruncated()["s3"]; got {
		t.Errorf("GetMenuIssueTruncated()[s3] = %v, want false — a truncation flag seeded by a prior rows-derived sync (not an authoritative enrichment result) may still be cleared by an equal-count exact rows-derived resync", got)
	}
}
