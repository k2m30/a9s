package unit

// related_fetch_test.go — Tests for FetchRelatedTarget generic helper.
//
// Phase 6 (#214/#212): Generic fallback + bounded cold-cache.
// FetchRelatedTarget checks the ResourceCache first, then calls the registered
// paginated fetcher (first page only). Returns (resources, isTruncated, error).

import (
	"context"
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestFetchRelatedTarget_CacheHit verifies that FetchRelatedTarget returns
// cached data without making any AWS calls.
func TestFetchRelatedTarget_CacheHit(t *testing.T) {
	cachedTG := resource.Resource{ID: "tg-cached-001", Name: "cached-tg"}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{cachedTG},
			IsTruncated: false,
		},
	}

	resources, truncated, err := aws.FetchRelatedTarget(context.Background(), nil, cache, "tg")
	if err != nil {
		t.Fatalf("FetchRelatedTarget cache hit: unexpected error: %v", err)
	}
	if truncated {
		t.Errorf("FetchRelatedTarget cache hit: IsTruncated should be false, got true")
	}
	if len(resources) != 1 || resources[0].ID != "tg-cached-001" {
		t.Errorf("FetchRelatedTarget cache hit: got %v, want [tg-cached-001]", resources)
	}
}

// TestFetchRelatedTarget_CacheHit_Truncated verifies that FetchRelatedTarget
// returns cached truncation state correctly.
func TestFetchRelatedTarget_CacheHit_Truncated(t *testing.T) {
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{{ID: "alarm-001"}},
			IsTruncated: true,
		},
	}

	resources, truncated, err := aws.FetchRelatedTarget(context.Background(), nil, cache, "alarm")
	if err != nil {
		t.Fatalf("FetchRelatedTarget truncated cache hit: unexpected error: %v", err)
	}
	if !truncated {
		t.Errorf("FetchRelatedTarget truncated cache hit: IsTruncated should be true, got false")
	}
	if len(resources) != 1 {
		t.Errorf("FetchRelatedTarget truncated cache hit: got %d resources, want 1", len(resources))
	}
}

// TestFetchRelatedTarget_CacheMiss_NoFetcher verifies that FetchRelatedTarget
// returns nil without error when no fetcher is registered for the target.
func TestFetchRelatedTarget_CacheMiss_NoFetcher(t *testing.T) {
	cache := resource.ResourceCache{}

	resources, truncated, err := aws.FetchRelatedTarget(context.Background(), nil, cache, "nonexistent-type")
	if err != nil {
		t.Fatalf("FetchRelatedTarget cache miss/no fetcher: unexpected error: %v", err)
	}
	if truncated {
		t.Errorf("FetchRelatedTarget cache miss/no fetcher: IsTruncated should be false")
	}
	if resources != nil {
		t.Errorf("FetchRelatedTarget cache miss/no fetcher: got %v, want nil", resources)
	}
}

// TestFetchRelatedTarget_CacheMiss_FetcherError verifies that FetchRelatedTarget
// propagates errors from the registered fetcher.
func TestFetchRelatedTarget_CacheMiss_FetcherError(t *testing.T) {
	// Register a temporary fetcher that always fails.
	fetchErr := errors.New("simulated fetch failure")
	resource.RegisterPaginated("test-fetch-err", func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, fetchErr
	})
	defer resource.UnregisterPaginated("test-fetch-err")

	cache := resource.ResourceCache{}

	_, _, err := aws.FetchRelatedTarget(context.Background(), nil, cache, "test-fetch-err")
	if err == nil {
		t.Error("FetchRelatedTarget cache miss with error fetcher: want error, got nil")
	}
	if !errors.Is(err, fetchErr) {
		t.Errorf("FetchRelatedTarget cache miss with error fetcher: got %v, want %v", err, fetchErr)
	}
}
