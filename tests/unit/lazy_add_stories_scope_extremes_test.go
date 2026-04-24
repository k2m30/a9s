package unit

// lazy_add_stories_scope_extremes_test.go — orchestration unit tests for
// Section B (LA-010..LA-017) and Section H (LA-070..LA-072) of
// tests/stories/lazy_add.md.
//
// Pattern: synthetic source + target types via unique "test-<la-id>-*" short
// names; t.Cleanup unregisters every Register* call so tests are isolated.
// Mirrors the style of lazy_add_orchestration_edges_test.go.

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ---------------------------------------------------------------------------
// LA-010 — Mixed in-scope + out-of-scope targets resolve together
// ---------------------------------------------------------------------------

// Test_LA_010_MixedInScopeAndOutOfScope verifies that when a checker emits
// two IDs — one already in cache (in-scope) and one not in cache
// (out-of-scope) — the LazyAddedResources slice contains only the
// out-of-scope resource, while Result.Count and Result.ResourceIDs preserve
// both IDs.
func Test_LA_010_MixedInScopeAndOutOfScope(t *testing.T) {
	const (
		srcType    = "test-la010-source"
		targetType = "test-la010-target"
	)

	inScopeID  := "customer-kms-la010"
	outScopeID := "aws-managed-kms-la010"

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-010 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       2,
					ResourceIDs: []string{inScopeID, outScopeID},
				}
			},
		},
	})

	var fetchByIDsCalled int32
	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		atomic.AddInt32(&fetchByIDsCalled, 1)
		var out []resource.Resource
		for _, id := range ids {
			out = append(out, resource.Resource{ID: id, Name: id})
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	// Pre-seed the target cache with the in-scope resource via CachedPages.
	inScopeRes := resource.Resource{ID: inScopeID, Name: inScopeID}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResultMsg{
		ResourceType:     srcType,
		SourceResourceID: "src-la010-seed",
		Result:           resource.RelatedCheckResult{TargetType: targetType, Count: 1},
		CachedPages: map[string]resource.ResourceCacheEntry{
			targetType: {
				Resources:   []resource.Resource{inScopeRes},
				IsTruncated: false,
			},
		},
	})

	srcRes := resource.Resource{ID: "src-la010-001"}
	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	// Count must equal the checker's declared count.
	if resultMsg.Result.Count != 2 {
		t.Errorf("Result.Count: got %d, want 2", resultMsg.Result.Count)
	}

	// ResourceIDs must contain both IDs.
	idSet := make(map[string]bool)
	for _, id := range resultMsg.Result.ResourceIDs {
		idSet[id] = true
	}
	if !idSet[inScopeID] {
		t.Errorf("Result.ResourceIDs missing in-scope ID %q; got %v", inScopeID, resultMsg.Result.ResourceIDs)
	}
	if !idSet[outScopeID] {
		t.Errorf("Result.ResourceIDs missing out-of-scope ID %q; got %v", outScopeID, resultMsg.Result.ResourceIDs)
	}

	// LazyAddedResources must contain only the out-of-scope resource (the in-scope
	// one was already in cache and must NOT be re-fetched).
	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LazyAddedResources is nil — out-of-scope resource was not fetched")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("LazyAddedResources[%s]: got %d resources, want 1 (only out-of-scope)", targetType, len(lazySlice))
	}
	if lazySlice[0].ID != outScopeID {
		t.Errorf("LazyAddedResources[%s][0].ID = %q, want %q", targetType, lazySlice[0].ID, outScopeID)
	}

	// FetchByIDs must have been called exactly once (only for the missing ID).
	if atomic.LoadInt32(&fetchByIDsCalled) != 1 {
		t.Errorf("FetchByIDs call count: got %d, want 1", atomic.LoadInt32(&fetchByIDsCalled))
	}
}

// ---------------------------------------------------------------------------
// LA-011 — All out-of-scope targets populate the drill
// ---------------------------------------------------------------------------

// Test_LA_011_AllOutOfScopePopulatesDrill verifies that when a checker emits
// 3 IDs none of which are in the target cache, all 3 are fetched and appear
// in LazyAddedResources.
func Test_LA_011_AllOutOfScopePopulatesDrill(t *testing.T) {
	const (
		srcType    = "test-la011-source"
		targetType = "test-la011-target"
	)

	ids := []string{"policy-aws-001-la011", "policy-aws-002-la011", "policy-aws-003-la011"}

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-011 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       3,
					ResourceIDs: ids,
				}
			},
		},
	})

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, fetchIDs []string) ([]resource.Resource, error) {
		var out []resource.Resource
		for _, id := range fetchIDs {
			out = append(out, resource.Resource{ID: id, Name: id})
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la011-001"}
	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LazyAddedResources is nil — FetchByIDs was not called for out-of-scope targets")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 3 {
		t.Fatalf("LazyAddedResources[%s]: got %d resources, want 3", targetType, len(lazySlice))
	}
}

// ---------------------------------------------------------------------------
// LA-012 — All in-scope targets: no lazy-add needed
// ---------------------------------------------------------------------------

// Test_LA_012_AllInScopeNoLazyAdd verifies that when a checker emits IDs
// that are all already in the target cache, FetchByIDs is never called and
// LazyAddedResources is nil.
func Test_LA_012_AllInScopeNoLazyAdd(t *testing.T) {
	const (
		srcType    = "test-la012-source"
		targetType = "test-la012-target"
	)

	res1 := resource.Resource{ID: "policy-customer-001-la012", Name: "MyAppPolicy"}
	res2 := resource.Resource{ID: "policy-customer-002-la012", Name: "BillingRead"}

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-012 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       2,
					ResourceIDs: []string{res1.ID, res2.ID},
				}
			},
		},
	})

	var fetchByIDsCallCount int32
	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		atomic.AddInt32(&fetchByIDsCallCount, 1)
		return nil, nil
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	// Pre-seed cache with both resources so they are "in-scope" / already loaded.
	m, _ = rootApplyMsg(m, messages.RelatedCheckResultMsg{
		ResourceType:     srcType,
		SourceResourceID: "src-la012-seed",
		Result:           resource.RelatedCheckResult{TargetType: targetType, Count: 2},
		CachedPages: map[string]resource.ResourceCacheEntry{
			targetType: {
				Resources:   []resource.Resource{res1, res2},
				IsTruncated: false,
			},
		},
	})

	srcRes := resource.Resource{ID: "src-la012-001"}
	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	// No lazy-add should have occurred — both IDs were in cache.
	if resultMsg.LazyAddedResources != nil {
		t.Errorf("LazyAddedResources should be nil when all IDs are in cache; got %v", resultMsg.LazyAddedResources)
	}

	// FetchByIDs must NOT have been called.
	if calls := atomic.LoadInt32(&fetchByIDsCallCount); calls != 0 {
		t.Errorf("FetchByIDs call count: got %d, want 0 (all IDs were in cache)", calls)
	}
}

// ---------------------------------------------------------------------------
// LA-013 — Duplicate IDs: already covered by existing test
// ---------------------------------------------------------------------------

// Test_LA_013_DuplicateIDs_Dedup is a coverage-map placeholder.
// The actual behavior is fully verified by
// TestLazyAdd_MissingFromCache_DedupsRepeatedIDsInChecker in
// lazy_add_orchestration_edges_test.go.
func Test_LA_013_DuplicateIDs_Dedup(t *testing.T) {
	t.Skip("covered by TestLazyAdd_MissingFromCache_DedupsRepeatedIDsInChecker in lazy_add_orchestration_edges_test.go")
}

// ---------------------------------------------------------------------------
// LA-014 — Empty pivot (Count=0): OCQ#1
// ---------------------------------------------------------------------------

// Test_LA_014_EmptyPivot_OCQ is skipped: the behavior when Count=0 (whether
// Enter is a no-op or opens the scope-filtered top-level list) is an open
// contract question and not yet specified. See lazy_add.md OCQ #1.
func Test_LA_014_EmptyPivot_OCQ(t *testing.T) {
	t.Skip("OCQ#1 — LA-014: empty pivot (Count=0) drill behavior is unspecified. See lazy_add.md.")
}

// ---------------------------------------------------------------------------
// LA-015 — ARN vs bare name: checker emits full ARN, resource has bare ID
// ---------------------------------------------------------------------------

// Test_LA_015_ARNvsBareNameTolerance verifies that when a checker emits a
// full ARN and FetchByIDs returns a resource with a bare ID, the wire is
// tolerant: Result.ResourceIDs preserves the ARN (the checker-emitted value)
// and LazyAddedResources contains the resource with the bare ID.
func Test_LA_015_ARNvsBareNameTolerance(t *testing.T) {
	const (
		srcType    = "test-la015-source"
		targetType = "test-la015-target"
	)

	fullARN := "arn:aws:iam::aws:policy/AdministratorAccess-la015"
	bareID  := "AdministratorAccess-la015"

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-015 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1,
					ResourceIDs: []string{fullARN},
				}
			},
		},
	})

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		// Returns resource with bare ID regardless of the ARN input.
		return []resource.Resource{{ID: bareID, Name: bareID}}, nil
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la015-001"}
	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	// The checker-emitted ARN must be preserved in ResourceIDs.
	if len(resultMsg.Result.ResourceIDs) == 0 {
		t.Fatal("Result.ResourceIDs is empty — checker ARN was not preserved")
	}
	if resultMsg.Result.ResourceIDs[0] != fullARN {
		t.Errorf("Result.ResourceIDs[0] = %q, want full ARN %q", resultMsg.Result.ResourceIDs[0], fullARN)
	}

	// LazyAddedResources must contain the resource with the bare ID.
	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LazyAddedResources is nil — FetchByIDs was not called")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("LazyAddedResources[%s]: got %d resources, want 1", targetType, len(lazySlice))
	}
	if lazySlice[0].ID != bareID {
		t.Errorf("LazyAddedResources[%s][0].ID = %q, want bare ID %q", targetType, lazySlice[0].ID, bareID)
	}
}

// ---------------------------------------------------------------------------
// LA-016 — UUID vs alias display
// ---------------------------------------------------------------------------

// Test_LA_016_UUIDvsAliasDisplay verifies that when a checker emits a KMS key
// UUID and FetchByIDs returns a resource with an alias field, the alias is
// preserved in the LazyAddedResources entry.
func Test_LA_016_UUIDvsAliasDisplay(t *testing.T) {
	const (
		srcType    = "test-la016-source"
		targetType = "test-la016-target"
	)

	keyUUID := "a1b2c3d4-e5f6-7890-abcd-ef0123456789"
	alias   := "alias/my-cmk-la016"

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-016 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1,
					ResourceIDs: []string{keyUUID},
				}
			},
		},
	})

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		return []resource.Resource{
			{
				ID:   keyUUID,
				Name: keyUUID,
				Fields: map[string]string{
					"alias": alias,
				},
			},
		}, nil
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la016-001"}
	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LazyAddedResources is nil — FetchByIDs was not called")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("LazyAddedResources[%s]: got %d resources, want 1", targetType, len(lazySlice))
	}
	got := lazySlice[0]
	if got.ID != keyUUID {
		t.Errorf("LazyAddedResources[0].ID = %q, want UUID %q", got.ID, keyUUID)
	}
	if got.Fields["alias"] != alias {
		t.Errorf("LazyAddedResources[0].Fields[\"alias\"] = %q, want %q", got.Fields["alias"], alias)
	}
}

// ---------------------------------------------------------------------------
// LA-017 — Inline IAM policy as policy rows: OCQ#2
// ---------------------------------------------------------------------------

// Test_LA_017_InlinePolicy_OCQ is skipped: whether inline IAM policies surface
// as rows in the policy resource list during a drill-through is an open
// contract question. See lazy_add.md OCQ #2.
func Test_LA_017_InlinePolicy_OCQ(t *testing.T) {
	t.Skip("OCQ#2 — LA-017: inline IAM policies as `policy` rows in drill-through is unspecified. See lazy_add.md.")
}

// ---------------------------------------------------------------------------
// LA-070 — 100 IDs drill without timeout
// ---------------------------------------------------------------------------

// Test_LA_070_100IDsDrillWithoutTimeout verifies that a checker emitting 100
// IDs completes within 5 seconds, all 100 resources are returned via
// LazyAddedResources, and there are no duplicate IDs.
func Test_LA_070_100IDsDrillWithoutTimeout(t *testing.T) {
	const (
		srcType    = "test-la070-source"
		targetType = "test-la070-target"
		idCount    = 100
	)

	ids := make([]string, idCount)
	for i := range ids {
		ids[i] = fmt.Sprintf("policy-arn-la070-%03d", i)
	}

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-070 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       idCount,
					ResourceIDs: ids,
				}
			},
		},
	})

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, fetchIDs []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(fetchIDs))
		for i, id := range fetchIDs {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la070-001"}

	done := make(chan messages.RelatedCheckResultMsg, 1)
	go func() {
		_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
			ResourceType:   srcType,
			SourceResource: srcRes,
		})
		resultMsg, found := collectRelatedResult(t, batchCmd)
		if found {
			done <- resultMsg
		} else {
			close(done)
		}
	}()

	select {
	case resultMsg, ok := <-done:
		if !ok {
			t.Fatal("no RelatedCheckResultMsg received")
		}
		if resultMsg.Result.Count != idCount {
			t.Errorf("Result.Count: got %d, want %d", resultMsg.Result.Count, idCount)
		}
		if resultMsg.LazyAddedResources == nil {
			t.Fatal("LazyAddedResources is nil — FetchByIDs was not called")
		}
		lazySlice := resultMsg.LazyAddedResources[targetType]
		if len(lazySlice) != idCount {
			t.Fatalf("LazyAddedResources[%s]: got %d resources, want %d", targetType, len(lazySlice), idCount)
		}
		// Verify no duplicate IDs.
		seen := make(map[string]bool, idCount)
		for _, r := range lazySlice {
			if seen[r.ID] {
				t.Errorf("duplicate ID %q in LazyAddedResources", r.ID)
			}
			seen[r.ID] = true
		}
	case <-time.After(5 * time.Second):
		t.Fatal("LA-070: drill of 100 IDs did not complete within 5 seconds")
	}
}

// ---------------------------------------------------------------------------
// LA-071 — Malformed IDs filtered
// ---------------------------------------------------------------------------

// Test_LA_071_MalformedIDsFiltered pins the missingFromCache behavior for
// malformed inputs: empty strings are filtered, duplicates are deduplicated
// in first-appearance order, but non-empty semantically-invalid strings like
// "arn:aws:" are passed through (the orchestrator does NOT validate semantic
// shape, only empty-string).
//
// Checker emits: ["", "arn:aws:", "kms-valid-la071", "", "kms-valid-la071"]
// Expected FetchByIDs input: ["arn:aws:", "kms-valid-la071"]
//   - "" is filtered (empty string guard in missingFromCache line 399)
//   - "kms-valid-la071" appears once (dedup, first appearance at index 2)
//   - "arn:aws:" is non-empty and passes through (no semantic validation)
func Test_LA_071_MalformedIDsFiltered(t *testing.T) {
	const (
		srcType    = "test-la071-source"
		targetType = "test-la071-target"
	)

	emittedIDs := []string{"", "arn:aws:", "kms-valid-la071", "", "kms-valid-la071"}

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-071 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1, // operator sees 1 valid resource
					ResourceIDs: emittedIDs,
				}
			},
		},
	})

	var capturedIDs []string
	var capturedOnce atomic.Bool

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		if capturedOnce.CompareAndSwap(false, true) {
			cp := make([]string, len(ids))
			copy(cp, ids)
			capturedIDs = cp
		}
		var out []resource.Resource
		for _, id := range ids {
			if id == "kms-valid-la071" {
				out = append(out, resource.Resource{ID: id, Name: id})
			}
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la071-001"}
	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	_, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if !capturedOnce.Load() {
		t.Fatal("FetchByIDs was not called — lazy-add path was not exercised")
	}

	// Pin observed behavior: "" is filtered, duplicates are deduped, but
	// "arn:aws:" (non-empty, semantically invalid) passes through.
	// Expected: ["arn:aws:", "kms-valid-la071"] (first-appearance order from
	// the non-empty, non-duplicate subset).
	wantIDs := []string{"arn:aws:", "kms-valid-la071"}
	if len(capturedIDs) != len(wantIDs) {
		t.Fatalf("FetchByIDs received %d IDs %v, want %d %v",
			len(capturedIDs), capturedIDs, len(wantIDs), wantIDs)
	}
	for i, want := range wantIDs {
		if capturedIDs[i] != want {
			t.Errorf("FetchByIDs ids[%d] = %q, want %q", i, capturedIDs[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// LA-072 — ID set grows across re-drill
// ---------------------------------------------------------------------------

// Test_LA_072_IDSetGrowsAcrossRedrill verifies that when a checker first emits
// 2 IDs (first drill), then emits 3 IDs (second drill, superset), the second
// result has Count==3 and LazyAddedResources contains only the new third ID
// (the first two were already cached from the prior lazy-add).
func Test_LA_072_IDSetGrowsAcrossRedrill(t *testing.T) {
	const (
		srcType    = "test-la072-source"
		targetType = "test-la072-target"
	)

	id1 := "policy-la072-001"
	id2 := "policy-la072-002"
	id3 := "policy-la072-003"

	// checkerIDs is swapped between the two drill phases.
	var checkerIDs []string
	checkerIDs = []string{id1, id2}

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-072 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				ids := checkerIDs
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       len(ids),
					ResourceIDs: ids,
				}
			},
		},
	})

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, fetchIDs []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(fetchIDs))
		for i, id := range fetchIDs {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la072-001"}

	// --- First drill: checker emits id1, id2 ---
	var batchCmd tea.Cmd
	m, batchCmd = rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	firstResult, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("first drill: no RelatedCheckResultMsg received")
	}
	if firstResult.Result.Count != 2 {
		t.Errorf("first drill Result.Count: got %d, want 2", firstResult.Result.Count)
	}
	if firstResult.LazyAddedResources == nil {
		t.Fatal("first drill: LazyAddedResources is nil — id1/id2 were not fetched")
	}
	if len(firstResult.LazyAddedResources[targetType]) != 2 {
		t.Fatalf("first drill LazyAddedResources[%s]: got %d, want 2", targetType, len(firstResult.LazyAddedResources[targetType]))
	}

	// Apply the first result's LazyAddedResources back to the model so the
	// model's cache is seeded with id1 and id2 (simulating the app write-back).
	m, _ = rootApplyMsg(m, firstResult)

	// --- Second drill: checker now emits id1, id2, id3 (superset) ---
	checkerIDs = []string{id1, id2, id3}

	_, batchCmd = rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	secondResult, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("second drill: no RelatedCheckResultMsg received")
	}

	if secondResult.Result.Count != 3 {
		t.Errorf("second drill Result.Count: got %d, want 3", secondResult.Result.Count)
	}

	// LazyAddedResources must contain only id3 — id1 and id2 are now in cache.
	if secondResult.LazyAddedResources == nil {
		t.Fatal("second drill: LazyAddedResources is nil — id3 was not fetched")
	}
	lazySlice := secondResult.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("second drill LazyAddedResources[%s]: got %d resources, want 1 (only id3)", targetType, len(lazySlice))
	}
	if lazySlice[0].ID != id3 {
		t.Errorf("second drill LazyAddedResources[%s][0].ID = %q, want %q", targetType, lazySlice[0].ID, id3)
	}
}
