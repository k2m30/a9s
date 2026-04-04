package unit

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func demoRelatedEC2Resource(t *testing.T) resource.Resource {
	t.Helper()
	resources, ok := demo.GetResources("ec2")
	if !ok || len(resources) == 0 {
		t.Fatal("demo ec2 fixtures missing")
	}
	return resources[0]
}

func demoRelatedClients() *awsclient.ServiceClients {
	cfg := demo.NewDemoAWSConfig()
	return awsclient.CreateServiceClients(cfg)
}

func TestEC2RelatedCheckers_FetchLiveDataOnColdCache(t *testing.T) {
	clients := demoRelatedClients()
	instance := demoRelatedEC2Resource(t)
	cache := resource.ResourceCache{}

	for _, target := range []string{"tg", "asg", "ebs-snap"} {
		checker := ec2CheckerByTarget(t, target)
		got := checker(context.Background(), clients, instance, cache)
		if got.Err != nil {
			t.Fatalf("%s checker returned unexpected error on cold cache: %v", target, got.Err)
		}
		if got.Count < 0 {
			t.Fatalf("%s checker should resolve from live fetch on cold cache; got %+v", target, got)
		}
		if got.Count == 0 {
			t.Fatalf("%s checker should find related resources for demo ec2 fixture on cold cache; got %+v", target, got)
		}
	}
}

func TestEC2RelatedCheckers_NodeGroupsAndCloudTrailResolveOnColdCache(t *testing.T) {
	clients := demoRelatedClients()
	instance := demoRelatedEC2Resource(t)
	cache := resource.ResourceCache{}

	for _, target := range []string{"ng", "ct-events"} {
		checker := ec2CheckerByTarget(t, target)
		got := checker(context.Background(), clients, instance, cache)
		if got.Err != nil {
			t.Fatalf("%s checker returned unexpected error on cold cache: %v", target, got.Err)
		}
		if got.Count < 0 {
			t.Fatalf("%s checker should not stay unknown on cold cache with live clients; got %+v", target, got)
		}
	}
}

func TestEC2RelatedCheckers_AlarmResolvesOnColdCache(t *testing.T) {
	clients := demoRelatedClients()
	instance := demoRelatedEC2Resource(t)

	checker := ec2CheckerByTarget(t, "alarm")
	got := checker(context.Background(), clients, instance, resource.ResourceCache{})
	if got.Err != nil {
		t.Fatalf("alarm checker returned unexpected error on cold cache: %v", got.Err)
	}
	if got.Count < 0 {
		t.Fatalf("alarm checker should resolve on cold cache with live clients; got %+v", got)
	}
}

func TestEC2RelatedCheckers_EIPResolvesOnColdCache(t *testing.T) {
	clients := demoRelatedClients()
	instance := demoRelatedEC2Resource(t)

	checker := ec2CheckerByTarget(t, "eip")
	got := checker(context.Background(), clients, instance, resource.ResourceCache{})
	if got.Err != nil {
		t.Fatalf("eip checker returned unexpected error on cold cache: %v", got.Err)
	}
	if got.Count < 0 {
		t.Fatalf("eip checker should resolve on cold cache with live clients; got %+v", got)
	}
}

// T003: verifies that on a cold cache miss the "tg" checker calls the registered
// paginated fetcher exactly once — NOT the old full-account FetchTargetGroups.
// Currently FAILS because ec2RelatedResources uses FetchTargetGroups directly and
// early-returns on nil clients before ever calling the paginated fetcher.
func TestEC2RelatedColdCache_FirstPageOnly_TG(t *testing.T) {
	var mockCallCount int
	mockFetcher := resource.PaginatedFetcher(func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		mockCallCount++
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "tg-mock-1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	})

	original := resource.GetPaginatedFetcher("tg")
	resource.RegisterPaginated("tg", mockFetcher)
	t.Cleanup(func() {
		if original != nil {
			resource.RegisterPaginated("tg", original)
		} else {
			resource.UnregisterPaginated("tg")
		}
	})

	instance := resource.Resource{ID: "i-t003"}
	checker := ec2CheckerByTarget(t, "tg")
	_ = checker(context.Background(), nil, instance, resource.ResourceCache{})

	if mockCallCount != 1 {
		t.Errorf("T003: expected mock paginated fetcher for 'tg' to be called exactly once; got %d calls", mockCallCount)
	}
}

// T004: verifies that on a cold cache miss the "cfn" checker calls the registered
// paginated fetcher exactly once — NOT the old full-account FetchCloudFormationStacks.
// Currently FAILS because ec2RelatedResources uses FetchCloudFormationStacks directly and
// early-returns on nil clients before ever calling the paginated fetcher.
func TestEC2RelatedColdCache_FirstPageOnly_CFN(t *testing.T) {
	var mockCallCount int
	mockFetcher := resource.PaginatedFetcher(func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		mockCallCount++
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "cfn-mock-stack"}},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	})

	original := resource.GetPaginatedFetcher("cfn")
	resource.RegisterPaginated("cfn", mockFetcher)
	t.Cleanup(func() {
		if original != nil {
			resource.RegisterPaginated("cfn", original)
		} else {
			resource.UnregisterPaginated("cfn")
		}
	})

	instance := resource.Resource{ID: "i-t004"}
	checker := ec2CheckerByTarget(t, "cfn")
	_ = checker(context.Background(), nil, instance, resource.ResourceCache{})

	if mockCallCount != 1 {
		t.Errorf("T004: expected mock paginated fetcher for 'cfn' to be called exactly once; got %d calls", mockCallCount)
	}
}

// T005: verifies that when the paginated fetcher returns a truncated first page with
// zero matches for the given EC2 instance, the checker returns Count=-1 (unknown),
// NOT Count=0 (definitive zero). This ensures partial pages are not treated as
// conclusive negatives.
// Currently FAILS because ec2RelatedResources calls FetchTargetGroups directly with
// nil clients and early-returns (nil, false, nil) — never reaching the truncation path —
// so the checker returns Count=0 instead of Count=-1.
func TestEC2RelatedColdCache_TruncatedZeroMatch_CountIsUnknown(t *testing.T) {
	mockFetcher := resource.PaginatedFetcher(func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  []resource.Resource{},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-t005"},
		}, nil
	})

	original := resource.GetPaginatedFetcher("tg")
	resource.RegisterPaginated("tg", mockFetcher)
	t.Cleanup(func() {
		if original != nil {
			resource.RegisterPaginated("tg", original)
		} else {
			resource.UnregisterPaginated("tg")
		}
	})

	// Use an instance with no matching TG — the truncated page contains zero entries,
	// so a correct implementation must return Count=-1 (can't know the full picture).
	instance := resource.Resource{ID: "i-no-matches"}
	checker := ec2CheckerByTarget(t, "tg")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count != -1 {
		t.Errorf("T005: expected Count=-1 (unknown) when paginated result is truncated with zero matches; got Count=%d", got.Count)
	}
}
