// session_rowstore_test.go — Stage-1 pin suite for the row-store
// unification plan (rowstore-unification-plan.md, Stage 1: "Introduce
// RowStore behind existing maps, dual-write scaffolding, zero behavior
// change"). Pins the CONTRACT of internal/session.RowStore /
// internal/session.TypeRows (internal/session/rowstore.go) against
// docs/design/cache-requirements.md C2/C5/C6/C6a/C6b/C9 and the defects
// those rules rule out (D7, D12-D17), mirroring the semantics
// internal/runtime/probes.go:reconcileTypeFile already enforces for the
// on-disk file and internal/app/list_body.go's dedupAgainstExisting/
// isStaleReplace already enforce for the per-screen ListState.
//
// API pinned here (internal/session/rowstore.go, landed):
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

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
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
// Pin 2 — stale truncated ID-subset replace rejected once exact (DEF-18/D14).
// -----------------------------------------------------------------------

// TestRowStore_Observe_StaleTruncatedSubsetRejectedOnceExact mirrors
// isStaleReplaceRows (which mirrors internal/app/list_body.go's
// isStaleReplace and internal/runtime/probes.go's reconcileTypeFile rule 1):
// a non-append, truncated replay whose row IDs are a strict subset of the
// already-stored fuller set must be REJECTED — the existing (fuller) rows
// come back unchanged and the store's own state is untouched — the
// in-memory analogue of D14 ("a stale page-1 replace landing after a deeper
// load-more stomped the 55-row list back to 50+").
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
	if len(accepted) != 55 {
		t.Errorf("Observe(stale truncated ID-subset replace) returned %d rows, want 55 (existing rows unchanged, rejected) — DEF-18/D14", len(accepted))
	}
	if genAfterStale != genAfterFull {
		t.Errorf("Gen after rejected stale replace = %d, want unchanged %d — a rejected observation must not bump Gen", genAfterStale, genAfterFull)
	}

	snap := store.Snapshot("s3")
	if len(snap.Rows) != 55 {
		t.Fatalf("len(Rows) = %d, want 55 — a rejected stale replay must not shrink the stored rows", len(snap.Rows))
	}
	if snap.Pagination == nil || snap.Pagination.IsTruncated {
		t.Errorf("Pagination = %+v, want IsTruncated=false preserved — the rejected stale replay must not downgrade exactness", snap.Pagination)
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
// Pin 5 — Amend is copy-on-write (structural DEF-7 pin); Gen bumped.
// -----------------------------------------------------------------------

// TestRowStore_Amend_CopyOnWrite_EarlierSnapshotUnchanged pins DEF-7: taking
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
		t.Errorf("earlier Snapshot's row Findings = %+v, want unchanged (empty) — Amend must be copy-on-write, never mutate a prior Snapshot's rows (DEF-7)", before.Rows[0].Findings)
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
