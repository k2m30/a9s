// session_rowstore_test.go — Stage-1 pin suite for the row-store
// unification plan (rowstore-unification-plan.md, Stage 1: "Introduce
// RowStore behind existing maps, dual-write scaffolding, zero behavior
// change"). Pins the CONTRACT of core/session.RowStore /
// core/session.TypeRows (core/session/rowstore.go) against
// docs/design/cache-requirements.md C2/C5/C6/C6a/C6b/C9 and the defects
// those rules rule out (D7, D12-D17), mirroring the semantics
// core/runtime/probes.go:reconcileTypeFile already enforces for the
// on-disk file and core/app/list_body.go's dedupAgainstExisting/
// isStaleReplace already enforce for the per-screen ListState.
//
// API pinned here (core/session/rowstore.go, landed):
//
//	type Origin int
//	const (OriginDisk Origin = iota; OriginProbe; OriginFetch)
//
//	type TypeRows struct {
//	    Rows       []resource.Resource
//	    Pagination *resource.PaginationMeta
//	    TotalCount int
//	    Origin     Origin
//	    Partial    bool
//	    Gen        domain.Gen
//	}
//
//	type RowStore struct { ... } // mu sync.Mutex; types map[string]TypeRows
//	func NewRowStore() *RowStore
//	func (s *RowStore) Observe(canon string, rows []resource.Resource, pagination *resource.PaginationMeta, origin Origin, appendPage bool) ([]resource.Resource, domain.Gen)
//	func (s *RowStore) ObserveCount(canon string, totalCount int) domain.Gen
//	func (s *RowStore) ObservePartial(canon string, rows []resource.Resource) ([]resource.Resource, domain.Gen)
//	func (s *RowStore) Amend(canon string, fn func([]resource.Resource) []resource.Resource) domain.Gen
//	func (s *RowStore) Snapshot(canon string) TypeRows // zero value when never observed
//	func (s *RowStore) SnapshotAll(includePartial bool) map[string]TypeRows
//	func (s *RowStore) Clear()
package unit_test

import (
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// -----------------------------------------------------------------------
// Pin 1 — Observe append dedups by row ID (D13 semantics).
// -----------------------------------------------------------------------

// TestRowStore_Observe_AppendDedupsByID pins D13: "Cold-open + `m` duplicated
// page 1 on screen ... the append path had no row-identity awareness" — an
// Observe call with appendPage=true and overlapping IDs must NOT duplicate
// rows; each ID appears once in the accepted/stored set.
func TestRowStore_Observe_AppendDedupsByID(t *testing.T) {
	store := session.NewRowStore()

	page1 := []resource.Resource{
		{ID: "bucket-1", Name: "bucket-1", Type: "s3"},
		{ID: "bucket-2", Name: "bucket-2", Type: "s3"},
	}
	store.Observe("s3", page1, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-1"}, session.OriginFetch, false)

	// A duplicate replay of page 1 (e.g. cold-open + `m` racing the initial
	// fetch) appended on top of itself must not double the stored rows.
	accepted, _ := store.Observe("s3", page1, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-1"}, session.OriginFetch, true)
	if len(accepted) != 2 {
		t.Fatalf("Observe returned accepted rows %+v, want len 2 (deduped)", accepted)
	}

	snap := store.Snapshot("s3")
	if len(snap.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2 — append must dedup by ID, not duplicate page 1 (D13)", len(snap.Rows))
	}
	seen := map[string]int{}
	for _, r := range snap.Rows {
		seen[r.ID]++
	}
	for _, id := range []string{"bucket-1", "bucket-2"} {
		if seen[id] != 1 {
			t.Errorf("row ID %q appears %d times, want exactly 1", id, seen[id])
		}
	}
}

// TestRowStore_Observe_AppendDedupsByID_NewRowsStillAdded verifies the dedup
// guard does not also drop genuinely new rows in the same append batch — a
// mixed replay (some already-known IDs, some new load-more IDs) keeps the
// new ones.
func TestRowStore_Observe_AppendDedupsByID_NewRowsStillAdded(t *testing.T) {
	store := session.NewRowStore()

	page1 := []resource.Resource{
		{ID: "bucket-1", Name: "bucket-1", Type: "s3"},
	}
	store.Observe("s3", page1, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, false)

	page2WithOverlap := []resource.Resource{
		{ID: "bucket-1", Name: "bucket-1", Type: "s3"}, // duplicate of page 1
		{ID: "bucket-2", Name: "bucket-2", Type: "s3"}, // genuinely new
	}
	store.Observe("s3", page2WithOverlap, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, true)

	snap := store.Snapshot("s3")
	if len(snap.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2 (bucket-1 deduped, bucket-2 added)", len(snap.Rows))
	}
}

// -----------------------------------------------------------------------
// Pin 2 — stale truncated ID-subset replace rejected once exact (D14).
// -----------------------------------------------------------------------

// INVERTED (listgen row 5): the store does not decide which of two results is
// older. It used to guess from content shape — a smaller, still-truncated,
// strict-ID-subset replace was rejected — and that guess could only see one
// shape while getting a legitimate Ctrl+R reset to page 1 wrong. Ordering is
// now decided before the store is reached, by the per-type request sequence
// (runtime.Core.ListResultSuperseded), so a replace that gets this far has
// already been established as the newest one and is applied. The guarantee
// the old assertion protected is pinned end to end by
// TestLateReplace_DoesNotStompDeeperList. Do not restore the rejection: a
// second, disagreeing opinion about staleness is what this row removed.
func TestRowStore_Observe_StaleTruncatedSubsetRejectedOnceExact(t *testing.T) {
	store := session.NewRowStore()

	full := make([]resource.Resource, 55)
	for i := range full {
		full[i] = resource.Resource{ID: idFor(i), Name: idFor(i), Type: "s3"}
	}
	// Deep load-more already landed: 55 rows, exact (untruncated) pagination.
	_, genAfterFull := store.Observe("s3", full, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	// A stale page-1 refetch arrives late: only the first 50 IDs, truncated —
	// a strict ID subset of the stored 55, delivered as a non-append replace
	// (append=false — a fresh fetch result, not a load-more page).
	stalePage1 := full[:50]
	accepted, genAfterStale := store.Observe("s3", stalePage1, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, false)
	if len(accepted) != 50 {
		t.Errorf("Observe(truncated ID-subset replace) returned %d rows, want the 50 it carried — the store applies what reaches it", len(accepted))
	}
	if genAfterStale == genAfterFull {
		t.Errorf("Gen after the replace = %d, want a bump from %d — an applied observation is a write", genAfterStale, genAfterFull)
	}

	snap := store.Snapshot("s3")
	if len(snap.Rows) != 50 {
		t.Fatalf("len(Rows) = %d, want 50", len(snap.Rows))
	}
	// The wider total survives: a truncated page does not claim to be the
	// whole list, so Observe's shrink guard keeps the 55 it already knew.
	if snap.TotalCount != 55 {
		t.Errorf("TotalCount = %d, want 55 — a truncated replace must not shrink the known population", snap.TotalCount)
	}
}

// -----------------------------------------------------------------------
// Pin 3 — Disk-origin Observe never overwrites Fetch-origin rows;
// Fetch/Probe replace Disk.
// -----------------------------------------------------------------------

// TestRowStore_Observe_DiskNeverOverwritesFetch pins the Origin precedence
// half of Observe's documented contract: once a Fetch-origin observation has
// landed, a later Disk-origin observation (e.g. a delayed disk-seed replay)
// must be rejected, not silently applied.
func TestRowStore_Observe_DiskNeverOverwritesFetch(t *testing.T) {
	store := session.NewRowStore()

	fetchRows := []resource.Resource{{ID: "i-fetch-1", Name: "fetch-1", Type: "ec2"}}
	store.Observe("ec2", fetchRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	diskRows := []resource.Resource{{ID: "i-disk-1", Name: "disk-1", Type: "ec2"}}
	accepted, _ := store.Observe("ec2", diskRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginDisk, false)
	if len(accepted) != 1 || accepted[0].ID != "i-fetch-1" {
		t.Errorf("Observe(Disk over Fetch) returned %+v, want unchanged [i-fetch-1] — a Disk-origin observation must never overwrite Fetch-origin rows", accepted)
	}

	snap := store.Snapshot("ec2")
	if len(snap.Rows) != 1 || snap.Rows[0].ID != "i-fetch-1" {
		t.Fatalf("Rows = %+v, want unchanged [i-fetch-1]", snap.Rows)
	}
	if snap.Origin != session.OriginFetch {
		t.Errorf("Origin = %v, want OriginFetch preserved after a rejected Disk overwrite attempt", snap.Origin)
	}
}

// TestRowStore_Observe_DiskNeverOverwritesProbe extends the same guard to
// Probe-origin rows (the Wave-1 sweep's retained first page) — a disk seed
// arriving after a probe result must also be rejected.
func TestRowStore_Observe_DiskNeverOverwritesProbe(t *testing.T) {
	store := session.NewRowStore()

	probeRows := []resource.Resource{{ID: "i-probe-1", Name: "probe-1", Type: "ec2"}}
	store.Observe("ec2", probeRows, &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)

	diskRows := []resource.Resource{{ID: "i-disk-1", Name: "disk-1", Type: "ec2"}}
	accepted, _ := store.Observe("ec2", diskRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginDisk, false)
	if len(accepted) != 1 || accepted[0].ID != "i-probe-1" {
		t.Errorf("Observe(Disk over Probe) returned %+v, want unchanged [i-probe-1] — a Disk-origin observation must never overwrite Probe-origin rows", accepted)
	}

	snap := store.Snapshot("ec2")
	if len(snap.Rows) != 1 || snap.Rows[0].ID != "i-probe-1" {
		t.Fatalf("Rows = %+v, want unchanged [i-probe-1]", snap.Rows)
	}
}

// TestRowStore_Observe_FetchReplacesDisk verifies the converse: a live Fetch
// (or Probe) observation legitimately supersedes a Disk-seeded entry.
func TestRowStore_Observe_FetchReplacesDisk(t *testing.T) {
	store := session.NewRowStore()

	diskRows := []resource.Resource{{ID: "i-disk-1", Name: "disk-1", Type: "ec2"}}
	store.Observe("ec2", diskRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginDisk, false)

	fetchRows := []resource.Resource{{ID: "i-fetch-1", Name: "fetch-1", Type: "ec2"}}
	accepted, _ := store.Observe("ec2", fetchRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	if len(accepted) != 1 || accepted[0].ID != "i-fetch-1" {
		t.Fatalf("Observe(Fetch over Disk) returned %+v, want [i-fetch-1] — a Fetch observation must replace a Disk-origin entry", accepted)
	}

	snap := store.Snapshot("ec2")
	if snap.Origin != session.OriginFetch {
		t.Errorf("Origin = %v, want OriginFetch after Fetch supersedes Disk", snap.Origin)
	}
}

// TestRowStore_Observe_ProbeReplacesDisk mirrors the Probe (Wave 1
// availability sweep) side of the same precedence rule.
func TestRowStore_Observe_ProbeReplacesDisk(t *testing.T) {
	store := session.NewRowStore()

	diskRows := []resource.Resource{{ID: "i-disk-1", Name: "disk-1", Type: "ec2"}}
	store.Observe("ec2", diskRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginDisk, false)

	probeRows := []resource.Resource{{ID: "i-probe-1", Name: "probe-1", Type: "ec2"}}
	accepted, _ := store.Observe("ec2", probeRows, &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)
	if len(accepted) != 1 || accepted[0].ID != "i-probe-1" {
		t.Fatalf("Observe(Probe over Disk) returned %+v, want [i-probe-1] — a Probe observation must replace a Disk-origin entry", accepted)
	}
}

// -----------------------------------------------------------------------
// Pin 4 — ObservePartial marks Partial; a later full Observe flips Partial
// off and wholesale-replaces the row set (full-beats-partial); partial-only
// entries are excluded from SnapshotAll(includePartial=false).
// -----------------------------------------------------------------------

// TestRowStore_ObservePartial_MarksPartial verifies ObservePartial's basic
// contract: the resulting TypeRows.Partial is true.
func TestRowStore_ObservePartial_MarksPartial(t *testing.T) {
	store := session.NewRowStore()

	lazyRows := []resource.Resource{{ID: "key-lazy-1", Name: "lazy-1", Type: "kms"}}
	accepted, _ := store.ObservePartial("kms", lazyRows)
	if len(accepted) != 1 || accepted[0].ID != "key-lazy-1" {
		t.Fatalf("ObservePartial returned %+v, want [key-lazy-1]", accepted)
	}

	snap := store.Snapshot("kms")
	if !snap.Partial {
		t.Error("Partial = false, want true after ObservePartial (LazyResourceCache-fold semantics)")
	}
	if len(snap.Rows) != 1 || snap.Rows[0].ID != "key-lazy-1" {
		t.Fatalf("Rows = %+v, want [key-lazy-1] after ObservePartial", snap.Rows)
	}
}

// TestRowStore_ObservePartial_IsCumulative pins ObservePartial's own
// append-dedup contract (distinct from Observe's stale-replace guard):
// successive ObservePartial calls for different IDs accumulate rather than
// replace, mirroring LazyResourceCache's existing merge-by-ID behavior.
func TestRowStore_ObservePartial_IsCumulative(t *testing.T) {
	store := session.NewRowStore()

	store.ObservePartial("kms", []resource.Resource{{ID: "key-lazy-1", Type: "kms"}})
	store.ObservePartial("kms", []resource.Resource{{ID: "key-lazy-2", Type: "kms"}})

	snap := store.Snapshot("kms")
	if len(snap.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2 — successive ObservePartial calls must accumulate, not replace", len(snap.Rows))
	}
}

// TestRowStore_Observe_FullReplacesPartial pins "full-beats-partial": once a
// full canonical Observe (appendPage=false) lands for a type that only had a
// partial (lazy-add) entry, Partial flips to false and the full observation's
// row set becomes authoritative (Observe's non-append branch replaces
// wholesale) — the partial entry never blocks or survives a real fetch.
func TestRowStore_Observe_FullReplacesPartial(t *testing.T) {
	store := session.NewRowStore()

	store.ObservePartial("kms", []resource.Resource{
		{ID: "key-shared-1", Name: "lazy-name", Type: "kms", Fields: map[string]string{"source": "lazy"}},
	})

	fullRows := []resource.Resource{
		{ID: "key-shared-1", Name: "full-name", Type: "kms", Fields: map[string]string{"source": "full"}},
		{ID: "key-full-2", Name: "full-2", Type: "kms", Fields: map[string]string{"source": "full"}},
	}
	accepted, _ := store.Observe("kms", fullRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	if len(accepted) != 2 {
		t.Fatalf("Observe(full over partial) returned %+v, want 2 rows — a full Observe must always supersede a partial-only entry", accepted)
	}

	snap := store.Snapshot("kms")
	if snap.Partial {
		t.Error("Partial = true, want false — a full Observe must flip Partial off (full-beats-partial)")
	}
	if len(snap.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2 after full Observe supersedes partial", len(snap.Rows))
	}
	for _, r := range snap.Rows {
		if r.ID == "key-shared-1" && r.Fields["source"] != "full" {
			t.Errorf("colliding row %q Fields[source] = %q, want %q — full observation must win over the earlier partial value", r.ID, r.Fields["source"], "full")
		}
	}
}

// TestRowStore_SnapshotAll_ExcludesPartialWhenRequested pins
// SnapshotAll(includePartial=false) filtering out partial-only entries
// (the LazyResourceCache fold's "never poison canonical seeds/saves" scope
// boundary, C6).
func TestRowStore_SnapshotAll_ExcludesPartialWhenRequested(t *testing.T) {
	store := session.NewRowStore()

	store.Observe("ec2", []resource.Resource{{ID: "i-canonical-1", Type: "ec2"}}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	store.ObservePartial("kms", []resource.Resource{{ID: "key-lazy-only-1", Type: "kms"}})

	withoutPartial := store.SnapshotAll(false)
	if _, ok := withoutPartial["kms"]; ok {
		t.Error(`SnapshotAll(false)["kms"] present, want absent — a partial-only entry must never surface in the canonical (non-partial) snapshot`)
	}
	if _, ok := withoutPartial["ec2"]; !ok {
		t.Error(`SnapshotAll(false)["ec2"] absent, want present — a full, non-partial entry must always surface`)
	}

	withPartial := store.SnapshotAll(true)
	if _, ok := withPartial["kms"]; !ok {
		t.Error(`SnapshotAll(true)["kms"] absent, want present — includePartial=true must surface partial entries too`)
	}
}

// -----------------------------------------------------------------------
// Pin 4b — ObserveCount (C6a): counts-only observation never touches Rows.
// -----------------------------------------------------------------------

// TestRowStore_ObserveCount_NeverTouchesRows pins C6a directly against
// RowStore's dedicated counts-only entry point: TotalCount updates, Rows
// (and Pagination/Origin/Partial) are carried forward untouched, even when
// this leaves TotalCount numerically disagreeing with len(Rows).
func TestRowStore_ObserveCount_NeverTouchesRows(t *testing.T) {
	store := session.NewRowStore()

	store.Observe("ec2", []resource.Resource{{ID: "i-1", Type: "ec2"}}, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, false)

	genAfterCount := store.ObserveCount("ec2", 50)

	snap := store.Snapshot("ec2")
	if snap.TotalCount != 50 {
		t.Errorf("TotalCount = %d, want 50 after ObserveCount", snap.TotalCount)
	}
	if len(snap.Rows) != 1 || snap.Rows[0].ID != "i-1" {
		t.Errorf("Rows = %+v, want unchanged [i-1] — ObserveCount must never touch Rows (C6a), even though TotalCount(50) now disagrees with len(Rows)=1", snap.Rows)
	}
	if snap.Gen != genAfterCount {
		t.Errorf("Snapshot().Gen = %d, want it to match ObserveCount's returned Gen %d", snap.Gen, genAfterCount)
	}
}

// TestRowStore_ObserveCount_OnAbsentType verifies ObserveCount creates a
// counts-only entry even for a type RowStore has never seen rows for (the
// disk-cache-loaded seed's placeholder-fallback case, C6a).
func TestRowStore_ObserveCount_OnAbsentType(t *testing.T) {
	store := session.NewRowStore()

	store.ObserveCount("lambda", 12)

	snap := store.Snapshot("lambda")
	if snap.TotalCount != 12 {
		t.Errorf("TotalCount = %d, want 12", snap.TotalCount)
	}
	if len(snap.Rows) != 0 {
		t.Errorf("Rows = %+v, want empty — a counts-only observation on a never-seen type must not fabricate Rows", snap.Rows)
	}
}

// -----------------------------------------------------------------------
// Pin 5 — Amend is copy-on-write (structural dispatch-time-freeze pin); Gen bumped.
// -----------------------------------------------------------------------

// TestRowStore_Amend_CopyOnWrite_EarlierSnapshotUnchanged pins Amend's
// copy-on-write contract: taking
// a Snapshot, then Amend-ing (adding a finding/field), must NOT mutate the
// rows already captured by the earlier Snapshot — the enrichment-fold rows
// mutated in place today (runtime/helpers.go, tui/app_enrich_fold.go) are
// exactly the bug class Amend's copy-on-write contract eliminates.
func TestRowStore_Amend_CopyOnWrite_EarlierSnapshotUnchanged(t *testing.T) {
	store := session.NewRowStore()

	store.Observe("ec2", []resource.Resource{
		{ID: "i-amend-1", Name: "amend-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	before := store.Snapshot("ec2")
	if len(before.Rows) != 1 {
		t.Fatalf("precondition: len(before.Rows) = %d, want 1", len(before.Rows))
	}
	beforeGen := before.Gen

	newGen := store.Amend("ec2", func(rows []resource.Resource) []resource.Resource {
		out := make([]resource.Resource, len(rows))
		for i, r := range rows {
			if r.ID == "i-amend-1" {
				r.Findings = append(append([]domain.Finding(nil), r.Findings...), domain.Finding{
					Code:     "test.amend",
					Phrase:   "amended finding",
					Severity: domain.SevBroken,
					Source:   "wave2:ec2",
				})
			}
			out[i] = r
		}
		return out
	})

	if len(before.Rows[0].Findings) != 0 {
		t.Errorf("earlier Snapshot's row Findings = %+v, want unchanged (empty) — Amend must be copy-on-write, never mutate a prior Snapshot's rows (the dispatch-time payload freeze)", before.Rows[0].Findings)
	}

	after := store.Snapshot("ec2")
	if len(after.Rows) != 1 || len(after.Rows[0].Findings) != 1 {
		t.Fatalf("after Amend, Rows = %+v, want the amended finding present in the new snapshot", after.Rows)
	}
	if newGen <= beforeGen {
		t.Errorf("Amend returned Gen %d, want strictly greater than pre-Amend Gen %d", newGen, beforeGen)
	}
	if after.Gen != newGen {
		t.Errorf("Snapshot().Gen = %d after Amend, want it to match Amend's returned Gen %d", after.Gen, newGen)
	}
}

// TestRowStore_Amend_BumpsGenEvenWithNoRows pins the documented "Bumps and
// returns Gen even when canon has no rows yet" behavior, so callers relying
// on Gen monotonicity are never surprised by a no-op Amend on an absent type.
func TestRowStore_Amend_BumpsGenEvenWithNoRows(t *testing.T) {
	store := session.NewRowStore()

	beforeGen := store.Snapshot("never-observed").Gen

	newGen := store.Amend("never-observed", func(rows []resource.Resource) []resource.Resource { return rows })

	if newGen <= beforeGen {
		t.Errorf("Amend on absent type returned Gen %d, want strictly greater than %d", newGen, beforeGen)
	}
	after := store.Snapshot("never-observed")
	if after.Gen != newGen {
		t.Errorf("Snapshot().Gen = %d, want %d to match Amend's returned Gen", after.Gen, newGen)
	}
	if len(after.Rows) != 0 {
		t.Errorf("Rows after Amend on absent type = %+v, want empty", after.Rows)
	}
}

// -----------------------------------------------------------------------
// Pin 6 — Snapshot immunity: mutating slices returned earlier never affects
// the store.
// -----------------------------------------------------------------------

// TestRowStore_Snapshot_MutatingReturnedSliceDoesNotAffectStore pins
// snapshot immunity independently of Amend: a caller that takes a Snapshot
// and mutates the returned Rows slice/elements in place (e.g. a renderer
// bug, or a stale test helper) must never leak that mutation back into the
// store's own state — TypeRows' documented immutability invariant.
func TestRowStore_Snapshot_MutatingReturnedSliceDoesNotAffectStore(t *testing.T) {
	store := session.NewRowStore()

	store.Observe("rds", []resource.Resource{
		{ID: "db-1", Name: "original-name", Type: "rds"},
	}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	snap := store.Snapshot("rds")
	if len(snap.Rows) != 1 {
		t.Fatalf("precondition: len(snap.Rows) = %d, want 1", len(snap.Rows))
	}
	snap.Rows[0].Name = "MUTATED-BY-CALLER"
	if snap.Rows[0].Fields == nil {
		snap.Rows[0].Fields = map[string]string{}
	}
	snap.Rows[0].Fields["poison"] = "yes"

	again := store.Snapshot("rds")
	if again.Rows[0].Name != "original-name" {
		t.Errorf("Snapshot().Rows[0].Name = %q after caller mutated a prior snapshot's slice, want %q unchanged — Snapshot must return caller-immune data", again.Rows[0].Name, "original-name")
	}
	if again.Rows[0].Fields["poison"] == "yes" {
		t.Error("Snapshot().Rows[0].Fields[poison] leaked from a mutated prior snapshot — Snapshot must not alias mutable state with the store")
	}
}

// TestRowStore_SnapshotAll_MutatingReturnedMapDoesNotAffectStore extends the
// immunity pin to SnapshotAll's returned map/slices.
func TestRowStore_SnapshotAll_MutatingReturnedMapDoesNotAffectStore(t *testing.T) {
	store := session.NewRowStore()
	store.Observe("lambda", []resource.Resource{{ID: "fn-1", Name: "fn-1", Type: "lambda"}}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	all := store.SnapshotAll(false)
	tr := all["lambda"]
	tr.Rows[0].Name = "MUTATED"
	all["lambda"] = tr // even reassigning the local map entry must not reach the store

	again := store.SnapshotAll(false)
	if again["lambda"].Rows[0].Name != "fn-1" {
		t.Errorf("SnapshotAll()[lambda].Rows[0].Name = %q after caller mutation, want %q unchanged", again["lambda"].Rows[0].Name, "fn-1")
	}
}

// -----------------------------------------------------------------------
// Pin 7 — Clear on Session.Rotate (C9): store empty after rotate,
// pair-scoped.
// -----------------------------------------------------------------------

// TestRowStore_Clear_EmptiesStore pins the store's own Clear contract in
// isolation.
func TestRowStore_Clear_EmptiesStore(t *testing.T) {
	store := session.NewRowStore()
	store.Observe("s3", []resource.Resource{{ID: "bucket-1", Type: "s3"}}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	store.ObservePartial("kms", []resource.Resource{{ID: "key-1", Type: "kms"}})

	store.Clear()

	all := store.SnapshotAll(true)
	if len(all) != 0 {
		t.Errorf("SnapshotAll(true) after Clear = %+v, want empty map", all)
	}
	empty := store.Snapshot("s3")
	if len(empty.Rows) != 0 {
		t.Errorf("Snapshot(s3).Rows after Clear = %+v, want empty", empty.Rows)
	}
}

// TestSession_Rotate_ClearsRowStore pins C9 (pair isolation) at the
// Session level: Session.Rotate() must clear the session's RowStore exactly
// like it clears ResourceCache/ProbeResources/LazyResourceCache today, so a
// profile/region switch never leaks rows from the old pair.
func TestSession_Rotate_ClearsRowStore(t *testing.T) {
	s := session.New()
	if s.RowStore == nil {
		t.Fatal("session.New().RowStore is nil — Session must construct a RowStore (dual-write scaffolding)")
	}
	s.RowStore.Observe("ec2", []resource.Resource{{ID: "i-preswitch-1", Type: "ec2"}}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	preRotate := s.RowStore.Snapshot("ec2")
	if len(preRotate.Rows) != 1 {
		t.Fatalf("precondition: len(preRotate.Rows) = %d, want 1", len(preRotate.Rows))
	}

	s.Rotate()

	postRotate := s.RowStore.Snapshot("ec2")
	if len(postRotate.Rows) != 0 {
		t.Errorf("after Rotate, RowStore.Snapshot(ec2).Rows = %+v, want empty — Rotate must clear the row store (C9 pair isolation)", postRotate.Rows)
	}
}

// -----------------------------------------------------------------------
// Pin 8 — Concurrency: parallel Observe/Amend/Snapshot under -race
// (D13 unguarded-RMW pin).
// -----------------------------------------------------------------------

// TestRowStore_ConcurrentObserveAmendSnapshot_NoRace pins D13's "the two
// save lanes raced an unguarded read-modify-write on the shared store" —
// concurrent Observe/Amend/Snapshot calls against the SAME type must never
// race under `go test -race`. Every store read-modify-write must run under
// one mutex end-to-end.
//
// Run in isolation to make the -race requirement explicit:
//
//	go test -race -run TestRowStore_ConcurrentObserveAmendSnapshot_NoRace ./tests/unit/
func TestRowStore_ConcurrentObserveAmendSnapshot_NoRace(t *testing.T) {
	store := session.NewRowStore()
	const goroutines = 8
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				store.Observe("ec2", []resource.Resource{
					{ID: idFor(g), Name: idFor(g), Type: "ec2"},
				}, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, true)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				store.Amend("ec2", func(rows []resource.Resource) []resource.Resource {
					out := make([]resource.Resource, len(rows))
					copy(out, rows)
					return out
				})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = store.Snapshot("ec2")
				_ = store.SnapshotAll(true)
			}
		}()
	}

	wg.Wait()

	final := store.Snapshot("ec2")
	if len(final.Rows) == 0 {
		t.Error("final Snapshot(ec2).Rows is empty after concurrent Observe/Amend/Snapshot — expected at least the deduped goroutine rows to survive")
	}
}

// idFor returns a deterministic, distinguishable row ID for test fixtures.
func idFor(i int) string {
	const letters = "0123456789abcdefghijklmnopqrstuvwxyz"
	if i < len(letters) {
		return "row-" + string(letters[i])
	}
	return "row-n" + string(rune('a'+i%26))
}

// -----------------------------------------------------------------------
// Pin 9 — Controller.Handle wiring into RowStore (ported from the retired
// rowstore_differential_test.go at Stage 5: those store-only assertions
// pinned the SAME contract this file pins at the RowStore-direct level, but
// through the real Controller.Handle event path rather than calling
// RowStore/Core methods directly — a regression here would mean the wiring
// between an inbound event and the store broke even though RowStore's own
// unit contract (Pins 1-8 above) stayed intact.
// -----------------------------------------------------------------------

// newRowStoreControllerPin mirrors the other per-file controller
// constructors in this package (newSeededTestController,
// newStage2PinTestController, newRowStorePinsTestController) — duplicated
// as its own small variant so this file has no cross-file coupling to
// another test file's helper lifetime.
func newRowStoreControllerPin(t *testing.T) (*session.Session, *runtime.Core, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return s, core, c
}

func rowStoreControllerPinIDSet(rows []resource.Resource) map[string]bool {
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[r.ID] = true
	}
	return out
}

func assertRowStoreControllerPinHasIDs(t *testing.T, s *session.Session, shortName string, wantIDs ...string) {
	t.Helper()
	snap := s.RowStore.Snapshot(shortName)

	want := make(map[string]bool, len(wantIDs))
	for _, id := range wantIDs {
		want[id] = true
	}
	got := rowStoreControllerPinIDSet(snap.Rows)
	if len(got) != len(want) {
		t.Errorf("type %q: RowStore.Snapshot(%q).Rows ID set = %v, want %v", shortName, shortName, got, want)
		return
	}
	for id := range want {
		if !got[id] {
			t.Errorf("type %q: RowStore.Snapshot(%q).Rows ID set = %v, want %v", shortName, shortName, got, want)
			return
		}
	}
}

// TestRowStoreControllerPin_AvailabilityCacheLoaded_RealDiskRows_SeedRowStore
// drives the disk-cache-loaded seed event through Controller.Handle with a
// populated on-disk per-type file (real row data, not the placeholder
// fallback) and asserts RowStore retains exactly those rows (OriginDisk).
func TestRowStoreControllerPin_AvailabilityCacheLoaded_RealDiskRows_SeedRowStore(t *testing.T) {
	s, core, c := newRowStoreControllerPin(t)

	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store")
	}
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        1,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "i-diskrow-1", Name: "disk-row-1", Fields: map[string]string{"state": "running"}},
		},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("seed fixture SaveType(ec2): %v", err)
	}

	_, _ = c.Handle(messages.AvailabilityCacheLoaded{
		Entries:   map[string]int{"ec2": 1},
		Truncated: map[string]bool{"ec2": false},
	})

	assertRowStoreControllerPinHasIDs(t, s, "ec2", "i-diskrow-1")
	snap := s.RowStore.Snapshot("ec2")
	if len(snap.Rows) != 1 || snap.Rows[0].ID != "i-diskrow-1" {
		t.Errorf("RowStore.Snapshot(ec2).Rows = %+v, want [i-diskrow-1] seeded from the real on-disk row data", snap.Rows)
	}
}

// TestRowStoreControllerPin_AvailabilityCacheLoaded_PlaceholderFallback_IsCountsOnly
// drives the disk-cache-loaded seed event through Controller.Handle with NO
// on-disk per-type file (the placeholder-row fallback path) and asserts
// RowStore treats it as a counts-only observation (C6a: TotalCount set, Rows
// untouched/empty) — a placeholder-only fallback must never fabricate Rows
// in the store.
func TestRowStoreControllerPin_AvailabilityCacheLoaded_PlaceholderFallback_IsCountsOnly(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	_, _ = c.Handle(messages.AvailabilityCacheLoaded{
		Entries:   map[string]int{"ec2": 3},
		Truncated: map[string]bool{"ec2": false},
	})

	snap := s.RowStore.Snapshot("ec2")
	if snap.TotalCount != 3 {
		t.Errorf("RowStore.Snapshot(ec2).TotalCount = %d, want 3 to mirror the cache-loaded count (C6a)", snap.TotalCount)
	}
	if len(snap.Rows) != 0 {
		t.Errorf("RowStore.Snapshot(ec2).Rows = %+v, want empty — a placeholder-only fallback must never fabricate Rows in the store (C6a)", snap.Rows)
	}
}

// TestRowStoreControllerPin_AvailabilityPrefetched_SeedsRowStore drives the
// synchronous no-cache-mode prefetch event through Controller.Handle and
// asserts RowStore retains exactly the prefetched rows for the type
// (OriginFetch).
func TestRowStoreControllerPin_AvailabilityPrefetched_SeedsRowStore(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	rows := []resource.Resource{
		{ID: "i-prefetch-1", Name: "prefetch-1", Type: "ec2"},
		{ID: "i-prefetch-2", Name: "prefetch-2", Type: "ec2"},
	}
	_, _ = c.Handle(messages.AvailabilityPrefetched{
		Entries:    map[string]int{"ec2": 2},
		Truncated:  map[string]bool{"ec2": false},
		Resources:  map[string][]resource.Resource{"ec2": rows},
		Pagination: map[string]*resource.PaginationMeta{"ec2": {IsTruncated: false}},
		Gen:        s.AvailabilityGen,
	})

	assertRowStoreControllerPinHasIDs(t, s, "ec2", "i-prefetch-1", "i-prefetch-2")
}

// TestRowStoreControllerPin_ListOpen_ThenLoadMore_AppendsIntoRowStore opens a
// list screen, drives a first-page ResourcesLoaded, then a load-more append,
// through Controller.Handle and asserts RowStore accumulates all three
// pages' worth of rows without duplication — the Controller-wired
// counterpart to TestRowStore_Observe_AppendDedupsByID above (that pin
// drives RowStore.Observe directly; this one drives the same outcome through
// core.HandleEvent's messages.ResourcesLoaded case).
func TestRowStoreControllerPin_ListOpen_ThenLoadMore_AppendsIntoRowStore(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	page1 := []resource.Resource{
		{ID: "bucket-1", Name: "bucket-1", Type: "s3"},
		{ID: "bucket-2", Name: "bucket-2", Type: "s3"},
	}
	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    page1,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-1"},
		Append:       false,
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})
	assertRowStoreControllerPinHasIDs(t, s, "s3", "bucket-1", "bucket-2")

	page2 := []resource.Resource{
		{ID: "bucket-3", Name: "bucket-3", Type: "s3"},
	}
	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    page2,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Append:       true,
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})

	final := s.RowStore.Snapshot("s3")
	if len(final.Rows) != 3 {
		t.Fatalf("after load-more append, RowStore.Snapshot(s3).Rows = %+v, want 3 rows total", final.Rows)
	}
	assertRowStoreControllerPinHasIDs(t, s, "s3", "bucket-1", "bucket-2", "bucket-3")
}

// rowStoreControllerPinSentinelWave2Type is a catalog-absent Wave-2 short
// name used to keep the enrichment queue open past the "ec2" completion in
// TestRowStoreControllerPin_EnrichmentChecked_FieldUpdates_FoldsIntoRowStore.
// A single-type queue would make that one EnrichmentChecked delivery the
// FINAL one (EnrichChecked >= EnrichTotal) — see
// TestRowStoreControllerPin_EnrichmentChecked_AllDone_RowStoreSurvives for
// the dedicated all-done-completion pin. Registering this second sentinel
// keeps EnrichTotal at 2 so the ec2 delivery is a genuine partial
// completion.
const rowStoreControllerPinSentinelWave2Type = "rowstore-ctrl-pin-sentinel-wave2"

// TestRowStoreControllerPin_EnrichmentChecked_FieldUpdates_FoldsIntoRowStore
// drives a Wave 2 enrichment completion carrying FieldUpdates through
// Controller.Handle and asserts RowStore's amended rows (via AmendRows)
// carry the merged FieldUpdates values.
func TestRowStoreControllerPin_EnrichmentChecked_FieldUpdates_FoldsIntoRowStore(t *testing.T) {
	awsclient.SetWave2EnricherForTest(t, rowStoreControllerPinSentinelWave2Type, awsclient.IssueEnricher{
		Fn:       awsclient.InFetcherWave2Sentinel,
		Priority: 100,
	})

	s, _, c := newRowStoreControllerPin(t)

	seed := []resource.Resource{
		{ID: "i-enrich-1", Name: "enrich-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources:    seed,
	})

	// Seed the sentinel type's own RowStore entry too — BuildEnrichQueue
	// (core/runtime/probes.go) only enqueues a Wave-2 entry when
	// RowStore.Snapshot(type).Gen != 0 (observed-at-all).
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: rowStoreControllerPinSentinelWave2Type,
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources: []resource.Resource{
			{ID: "sentinel-1", Name: "sentinel-1", Type: rowStoreControllerPinSentinelWave2Type},
		},
	})

	if s.EnrichTotal != 2 {
		t.Fatalf("precondition: want EnrichTotal=2 (ec2 + sentinel) after both AvailabilityChecked deliveries drained the avail queue and startEnrichment ran, got %d", s.EnrichTotal)
	}

	_, _ = c.Handle(messages.EnrichmentChecked{
		ResourceType: "ec2",
		Gen:          s.EnrichmentGen,
		TypeGen:      s.EnrichmentTypeGen["ec2"],
		FieldUpdates: map[string]map[string]string{
			"i-enrich-1": {"cost_estimate": "12.50"},
		},
	})

	if s.EnrichChecked >= s.EnrichTotal {
		t.Fatalf("precondition: want a partial enrichment completion (EnrichChecked < EnrichTotal) so handleEnrichmentChecked's all-done free does not fire; got EnrichChecked=%d EnrichTotal=%d", s.EnrichChecked, s.EnrichTotal)
	}

	assertRowStoreControllerPinHasIDs(t, s, "ec2", "i-enrich-1")

	snap := s.RowStore.Snapshot("ec2")
	if len(snap.Rows) != 1 || snap.Rows[0].Fields["cost_estimate"] != "12.50" {
		t.Errorf("RowStore.Snapshot(ec2).Rows = %+v, want Fields[cost_estimate]=12.50 from the applyEnrichment FieldUpdates fold", snap.Rows)
	}
}

// TestRowStoreControllerPin_EnrichmentChecked_AllDone_RowStoreSurvives pins
// the D12-class survival behavior RowStore exists for: when a single-type
// enrichment queue drains on the FIRST EnrichmentChecked delivery
// (handleEnrichmentChecked's "all done" branch), RowStore's rows for that
// type are retained after the sweep completes.
func TestRowStoreControllerPin_EnrichmentChecked_AllDone_RowStoreSurvives(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	seed := []resource.Resource{
		{ID: "i-enrich-2", Name: "enrich-2", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources:    seed,
	})

	if s.EnrichTotal != 1 {
		t.Fatalf("precondition: want EnrichTotal=1 (ec2 only) so the next EnrichmentChecked is the terminal one, got %d", s.EnrichTotal)
	}

	_, _ = c.Handle(messages.EnrichmentChecked{
		ResourceType: "ec2",
		Gen:          s.EnrichmentGen,
		TypeGen:      s.EnrichmentTypeGen["ec2"],
		FieldUpdates: map[string]map[string]string{
			"i-enrich-2": {"cost_estimate": "9.99"},
		},
	})

	if s.EnrichChecked < s.EnrichTotal {
		t.Fatalf("precondition: want the all-done branch to have fired (EnrichChecked >= EnrichTotal), got EnrichChecked=%d EnrichTotal=%d", s.EnrichChecked, s.EnrichTotal)
	}

	snap := s.RowStore.Snapshot("ec2")
	if len(snap.Rows) != 1 || snap.Rows[0].Fields["cost_estimate"] != "9.99" {
		t.Errorf("RowStore.Snapshot(ec2).Rows = %+v, want the merged cost_estimate=9.99 row to survive enrichment-sweep completion", snap.Rows)
	}
}

// TestRowStoreControllerPin_RelatedCheckResult_DualLane_BothWriteRowStore
// drives a related lazy-add result through BOTH lanes that legitimately
// exist for this message today:
//
//  1. runtime.Core.HandleRelatedCheckResult (the intent-returning method) +
//     Controller.ApplyIntents — populates RowStore's Partial entry via
//     PatchLazyResourceCache, exactly as the TUI adapter and the headless
//     RelatedCheckBatch executor do.
//  2. app.Controller.Handle(messages.RelatedCheckResult{...}) — feeds
//     RowStore via core.HandleEvent's dedicated observeRelatedCheckResultRows
//     case, deliberately side-effect-only so Controller.Handle never
//     double-applies path 1's intents for a bare RelatedCheckResult.
//
// A real production caller normally only exercises ONE of these two paths
// per message; driving both here is deliberate — it is the only test that
// exercises path 2 (a bare Controller.Handle(messages.RelatedCheckResult{})
// call) at all.
func TestRowStoreControllerPin_RelatedCheckResult_DualLane_BothWriteRowStore(t *testing.T) {
	s, core, c := newRowStoreControllerPin(t)

	lazyRows := []resource.Resource{
		{ID: "key-lazy-1", Name: "lazy-key-1", Type: "kms"},
	}

	intents, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:       "ec2",
		SourceResourceID:   "i-source-1",
		DefDisplayName:     "KMS Keys",
		LazyAddedResources: map[string][]resource.Resource{"kms": lazyRows},
	})
	c.ApplyIntents(intents)

	_, _ = c.Handle(messages.RelatedCheckResult{
		ResourceType:       "ec2",
		SourceResourceID:   "i-source-1",
		DefDisplayName:     "KMS Keys",
		OperationID:        core.ActiveDetailOp(),
		LazyAddedResources: map[string][]resource.Resource{"kms": lazyRows},
	})

	afterLane1 := s.RowStore.Snapshot("kms")
	if len(afterLane1.Rows) != 1 || afterLane1.Rows[0].ID != "key-lazy-1" {
		t.Fatalf("precondition: RowStore.Snapshot(kms) after lane 1 (PatchLazyResourceCache) = %+v, want [key-lazy-1]", afterLane1.Rows)
	}

	all := s.RowStore.SnapshotAll(true)
	storeSnap := all["kms"]
	storeIDs := rowStoreControllerPinIDSet(storeSnap.Rows)
	if !storeIDs["key-lazy-1"] {
		t.Errorf("RowStore partial view for kms = %+v, want key-lazy-1 present", storeSnap.Rows)
	}
	if !storeSnap.Partial {
		t.Error("RowStore SnapshotAll(true)[kms].Partial = false, want true — a lazy-add observation must mark Partial")
	}

	// Scope boundary (C6): a lazy-add row must never poison the canonical
	// (non-partial) view of a type it was added under.
	canonical := s.RowStore.SnapshotAll(false)
	if tr, ok := canonical["kms"]; ok {
		for _, r := range tr.Rows {
			if r.ID == "key-lazy-1" {
				t.Error("lazy-added row leaked into the canonical (non-partial) RowStore view — violates C6 scope boundary")
			}
		}
	}
}

// -----------------------------------------------------------------------
// Pin 10 — Observe/ObservePartial/Amend clone-on-ingress/egress (D13's
// two-mutex aliasing fix): rows is deep-copied (cloneRows) on entry to
// Observe/ObservePartial and Amend's fn receives a deep copy too, and every
// returned/retained row set is likewise a deep copy — the store's own
// backing array/Fields maps are never the same allocation as what a caller
// passed in or received back. Before this fix, the caller's per-screen
// ListState.Rows and RowStore.mu each held a reference into the SAME
// backing array; a caller mutating either side (including an append into
// spare capacity) silently corrupted the other with no Gen bump, and two
// goroutines touching the two sides raced under `go test -race`. Pin 6
// above covers Snapshot/SnapshotAll's PRE-EXISTING clone immunity; these
// pin the NEW ingress/egress clones this fix adds to Observe/ObservePartial/
// Amend specifically.
// -----------------------------------------------------------------------

// TestRowStore_Observe_MutatingPassedInSliceAfterCallDoesNotAffectStore pins
// clone-on-ingress: mutating the slice (including a row's Fields map) the
// caller passed INTO Observe, after the call returns, must never reach the
// store's retained rows.
func TestRowStore_Observe_MutatingPassedInSliceAfterCallDoesNotAffectStore(t *testing.T) {
	store := session.NewRowStore()

	rows := []resource.Resource{
		{ID: "i-1", Name: "original-name", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	store.Observe("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	// Mutate the caller's own slice AFTER the call — including the row's
	// Fields map, which cloneRows must give its own backing map too.
	rows[0].Name = "MUTATED-AFTER-OBSERVE"
	rows[0].Fields["state"] = "poisoned"
	rows[0].Fields["poison"] = "yes"

	snap := store.Snapshot("ec2")
	if len(snap.Rows) != 1 {
		t.Fatalf("precondition: Snapshot(ec2).Rows = %d, want 1", len(snap.Rows))
	}
	if snap.Rows[0].Name != "original-name" {
		t.Errorf("Snapshot(ec2).Rows[0].Name = %q after the caller mutated its own passed-in slice post-call, want %q unchanged", snap.Rows[0].Name, "original-name")
	}
	if snap.Rows[0].Fields["state"] != "running" {
		t.Errorf("Snapshot(ec2).Rows[0].Fields[state] = %q, want %q unchanged — Observe must clone the Fields map on ingress too", snap.Rows[0].Fields["state"], "running")
	}
	if _, poisoned := snap.Rows[0].Fields["poison"]; poisoned {
		t.Error("Snapshot(ec2).Rows[0].Fields[poison] leaked from the caller's post-call mutation of its passed-in slice")
	}
}

// TestRowStore_Observe_MutatingReturnedSliceAndAppendIntoSpareCapacityDoesNotAffectStore
// pins clone-on-egress: mutating the slice Observe RETURNED — including a
// row's Fields map, AND an append that fits in the returned slice's spare
// capacity (the classic shared-backing-array corruption shape: slicing the
// returned slice down by one leaves cap > len, so appending a new row
// reuses that same backing array without reallocating) — must never affect
// the store's retained rows or bump its Gen.
func TestRowStore_Observe_MutatingReturnedSliceAndAppendIntoSpareCapacityDoesNotAffectStore(t *testing.T) {
	store := session.NewRowStore()

	returned, gen1 := store.Observe("ec2", []resource.Resource{
		{ID: "i-1", Name: "row-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
		{ID: "i-2", Name: "row-2", Type: "ec2"},
	}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	if len(returned) != 2 {
		t.Fatalf("precondition: Observe returned %d rows, want 2", len(returned))
	}

	// Mutate a returned row's Fields map directly.
	if returned[0].Fields == nil {
		t.Fatal("precondition: returned[0].Fields must be non-nil (seeded above)")
	}
	returned[0].Fields["state"] = "poisoned"

	// Append into the returned slice's spare capacity: slicing down by one
	// leaves cap(sub) == cap(returned) > len(sub), so this append reuses
	// the SAME backing array as `returned` without reallocating — silently
	// overwriting returned[1] in the caller's own copy. Since `returned` is
	// already a private clone (not the store's own slice), this must be
	// invisible to the store.
	sub := returned[:len(returned)-1]
	appended := append(sub, resource.Resource{ID: "poison-append", Name: "poison-append", Type: "ec2"})
	if appended[len(appended)-1].ID != "poison-append" {
		t.Fatal("test setup problem: append into spare capacity did not land as expected")
	}

	snap := store.Snapshot("ec2")
	if len(snap.Rows) != 2 {
		t.Fatalf("Snapshot(ec2).Rows = %d rows after a caller mutated its returned slice, want 2 unchanged (a poisoned 3rd row must never reach the store)", len(snap.Rows))
	}
	if snap.Rows[0].Fields["state"] != "running" {
		t.Errorf("Snapshot(ec2).Rows[0].Fields[state] = %q, want %q unchanged — Observe must clone the Fields map on egress too", snap.Rows[0].Fields["state"], "running")
	}
	if snap.Rows[1].ID == "poison-append" {
		t.Error("Snapshot(ec2).Rows[1] is the caller's spare-capacity append — the store must never alias the slice it returns")
	}
	if snap.Gen != gen1 {
		t.Errorf("Snapshot(ec2).Gen = %d, want %d (the original Observe's Gen) — a caller-side mutation of the returned slice must never bump Gen on its own", snap.Gen, gen1)
	}
}

// TestRowStore_ObservePartial_MutatingPassedInSliceAfterCallDoesNotAffectStore
// mirrors the Observe ingress pin above for ObservePartial.
func TestRowStore_ObservePartial_MutatingPassedInSliceAfterCallDoesNotAffectStore(t *testing.T) {
	store := session.NewRowStore()

	rows := []resource.Resource{
		{ID: "key-1", Name: "original-name", Type: "kms", Fields: map[string]string{"state": "enabled"}},
	}
	store.ObservePartial("kms", rows)

	rows[0].Name = "MUTATED-AFTER-OBSERVE-PARTIAL"
	rows[0].Fields["state"] = "poisoned"

	snap := store.Snapshot("kms")
	if len(snap.Rows) != 1 || snap.Rows[0].Name != "original-name" {
		t.Errorf("Snapshot(kms).Rows = %+v after the caller mutated its passed-in slice post-call, want Name %q unchanged", snap.Rows, "original-name")
	}
	if snap.Rows[0].Fields["state"] != "enabled" {
		t.Errorf("Snapshot(kms).Rows[0].Fields[state] = %q, want %q unchanged — ObservePartial must clone the Fields map on ingress too", snap.Rows[0].Fields["state"], "enabled")
	}
}

// TestRowStore_ObservePartial_MutatingReturnedSliceAndAppendIntoSpareCapacityDoesNotAffectStore
// mirrors the Observe egress + spare-capacity-append pin above for
// ObservePartial.
func TestRowStore_ObservePartial_MutatingReturnedSliceAndAppendIntoSpareCapacityDoesNotAffectStore(t *testing.T) {
	store := session.NewRowStore()

	returned, gen1 := store.ObservePartial("kms", []resource.Resource{
		{ID: "key-1", Name: "key-1", Type: "kms", Fields: map[string]string{"state": "enabled"}},
		{ID: "key-2", Name: "key-2", Type: "kms"},
	})
	if len(returned) != 2 {
		t.Fatalf("precondition: ObservePartial returned %d rows, want 2", len(returned))
	}
	returned[0].Fields["state"] = "poisoned"
	sub := returned[:len(returned)-1]
	_ = append(sub, resource.Resource{ID: "poison-append", Name: "poison-append", Type: "kms"})

	snap := store.Snapshot("kms")
	if len(snap.Rows) != 2 {
		t.Fatalf("Snapshot(kms).Rows = %d rows after a caller mutated its returned slice, want 2 unchanged", len(snap.Rows))
	}
	if snap.Rows[0].Fields["state"] != "enabled" {
		t.Errorf("Snapshot(kms).Rows[0].Fields[state] = %q, want %q unchanged — ObservePartial must clone the Fields map on egress too", snap.Rows[0].Fields["state"], "enabled")
	}
	if snap.Gen != gen1 {
		t.Errorf("Snapshot(kms).Gen = %d, want %d unchanged — a caller-side mutation of the returned slice must never bump Gen on its own", snap.Gen, gen1)
	}
}

// TestRowStore_Amend_FnMutatesInputSliceInPlace_RetainedRowsAndPriorSnapshotUnaffected
// pins Amend's clone-on-ingress: fn receives a deep copy (cloneRows) of the
// existing retained slice, so an fn that mutates its input IN PLACE
// (returning that same, now-mutated slice — exactly the shape
// core/runtime/helpers.go and internal/tui/app_enrich_fold.go's own
// enrich-fold implementations use, each already defensively copying before
// calling Amend) must never corrupt the store's retained rows relative to a
// Snapshot taken BEFORE the Amend call.
func TestRowStore_Amend_FnMutatesInputSliceInPlace_RetainedRowsAndPriorSnapshotUnaffected(t *testing.T) {
	store := session.NewRowStore()

	// The prior reference MUST come from Observe's own return value, not
	// Snapshot's: Snapshot has always cloned (Pin 6, pre-D), so a
	// Snapshot-derived "before" would stay protected even without this
	// fix's Amend-ingress clone and this pin would be accidentally-green.
	// Observe's return was the actual unprotected reference pre-D — exactly
	// what a caller assigns onto its own ListState.Rows.
	before, _ := store.Observe("s3", []resource.Resource{
		{ID: "bucket-1", Name: "bucket-1", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	if len(before) != 1 {
		t.Fatalf("precondition: Observe returned %d rows, want 1", len(before))
	}

	store.Amend("s3", func(rows []resource.Resource) []resource.Resource {
		// Mutate the fn's input slice in place — including its Fields map —
		// and return that SAME slice, exactly the "didn't bother to copy"
		// shape Amend's clone-on-ingress exists to make harmless. Pre-fix,
		// rows here WAS the store's own backing array, which WAS also
		// `before`'s backing array (Observe returned it directly) — so this
		// in-place mutation would silently corrupt `before` too.
		rows[0].Name = "MUTATED-IN-PLACE-BY-FN"
		rows[0].Fields["region"] = "eu-west-1"
		return rows
	})

	// The slice returned by the EARLIER Observe call must be completely
	// unaffected by the in-place mutation Amend's fn performed on its
	// (cloned) input.
	if before[0].Name != "bucket-1" {
		t.Errorf("pre-Amend Observe-returned Rows[0].Name = %q after Amend's fn mutated its input in place, want %q unchanged — Amend must clone before handing rows to fn", before[0].Name, "bucket-1")
	}
	if before[0].Fields["region"] != "us-east-1" {
		t.Errorf("pre-Amend Observe-returned Rows[0].Fields[region] = %q, want %q unchanged", before[0].Fields["region"], "us-east-1")
	}

	// The store's own retained rows DO reflect the fn's returned (mutated)
	// result — Amend applies whatever fn returns; only the EARLIER
	// Observe-returned reference and the fn's own input clone are protected.
	after := store.Snapshot("s3")
	if after.Rows[0].Name != "MUTATED-IN-PLACE-BY-FN" {
		t.Errorf("post-Amend Snapshot.Rows[0].Name = %q, want %q — Amend must still apply fn's returned replacement", after.Rows[0].Name, "MUTATED-IN-PLACE-BY-FN")
	}
}

// TestRowStore_ConcurrentReturnedSliceWrite_vs_SnapshotObserve_NoRace pins
// the actual crash mechanism directly, distinct from the existing Pin 8
// concurrency test above: Pin 8 only exercises concurrent CALLS to
// Observe/Amend/Snapshot (each internally mutex-guarded, so nothing there
// ever raced even before this fix). The real crash mechanism this fix
// closes is a goroutine writing through a slice/map an EARLIER
// Observe/ObservePartial call had returned — entirely OUTSIDE the store's
// mutex — while a second goroutine concurrently called Observe/Snapshot
// against the SAME type. Before clone-on-egress, that returned slice/map
// WAS the store's own backing allocation, so the two goroutines raced on
// the same memory with no lock between them; go test -race must report
// nothing here.
func TestRowStore_ConcurrentReturnedSliceWrite_vs_SnapshotObserve_NoRace(t *testing.T) {
	store := session.NewRowStore()
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine A: repeatedly Observe, then mutate the slice/map it got
	// back — entirely outside any lock, exactly like a caller assigning an
	// Observe result onto its own ListState.Rows and then editing it.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			returned, _ := store.Observe("ec2", []resource.Resource{
				{ID: idFor(i % 10), Name: idFor(i % 10), Type: "ec2", Fields: map[string]string{"k": "v"}},
			}, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, true)
			for j := range returned {
				returned[j].Name = "writer-A"
				if returned[j].Fields != nil {
					returned[j].Fields["k"] = "writer-A"
				}
			}
		}
	}()

	// Goroutine B: concurrently Snapshot and Observe the SAME type — if
	// goroutine A's returned slice ever aliased the store's own backing
	// array, this races with A's unguarded writes above.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = store.Snapshot("ec2")
			store.Observe("ec2", []resource.Resource{
				{ID: idFor((i + 1) % 10), Name: idFor((i + 1) % 10), Type: "ec2"},
			}, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, true)
		}
	}()

	wg.Wait()

	if got := store.Snapshot("ec2"); len(got.Rows) == 0 {
		t.Error("final Snapshot(ec2).Rows is empty after the concurrent returned-slice-write/Snapshot-Observe run — expected at least the deduped rows to survive")
	}
}
