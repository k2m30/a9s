package unit

// issue235_related_check_race_test.go — regression test for issue #235.
//
// Bug: handleRelatedCheckStarted captured the shared outer `cache` variable by
// reference across all closures dispatched via tea.Batch. When two closures ran
// concurrently and both found a cold-cache miss, both wrote to the same `cache`
// map, producing a data race detected by `go test -race`.
//
// Fix: each closure captures its own per-closure `localCache := cache` copy so
// concurrent goroutines never share the same mutable variable.
//
// Run with -race to verify concurrent safety:
//   go test -race ./tests/unit/ -run TestIssue235

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// TestIssue235_EachCheckerGetsIsolatedCacheSnapshot verifies that when three
// related checkers all run on a cold cache, each one independently fetches its
// own target type and resolves to Count=1 — not contaminated by the other
// checkers' fetched pages.
//
// This test detects the pre-fix bug (shared outer cache variable) both as a
// correctness failure (wrong Count) and as a data race under -race.
func TestIssue235_EachCheckerGetsIsolatedCacheSnapshot(t *testing.T) {
	const (
		srcType = "_t235_src"
		typeX   = "_t235_x"
		typeY   = "_t235_y"
		typeZ   = "_t235_z"
	)
	resX := resource.Resource{ID: "x-resource-235"}
	resY := resource.Resource{ID: "y-resource-235"}
	resZ := resource.Resource{ID: "z-resource-235"}

	// Register mock paginated fetchers: each returns exactly one resource of its own type.
	fetcherX := resource.PaginatedFetcher(func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{resX},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	})
	fetcherY := resource.PaginatedFetcher(func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{resY},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	})
	fetcherZ := resource.PaginatedFetcher(func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{resZ},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	})

	resource.SetPaginatedForTest(typeX, fetcherX)
	resource.SetPaginatedForTest(typeY, fetcherY)
	resource.SetPaginatedForTest(typeZ, fetcherZ)

	// Register related defs: each checker receives the cache snapshot and counts
	// only its own type's resource. If cache isolation is broken, a checker may
	// incorrectly see another type's resource under its own key.
	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       typeX,
			DisplayName:      "Type X",
			NeedsTargetCache: true, // reads target from cache; must be true to trigger cold-miss prefetch
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				entry, ok := cache[typeX]
				if !ok || len(entry.Resources) == 0 {
					return resource.RelatedCheckResult{Count: 0}
				}
				return resource.RelatedCheckResult{Count: len(entry.Resources), ResourceIDs: []string{entry.Resources[0].ID}}
			},
		},
		{
			TargetType:       typeY,
			DisplayName:      "Type Y",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				entry, ok := cache[typeY]
				if !ok || len(entry.Resources) == 0 {
					return resource.RelatedCheckResult{Count: 0}
				}
				return resource.RelatedCheckResult{Count: len(entry.Resources), ResourceIDs: []string{entry.Resources[0].ID}}
			},
		},
		{
			TargetType:       typeZ,
			DisplayName:      "Type Z",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				entry, ok := cache[typeZ]
				if !ok || len(entry.Resources) == 0 {
					return resource.RelatedCheckResult{Count: 0}
				}
				return resource.RelatedCheckResult{Count: len(entry.Resources), ResourceIDs: []string{entry.Resources[0].ID}}
			},
		},
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupPaginatedForTest(typeX)
		resource.CleanupPaginatedForTest(typeY)
		resource.CleanupPaginatedForTest(typeZ)
	})

	// Non-demo model so handleRelatedCheckStarted hits the live-mode path
	// (demo mode returns early before touching localCache).
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-235-instance"}

	// Send RelatedCheckStartedMsg — root returns a tea.Batch of 3 checker cmds.
	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})
	if batchCmd == nil {
		t.Fatal("handleRelatedCheckStarted returned nil cmd — expected batch of 3 checker cmds")
	}

	// Execute the batch to get the BatchMsg (all 3 sub-commands).
	rawMsg := batchCmd()
	if rawMsg == nil {
		t.Fatal("batch cmd returned nil msg")
	}

	batchMsg, ok := rawMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", rawMsg)
	}

	// Execute each sub-command and collect RelatedCheckResultMsg by target type.
	results := make(map[string]messages.RelatedCheckResult)
	for _, cmd := range batchMsg {
		if cmd == nil {
			continue
		}
		msg := cmd()
		if msg == nil {
			continue
		}
		if r, ok2 := msg.(messages.RelatedCheckResult); ok2 {
			results[r.Result.TargetType] = r
		}
	}

	// Each checker must have independently fetched its own target type on cold cache
	// and resolved to Count=1 with the correct resource ID.
	for _, tc := range []struct {
		targetType string
		wantID     string
	}{
		{typeX, resX.ID},
		{typeY, resY.ID},
		{typeZ, resZ.ID},
	} {
		r, found := results[tc.targetType]
		if !found {
			t.Errorf("no RelatedCheckResultMsg for target type %q — checker did not run", tc.targetType)
			continue
		}
		if r.Result.Count != 1 {
			t.Errorf("target %q: expected Count=1 (isolated cold-cache fetch), got Count=%d", tc.targetType, r.Result.Count)
		}
		if len(r.Result.ResourceIDs) != 1 || r.Result.ResourceIDs[0] != tc.wantID {
			t.Errorf("target %q: expected ResourceIDs=[%q], got %v", tc.targetType, tc.wantID, r.Result.ResourceIDs)
		}
	}
}
