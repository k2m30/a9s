package unit

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestDetailEnricherRegistry_RegisterAndGet(t *testing.T) {
	called := false
	enricher := func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		called = true
		res.Fields["enriched"] = "yes"
		return res, nil
	}
	resource.RegisterDetailEnricher("test-enrich-type", enricher)
	t.Cleanup(func() { resource.UnregisterDetailEnricher("test-enrich-type") })

	got := resource.GetDetailEnricher("test-enrich-type")
	if got == nil {
		t.Fatal("expected detail enricher to be registered")
	}

	res := resource.Resource{ID: "r1", Fields: map[string]string{}}
	result, err := got(context.Background(), nil, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("detail enricher was not called")
	}
	if result.Fields["enriched"] != "yes" {
		t.Fatal("detail enricher did not modify resource")
	}
}

func TestDetailEnricherRegistry_GetUnregistered_ReturnsNil(t *testing.T) {
	got := resource.GetDetailEnricher("nonexistent-enrich-type")
	if got != nil {
		t.Fatal("expected nil for unregistered type")
	}
}

func TestDetailEnricherRegistry_HasDetailEnricher(t *testing.T) {
	if resource.HasDetailEnricher("has-enrich-test") {
		t.Fatal("expected false before registration")
	}
	resource.RegisterDetailEnricher("has-enrich-test", func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		return res, nil
	})
	t.Cleanup(func() { resource.UnregisterDetailEnricher("has-enrich-test") })

	if !resource.HasDetailEnricher("has-enrich-test") {
		t.Fatal("expected true after registration")
	}
}

func TestDetailEnricherRegistry_UnregisterDetailEnricher(t *testing.T) {
	resource.RegisterDetailEnricher("unregister-test", func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		return res, nil
	})

	if !resource.HasDetailEnricher("unregister-test") {
		t.Fatal("expected detail enricher to be registered before unregister")
	}

	resource.UnregisterDetailEnricher("unregister-test")

	if resource.HasDetailEnricher("unregister-test") {
		t.Fatal("expected detail enricher to be absent after unregister")
	}
	if resource.GetDetailEnricher("unregister-test") != nil {
		t.Fatal("GetDetailEnricher should return nil after unregister")
	}
}

func TestDetailEnricherRegistry_PanicsOnDuplicate(t *testing.T) {
	stub := func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		return res, nil
	}
	stub2 := func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		return res, nil
	}

	resource.RegisterDetailEnricher("overwrite-test", stub)
	t.Cleanup(func() { resource.UnregisterDetailEnricher("overwrite-test") })

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	resource.RegisterDetailEnricher("overwrite-test", stub2)
}

func TestDetailEnricherRegistry_PanicsOnEmptyName(t *testing.T) {
	stubFn := func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
		return res, nil
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty name registration")
		}
	}()
	resource.RegisterDetailEnricher("", stubFn)
}

func TestDetailEnricherRegistry_PanicsOnNilFn(t *testing.T) {
	t.Cleanup(func() { resource.UnregisterDetailEnricher("nil-fn-test") })

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil function registration")
		}
	}()
	resource.RegisterDetailEnricher("nil-fn-test", nil)
}

func TestDetailEnricherRegistry_RolePolicies_IsRegistered(t *testing.T) {
	// Verify the role_policies detail enricher is registered via init() in role_policies_enrich.go.
	// This acts as a smoke test that the init() ran and wired up the enricher.
	if !resource.HasDetailEnricher("role_policies") {
		t.Fatal("expected role_policies detail enricher to be registered via init()")
	}
	if resource.GetDetailEnricher("role_policies") == nil {
		t.Fatal("GetDetailEnricher(role_policies) returned nil but HasDetailEnricher returned true")
	}
}
