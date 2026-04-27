package session_test

import (
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/internal/session"
)

// TestIdentityStore_GetUnset verifies that a fresh IdentityStore returns
// (nil, false) when no identity has been stored yet.
func TestIdentityStore_GetUnset(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()

	got, ok := store.Get()
	if ok {
		t.Error("expected ok=false on fresh store, got true")
	}
	if got != nil {
		t.Errorf("expected nil on fresh store, got %v", got)
	}
}

// TestIdentityStore_SetThenGet verifies that Set followed by Get returns the
// stored identity with ok=true.
func TestIdentityStore_SetThenGet(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()

	const payload = "identity-payload"
	store.Set(payload)

	got, ok := store.Get()
	if !ok {
		t.Fatal("expected ok=true after Set, got false")
	}
	if got != payload {
		t.Errorf("Get returned %v, want %q", got, payload)
	}
}

// TestIdentityStore_Clear verifies that Clear() removes the stored identity
// so a subsequent Get returns (nil, false).
func TestIdentityStore_Clear(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()

	store.Set("identity-payload")
	store.Clear()

	got, ok := store.Get()
	if ok {
		t.Errorf("expected ok=false after Clear(), got true (value=%v)", got)
	}
	if got != nil {
		t.Errorf("expected nil after Clear(), got %v", got)
	}
}

// TestIdentityStore_ConcurrentSafe verifies that the IdentityStore
// implementation is safe for concurrent use. 100 goroutines mix Set, Get, and
// Clear; the race detector validates no data race occurs.
func TestIdentityStore_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()

	const workers = 100
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		go func(n int) {
			defer wg.Done()
			switch n % 3 {
			case 0:
				store.Set(n)
			case 1:
				_, _ = store.Get()
			case 2:
				store.Clear()
			}
		}(i)
	}

	wg.Wait()
}
