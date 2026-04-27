package session_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
)

// TestPolicyStore_LookupMissing verifies that a fresh PolicyStore returns
// (resource.Resource{}, false) for an absent key, and that both build flags
// start false.
func TestPolicyStore_LookupMissing(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	got, ok := store.Lookup("any")
	if ok {
		t.Error("expected ok=false for absent key, got true")
	}
	// Resource contains slice/map fields so direct == is invalid; check the
	// identifying fields that must be empty on a zero-value return.
	if got.ID != "" || got.Name != "" {
		t.Errorf("expected zero Resource for absent key, got %+v", got)
	}

	if store.ManagedBuilt() {
		t.Error("expected ManagedBuilt=false on fresh store")
	}
	if store.InlineBuilt() {
		t.Error("expected InlineBuilt=false on fresh store")
	}
}

// TestPolicyStore_SetThenLookup verifies that a value stored via Set is
// retrievable via Lookup with ok=true and the same value, and that a
// different key still misses.
func TestPolicyStore_SetThenLookup(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()
	r := resource.Resource{ID: "policy-A", Name: "policy-A"}

	store.Set("policy-A", r)

	got, ok := store.Lookup("policy-A")
	if !ok {
		t.Fatal("expected ok=true after Set, got false")
	}
	if got.ID != r.ID || got.Name != r.Name {
		t.Errorf("Lookup returned %+v, want %+v", got, r)
	}

	_, ok2 := store.Lookup("policy-B")
	if ok2 {
		t.Error("expected ok=false for key 'policy-B' that was never set")
	}
}

// TestPolicyStore_OverwriteSet verifies that a second Set on the same key
// replaces the stored value, and Lookup returns the most recent resource.
func TestPolicyStore_OverwriteSet(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()
	res1 := resource.Resource{ID: "id-1", Name: "first"}
	res2 := resource.Resource{ID: "id-2", Name: "second"}

	store.Set("k", res1)
	store.Set("k", res2)

	got, ok := store.Lookup("k")
	if !ok {
		t.Fatal("expected ok=true after overwrite, got false")
	}
	if got.ID != "id-2" || got.Name != "second" {
		t.Errorf("Lookup returned %+v, want %+v", got, res2)
	}
}

// TestPolicyStore_KeyByNameAndARN pins the dual-key contract: both the policy
// short name and its ARN must resolve to the same resource after two Set calls.
func TestPolicyStore_KeyByNameAndARN(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()
	r := resource.Resource{ID: "arn:aws:iam::123456789012:policy/AdminPolicy", Name: "AdminPolicy"}

	store.Set("AdminPolicy", r)
	store.Set("arn:aws:iam::123456789012:policy/AdminPolicy", r)

	gotName, okName := store.Lookup("AdminPolicy")
	if !okName {
		t.Error("expected ok=true for short-name key")
	}
	if gotName.ID != r.ID {
		t.Errorf("Lookup by name returned ID=%q, want %q", gotName.ID, r.ID)
	}

	gotARN, okARN := store.Lookup("arn:aws:iam::123456789012:policy/AdminPolicy")
	if !okARN {
		t.Error("expected ok=true for ARN key")
	}
	if gotARN.ID != r.ID {
		t.Errorf("Lookup by ARN returned ID=%q, want %q", gotARN.ID, r.ID)
	}
}

// TestPolicyStore_ManagedBuiltFlag verifies the ManagedBuilt flag lifecycle:
// starts false, becomes true after MarkManagedBuilt, while InlineBuilt remains
// independently false.
func TestPolicyStore_ManagedBuiltFlag(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	if store.ManagedBuilt() {
		t.Error("expected ManagedBuilt=false on fresh store")
	}

	store.MarkManagedBuilt()

	if !store.ManagedBuilt() {
		t.Error("expected ManagedBuilt=true after MarkManagedBuilt")
	}
	if store.InlineBuilt() {
		t.Error("expected InlineBuilt=false — flags are independent")
	}
}

// TestPolicyStore_InlineBuiltFlag verifies the InlineBuilt flag lifecycle:
// starts false, becomes true after MarkInlineBuilt, while ManagedBuilt remains
// independently false.
func TestPolicyStore_InlineBuiltFlag(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	if store.InlineBuilt() {
		t.Error("expected InlineBuilt=false on fresh store")
	}

	store.MarkInlineBuilt()

	if !store.InlineBuilt() {
		t.Error("expected InlineBuilt=true after MarkInlineBuilt")
	}
	if store.ManagedBuilt() {
		t.Error("expected ManagedBuilt=false — flags are independent")
	}
}

// TestPolicyStore_ClearResetsAll verifies that Clear removes all stored
// entries and resets both build flags to false.
func TestPolicyStore_ClearResetsAll(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	keys := []string{
		"arn:aws:iam::111111111111:policy/A",
		"arn:aws:iam::222222222222:policy/B",
		"arn:aws:iam::333333333333:policy/C",
	}
	for i, k := range keys {
		store.Set(k, resource.Resource{ID: k, Name: fmt.Sprintf("policy-%d", i)})
	}
	store.MarkManagedBuilt()
	store.MarkInlineBuilt()

	store.Clear()

	for _, k := range keys {
		_, ok := store.Lookup(k)
		if ok {
			t.Errorf("expected Lookup(%q)=false after Clear, got true", k)
		}
	}
	if store.ManagedBuilt() {
		t.Error("expected ManagedBuilt=false after Clear")
	}
	if store.InlineBuilt() {
		t.Error("expected InlineBuilt=false after Clear")
	}
}

// TestPolicyStore_ClearIdempotent verifies that calling Clear multiple times
// on a fresh store does not panic, and that a second Clear on a populated-then-
// cleared store also does not panic.
func TestPolicyStore_ClearIdempotent(t *testing.T) {
	t.Parallel()

	fresh := session.NewPolicyStore()
	fresh.Clear()
	fresh.Clear() // must not panic

	populated := session.NewPolicyStore()
	populated.Set("k", resource.Resource{ID: "k"})
	populated.MarkManagedBuilt()
	populated.Clear()
	populated.Clear() // must not panic
}

// TestPolicyStore_ConcurrentSetLookup verifies race-free concurrent access:
// 100 goroutines each Set and Lookup on disjoint keys; the race detector
// validates no data race occurs.
func TestPolicyStore_ConcurrentSetLookup(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	const workers = 100
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("arn:aws:iam::%012d:policy/P%d", n, n)
			r := resource.Resource{ID: key, Name: fmt.Sprintf("policy-%d", n)}
			store.Set(key, r)
			_, _ = store.Lookup(key)
		}(i)
	}

	wg.Wait()
}

// TestPolicyStore_ConcurrentBuildFlagToggle verifies race-free concurrent
// access to build flags: 50 goroutines mixing MarkManagedBuilt, ManagedBuilt,
// MarkInlineBuilt, InlineBuilt, and Clear; the race detector validates no data
// race occurs.
func TestPolicyStore_ConcurrentBuildFlagToggle(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	const workers = 50
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		go func(n int) {
			defer wg.Done()
			switch n % 5 {
			case 0:
				store.MarkManagedBuilt()
			case 1:
				_ = store.ManagedBuilt()
			case 2:
				store.MarkInlineBuilt()
			case 3:
				_ = store.InlineBuilt()
			case 4:
				store.Clear()
			}
		}(i)
	}

	wg.Wait()
}

// TestPolicyStore_PartialFailureContract_InlineBuiltStaysFalseOnError pins the
// IAM two-phase build contract:
//
//   - Phase 1 (managed): ListPolicies(Scope=All) — must fully succeed before
//     MarkManagedBuilt is called.
//   - Phase 2 (inline): ListGroups + per-group ListGroupPolicies — partial
//     failures must NOT call MarkInlineBuilt, so that InlineBuilt stays false
//     and the next call retries the entire inline phase.
//
// This test simulates a partial-failure inline build: some entries are Set but
// MarkInlineBuilt is never called. It asserts InlineBuilt remains false while
// the partial entries are still findable.
func TestPolicyStore_PartialFailureContract_InlineBuiltStaysFalseOnError(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	// Phase 1 completes successfully.
	managed := resource.Resource{ID: "arn:aws:iam::123456789012:policy/Managed", Name: "Managed"}
	store.Set("Managed", managed)
	store.Set("arn:aws:iam::123456789012:policy/Managed", managed)
	store.MarkManagedBuilt()

	if !store.ManagedBuilt() {
		t.Fatal("expected ManagedBuilt=true after phase 1")
	}

	// Phase 2 partial failure: some inline entries are written but the build
	// aborts before completion — MarkInlineBuilt is intentionally NOT called.
	partial := resource.Resource{ID: "arn:aws:iam::123456789012:policy/InlinePartial", Name: "InlinePartial"}
	store.Set("InlinePartial", partial)

	// Contract: InlineBuilt must remain false so callers know to retry phase 2.
	if store.InlineBuilt() {
		t.Error("InlineBuilt must be false when inline build did not complete — production must retry")
	}

	// Partial entries written before the failure are still accessible (callers
	// may use whatever is present as a best-effort cache).
	got, ok := store.Lookup("InlinePartial")
	if !ok {
		t.Error("partial inline entry must be findable via Lookup")
	}
	if got.ID != partial.ID {
		t.Errorf("partial entry ID=%q, want %q", got.ID, partial.ID)
	}
}
