package session_test

import (
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/internal/session"
)

// TestRuleSetStore_FreshIsEmpty pins that a freshly constructed store
// reports no cached value: Get returns (nil, false).
func TestRuleSetStore_FreshIsEmpty(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()

	got, ok := store.Get()
	if ok {
		t.Errorf("Get on fresh store: ok=true, want false")
	}
	if got != nil {
		t.Errorf("Get on fresh store: value=%v, want nil", got)
	}
}

// TestRuleSetStore_SetGet pins the round-trip: the value passed to Set is
// returned verbatim by Get, with ok=true.
func TestRuleSetStore_SetGet(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()
	type ruleSetPayload struct {
		Name  string
		Rules []string
	}
	want := ruleSetPayload{Name: "default-inbound", Rules: []string{"rule-a", "rule-b"}}

	store.Set(want)

	got, ok := store.Get()
	if !ok {
		t.Fatal("Get after Set: ok=false, want true")
	}
	gotPayload, isPayload := got.(ruleSetPayload)
	if !isPayload {
		t.Fatalf("Get type: got %T, want ruleSetPayload", got)
	}
	if gotPayload.Name != want.Name || len(gotPayload.Rules) != len(want.Rules) {
		t.Errorf("Get value: got %+v, want %+v", gotPayload, want)
	}
}

// TestRuleSetStore_Overwrite pins that Set replaces the prior cached value.
func TestRuleSetStore_Overwrite(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()

	store.Set("first")
	store.Set("second")

	got, ok := store.Get()
	if !ok {
		t.Fatal("Get after overwrite: ok=false, want true")
	}
	if got != "second" {
		t.Errorf("Get after overwrite: got %v, want %q", got, "second")
	}
}

// TestRuleSetStore_Clear pins that Clear empties the store so subsequent Get
// returns (nil, false). Important for: Session.Rotate on profile/region
// switch and Ctrl+R on the SES detail view.
func TestRuleSetStore_Clear(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()
	store.Set("payload")
	store.Clear()

	got, ok := store.Get()
	if ok {
		t.Errorf("Get after Clear: ok=true, want false (value=%v)", got)
	}
	if got != nil {
		t.Errorf("Get after Clear: value=%v, want nil", got)
	}
}

// TestRuleSetStore_ClearIdempotent pins that Clear() on a fresh store and
// twice in a row do not panic.
func TestRuleSetStore_ClearIdempotent(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()
	store.Clear()
	store.Clear()

	store.Set("x")
	store.Clear()
	store.Clear()
}

// TestRuleSetStore_ConcurrentSafe runs 100 goroutines mixing Set/Get/Clear;
// the race detector validates no data race.
func TestRuleSetStore_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	store := session.NewRuleSetStore()

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
