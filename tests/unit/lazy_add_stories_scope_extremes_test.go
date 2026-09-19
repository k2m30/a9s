package unit

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// Only IDs missing from the target cache are lazy-added; Count and ResourceIDs
// keep every checker-emitted ID.
func Test_LA_010_MixedInScopeAndOutOfScope(t *testing.T) {
	const (
		srcType    = "test-la010-source"
		targetType = "test-la010-target"
	)

	inScopeID := "customer-kms-la010"
	outScopeID := "aws-managed-kms-la010"

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-010 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, []string{inScopeID, outScopeID}, false)
			},
		},
	})

	var fetchByIDsCalled int32
	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		atomic.AddInt32(&fetchByIDsCalled, 1)
		var out []resource.Resource
		for _, id := range ids {
			out = append(out, resource.Resource{ID: id, Name: id})
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	inScopeRes := resource.Resource{ID: inScopeID, Name: inScopeID}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     srcType,
		SourceResourceID: "src-la010-seed",
		Result:           resource.KnownRelated(targetType, nil, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			targetType: {
				Resources:   []resource.Resource{inScopeRes},
				IsTruncated: false,
			},
		},
	})

	srcRes := resource.Resource{ID: "src-la010-001"}
	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if resultMsg.Result.Count() != 2 {
		t.Errorf("Result.Count: got %d, want 2", resultMsg.Result.Count())
	}

	idSet := make(map[string]bool)
	for _, id := range resultMsg.Result.ResourceIDs() {
		idSet[id] = true
	}
	if !idSet[inScopeID] {
		t.Errorf("Result.ResourceIDs missing in-scope ID %q; got %v", inScopeID, resultMsg.Result.ResourceIDs())
	}
	if !idSet[outScopeID] {
		t.Errorf("Result.ResourceIDs missing out-of-scope ID %q; got %v", outScopeID, resultMsg.Result.ResourceIDs())
	}

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

	if atomic.LoadInt32(&fetchByIDsCalled) != 1 {
		t.Errorf("FetchByIDs call count: got %d, want 1", atomic.LoadInt32(&fetchByIDsCalled))
	}
}

func Test_LA_011_AllOutOfScopePopulatesDrill(t *testing.T) {
	const (
		srcType    = "test-la011-source"
		targetType = "test-la011-target"
	)

	ids := []string{"policy-aws-001-la011", "policy-aws-002-la011", "policy-aws-003-la011"}

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-011 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, ids, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, fetchIDs []string) ([]resource.Resource, error) {
		var out []resource.Resource
		for _, id := range fetchIDs {
			out = append(out, resource.Resource{ID: id, Name: id})
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la011-001"}
	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
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

func Test_LA_012_AllInScopeNoLazyAdd(t *testing.T) {
	const (
		srcType    = "test-la012-source"
		targetType = "test-la012-target"
	)

	res1 := resource.Resource{ID: "policy-customer-001-la012", Name: "MyAppPolicy"}
	res2 := resource.Resource{ID: "policy-customer-002-la012", Name: "BillingRead"}

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-012 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, []string{res1.ID, res2.ID}, false)
			},
		},
	})

	var fetchByIDsCallCount int32
	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		atomic.AddInt32(&fetchByIDsCallCount, 1)
		return nil, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     srcType,
		SourceResourceID: "src-la012-seed",
		Result:           resource.KnownRelated(targetType, nil, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			targetType: {
				Resources:   []resource.Resource{res1, res2},
				IsTruncated: false,
			},
		},
	})

	srcRes := resource.Resource{ID: "src-la012-001"}
	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if resultMsg.LazyAddedResources != nil {
		t.Errorf("LazyAddedResources should be nil when all IDs are in cache; got %v", resultMsg.LazyAddedResources)
	}

	if calls := atomic.LoadInt32(&fetchByIDsCallCount); calls != 0 {
		t.Errorf("FetchByIDs call count: got %d, want 0 (all IDs were in cache)", calls)
	}
}

func Test_LA_013_DuplicateIDs_Dedup(t *testing.T) {
	t.Skip("covered by TestLazyAdd_MissingFromCache_DedupsRepeatedIDsInChecker in lazy_add_orchestration_edges_test.go")
}

func Test_LA_014_EmptyPivot_OCQ(t *testing.T) {
	t.Skip("OCQ#1 — LA-014: empty pivot (Count=0) drill behavior is unspecified. See lazy_add.md.")
}

// ResourceIDs keep the checker-emitted ARN even when FetchByIDs returns the
// resource under its bare ID.
func Test_LA_015_ARNvsBareNameTolerance(t *testing.T) {
	const (
		srcType    = "test-la015-source"
		targetType = "test-la015-target"
	)

	fullARN := "arn:aws:iam::aws:policy/AdministratorAccess-la015"
	bareID := "AdministratorAccess-la015"

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-015 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, []string{fullARN}, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		return []resource.Resource{{ID: bareID, Name: bareID}}, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la015-001"}
	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if len(resultMsg.Result.ResourceIDs()) == 0 {
		t.Fatal("Result.ResourceIDs is empty — checker ARN was not preserved")
	}
	if resultMsg.Result.ResourceIDs()[0] != fullARN {
		t.Errorf("Result.ResourceIDs()[0] = %q, want full ARN %q", resultMsg.Result.ResourceIDs()[0], fullARN)
	}

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

func Test_LA_016_UUIDvsAliasDisplay(t *testing.T) {
	const (
		srcType    = "test-la016-source"
		targetType = "test-la016-target"
	)

	keyUUID := "a1b2c3d4-e5f6-7890-abcd-ef0123456789"
	alias := "alias/my-cmk-la016"

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-016 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, []string{keyUUID}, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
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
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la016-001"}
	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
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

func Test_LA_017_InlinePolicy_OCQ(t *testing.T) {
	t.Skip("OCQ#2 — LA-017: inline IAM policies as `policy` rows in drill-through is unspecified. See lazy_add.md.")
}

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

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-070 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, ids, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, fetchIDs []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(fetchIDs))
		for i, id := range fetchIDs {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la070-001"}

	done := make(chan messages.RelatedCheckResult, 1)
	go func() {
		_, batchCmd := rootApplyMsg(m, messages.Navigate{
			Target:       messages.TargetDetail,
			ResourceType: srcType,
			Resource:     &srcRes,
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
		if resultMsg.Result.Count() != idCount {
			t.Errorf("Result.Count: got %d, want %d", resultMsg.Result.Count(), idCount)
		}
		if resultMsg.LazyAddedResources == nil {
			t.Fatal("LazyAddedResources is nil — FetchByIDs was not called")
		}
		lazySlice := resultMsg.LazyAddedResources[targetType]
		if len(lazySlice) != idCount {
			t.Fatalf("LazyAddedResources[%s]: got %d resources, want %d", targetType, len(lazySlice), idCount)
		}
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

// missingFromCache drops empty IDs and duplicates but does not validate ID
// shape: a malformed non-empty ID reaches FetchByIDs.
func Test_LA_071_MalformedIDsFiltered(t *testing.T) {
	const (
		srcType    = "test-la071-source"
		targetType = "test-la071-target"
	)

	emittedIDs := []string{"", "arn:aws:", "kms-valid-la071", "", "kms-valid-la071"}

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-071 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, emittedIDs, false)
			},
		},
	})

	var capturedIDs []string
	var capturedOnce atomic.Bool

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
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
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la071-001"}
	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	_, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if !capturedOnce.Load() {
		t.Fatal("FetchByIDs was not called — lazy-add path was not exercised")
	}

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

func Test_LA_072_IDSetGrowsAcrossRedrill(t *testing.T) {
	const (
		srcType    = "test-la072-source"
		targetType = "test-la072-target"
	)

	id1 := "policy-la072-001"
	id2 := "policy-la072-002"
	id3 := "policy-la072-003"

	checkerIDs := []string{id1, id2}

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-072 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				ids := checkerIDs
				return resource.KnownRelated(targetType, ids, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, fetchIDs []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(fetchIDs))
		for i, id := range fetchIDs {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-la072-001"}

	var batchCmd tea.Cmd
	m, batchCmd = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	firstResult, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("first drill: no RelatedCheckResultMsg received")
	}
	if firstResult.Result.Count() != 2 {
		t.Errorf("first drill Result.Count: got %d, want 2", firstResult.Result.Count())
	}
	if firstResult.LazyAddedResources == nil {
		t.Fatal("first drill: LazyAddedResources is nil — id1/id2 were not fetched")
	}
	if len(firstResult.LazyAddedResources[targetType]) != 2 {
		t.Fatalf("first drill LazyAddedResources[%s]: got %d, want 2", targetType, len(firstResult.LazyAddedResources[targetType]))
	}

	m, _ = rootApplyMsg(m, firstResult)

	checkerIDs = []string{id1, id2, id3}

	// A plain re-navigate replays the cached related result without running the
	// checker; Ctrl+R invalidates the cache and runs it again.
	_, batchCmd = rootApplyMsg(m, ctrlR())

	secondResult, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("second drill: no RelatedCheckResultMsg received")
	}

	if secondResult.Result.Count() != 3 {
		t.Errorf("second drill Result.Count: got %d, want 3", secondResult.Result.Count())
	}

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
