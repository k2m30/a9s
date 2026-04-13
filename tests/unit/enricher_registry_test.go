package unit

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEnricherRegistry_RegisterAndGet(t *testing.T) {
	called := false
	enricher := func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		called = true
		res.Fields["enriched"] = "yes"
		return res, nil
	}
	resource.RegisterEnricher("test-enrich-type", enricher)
	t.Cleanup(func() { resource.UnregisterEnricher("test-enrich-type") })

	got := resource.GetEnricher("test-enrich-type")
	if got == nil {
		t.Fatal("expected enricher to be registered")
	}

	res := resource.Resource{ID: "r1", Fields: map[string]string{}}
	result, err := got(context.Background(), nil, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("enricher was not called")
	}
	if result.Fields["enriched"] != "yes" {
		t.Fatal("enricher did not modify resource")
	}
}

func TestEnricherRegistry_GetUnregistered_ReturnsNil(t *testing.T) {
	got := resource.GetEnricher("nonexistent-enrich-type")
	if got != nil {
		t.Fatal("expected nil for unregistered type")
	}
}

func TestEnricherRegistry_HasEnricher(t *testing.T) {
	if resource.HasEnricher("has-enrich-test") {
		t.Fatal("expected false before registration")
	}
	resource.RegisterEnricher("has-enrich-test", func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		return res, nil
	})
	t.Cleanup(func() { resource.UnregisterEnricher("has-enrich-test") })

	if !resource.HasEnricher("has-enrich-test") {
		t.Fatal("expected true after registration")
	}
}

func TestEnricherRegistry_UnregisterEnricher(t *testing.T) {
	resource.RegisterEnricher("unregister-test", func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		return res, nil
	})

	if !resource.HasEnricher("unregister-test") {
		t.Fatal("expected enricher to be registered before unregister")
	}

	resource.UnregisterEnricher("unregister-test")

	if resource.HasEnricher("unregister-test") {
		t.Fatal("expected enricher to be absent after unregister")
	}
	if resource.GetEnricher("unregister-test") != nil {
		t.Fatal("GetEnricher should return nil after unregister")
	}
}

func TestEnricherRegistry_OverwriteExisting(t *testing.T) {
	firstCalled := false
	secondCalled := false

	resource.RegisterEnricher("overwrite-test", func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		firstCalled = true
		return res, nil
	})
	t.Cleanup(func() { resource.UnregisterEnricher("overwrite-test") })

	// Overwrite with a second enricher
	resource.RegisterEnricher("overwrite-test", func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		secondCalled = true
		return res, nil
	})

	got := resource.GetEnricher("overwrite-test")
	if got == nil {
		t.Fatal("expected enricher to be registered after overwrite")
	}

	res := resource.Resource{ID: "r1", Fields: map[string]string{}}
	if _, err := got(context.Background(), nil, res); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if firstCalled {
		t.Error("first enricher should NOT have been called after overwrite")
	}
	if !secondCalled {
		t.Error("second enricher should have been called after overwrite")
	}
}

func TestEnricherRegistry_RolePolicies_IsRegistered(t *testing.T) {
	// Verify the role_policies enricher is registered via init() in role_policies_enrich.go.
	// This acts as a smoke test that the init() ran and wired up the enricher.
	if !resource.HasEnricher("role_policies") {
		t.Fatal("expected role_policies enricher to be registered via init()")
	}
	if resource.GetEnricher("role_policies") == nil {
		t.Fatal("GetEnricher(role_policies) returned nil but HasEnricher returned true")
	}
}
