package unit

// resource_fetch_by_ids_test.go — registry tests for lazy-add
// (SetFetchByIDsForTest / GetFetchByIDs). Exercises the round-trip contract
// without hitting any AWS fake: callers that register a fetcher can recover
// it by short name, and unregistered types return nil.

import (
	"context"
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRegisterFetchByIDs_RoundTrip(t *testing.T) {
	const shortName = "test-lazy-add-roundtrip"
	t.Cleanup(func() { resource.CleanupFetchByIDsForTest(shortName) })

	want := []resource.Resource{
		{ID: "id-a", Fields: map[string]string{"k": "v"}},
	}
	called := false
	fn := func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		called = true
		if len(ids) != 1 || ids[0] != "id-a" {
			return nil, errors.New("unexpected ids")
		}
		return want, nil
	}

	resource.SetFetchByIDsForTest(shortName, fn)

	got := resource.GetFetchByIDs(shortName)
	if got == nil {
		t.Fatalf("GetFetchByIDs(%q) returned nil after registration", shortName)
	}

	res, err := got(context.Background(), nil, []string{"id-a"})
	if err != nil {
		t.Fatalf("fn returned unexpected error: %v", err)
	}
	if !called {
		t.Errorf("fn was not invoked via registry pointer")
	}
	if len(res) != 1 || res[0].ID != "id-a" {
		t.Errorf("fn result = %+v, want one resource with ID=id-a", res)
	}
}

func TestGetFetchByIDs_Unregistered_ReturnsNil(t *testing.T) {
	if got := resource.GetFetchByIDs("test-lazy-add-not-registered-" + t.Name()); got != nil {
		t.Errorf("GetFetchByIDs for unregistered short name returned non-nil: %v", got)
	}
}
