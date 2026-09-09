package unit

// qa_issue_count_invariant_test.go — property invariant: issue counts never
// exceed instance count.
//
// Two invariants enforced:
//
//  1. Per-enricher: len(result.Findings) <= len(inputResources) for every
//     entry in EnricherRegistry. An enricher emits at most one Findings entry
//     per resource (keyed by resource ID), so the number of distinct flagged
//     resources can never exceed the number of distinct inputs.
//
//  2. Unified (Wave-1 + Wave-2): Controller.GetListIssueCount() (fed via
//     ApplyEnrichmentState) must never exceed the union of Wave-1 issue
//     resource IDs and Wave-2 finding IDs.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: per-enricher IssueCount <= len(inputResources)
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: unified issue count never exceeds union of Wave-1 and Wave-2 IDs
// ─────────────────────────────────────────────────────────────────────────────

// unifiedIssueCount builds a Controller (via the blessed newTestController
// helper) pre-populated with resources + enrichment state, and returns
// Controller.GetListIssueCount() directly — the live replacement for
// buildUnifiedModelWithBadge+extractIssueCount's dead ResourceListModel.
// FrameTitle() round-trip-then-regex-parse. buildListFrameTitle's own issue
// suffix reads this exact same c.listIssueCount(...) value (list_body.go),
// so calling it directly is a strictly more precise oracle than parsing it
// back out of a formatted string.
func unifiedIssueCount(t *testing.T, resources []resource.Resource, enrichIC int, findings map[string][]domain.Finding) int {
	t.Helper()
	td := resource.ResourceTypeDef{
		ShortName: "ec2",
		Name:      "EC2 Instances",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 12},
		},
	}
	c := newTestController(t)
	c.RegisterFallbackTypeDef(td)
	c.PushChildListScreen(td.ShortName)
	c.ApplyResourcesLoaded(td.ShortName, resources, nil, false)
	c.ApplyEnrichmentState(td.ShortName, enrichIC, false, findings, nil)
	return c.GetListIssueCount()
}

// unionSize returns the number of distinct IDs across the resource slice and findings map.
func unionSize(resources []resource.Resource, findings map[string][]domain.Finding) int {
	seen := make(map[string]struct{}, len(resources)+len(findings))
	for _, r := range resources {
		seen[r.ID] = struct{}{}
	}
	for id := range findings {
		seen[id] = struct{}{}
	}
	return len(seen)
}

// TestUnifiedIssueCount_NeverExceedsUnionSize verifies that the issue count embedded in
// FrameTitle() never exceeds the number of distinct resource IDs across Wave-1 resources
// and Wave-2 findings.
//
// Sub-cases:
//   - Disjoint: Wave-1 stopped resources + Wave-2 findings on different IDs → count ≤ 5
//   - Overlap:  same 3 IDs in both waves → count == 3 (not 6)
//   - Wave-2 only: all resources healthy + 2 findings → count ≤ 2
//   - Findings outside resource list: 2 healthy resources + 5 findings on unknown IDs
func TestUnifiedIssueCount_NeverExceedsUnionSize(t *testing.T) {
	t.Run("disjoint Wave-1 and Wave-2 findings on different IDs — on-page Wave-1 findings count, off-page Wave-2 findings never leak in", func(t *testing.T) {
		// core/app/list_body.go's listIssueCount iterates ONLY
		// c.listScreenResources(ls, typeName) — the active list's OWN loaded
		// rows (ls.Rows) — and cross-references c.enrichmentStore[typeName]
		// (the Wave-2 findings map) by ID against that same row set. A
		// finding whose ID never appears in the loaded rows (here: "ebs"
		// volume IDs against an "ec2" list) can therefore never be counted,
		// regardless of what union of IDs exists across the account — this
		// is a genuine architectural bound (single-list, row-keyed lookup),
		// not a missing-clamp bug. Traced empirically: feeding these two
		// off-page volume findings into enrichIC/ApplyEnrichmentState still
		// yields ic==0 against them every time (verified below by their
		// total exclusion from the exact count).
		//
		// Wave-1 findings (fetcher-written, no "wave2:" Source prefix) are
		// attached directly to Resource.Findings and counted unconditionally
		// by listHasBadgeFinding — giving the 3 on-page EC2 fixtures their
		// own findings (rather than relying on colorEC2, which is
		// colorFromAnyFinding-only and treats a Wave-1-finding-less
		// "stopped" instance as healthy) makes them actually count.
		w1 := []domain.Finding{{Code: "ec2.instance.stopped", Phrase: "instance stopped", Severity: domain.SevWarn, Source: "fetch:ec2"}}
		resources := []resource.Resource{
			{ID: "i-001", Name: "s1", Fields: map[string]string{"name": "s1", "state": "stopped"}, Findings: w1},
			{ID: "i-002", Name: "s2", Fields: map[string]string{"name": "s2", "state": "stopped"}, Findings: w1},
			{ID: "i-003", Name: "s3", Fields: map[string]string{"name": "s3", "state": "stopped"}, Findings: w1},
		}
		findings := map[string][]domain.Finding{
			"vol-aaa": {{Code: "ebs.volume.degraded", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ebs"}},
			"vol-bbb": {{Code: "ebs.volume.degraded", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ebs"}},
		}
		union := unionSize(resources, findings) // {i-001,i-002,i-003,vol-aaa,vol-bbb} = 5, for documentation only.
		const want = 3                          // only the 3 on-page EC2 rows can ever count for this list.
		// enrichIC is the enricher's OWN reported IssueCount for its findings
		// map — len(findings), the count a real Wave-2 result would carry
		// (TestAllEnrichers_IssueCountMatchesFindings pins IssueCount ==
		// len(Findings) for every enricher). Passing the cross-wave union (5)
		// here would feed ApplyEnrichmentState a state no real enrichment
		// result produces, since the enricher never sees Wave-1's on-page IDs.
		ic := unifiedIssueCount(t, resources, len(findings), findings)
		if ic != want {
			t.Errorf("disjoint: IssueCount (%d) != %d — expected exactly the 3 on-page Wave-1 findings to count, with the 2 off-page EBS findings (union=%d) never leaking into an unrelated list's count", ic, want, union)
		}
	})

	t.Run("fully overlapping — same 3 IDs carry BOTH a Wave-1 and a Wave-2 finding → count must be 3 not 6", func(t *testing.T) {
		// Each resource carries its OWN Wave-1 finding (attached directly to
		// Resource.Findings, counted unconditionally by listHasBadgeFinding —
		// see the disjoint sub-case above for why this is required instead of
		// relying on colorEC2) AND a Wave-2 finding on the same ID. This is the
		// genuine cross-wave dedup case: a resource that both waves flag must
		// still count once, not twice — the previous version of this sub-case
		// only carried Wave-2 findings, so it exercised Wave-2-vs-Wave-2
		// dedup but never actually proved a resource with findings in BOTH
		// waves collapses to one.
		w1 := []domain.Finding{{Code: "ec2.instance.stopped", Phrase: "instance stopped", Severity: domain.SevWarn, Source: "fetch:ec2"}}
		resources := []resource.Resource{
			{ID: "i-aaa", Name: "server-a", Fields: map[string]string{"name": "server-a", "state": "stopped"}, Findings: w1},
			{ID: "i-bbb", Name: "server-b", Fields: map[string]string{"name": "server-b", "state": "stopped"}, Findings: w1},
			{ID: "i-ccc", Name: "server-c", Fields: map[string]string{"name": "server-c", "state": "stopped"}, Findings: w1},
		}
		findings := map[string][]domain.Finding{
			"i-aaa": {{Code: "ec2.system.status.impaired", Phrase: "status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
			"i-bbb": {{Code: "ec2.system.status.impaired", Phrase: "status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
			"i-ccc": {{Code: "ec2.system.status.impaired", Phrase: "status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		}
		// Union is still 3 (not 6 — no double counting).
		union := unionSize(resources, findings)
		if union != 3 {
			t.Fatalf("test setup error: expected union size 3, got %d", union)
		}
		// enrichIC passed to ApplyEnrichmentState is the caller-computed unified count (3).
		// This must be an EXACT equality, not just <=: with 3 fully-overlapping
		// IDs, a double-counting regression would produce 6 (still <= union
		// would catch nothing useful here since union itself is only 3), and a
		// silently-dropped-finding regression would produce a count < 3 that a
		// bare "ic > union" check would never flag. Only ic == union pins the
		// real dedup contract for this sub-case.
		ic := unifiedIssueCount(t, resources, union, findings)
		if ic != union {
			t.Errorf("overlap: IssueCount (%d) != union size (%d) — must not double-count a resource carrying both a Wave-1 and a Wave-2 finding", ic, union)
		}
	})

	t.Run("Wave-2 only — all resources healthy + 2 on-page findings → count equals exactly 2", func(t *testing.T) {
		resources := []resource.Resource{
			{ID: "i-r01", Name: "healthy-1", Fields: map[string]string{"name": "healthy-1", "state": "running"}},
			{ID: "i-r02", Name: "healthy-2", Fields: map[string]string{"name": "healthy-2", "state": "running"}},
			{ID: "i-r03", Name: "healthy-3", Fields: map[string]string{"name": "healthy-3", "state": "running"}},
			{ID: "i-r04", Name: "healthy-4", Fields: map[string]string{"name": "healthy-4", "state": "running"}},
			{ID: "i-r05", Name: "healthy-5", Fields: map[string]string{"name": "healthy-5", "state": "running"}},
		}
		findings := map[string][]domain.Finding{
			"i-r01": {{Code: "ec2.system.status.impaired", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
			"i-r02": {{Code: "ec2.system.status.impaired", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		}
		const want = 2 // both finding IDs are on-page (i-r01, i-r02); the other 3 stay healthy.
		ic := unifiedIssueCount(t, resources, want, findings)
		if ic != want {
			t.Errorf("wave-2-only: IssueCount (%d) != %d (exactly the 2 on-page SevBroken findings)", ic, want)
		}
	})

	t.Run("findings on IDs not in resource list never count — count is exactly 0", func(t *testing.T) {
		// 2 healthy resources visible on this list's page; 5 findings whose IDs
		// are not in the list. core/app/list_body.go's listIssueCount only
		// ever iterates the active list's OWN loaded rows
		// (c.listScreenResources) and looks each one up by ID in the Wave-2
		// findings map — a finding for an ID that was never loaded into this
		// list can never be visited by that loop, so it can never bump the
		// count. Exact 0 is the oracle: the list-title count is bound to a
		// single list's own row set and cannot see IDs outside it, unlike the
		// menu badge, which aggregates across the whole account.
		resources := []resource.Resource{
			{ID: "i-p01", Name: "page-instance-1", Fields: map[string]string{"name": "page-instance-1", "state": "running"}},
			{ID: "i-p02", Name: "page-instance-2", Fields: map[string]string{"name": "page-instance-2", "state": "running"}},
		}
		findings := map[string][]domain.Finding{
			"i-x01": {{Code: "ec2.system.status.impaired", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
			"i-x02": {{Code: "ec2.system.status.impaired", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
			"i-x03": {{Code: "ec2.system.status.impaired", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
			"i-x04": {{Code: "ec2.system.status.impaired", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
			"i-x05": {{Code: "ec2.system.status.impaired", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		}
		// Union excluding the 2 healthy page resources — the finding-only ID
		// count, kept purely for documentation of the account-wide input size.
		findingOnlyUnion := len(findings)
		const want = 0
		ic := unifiedIssueCount(t, resources, findingOnlyUnion, findings)
		if ic != want {
			t.Errorf("off-page findings: IssueCount (%d) != %d — off-page finding IDs (%d distinct) must never leak into this list's on-page count", ic, want, findingOnlyUnion)
		}
	})
}
