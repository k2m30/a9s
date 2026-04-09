package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ═══════════════════════════════════════════════════════════════════════════
// Registry unit tests
// ═══════════════════════════════════════════════════════════════════════════

func TestRegistry_GetFetcher_ReturnsRegisteredFetcher(t *testing.T) {
	// All types should be registered via init() in the aws package (spot-check 10)
	types := []string{"s3", "ec2", "dbi", "redis", "dbc", "eks", "secrets", "vpc", "sg", "ng"}
	for _, rt := range types {
		t.Run(rt, func(t *testing.T) {
			f := resource.GetPaginatedFetcher(rt)
			if f == nil {
				t.Errorf("GetPaginatedFetcher(%q) should return a non-nil fetcher", rt)
			}
		})
	}
}

func TestRegistry_MockFetcher_CanBeCalledAndReturnsResources(t *testing.T) {
	// Register a temporary test fetcher
	testResources := []resource.Resource{
		{ID: "test-1", Name: "Test Resource 1"},
		{ID: "test-2", Name: "Test Resource 2"},
	}
	resource.RegisterPaginated("_test_mock", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources: testResources,
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				TotalHint:   len(testResources),
				PageSize:    len(testResources),
			},
		}, nil
	})
	defer resource.UnregisterPaginated("_test_mock") // clean up

	f := resource.GetPaginatedFetcher("_test_mock")
	if f == nil {
		t.Fatal("GetPaginatedFetcher('_test_mock') should return the registered fetcher")
	}

	result, err := f(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("mock fetcher should not return error, got: %v", err)
	}
	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "test-1" {
		t.Errorf("expected first resource ID 'test-1', got %q", result.Resources[0].ID)
	}
}

func TestRegistry_MockFetcher_CanReturnError(t *testing.T) {
	resource.RegisterPaginated("_test_err", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		return resource.FetchResult{}, fmt.Errorf("simulated AWS error")
	})
	defer resource.UnregisterPaginated("_test_err")

	f := resource.GetPaginatedFetcher("_test_err")
	if f == nil {
		t.Fatal("GetPaginatedFetcher should return registered fetcher")
	}

	_, err := f(context.Background(), nil, "")
	if err == nil {
		t.Fatal("error-returning fetcher should return an error")
	}
	if !strings.Contains(err.Error(), "simulated AWS error") {
		t.Errorf("expected 'simulated AWS error', got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Integration: fetchResources via registry still works for all 7 types
// (existing qa_fetch_test.go covers this through the model, but we also
// verify the registry is populated correctly)
// ═══════════════════════════════════════════════════════════════════════════

func TestRegistry_AllSevenTypes_HaveFetchers(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	for _, rt := range allTypes {
		t.Run(rt.ShortName, func(t *testing.T) {
			f := resource.GetPaginatedFetcher(rt.ShortName)
			if f == nil {
				t.Errorf("resource type %q should have a registered paginated fetcher", rt.ShortName)
			}
		})
	}
}
