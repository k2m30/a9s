// review_wave_cache_defects_test.go pins five externally-reviewed defects
// against HEAD (feat/related-completeness). Each test is RED until the
// paired coder task lands the fix; assertions encode the documented/correct
// mechanism, not today's (broken) behavior. Fictional data only (account
// 123456789012).
//
// Pin 1 — core/session/rowstore.go Observe: a probe replace must not
// shrink an existing Fetch-origin row set (append=false, smaller incoming).
// Pin 2 — core/runtime/handlers_availability.go (~line 124): a disk seed
// WITH real rows but a C6a-style larger Count must still land the larger
// TotalCount, not silently drop it to len(rows).
// Pin 3 — core/runtime/accessors.go ResourceCacheKeys / a new
// origin-gated FetchOriginCacheKeys: the related-freshness key set must
// exclude Disk/Probe-origin entries, matching HasResourceCache's own
// Origin==OriginFetch gate. This pin defines the fix's contract: a
// FetchOriginCacheKeys accessor does not exist yet (red = missing symbol).
// Pin 4 — core/runtime/helpers.go ApplyWave2ToRow: the in-place
// compaction of r.Findings mutates the input row's backing array, so any
// other domain.Resource value sharing that slice header is corrupted too.
// Pin 5 — core/aws/related_common.go
// lambdaEventSourceMappingLambdaCheck: when only SOME ListEventSourceMappings
// FunctionArns are present in the lambda ResourceCache, the checker must
// union the cache-matched IDs with ARN-parsed IDs for the cache-missing
// ones, not silently drop the unmatched ARNs.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// -----------------------------------------------------------------------
// Pin 1 — probe replace must not shrink fetched rows.
// -----------------------------------------------------------------------

// TestRowStore_Observe_ProbeReplaceNeverShrinksFetchRows pins the review
// finding: an existing OriginFetch entry with 4 rows and IsTruncated=true
// (the user has explicitly loaded more, so this is NOT yet a confirmed exact
// total) must not be regressed by a later OriginProbe replace (append=false)
// carrying fewer rows. Observe's existing isStaleReplaceRows guard only
// fires when existing.Pagination reports IsTruncated=false (exact) — a
// still-truncated Fetch entry sails past that guard today and gets replaced
// wholesale by the smaller Probe payload, which is the bug this pins.
func TestRowStore_Observe_ProbeReplaceNeverShrinksFetchRows(t *testing.T) {
	store := session.NewRowStore()

	fetchRows := []resource.Resource{
		{ID: "a", Name: "a", Type: "ec2"},
		{ID: "b", Name: "b", Type: "ec2"},
		{ID: "c", Name: "c", Type: "ec2"},
		{ID: "d", Name: "d", Type: "ec2"},
	}
	store.Observe("ec2", fetchRows, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, false)

	probeRows := []resource.Resource{
		{ID: "a", Name: "a", Type: "ec2"},
		{ID: "b", Name: "b", Type: "ec2"},
	}
	accepted, _ := store.Observe("ec2", probeRows, &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)
	if len(accepted) != 4 {
		t.Errorf("Observe(Probe replace over Fetch, smaller) returned %d rows, want 4 (existing Fetch-origin rows retained) — a probe must never regress a Fetch-origin row set", len(accepted))
	}

	snap := store.Snapshot("ec2")
	if len(snap.Rows) != 4 {
		t.Fatalf("len(Rows) = %d, want 4 — a smaller Probe replace must not shrink a larger Fetch-origin row set", len(snap.Rows))
	}
	if snap.Origin != session.OriginFetch {
		t.Errorf("Origin = %v, want OriginFetch preserved after a rejected Probe shrink attempt", snap.Origin)
	}
}

// TestRowStore_Observe_ProbeReplacesProbe_NonRegressionStillAllowed is the
// non-regression companion: a Probe replace over an existing Probe-origin
// entry must still win (a fresh Wave-1 sweep round legitimately supersedes
// the prior round's retained first page), even though Pin 1 above blocks a
// Probe from shrinking a Fetch-origin entry.
func TestRowStore_Observe_ProbeReplacesProbe_NonRegressionStillAllowed(t *testing.T) {
	store := session.NewRowStore()

	firstRound := []resource.Resource{{ID: "p1", Name: "p1", Type: "ec2"}, {ID: "p2", Name: "p2", Type: "ec2"}}
	store.Observe("ec2", firstRound, &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)

	secondRound := []resource.Resource{{ID: "p3", Name: "p3", Type: "ec2"}}
	accepted, _ := store.Observe("ec2", secondRound, &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)
	if len(accepted) != 1 || accepted[0].ID != "p3" {
		t.Errorf("Observe(Probe replace over Probe) = %+v, want [p3] — a fresh probe round must still supersede the prior probe round's rows", accepted)
	}
}

// TestRowStore_Observe_ProbeReplacesDiskEntry_NonRegressionStillAllowed pins
// the second non-regression case: a Probe replace over an existing
// Disk-origin entry must still win, mirroring
// TestRowStore_Observe_ProbeReplacesDisk in session_rowstore_test.go.
func TestRowStore_Observe_ProbeReplacesDiskEntry_NonRegressionStillAllowed(t *testing.T) {
	store := session.NewRowStore()

	diskRows := []resource.Resource{{ID: "disk-1", Name: "disk-1", Type: "ec2"}, {ID: "disk-2", Name: "disk-2", Type: "ec2"}}
	store.Observe("ec2", diskRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginDisk, false)

	probeRows := []resource.Resource{{ID: "probe-1", Name: "probe-1", Type: "ec2"}}
	accepted, _ := store.Observe("ec2", probeRows, &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)
	if len(accepted) != 1 || accepted[0].ID != "probe-1" {
		t.Errorf("Observe(Probe replace over Disk) = %+v, want [probe-1] — a Probe observation must still replace a Disk-origin entry even after Pin 1's Fetch-shrink guard lands", accepted)
	}
}

// TestRowStore_Observe_FetchReplacesFetch_CtrlR_ResetSemanticsPreserved pins
// the Ctrl+R reset non-regression named in the dispatch: a Fetch replace
// over an existing Fetch entry must stay allowed even when it carries fewer
// rows than the existing (still-truncated) entry — mirrors
// TestStoryF1_CtrlR_ResetsPagination's contract, which this pin must not
// break while fixing Pin 1's Probe-vs-Fetch guard.
func TestRowStore_Observe_FetchReplacesFetch_CtrlR_ResetSemanticsPreserved(t *testing.T) {
	store := session.NewRowStore()

	original := []resource.Resource{
		{ID: "x", Name: "x", Type: "ec2"},
		{ID: "y", Name: "y", Type: "ec2"},
		{ID: "z", Name: "z", Type: "ec2"},
	}
	store.Observe("ec2", original, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, false)

	// Ctrl+R full reset: replays exactly page 1 (a strict ID subset, smaller,
	// still truncated) via a fresh Fetch-origin observation.
	resetPage1 := original[:2]
	accepted, _ := store.Observe("ec2", resetPage1, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, false)
	if len(accepted) != 2 {
		t.Errorf("Observe(Fetch replace over Fetch, Ctrl+R reset shape) returned %d rows, want 2 — a same-origin Fetch replace must win (Ctrl+R reset semantics), not be rejected as a stale shrink", len(accepted))
	}
}

// -----------------------------------------------------------------------
// Pin 2 — disk seed with real rows must preserve the larger C6a count.
// -----------------------------------------------------------------------

// TestRowStoreControllerPin_AvailabilityCacheLoaded_RealDiskRowsWithLargerCount_PreservesCount
// pins the review finding at handlers_availability.go's disk-seed branch
// (~line 124): when the on-disk per-type file has SOME real rows (3) but a
// C6a-style larger authoritative Count (7, count > rows — e.g. a
// counts-only sync-back landed after the disk file's own row page was
// written), the seed must still land TotalCount=7 in RowStore, not silently
// downgrade to len(rows)=3. Today's code feeds ObserveRows with a
// PaginationMeta built only from Truncated, so RowStore.Observe sets
// TotalCount=len(newRows)=3 (see Observe's TotalCount: len(newRows) line),
// discarding the disk-cache-loaded event's own larger Count.
func TestRowStoreControllerPin_AvailabilityCacheLoaded_RealDiskRowsWithLargerCount_PreservesCount(t *testing.T) {
	s, core, c := newRowStoreControllerPin(t)

	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store")
	}
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        7,
		Exact:        false,
		Rows: []cache.Row{
			{ID: "i-diskrow-1", Name: "disk-row-1", Fields: map[string]string{"state": "running"}},
			{ID: "i-diskrow-2", Name: "disk-row-2", Fields: map[string]string{"state": "running"}},
			{ID: "i-diskrow-3", Name: "disk-row-3", Fields: map[string]string{"state": "stopped"}},
		},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("seed fixture SaveType(ec2): %v", err)
	}

	_, _ = c.Handle(messages.AvailabilityCacheLoaded{
		Entries:   map[string]int{"ec2": 7},
		Truncated: map[string]bool{"ec2": true},
	})

	snap := s.RowStore.Snapshot("ec2")
	if len(snap.Rows) != 3 {
		t.Fatalf("precondition: RowStore.Snapshot(ec2).Rows = %+v, want 3 real disk rows seeded", snap.Rows)
	}
	if snap.TotalCount != 7 {
		t.Errorf("RowStore.Snapshot(ec2).TotalCount = %d, want 7 (C6a: the disk-cache-loaded event's larger authoritative count must survive a rows-carrying disk seed, count > rows)", snap.TotalCount)
	}
}

// -----------------------------------------------------------------------
// Pin 3 — related freshness gating must ignore Disk/Probe-origin entries.
// -----------------------------------------------------------------------

// TestCore_FetchOriginCacheKeys_ExcludesDiskAndProbeOrigin pins the review
// finding: core/runtime/accessors.go's ResourceCacheKeys (backing
// executor.go's mainCacheKeys, which gates RelatedDef.NeedsTargetCache
// freshness) filters SnapshotAll(false) only on Partial, never on Origin —
// so a Disk- or Probe-origin, non-Partial, rows-carrying entry is
// (incorrectly) treated as "fresh" alongside a genuine OriginFetch entry,
// unlike HasResourceCache which explicitly requires
// tr.Origin == session.OriginFetch. This pin names the fix's intended
// contract as a NEW accessor, FetchOriginCacheKeys, origin-gated identically
// to HasResourceCache — red because core/runtime.Core has no such method
// yet (compile-time missing symbol).
func TestCore_FetchOriginCacheKeys_ExcludesDiskAndProbeOrigin(t *testing.T) {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)

	// Type X: Disk-origin, rows-carrying, non-Partial — must NOT be treated
	// as a fresh main-cache key for related-target freshness gating.
	s.RowStore.Observe("x-disk-type", []resource.Resource{{ID: "disk-1", Type: "x-disk-type"}}, &resource.PaginationMeta{IsTruncated: false}, session.OriginDisk, false)

	// Type Y: Fetch-origin, rows-carrying, non-Partial — must be included.
	s.RowStore.Observe("y-fetch-type", []resource.Resource{{ID: "fetch-1", Type: "y-fetch-type"}}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	keys := core.FetchOriginCacheKeys()
	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}

	if keySet["x-disk-type"] {
		t.Errorf("FetchOriginCacheKeys() = %v, want x-disk-type (OriginDisk) EXCLUDED — matches HasResourceCache's OriginFetch-only gating", keys)
	}
	if !keySet["y-fetch-type"] {
		t.Errorf("FetchOriginCacheKeys() = %v, want y-fetch-type (OriginFetch) INCLUDED", keys)
	}

	// Cross-check against HasResourceCache's own per-type verdict so the two
	// accessors cannot silently drift apart again.
	if core.HasResourceCache("x-disk-type") != keySet["x-disk-type"] {
		t.Errorf("FetchOriginCacheKeys membership for x-disk-type (%v) disagrees with HasResourceCache (%v)", keySet["x-disk-type"], core.HasResourceCache("x-disk-type"))
	}
	if core.HasResourceCache("y-fetch-type") != keySet["y-fetch-type"] {
		t.Errorf("FetchOriginCacheKeys membership for y-fetch-type (%v) disagrees with HasResourceCache (%v)", keySet["y-fetch-type"], core.HasResourceCache("y-fetch-type"))
	}
}

// -----------------------------------------------------------------------
// Pin 4 — ApplyWave2ToRow must not mutate the input row's Findings backing
// array (aliasing / shared-snapshot corruption).
// -----------------------------------------------------------------------

// TestApplyWave2ToRow_DoesNotMutateSharedFindingsBackingArray pins the
// review finding: ApplyWave2ToRow's in-place compaction
// (r.Findings[n] = f; ... ; r.Findings = r.Findings[:n]) writes into the
// caller-owned backing array before re-slicing, so any OTHER
// domain.Resource value that shares the same Findings slice header (e.g. a
// Snapshot taken before the Amend closure ran, which copies the Resource
// struct but not per-row Findings/Fields — see RowStore.Amend's copy-on-write
// doc comment: fn is responsible for its own copy-on-write) observes its
// Findings silently corrupted even though its own value was never passed to
// ApplyWave2ToRow.
func TestApplyWave2ToRow_DoesNotMutateSharedFindingsBackingArray(t *testing.T) {
	wave1 := domain.Finding{Code: "wave1.finding", Phrase: "wave1 phrase", Severity: domain.SevBroken, Source: "wave1"}
	wave2Original := domain.Finding{Code: "wave2.original", Phrase: "original wave2 phrase", Severity: domain.SevBroken, Source: "wave2:ec2"}

	sharedFindings := []domain.Finding{wave1, wave2Original}

	live := domain.Resource{ID: "i-shared-1", Name: "shared-1", Type: "ec2", Findings: sharedFindings}
	snapshot := domain.Resource{ID: "i-shared-1", Name: "shared-1", Type: "ec2", Findings: sharedFindings}

	td := resource.ResourceTypeDef{ShortName: "ec2"}
	newWave2 := domain.Finding{Code: "wave2.updated", Phrase: "updated wave2 phrase", Severity: domain.SevBroken, Source: "wave2:ec2"}
	runtime.ApplyWave2ToRow(&live, td, map[string][]domain.Finding{"i-shared-1": {newWave2}}, nil)

	if len(snapshot.Findings) != 2 {
		t.Fatalf("snapshot.Findings = %+v, want len 2 unchanged — ApplyWave2ToRow must never mutate a shared Findings backing array (aliasing bug)", snapshot.Findings)
	}
	if snapshot.Findings[0].Code != wave1.Code || snapshot.Findings[0].Phrase != wave1.Phrase {
		t.Errorf("snapshot.Findings[0] = %+v, want unchanged wave1 finding %+v — the earlier snapshot's row must not observe live's mutation", snapshot.Findings[0], wave1)
	}
	if snapshot.Findings[1].Code != wave2Original.Code || snapshot.Findings[1].Phrase != wave2Original.Phrase {
		t.Errorf("snapshot.Findings[1] = %+v, want unchanged ORIGINAL wave2 finding %+v — got the live row's mutation leaking through the shared backing array", snapshot.Findings[1], wave2Original)
	}

	if len(live.Findings) != 2 || live.Findings[1].Code != newWave2.Code {
		t.Fatalf("live.Findings = %+v, want [wave1, wave2.updated] — the mutated row itself must still reflect the new wave2 finding", live.Findings)
	}
}

// -----------------------------------------------------------------------
// Pin 5 — lambda ESM count must union cache-matched and cache-missing
// mappings.
// -----------------------------------------------------------------------

type fakeLambdaListEventSourceMappingsUnionPin struct {
	awsclient.LambdaAPI
	byEventSourceArn map[string][]string
}

func (f *fakeLambdaListEventSourceMappingsUnionPin) ListEventSourceMappings(_ context.Context, params *lambda.ListEventSourceMappingsInput, _ ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	if params.EventSourceArn == nil {
		return &lambda.ListEventSourceMappingsOutput{}, nil
	}
	fnArns, ok := f.byEventSourceArn[*params.EventSourceArn]
	if !ok {
		return &lambda.ListEventSourceMappingsOutput{}, nil
	}
	mappings := make([]lambdatypes.EventSourceMappingConfiguration, 0, len(fnArns))
	for _, arn := range fnArns {
		mappings = append(mappings, lambdatypes.EventSourceMappingConfiguration{FunctionArn: aws.String(arn)})
	}
	return &lambda.ListEventSourceMappingsOutput{EventSourceMappings: mappings}, nil
}

// TestKinesis_Related_Lambda_PartialCacheMatch_UnionsCachedAndUncachedIDs
// pins the review finding in lambdaEventSourceMappingLambdaCheck
// (core/aws/related_common.go): when the API returns TWO distinct
// FunctionArns and the lambda ResourceCache entry exists (not truncated) but
// only resolves ONE of them by Fields["arn"], today's code only appends
// cache-matched IDs (entry.Resources loop) and drops the cache-missing ARN
// entirely instead of falling back to lambdaFunctionNameFromARN for it —
// undercounting a real, API-confirmed trigger.
func TestKinesis_Related_Lambda_PartialCacheMatch_UnionsCachedAndUncachedIDs(t *testing.T) {
	streamARN := "arn:aws:kinesis:us-east-1:123456789012:stream/checkout-events"
	cachedFnArn := "arn:aws:lambda:us-east-1:123456789012:function:fn-cached"
	uncachedFnArn := "arn:aws:lambda:us-east-1:123456789012:function:fn-uncached"

	fake := &fakeLambdaListEventSourceMappingsUnionPin{
		byEventSourceArn: map[string][]string{streamARN: {cachedFnArn, uncachedFnArn}},
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	streamRes := resource.Resource{
		ID:   "checkout-events",
		Name: "checkout-events",
		Fields: map[string]string{
			"stream_arn": streamARN,
		},
	}

	cacheEntry := resource.ResourceCacheEntry{
		IsTruncated: false,
		Resources: []resource.Resource{
			{ID: "fn-cached", Name: "fn-cached", Fields: map[string]string{"arn": cachedFnArn}},
		},
	}
	cache := resource.ResourceCache{"lambda": cacheEntry}

	checker := checkerByTarget(t, "kinesis", "lambda")
	result := checker(context.Background(), clients, streamRes, cache)

	if result.Count() != 2 {
		t.Fatalf("Count = %d, want 2 — both ListEventSourceMappings-confirmed FunctionArns must be counted, whether or not each happens to resolve via the lambda ResourceCache", result.Count())
	}

	gotIDs := make(map[string]bool, len(result.ResourceIDs()))
	for _, id := range result.ResourceIDs() {
		gotIDs[id] = true
	}
	if !gotIDs["fn-cached"] {
		t.Errorf("ResourceIDs = %v, want fn-cached present (cache-resolved)", result.ResourceIDs())
	}
	if !gotIDs["fn-uncached"] {
		t.Errorf("ResourceIDs = %v, want fn-uncached present (ARN-parsed fallback for the cache-missing mapping) — today's code drops any FunctionArn absent from the lambda cache instead of unioning in the ARN-parsed bare name", result.ResourceIDs())
	}
}
