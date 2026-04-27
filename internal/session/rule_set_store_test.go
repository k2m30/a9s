package session_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/internal/session"
)

// TestRuleSetStore_GetMissing verifies that a fresh RuleSetStore returns
// (nil, false) for any key.
func TestRuleSetStore_GetMissing(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()

	got, ok := store.Get("no-such-key")
	if ok {
		t.Error("expected ok=false for absent key, got true")
	}
	if got != nil {
		t.Errorf("expected nil for absent key, got %v", got)
	}
}

// TestRuleSetStore_SetGet verifies the Set + Get round-trip: a stored value
// is returned verbatim with ok=true.
func TestRuleSetStore_SetGet(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()
	const key = "ses-rule-set:default"
	const value = "rule-set-payload"

	store.Set(key, value)

	got, ok := store.Get(key)
	if !ok {
		t.Fatal("expected ok=true after Set, got false")
	}
	if got != value {
		t.Errorf("Get returned %v, want %q", got, value)
	}
}

// TestRuleSetStore_Delete verifies that Delete removes the target key while
// leaving unrelated keys intact.
func TestRuleSetStore_Delete(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()

	store.Set("k1", "v1")
	store.Set("k2", "v2")

	store.Delete("k1")

	if _, ok := store.Get("k1"); ok {
		t.Error("expected k1 to be absent after Delete, but Get returned ok=true")
	}
	got, ok := store.Get("k2")
	if !ok {
		t.Fatal("expected k2 to still exist after deleting k1")
	}
	if got != "v2" {
		t.Errorf("k2 value = %v, want %q", got, "v2")
	}
}

// TestRuleSetStore_Clear verifies that Clear() removes all entries so all
// subsequent Gets return (nil, false).
func TestRuleSetStore_Clear(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()

	keys := []string{"a", "b", "c"}
	for _, k := range keys {
		store.Set(k, "payload")
	}

	store.Clear()

	for _, k := range keys {
		got, ok := store.Get(k)
		if ok {
			t.Errorf("expected ok=false for %q after Clear(), got true (value=%v)", k, got)
		}
	}
}

// TestRuleSetStore_ConcurrentSafe verifies that the RuleSetStore
// implementation is safe for concurrent use. 100 goroutines perform Set/Get
// operations on disjoint keys; the race detector validates no data race occurs.
func TestRuleSetStore_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()

	const workers = 100
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("rule-set-%d", n)
			store.Set(key, n)
			_, _ = store.Get(key)
		}(i)
	}

	wg.Wait()
}
