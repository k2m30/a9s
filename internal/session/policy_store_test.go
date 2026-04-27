package session_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/internal/session"
)

// TestPolicyStore_GetMissing verifies that a fresh PolicyStore returns
// (nil, false) for an absent key.
func TestPolicyStore_GetMissing(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	got, ok := store.Get("missing")
	if ok {
		t.Error("expected ok=false for absent key, got true")
	}
	if got != nil {
		t.Errorf("expected nil for absent key, got %v", got)
	}
}

// TestPolicyStore_SetThenGet verifies that a value stored via Set is
// retrievable via Get with ok=true and the same value.
func TestPolicyStore_SetThenGet(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()
	const arn = "arn:aws:iam::123456789012:policy/MyPolicy"
	const doc = "policy-doc-string"

	store.Set(arn, doc)

	got, ok := store.Get(arn)
	if !ok {
		t.Fatal("expected ok=true after Set, got false")
	}
	if got != doc {
		t.Errorf("Get returned %v, want %q", got, doc)
	}
}

// TestPolicyStore_OverwriteSet verifies that a second Set on the same key
// replaces the stored value, and Get returns the most recent value.
func TestPolicyStore_OverwriteSet(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()
	const arn = "arn:aws:iam::123456789012:policy/Foo"

	store.Set(arn, "first-value")
	store.Set(arn, "second-value")

	got, ok := store.Get(arn)
	if !ok {
		t.Fatal("expected ok=true after overwrite, got false")
	}
	if got != "second-value" {
		t.Errorf("Get returned %v, want %q", got, "second-value")
	}
}

// TestPolicyStore_Clear verifies that Clear() removes all stored entries so
// subsequent Get calls return (nil, false).
func TestPolicyStore_Clear(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	arns := []string{
		"arn:aws:iam::111111111111:policy/A",
		"arn:aws:iam::222222222222:policy/B",
		"arn:aws:iam::333333333333:policy/C",
	}
	for _, arn := range arns {
		store.Set(arn, "doc")
	}

	store.Clear()

	for _, arn := range arns {
		got, ok := store.Get(arn)
		if ok {
			t.Errorf("expected ok=false for %q after Clear(), got true (value=%v)", arn, got)
		}
	}
}

// TestPolicyStore_ConcurrentSafe verifies that the PolicyStore implementation
// is safe for concurrent use. 100 goroutines each Set and Get on disjoint
// keys; the race detector validates no data race occurs.
func TestPolicyStore_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	store := session.NewPolicyStore()

	const workers = 100
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("arn:aws:iam::%012d:policy/P%d", n, n)
			store.Set(key, n)
			_, _ = store.Get(key)
		}(i)
	}

	wg.Wait()
}
