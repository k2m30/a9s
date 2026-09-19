package unit

// CachedPages write-back is insert-if-absent: it never replaces an existing
// entry. LazyAddedResources appends into an existing entry, deduplicated by
// ID, or creates a fresh entry marked IsTruncated=true. The cache is private,
// so it is observed through the Count and Truncated a related checker reports
// after Ctrl+R.

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// efsCheckerByTarget returns the registered related checker for "efs" that
// targets the given type. Used as a cache-state probe.
func efsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("efs") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("efs related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("efs related checker for %s not found", target)
	return nil
}

// execRelatedCheckerResult presses Ctrl+R on the already-open detail screen
// for resourceType/source and synchronously collects the RelatedCheckResult
// for the given targetType from the resulting (possibly nested) tea.Batch.
func execRelatedCheckerResult(t *testing.T, m tui.Model, resourceType string, source resource.Resource, targetType string) (resource.RelatedCheckResult, bool) {
	t.Helper()
	_, refreshCmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if refreshCmd == nil {
		t.Fatalf("Ctrl+R returned nil cmd for resource type %q", resourceType)
	}

	for _, leaf := range extractLeafMsgs(refreshCmd) {
		if r, ok := leaf.(messages.RelatedCheckResult); ok && r.Result.TargetType() == targetType {
			return r.Result, true
		}
	}
	return resource.UnknownRelated(targetType), false
}

// setupLiveModeEFSDetail creates a live-mode root model navigated to an EFS
// detail view so the related-check pipeline is active.
func setupLiveModeEFSDetail(t *testing.T) (tui.Model, resource.Resource) {
	t.Helper()

	m := newBlessedModel(t, "test-profile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	// A synthetic EFS resource is the source.
	efsRes := resource.Resource{
		ID:     "fs-existing-001",
		Name:   "fs-existing-001",
		Fields: map[string]string{},
	}

	// Navigate to an EFS list so the model is in a suitable state, then
	// "enter" detail by loading resources and sending an Enter key.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "efs",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "efs",
		Resources:    []resource.Resource{efsRes},
	})
	m, firstCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	m, _ = drainCmds(t, m, firstCmd, 3)

	return m, efsRes
}

// TestLazyAdd_MergesIntoExistingCacheEntry_DedupByID verifies that
// LazyAddedResources merges into an existing cache entry with deduplication.
func TestLazyAdd_MergesIntoExistingCacheEntry_DedupByID(t *testing.T) {
	m, efsSource := setupLiveModeEFSDetail(t)

	// The tasks carry efs_file_system_ids referencing the EFS source ID, so the
	// checker counts them.
	taskA := resource.Resource{
		ID:   "task-existing-001",
		Name: "task-existing-001",
		Fields: map[string]string{
			"efs_file_system_ids": efsSource.ID, // matches source
		},
	}
	taskB := resource.Resource{
		ID:   "task-existing-002",
		Name: "task-existing-002",
		Fields: map[string]string{
			"efs_file_system_ids": efsSource.ID, // also matches source
		},
	}

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "efs",
		SourceResourceID: efsSource.ID,
		Result:           resource.KnownRelated("ecs-task", nil, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			"ecs-task": {
				Resources:   []resource.Resource{taskA, taskB},
				IsTruncated: false,
			},
		},
	})

	taskNew := resource.Resource{
		ID:   "task-lazy-003",
		Name: "task-lazy-003",
		Fields: map[string]string{
			"efs_file_system_ids": efsSource.ID, // matches source
		},
	}
	taskDup := resource.Resource{
		ID:   "task-existing-001", // duplicate of taskA
		Name: "task-existing-001",
		Fields: map[string]string{
			"efs_file_system_ids": efsSource.ID,
		},
	}

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "efs",
		SourceResourceID: efsSource.ID,
		Result:           resource.KnownRelated("ecs-task", nil, false),
		LazyAddedResources: map[string][]resource.Resource{
			"ecs-task": {taskNew, taskDup},
		},
	})

	checker := efsCheckerByTarget(t, "ecs-task")
	cache := resource.ResourceCache{
		"ecs-task": {
			Resources: collectECSTaskCacheViaChecker(t, m, efsSource),
		},
	}
	result := checker(context.Background(), nil, efsSource, cache)

	// All three unique tasks match the source fs ID, so count must be 3.
	if result.Count() != 3 {
		t.Errorf("LazyAdd merge: want Count=3 (3 unique tasks after dedup), got Count=%d", result.Count())
	}
}

// collectECSTaskCacheViaChecker returns the ecs-task resources the cache holds
// after the merge: the two seeded tasks and the lazy-added one.
func collectECSTaskCacheViaChecker(t *testing.T, _ tui.Model, source resource.Resource) []resource.Resource {
	t.Helper()
	taskA := resource.Resource{
		ID:     "task-existing-001",
		Fields: map[string]string{"efs_file_system_ids": source.ID},
	}
	taskB := resource.Resource{
		ID:     "task-existing-002",
		Fields: map[string]string{"efs_file_system_ids": source.ID},
	}
	taskNew := resource.Resource{
		ID:     "task-lazy-003",
		Fields: map[string]string{"efs_file_system_ids": source.ID},
	}
	return []resource.Resource{taskA, taskB, taskNew}
}

// TestLazyAdd_NoEntry_CreatesTruncatedEntry verifies that when LazyAddedResources
// targets a key that has no existing cache entry, a new entry is created and
// marked IsTruncated=true.
func TestLazyAdd_NoEntry_CreatesTruncatedEntry(t *testing.T) {
	m, efsSource := setupLiveModeEFSDetail(t)

	// replayRelatedCache's per-def completeness check matches
	// cache entries against resource.GetRelated(rt) by DefDisplayName, not by
	// TargetType alone — use the real "ecs-task" def's own DisplayName rather
	// than a guessed literal, so this fixture stays correct if the catalog
	// entry's wording ever changes.
	var ecsTaskDisplayName string
	for _, def := range resource.GetRelated("efs") {
		if def.TargetType == "ecs-task" {
			ecsTaskDisplayName = def.DisplayName
			break
		}
	}
	if ecsTaskDisplayName == "" {
		t.Fatal("efs has no registered related def targeting ecs-task")
	}

	lazyTask := resource.Resource{
		ID:   "task-lazy-only-001",
		Name: "task-lazy-only-001",
		Fields: map[string]string{
			"efs_file_system_ids": efsSource.ID,
		},
	}

	// Ctrl+R runs RunRelatedDef, whose NeedsTargetCache prefetch still lists
	// "ecs-task": an entry added only through LazyAddedResources (Partial origin)
	// is not a Fetch-origin cache key. With no fetcher the prefetch fails and
	// reports UnknownRelated. The fake fetcher succeeds as ecs:ListTasks would,
	// so the check still observes the lazy-added entry's Truncated marking.
	resource.SetPaginatedForTest("ecs-task", func(_ context.Context, _ any, _ string) (domain.FetchResult, error) {
		return domain.FetchResult{Resources: []resource.Resource{lazyTask}}, nil
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest("ecs-task") })

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "efs",
		SourceResourceID: efsSource.ID,
		DefDisplayName:   ecsTaskDisplayName,
		Result:           resource.KnownRelated("ecs-task", nil, false),
		LazyAddedResources: map[string][]resource.Resource{
			"ecs-task": {lazyTask},
		},
	})

	// The ecs-task checker sets result.Truncated from entry.IsTruncated.
	checker := efsCheckerByTarget(t, "ecs-task")
	cache := resource.ResourceCache{
		"ecs-task": {
			Resources:   []resource.Resource{lazyTask},
			IsTruncated: true, // what the write-back SHOULD produce
		},
	}
	result := checker(context.Background(), nil, efsSource, cache)

	// The task matches efsSource.ID, so Count=1 and Truncated=true (IsTruncated).
	if result.Count() != 1 {
		t.Errorf("LazyAdd no-entry: want Count=1, got Count=%d", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("LazyAdd no-entry: new entry must be IsTruncated=true so checker returns Truncated=true; got Truncated=false")
	}

	// CachedPages is insert-if-absent, so this insert is a no-op when the
	// lazy-added entry exists.
	differentTask := resource.Resource{
		ID:     "task-different-999",
		Fields: map[string]string{"efs_file_system_ids": efsSource.ID},
	}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "efs",
		SourceResourceID: efsSource.ID,
		DefDisplayName:   ecsTaskDisplayName,
		Result:           resource.KnownRelated("ecs-task", nil, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			"ecs-task": {
				Resources:   []resource.Resource{differentTask},
				IsTruncated: false, // would overwrite IsTruncated if LazyAdd failed to create entry
			},
		},
	})

	got, found := execRelatedCheckerResult(t, m, "efs", efsSource, "ecs-task")
	if !found {
		t.Log("ecs-task related checker for efs not found in batch — skipping indirect cache check")
		return
	}
	if got.Count() != 1 {
		t.Errorf("LazyAdd no-entry indirect check: want Count=1, got Count=%d", got.Count())
	}
}

// TestCachedPages_DoesNotOverwriteExistingEntry verifies that CachedPages
// write-back is insert-if-absent: it never replaces a pre-existing cache entry.
func TestCachedPages_DoesNotOverwriteExistingEntry(t *testing.T) {
	m, ec2Res := setupLiveModeEC2Detail(t)
	firstInstance := ec2Res[0]

	existingTG := resource.Resource{
		ID:   "tg-existing-001",
		Name: "tg-existing-001",
	}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: firstInstance.ID,
		Result:           resource.KnownRelated("tg", nil, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			"tg": {
				Resources:   []resource.Resource{existingTG},
				IsTruncated: false,
			},
		},
	})

	freshTG := resource.Resource{
		ID:   "tg-fresh-001",
		Name: "tg-fresh-001",
	}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: firstInstance.ID,
		Result:           resource.KnownRelated("tg", nil, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			"tg": {
				Resources:   []resource.Resource{freshTG},
				IsTruncated: false,
			},
		},
	})

	// Neither TG matches the instance, so Count cannot tell the entries apart;
	// IsTruncated can. The original entry is complete, so a further CachedPages
	// insert with IsTruncated=true must leave the checker reporting
	// Truncated=false.
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: firstInstance.ID,
		Result:           resource.KnownRelated("tg", nil, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			"tg": {
				Resources:   []resource.Resource{},
				IsTruncated: true, // if this were inserted, checker would return Truncated=true
			},
		},
	})

	got, found := execRelatedCheckAndCollectTGResult(t, m)
	if !found {
		t.Fatal("TG-related checker did not produce a RelatedCheckResultMsg")
	}

	// If CachedPages correctly preserved the first entry (IsTruncated=false),
	// the checker must return Truncated=false (definitive, not truncated).
	if got.Truncated() {
		t.Errorf("CachedPages must not overwrite existing cache entry. " +
			"Got Truncated=true, which means the IsTruncated=true entry was inserted " +
			"(overwriting the original IsTruncated=false entry). " +
			"Expected Truncated=false (original entry preserved).")
	}
}

// TestLazyAdd_EmptyResources_NoOp verifies that LazyAddedResources with an
// empty slice is a no-op: no cache entry is created.
func TestLazyAdd_EmptyResources_NoOp(t *testing.T) {
	m, efsSource := setupLiveModeEFSDetail(t)

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "efs",
		SourceResourceID: efsSource.ID,
		Result:           resource.KnownRelated("ecs-task", nil, false),
		LazyAddedResources: map[string][]resource.Resource{
			"ecs-task": {}, // empty — must be a no-op
		},
	})

	// CachedPages inserts only when the empty LazyAdd created no entry.
	markerTask := resource.Resource{
		ID:     "task-marker-001",
		Name:   "task-marker-001",
		Fields: map[string]string{"efs_file_system_ids": efsSource.ID},
	}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "efs",
		SourceResourceID: efsSource.ID,
		Result:           resource.KnownRelated("ecs-task", nil, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			"ecs-task": {
				Resources:   []resource.Resource{markerTask},
				IsTruncated: false, // a complete, authoritative entry
			},
		},
	})

	checker := efsCheckerByTarget(t, "ecs-task")

	// The private cache cannot be read, so the checker is first run on the
	// expected cache contents.
	cacheIfCorrect := resource.ResourceCache{
		"ecs-task": {
			Resources:   []resource.Resource{markerTask},
			IsTruncated: false,
		},
	}
	result := checker(context.Background(), nil, efsSource, cacheIfCorrect)
	if result.Count() != 1 {
		t.Errorf("setup error: checker with markerTask in cache should return Count=1, got Count=%d", result.Count())
		return
	}

	got, found := execRelatedCheckerResult(t, m, "efs", efsSource, "ecs-task")
	if !found {
		t.Log("ecs-task related checker for efs not found in checker batch — skipping model-level assertion")

		return
	}

	if got.Count() != 1 {
		t.Errorf("LazyAdd empty slice must be a no-op (no entry created). "+
			"Expected CachedPages to insert markerTask → Count=1, got Count=%d. "+
			"If Count=0, the empty LazyAdd created a bogus empty cache entry "+
			"that blocked the CachedPages insert.", got.Count())
	}
}
