package unit

// lazy_add_stories_happy_test.go — orchestration pin tests for
// Section A (LA-001..LA-004) and Section I (LA-080..LA-082) of
// tests/stories/lazy_add.md.
//
// Pattern: synthetic source + target types with unique "test-<la-id>-*" short
// names; t.Cleanup unregisters every Register* call so tests are isolated.
// Mirrors the style of lazy_add_orchestration_edges_test.go.

import (
	"context"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ---------------------------------------------------------------------------
// LA-001 — KMS drill shows AWS-managed key used by RDS
// ---------------------------------------------------------------------------

// Test_LA_001_KMSDrillAWSManagedKey verifies that a checker emitting an
// AWS-managed KMS key UUID (alias aws/rds) causes FetchByIDs to be called and
// the lazy-added resource carries the correct alias field.
func Test_LA_001_KMSDrillAWSManagedKey(t *testing.T) {
	const (
		srcType    = "test-la001-rds-src"
		targetType = "test-la001-kms"
	)

	const keyID = "aws-managed-rds-uuid-la001"
	const alias = "aws/rds"

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "KMS Keys",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1,
					ResourceIDs: []string{keyID},
				}
			},
		},
	})

	var fetchByIDsCalled int32
	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		atomic.AddInt32(&fetchByIDsCalled, 1)
		return []resource.Resource{{
			ID:   keyID,
			Name: alias,
			Fields: map[string]string{
				"key_id": keyID,
				"alias":  alias,
			},
		}}, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: resource.Resource{ID: "src-la001-rds"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("LA-001: no RelatedCheckResultMsg received")
	}

	if resultMsg.Result.Count != 1 {
		t.Errorf("LA-001: Count = %d, want 1", resultMsg.Result.Count)
	}
	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LA-001: LazyAddedResources is nil — FetchByIDs was not called")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("LA-001: LazyAddedResources[%s] len = %d, want 1", targetType, len(lazySlice))
	}
	got := lazySlice[0]
	if got.ID != keyID {
		t.Errorf("LA-001: resource.ID = %q, want %q", got.ID, keyID)
	}
	if got.Fields["alias"] != alias {
		t.Errorf("LA-001: Fields[alias] = %q, want %q", got.Fields["alias"], alias)
	}
	if atomic.LoadInt32(&fetchByIDsCalled) != 1 {
		t.Errorf("LA-001: FetchByIDs call count = %d, want 1", atomic.LoadInt32(&fetchByIDsCalled))
	}
}

// ---------------------------------------------------------------------------
// LA-002 — AMI drill shows public marketplace AMI used by EC2
// ---------------------------------------------------------------------------

// Test_LA_002_AMIDrillPublicAMI verifies that a checker emitting a public AMI
// ID (owner: amazon) causes lazy-add to surface it with the expected fields.
func Test_LA_002_AMIDrillPublicAMI(t *testing.T) {
	const (
		srcType    = "test-la002-ec2-src"
		targetType = "test-la002-ami"
	)

	const imageID = "ami-0abc1234-la002"
	const ownerID = "amazon"
	const amiName = "amzn2-ami-hvm"

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "AMIs",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1,
					ResourceIDs: []string{imageID},
				}
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		return []resource.Resource{{
			ID:   imageID,
			Name: amiName,
			Fields: map[string]string{
				"image_id": imageID,
				"owner_id": ownerID,
				"name":     amiName,
			},
		}}, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: resource.Resource{ID: "src-la002-ec2"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("LA-002: no RelatedCheckResultMsg received")
	}

	if resultMsg.Result.Count != 1 {
		t.Errorf("LA-002: Count = %d, want 1", resultMsg.Result.Count)
	}
	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LA-002: LazyAddedResources is nil — FetchByIDs was not called")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("LA-002: LazyAddedResources[%s] len = %d, want 1", targetType, len(lazySlice))
	}
	got := lazySlice[0]
	if got.ID != imageID {
		t.Errorf("LA-002: resource.ID = %q, want %q", got.ID, imageID)
	}
	if got.Fields["owner_id"] != ownerID {
		t.Errorf("LA-002: Fields[owner_id] = %q, want %q", got.Fields["owner_id"], ownerID)
	}
	if got.Fields["name"] != amiName {
		t.Errorf("LA-002: Fields[name] = %q, want %q", got.Fields["name"], amiName)
	}
}

// ---------------------------------------------------------------------------
// LA-003 — EBS-snapshot drill shows shared snapshot used by EBS volume
// ---------------------------------------------------------------------------

// Test_LA_003_EBSSnapDrillSharedSnapshot verifies that a checker emitting a
// shared snapshot ID (foreign account owner) causes lazy-add to surface it.
func Test_LA_003_EBSSnapDrillSharedSnapshot(t *testing.T) {
	const (
		srcType    = "test-la003-ebs-src"
		targetType = "test-la003-ebs-snap"
	)

	const snapID = "snap-0def5678-la003"
	const ownerID = "999999999999"

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "EBS Snapshots",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1,
					ResourceIDs: []string{snapID},
				}
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		return []resource.Resource{{
			ID:   snapID,
			Name: snapID,
			Fields: map[string]string{
				"snapshot_id": snapID,
				"owner_id":    ownerID,
			},
		}}, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: resource.Resource{ID: "src-la003-ebs"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("LA-003: no RelatedCheckResultMsg received")
	}

	if resultMsg.Result.Count != 1 {
		t.Errorf("LA-003: Count = %d, want 1", resultMsg.Result.Count)
	}
	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LA-003: LazyAddedResources is nil — FetchByIDs was not called")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("LA-003: LazyAddedResources[%s] len = %d, want 1", targetType, len(lazySlice))
	}
	got := lazySlice[0]
	if got.ID != snapID {
		t.Errorf("LA-003: resource.ID = %q, want %q", got.ID, snapID)
	}
	if got.Fields["owner_id"] != ownerID {
		t.Errorf("LA-003: Fields[owner_id] = %q, want %q", got.Fields["owner_id"], ownerID)
	}
}

// ---------------------------------------------------------------------------
// LA-004 — IAM-policy drill shows AWS-managed policy attached to role
// ---------------------------------------------------------------------------

// Test_LA_004_IAMPolicyDrillAWSManaged verifies that a checker emitting an
// AWS-managed policy ARN causes lazy-add to surface it with the correct
// policy_type and policy_name fields.
func Test_LA_004_IAMPolicyDrillAWSManaged(t *testing.T) {
	const (
		srcType    = "test-la004-role-src"
		targetType = "test-la004-policy"
	)

	const policyARN = "arn:aws:iam::aws:policy/AdministratorAccess-la004"
	const policyName = "AdministratorAccess-la004"
	const policyType = "aws-managed"

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "IAM Policies",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1,
					ResourceIDs: []string{policyARN},
				}
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		return []resource.Resource{{
			ID:   policyARN,
			Name: policyName,
			Fields: map[string]string{
				"policy_type": policyType,
				"policy_name": policyName,
			},
		}}, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: resource.Resource{ID: "src-la004-role"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("LA-004: no RelatedCheckResultMsg received")
	}

	if resultMsg.Result.Count != 1 {
		t.Errorf("LA-004: Count = %d, want 1", resultMsg.Result.Count)
	}
	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LA-004: LazyAddedResources is nil — FetchByIDs was not called")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("LA-004: LazyAddedResources[%s] len = %d, want 1", targetType, len(lazySlice))
	}
	got := lazySlice[0]
	if got.Fields["policy_type"] != policyType {
		t.Errorf("LA-004: Fields[policy_type] = %q, want %q", got.Fields["policy_type"], policyType)
	}
	if got.Fields["policy_name"] != policyName {
		t.Errorf("LA-004: Fields[policy_name] = %q, want %q", got.Fields["policy_name"], policyName)
	}
}

// ---------------------------------------------------------------------------
// LA-080 — Demo mode baseline
// ---------------------------------------------------------------------------

// Test_LA_080_DemoModeBaseline is skipped: demo-mode drill-through is exercised
// end-to-end by tests/integration/scenario_*.go. Synthetic unit coverage for
// the orchestration contract is pinned by LA-001..LA-004, LA-010..LA-012.
func Test_LA_080_DemoModeBaseline(t *testing.T) {
	t.Skip("demo-mode drill-through is exercised end-to-end by tests/integration/scenario_*.go. Synthetic unit coverage for the orchestration contract is pinned by LA-001..LA-004, LA-010..LA-012.")
}

// ---------------------------------------------------------------------------
// LA-081 — Cold-cache drill triggers prefetch for the target type
// ---------------------------------------------------------------------------

// Test_LA_081_ColdCacheDrillTriggersPrefetch verifies that when NeedsTargetCache
// is true and the cache is empty, the orchestrator runs the paginated fetcher
// first, then uses the resulting pages as the cache when checking for missing
// IDs, and finally lazy-adds only the IDs not found in the prefetched pages.
func Test_LA_081_ColdCacheDrillTriggersPrefetch(t *testing.T) {
	const (
		srcType    = "test-la081-src"
		targetType = "test-la081-kms"
	)

	var paginatedCalls int32

	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		atomic.AddInt32(&paginatedCalls, 1)
		return resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "pre-1", Name: "pre-1"},
				{ID: "pre-2", Name: "pre-2"},
			},
		}, nil
	})

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "KMS Keys (cold)",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       2,
					ResourceIDs: []string{"pre-1", "lazy-1"},
				}
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		var out []resource.Resource
		for _, id := range ids {
			out = append(out, resource.Resource{ID: id, Name: id})
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
		resource.CleanupPaginatedForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: resource.Resource{ID: "src-la081"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("LA-081: no RelatedCheckResultMsg received")
	}

	// Paginated fetcher must have been called for the cold-cache prefetch.
	if atomic.LoadInt32(&paginatedCalls) == 0 {
		t.Error("LA-081: paginated fetcher was not called — cold-cache prefetch did not fire")
	}

	// CachedPages must contain the 2 pre-fetched resources.
	if resultMsg.CachedPages == nil {
		t.Fatal("LA-081: CachedPages is nil — prefetch result not returned in msg")
	}
	cachedEntry := resultMsg.CachedPages[targetType]
	if len(cachedEntry.Resources) != 2 {
		t.Errorf("LA-081: CachedPages[%s] has %d resources, want 2", targetType, len(cachedEntry.Resources))
	}

	// LazyAddedResources must contain only the missing ID (lazy-1), not pre-1.
	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LA-081: LazyAddedResources is nil — lazy-add path did not fire for missing ID")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("LA-081: LazyAddedResources[%s] len = %d, want 1 (only lazy-1)", targetType, len(lazySlice))
	}
	if lazySlice[0].ID != "lazy-1" {
		t.Errorf("LA-081: LazyAddedResources[%s][0].ID = %q, want %q", targetType, lazySlice[0].ID, "lazy-1")
	}
}

// ---------------------------------------------------------------------------
// LA-082 — Warm-cache drill re-uses top-level fetch, adds only missing IDs
// ---------------------------------------------------------------------------

// Test_LA_082_WarmCacheDrillReusesCache verifies that when the target-type
// cache is already warm (seeded via a prior CachedPages write-back) and
// NeedsTargetCache is true, the paginated fetcher is NOT called again — the
// orchestrator reuses the existing cache, and only the missing ID is
// lazy-added via FetchByIDs.
func Test_LA_082_WarmCacheDrillReusesCache(t *testing.T) {
	const (
		srcType    = "test-la082-src"
		targetType = "test-la082-kms"
	)

	var paginatedCalls int32

	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		atomic.AddInt32(&paginatedCalls, 1)
		return resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "cmk-warm-la082", Name: "cmk-warm-la082"},
			},
		}, nil
	})

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "KMS Keys (warm)",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				// Checker emits an AWS-managed key not present in the warm cache.
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1,
					ResourceIDs: []string{"aws-managed-la082"},
				}
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		var out []resource.Resource
		for _, id := range ids {
			out = append(out, resource.Resource{ID: id, Name: id})
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
		resource.CleanupPaginatedForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	// Pre-seed the cache with the warm entry, simulating "user previously
	// opened the main-menu KMS list".
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     srcType,
		SourceResourceID: "seed-la082",
		Result:           resource.RelatedCheckResult{TargetType: targetType, Count: 1},
		CachedPages: map[string]resource.ResourceCacheEntry{
			targetType: {
				Resources:   []resource.Resource{{ID: "cmk-warm-la082", Name: "cmk-warm-la082"}},
				IsTruncated: false,
			},
		},
	})

	// Reset the paginated counter after pre-seed (we only care about calls
	// during the actual drill below, not the seed step).
	atomic.StoreInt32(&paginatedCalls, 0)

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: resource.Resource{ID: "src-la082"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("LA-082: no RelatedCheckResultMsg received")
	}

	// The paginated fetcher must NOT have been called — cache was warm.
	if calls := atomic.LoadInt32(&paginatedCalls); calls != 0 {
		t.Errorf("LA-082: paginated fetcher called %d time(s), want 0 — warm-cache drill must reuse existing cache", calls)
	}

	// LazyAddedResources must contain only the missing AWS-managed key.
	if resultMsg.LazyAddedResources == nil {
		t.Fatal("LA-082: LazyAddedResources is nil — aws-managed-la082 should have been lazy-added")
	}
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 {
		t.Fatalf("LA-082: LazyAddedResources[%s] len = %d, want 1", targetType, len(lazySlice))
	}
	if lazySlice[0].ID != "aws-managed-la082" {
		t.Errorf("LA-082: LazyAddedResources[%s][0].ID = %q, want %q", targetType, lazySlice[0].ID, "aws-managed-la082")
	}
}
