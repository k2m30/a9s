package unit

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ═══════════════════════════════════════════════════════════════════════════
// Pagination types and registry tests
// ═══════════════════════════════════════════════════════════════════════════

func TestPaginatedRegistry_RegisterAndGet_RoundTrip(t *testing.T) {
	called := false
	resource.SetPaginatedForTest("_test_paginated", func(ctx context.Context, clients any, token string) (resource.FetchResult, error) {
		called = true
		return resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "pg-1", Name: "Paginated Resource 1"},
			},
			Pagination: &resource.PaginationMeta{
				IsTruncated: true,
				NextToken:   "tok-abc",
				TotalHint:   100,
				PageSize:    50,
			},
		}, nil
	})
	defer resource.CleanupPaginatedForTest("_test_paginated")

	f := resource.GetPaginatedFetcher("_test_paginated")
	if f == nil {
		t.Fatal("GetPaginatedFetcher should return a non-nil fetcher")
	}

	result, err := f(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("fetcher was not called")
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "pg-1" {
		t.Errorf("expected ID 'pg-1', got %q", result.Resources[0].ID)
	}
	if result.Pagination == nil {
		t.Fatal("Pagination should not be nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("IsTruncated should be true")
	}
	if result.Pagination.NextToken != "tok-abc" {
		t.Errorf("expected NextToken 'tok-abc', got %q", result.Pagination.NextToken)
	}
	if result.Pagination.TotalHint != 100 {
		t.Errorf("expected TotalHint 100, got %d", result.Pagination.TotalHint)
	}
	if result.Pagination.PageSize != 50 {
		t.Errorf("expected PageSize 50, got %d", result.Pagination.PageSize)
	}
}

func TestPaginatedChildRegistry_RegisterAndGet_RoundTrip(t *testing.T) {
	called := false
	resource.SetPaginatedChildForTest("_test_paginated_child", func(ctx context.Context, clients any, parentCtx resource.ParentContext, token string) (resource.FetchResult, error) {
		called = true
		if parentCtx["bucket"] != "my-bucket" {
			t.Errorf("expected parentCtx[bucket]='my-bucket', got %q", parentCtx["bucket"])
		}
		if token != "page2" {
			t.Errorf("expected token 'page2', got %q", token)
		}
		return resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "child-1", Name: "Child Resource 1"},
				{ID: "child-2", Name: "Child Resource 2"},
			},
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				NextToken:   "",
				TotalHint:   2,
				PageSize:    2,
			},
		}, nil
	})
	defer resource.CleanupPaginatedChildForTest("_test_paginated_child")

	f := resource.GetPaginatedChildFetcher("_test_paginated_child")
	if f == nil {
		t.Fatal("GetPaginatedChildFetcher should return a non-nil fetcher")
	}

	parentCtx := resource.ParentContext{"bucket": "my-bucket"}
	result, err := f(context.Background(), nil, parentCtx, "page2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("child fetcher was not called")
	}
	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("IsTruncated should be false for last page")
	}
}

func TestPaginatedRegistry_GetReturnsNilForUnregistered(t *testing.T) {
	f := resource.GetPaginatedFetcher("nonexistent_paginated_type")
	if f != nil {
		t.Error("GetPaginatedFetcher should return nil for unregistered type")
	}
}

func TestPaginatedChildRegistry_GetReturnsNilForUnregistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("nonexistent_paginated_child_type")
	if f != nil {
		t.Error("GetPaginatedChildFetcher should return nil for unregistered type")
	}
}

func TestFetchResult_NilPagination_LegacyCompat(t *testing.T) {
	result := resource.FetchResult{
		Resources: []resource.Resource{
			{ID: "legacy-1", Name: "Legacy Resource"},
		},
		Pagination: nil,
	}

	if result.Pagination != nil {
		t.Error("Pagination should be nil for legacy unpaginated results")
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "legacy-1" {
		t.Errorf("expected ID 'legacy-1', got %q", result.Resources[0].ID)
	}
}

func TestPaginationMeta_UnknownTotalHint(t *testing.T) {
	meta := resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "next-page-token",
		TotalHint:   -1,
		PageSize:    25,
	}

	if meta.TotalHint != -1 {
		t.Errorf("TotalHint should be -1 for unknown total, got %d", meta.TotalHint)
	}
	if !meta.IsTruncated {
		t.Error("IsTruncated should be true")
	}
	if meta.NextToken != "next-page-token" {
		t.Errorf("expected NextToken 'next-page-token', got %q", meta.NextToken)
	}
	if meta.PageSize != 25 {
		t.Errorf("expected PageSize 25, got %d", meta.PageSize)
	}
}

func TestLoadMoreMsg_WithNilParentContext(t *testing.T) {
	msg := messages.LoadMore{
		ResourceType:      "s3",
		ContinuationToken: "tok-123",
		ParentContext:     nil,
	}

	if msg.ResourceType != "s3" {
		t.Errorf("expected ResourceType 's3', got %q", msg.ResourceType)
	}
	if msg.ContinuationToken != "tok-123" {
		t.Errorf("expected ContinuationToken 'tok-123', got %q", msg.ContinuationToken)
	}
	if msg.ParentContext != nil {
		t.Error("ParentContext should be nil for top-level resources")
	}
}

func TestLoadMoreMsg_WithParentContext(t *testing.T) {
	msg := messages.LoadMore{
		ResourceType:      "s3_objects",
		ContinuationToken: "tok-456",
		ParentContext:     map[string]string{"bucket": "my-bucket", "prefix": "data/"},
	}

	if msg.ResourceType != "s3_objects" {
		t.Errorf("expected ResourceType 's3_objects', got %q", msg.ResourceType)
	}
	if msg.ContinuationToken != "tok-456" {
		t.Errorf("expected ContinuationToken 'tok-456', got %q", msg.ContinuationToken)
	}
	if msg.ParentContext == nil {
		t.Fatal("ParentContext should not be nil for child views")
	}
	if msg.ParentContext["bucket"] != "my-bucket" {
		t.Errorf("expected bucket 'my-bucket', got %q", msg.ParentContext["bucket"])
	}
	if msg.ParentContext["prefix"] != "data/" {
		t.Errorf("expected prefix 'data/', got %q", msg.ParentContext["prefix"])
	}
}

func TestResourcesLoadedMsg_PaginationFields(t *testing.T) {
	msg := messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "s3",
		Resources:    []resource.Resource{{ID: "r1"}},
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "tok-next",
			TotalHint:   -1,
			PageSize:    50,
		},
		Append: true,
	}

	if msg.ResourceType != "s3" {
		t.Errorf("expected ResourceType 's3', got %q", msg.ResourceType)
	}
	if len(msg.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(msg.Resources))
	}
	if msg.Pagination == nil {
		t.Fatal("Pagination should not be nil")
	}
	if !msg.Pagination.IsTruncated {
		t.Error("IsTruncated should be true")
	}
	if msg.Pagination.NextToken != "tok-next" {
		t.Errorf("expected NextToken 'tok-next', got %q", msg.Pagination.NextToken)
	}
	if !msg.Append {
		t.Error("Append should be true")
	}
}

func TestResourcesLoadedMsg_LegacyNilPagination(t *testing.T) {
	msg := messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-123"}},
		Pagination:   nil,
		Append:       false,
	}

	if msg.ResourceType != "ec2" {
		t.Errorf("expected ResourceType 'ec2', got %q", msg.ResourceType)
	}
	if len(msg.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(msg.Resources))
	}
	if msg.Pagination != nil {
		t.Error("Pagination should be nil for unpaginated fetchers")
	}
	if msg.Append {
		t.Error("Append should be false for unpaginated fetchers")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// sanitizeFetchResult boundary (core/resource/accessors.go, unexported): all
// 4 registered-fetcher accessors route a fetcher's raw result through it. A
// truncated result with an empty NextToken is not a legitimate pagination
// state anywhere in this codebase's contract (no working cursor to resume
// from) and must be downgraded to exact, or "load more" would refetch page 1
// forever. A truncated result WITH a real cursor is genuine pagination and
// must pass through completely untouched — an over-broad sanitizer that
// flattened real pagination would make load-more impossible for every
// genuinely truncated type, which is worse than the bug it prevents.
// ═══════════════════════════════════════════════════════════════════════════

func assertSanitizedToExact(t *testing.T, p *resource.PaginationMeta, wantResourceCount int) {
	t.Helper()
	if p == nil {
		t.Fatal("Pagination should not be nil")
	}
	if p.IsTruncated {
		t.Error("IsTruncated should be sanitized to false — truncated with no cursor is a page that can never be resumed")
	}
	if p.TotalHint != wantResourceCount {
		t.Errorf("TotalHint = %d, want %d (len(Resources)) after sanitizing an unresumable truncation", p.TotalHint, wantResourceCount)
	}
}

func assertSanitizerPassthrough(t *testing.T, p *resource.PaginationMeta, wantToken string, wantHint int) {
	t.Helper()
	if p == nil {
		t.Fatal("Pagination should not be nil")
	}
	if !p.IsTruncated {
		t.Error("IsTruncated must remain true — a genuine cursor means there IS more to fetch; sanitizing this away would break real pagination")
	}
	if p.NextToken != wantToken {
		t.Errorf("NextToken = %q, want unchanged %q", p.NextToken, wantToken)
	}
	if p.TotalHint != wantHint {
		t.Errorf("TotalHint = %d, want unchanged %d — the sanitizer must not touch a legitimately truncated result's hint", p.TotalHint, wantHint)
	}
}

func TestGetPaginatedFetcher_TruncatedEmptyCursor_SanitizedToExact(t *testing.T) {
	resource.SetPaginatedForTest("_test_sanitize_paginated", func(ctx context.Context, clients any, token string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "r1"}, {ID: "r2"}},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "", TotalHint: -1},
		}, nil
	})
	defer resource.CleanupPaginatedForTest("_test_sanitize_paginated")

	result, err := resource.GetPaginatedFetcher("_test_sanitize_paginated")(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSanitizedToExact(t, result.Pagination, len(result.Resources))
}

func TestGetPaginatedFetcher_TruncatedWithCursor_PassesThroughUntouched(t *testing.T) {
	resource.SetPaginatedForTest("_test_sanitize_paginated_real", func(ctx context.Context, clients any, token string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "r1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-real-cursor", TotalHint: 100},
		}, nil
	})
	defer resource.CleanupPaginatedForTest("_test_sanitize_paginated_real")

	result, err := resource.GetPaginatedFetcher("_test_sanitize_paginated_real")(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSanitizerPassthrough(t, result.Pagination, "tok-real-cursor", 100)
}

func TestGetAvailabilityFetcher_TruncatedEmptyCursor_SanitizedToExact(t *testing.T) {
	resource.SetAvailabilityFetcherForTest("_test_sanitize_avail", func(ctx context.Context, clients any, token string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "a1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "", TotalHint: -1},
		}, nil
	})
	defer resource.CleanupAvailabilityFetcherForTest("_test_sanitize_avail")

	result, err := resource.GetAvailabilityFetcher("_test_sanitize_avail")(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSanitizedToExact(t, result.Pagination, len(result.Resources))
}

func TestGetAvailabilityFetcher_TruncatedWithCursor_PassesThroughUntouched(t *testing.T) {
	resource.SetAvailabilityFetcherForTest("_test_sanitize_avail_real", func(ctx context.Context, clients any, token string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "a1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-avail-cursor", TotalHint: 42},
		}, nil
	})
	defer resource.CleanupAvailabilityFetcherForTest("_test_sanitize_avail_real")

	result, err := resource.GetAvailabilityFetcher("_test_sanitize_avail_real")(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSanitizerPassthrough(t, result.Pagination, "tok-avail-cursor", 42)
}

func TestGetPaginatedChildFetcher_TruncatedEmptyCursor_SanitizedToExact(t *testing.T) {
	resource.SetPaginatedChildForTest("_test_sanitize_child", func(ctx context.Context, clients any, parentCtx resource.ParentContext, token string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "c1"}, {ID: "c2"}, {ID: "c3"}},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "", TotalHint: -1},
		}, nil
	})
	defer resource.CleanupPaginatedChildForTest("_test_sanitize_child")

	result, err := resource.GetPaginatedChildFetcher("_test_sanitize_child")(context.Background(), nil, resource.ParentContext{"bucket": "b"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSanitizedToExact(t, result.Pagination, len(result.Resources))
}

func TestGetPaginatedChildFetcher_TruncatedWithCursor_PassesThroughUntouched(t *testing.T) {
	resource.SetPaginatedChildForTest("_test_sanitize_child_real", func(ctx context.Context, clients any, parentCtx resource.ParentContext, token string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "c1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-child-cursor", TotalHint: 7},
		}, nil
	})
	defer resource.CleanupPaginatedChildForTest("_test_sanitize_child_real")

	result, err := resource.GetPaginatedChildFetcher("_test_sanitize_child_real")(context.Background(), nil, resource.ParentContext{"bucket": "b"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSanitizerPassthrough(t, result.Pagination, "tok-child-cursor", 7)
}

func TestGetFilteredPaginatedFetcher_TruncatedEmptyCursor_SanitizedToExact(t *testing.T) {
	resource.SetFilteredPaginatedForTest("_test_sanitize_filtered", func(ctx context.Context, clients any, filter map[string]string, token string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "f1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "", TotalHint: -1},
		}, nil
	})
	defer resource.CleanupFilteredPaginatedForTest("_test_sanitize_filtered")

	result, err := resource.GetFilteredPaginatedFetcher("_test_sanitize_filtered")(context.Background(), nil, map[string]string{"vpc-id": "vpc-1"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSanitizedToExact(t, result.Pagination, len(result.Resources))
}

func TestGetFilteredPaginatedFetcher_TruncatedWithCursor_PassesThroughUntouched(t *testing.T) {
	resource.SetFilteredPaginatedForTest("_test_sanitize_filtered_real", func(ctx context.Context, clients any, filter map[string]string, token string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "f1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-filtered-cursor", TotalHint: 12},
		}, nil
	})
	defer resource.CleanupFilteredPaginatedForTest("_test_sanitize_filtered_real")

	result, err := resource.GetFilteredPaginatedFetcher("_test_sanitize_filtered_real")(context.Background(), nil, map[string]string{"vpc-id": "vpc-1"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSanitizerPassthrough(t, result.Pagination, "tok-filtered-cursor", 12)
}
