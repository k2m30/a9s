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

func TestRegistry_GetFetcher_ReturnsNilForUnregistered(t *testing.T) {
	f := resource.GetFetcher("nonexistent")
	if f != nil {
		t.Error("GetFetcher should return nil for unregistered resource type")
	}
}

func TestRegistry_GetFetcher_ReturnsRegisteredFetcher(t *testing.T) {
	// All 10 types should be registered via init() in the aws package
	types := []string{"s3", "ec2", "dbi", "redis", "dbc", "eks", "secrets", "vpc", "sg", "ng"}
	for _, rt := range types {
		t.Run(rt, func(t *testing.T) {
			f := resource.GetFetcher(rt)
			if f == nil {
				t.Errorf("GetFetcher(%q) should return a non-nil fetcher", rt)
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
	resource.Register("_test_mock", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		return testResources, nil
	})
	defer resource.Unregister("_test_mock") // clean up

	f := resource.GetFetcher("_test_mock")
	if f == nil {
		t.Fatal("GetFetcher('_test_mock') should return the registered fetcher")
	}

	results, err := f(context.Background(), nil)
	if err != nil {
		t.Fatalf("mock fetcher should not return error, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(results))
	}
	if results[0].ID != "test-1" {
		t.Errorf("expected first resource ID 'test-1', got %q", results[0].ID)
	}
}

func TestRegistry_MockFetcher_CanReturnError(t *testing.T) {
	resource.Register("_test_err", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		return nil, fmt.Errorf("simulated AWS error")
	})
	defer resource.Unregister("_test_err")

	f := resource.GetFetcher("_test_err")
	if f == nil {
		t.Fatal("GetFetcher should return registered fetcher")
	}

	_, err := f(context.Background(), nil)
	if err == nil {
		t.Fatal("error-returning fetcher should return an error")
	}
	if !strings.Contains(err.Error(), "simulated AWS error") {
		t.Errorf("expected 'simulated AWS error', got: %v", err)
	}
}

func TestRegistry_NilClients_FetcherReceivesNil(t *testing.T) {
	var receivedClients interface{}
	resource.Register("_test_nil_clients", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		receivedClients = clients
		return nil, nil
	})
	defer resource.Unregister("_test_nil_clients")

	f := resource.GetFetcher("_test_nil_clients")
	_, _ = f(context.Background(), nil)
	if receivedClients != nil {
		t.Error("fetcher should receive nil clients when passed nil")
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
			f := resource.GetFetcher(rt.ShortName)
			if f == nil {
				t.Errorf("resource type %q should have a registered fetcher", rt.ShortName)
			}
		})
	}
}
