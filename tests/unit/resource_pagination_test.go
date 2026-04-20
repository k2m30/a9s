package unit

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ═══════════════════════════════════════════════════════════════════════════
// Pagination types and registry tests
// ═══════════════════════════════════════════════════════════════════════════

func TestPaginatedRegistry_RegisterAndGet_RoundTrip(t *testing.T) {
	called := false
	resource.RegisterPaginated("_test_paginated", func(ctx context.Context, clients any, token string) (resource.FetchResult, error) {
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
	defer resource.UnregisterPaginated("_test_paginated")

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
	resource.RegisterPaginatedChild("_test_paginated_child", func(ctx context.Context, clients any, parentCtx resource.ParentContext, token string) (resource.FetchResult, error) {
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
	defer resource.UnregisterPaginatedChild("_test_paginated_child")

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
	msg := messages.LoadMoreMsg{
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
	msg := messages.LoadMoreMsg{
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
	msg := messages.ResourcesLoadedMsg{
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
	msg := messages.ResourcesLoadedMsg{
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
