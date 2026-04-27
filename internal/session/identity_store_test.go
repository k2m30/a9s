package session_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/internal/session"
)

// TestIdentityStore_FreshIsEmpty pins that a freshly constructed store
// reports no cached account and no error — both AccountID() and Err() must
// return their zero values.
func TestIdentityStore_FreshIsEmpty(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()

	if id := store.AccountID(); id != "" {
		t.Errorf("AccountID() on fresh store: got %q, want \"\"", id)
	}
	if err := store.Err(); err != nil {
		t.Errorf("Err() on fresh store: got %v, want nil", err)
	}
}

// TestIdentityStore_SetSuccessThenAccountID pins the success path: a
// successful Set(id, nil) is reflected by both AccountID() and Err().
func TestIdentityStore_SetSuccessThenAccountID(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()
	const acct = "111111111111"

	store.Set(acct, nil)

	if got := store.AccountID(); got != acct {
		t.Errorf("AccountID() after success Set: got %q, want %q", got, acct)
	}
	if err := store.Err(); err != nil {
		t.Errorf("Err() after success Set: got %v, want nil", err)
	}
}

// TestIdentityStore_SetFailureThenErr pins the sticky-failure path: when a
// fetch fails, Set("", err) records the error and AccountID() stays empty.
// The contract: AccountID()=="" + Err()!=nil → caller skips retry.
func TestIdentityStore_SetFailureThenErr(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()
	failure := errors.New("AccessDenied: sts:GetCallerIdentity")

	store.Set("", failure)

	if got := store.AccountID(); got != "" {
		t.Errorf("AccountID() after failure Set: got %q, want \"\"", got)
	}
	if err := store.Err(); !errors.Is(err, failure) {
		t.Errorf("Err() after failure Set: got %v, want %v", err, failure)
	}
}

// TestIdentityStore_OverwriteSet pins last-write-wins semantics under
// non-concurrent overwrite — Set followed by another Set replaces the prior
// value. Important for: a transient error followed by a successful retry
// (rare in practice — Err() suppresses retry — but the API allows it after
// Clear or in test setups).
func TestIdentityStore_OverwriteSet(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()

	store.Set("", errors.New("transient"))
	store.Set("222222222222", nil)

	if got := store.AccountID(); got != "222222222222" {
		t.Errorf("AccountID() after overwrite: got %q, want %q", got, "222222222222")
	}
	if err := store.Err(); err != nil {
		t.Errorf("Err() after success overwrite: got %v, want nil", err)
	}
}

// TestIdentityStore_Clear pins that Clear resets BOTH the account and the
// error so a subsequent fetch path runs fresh.
func TestIdentityStore_Clear(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()
	store.Set("333333333333", nil)
	store.Clear()

	if got := store.AccountID(); got != "" {
		t.Errorf("AccountID() after Clear: got %q, want \"\"", got)
	}
	if err := store.Err(); err != nil {
		t.Errorf("Err() after Clear: got %v, want nil", err)
	}

	// And from the failure side.
	store.Set("", errors.New("sticky"))
	store.Clear()

	if got := store.AccountID(); got != "" {
		t.Errorf("AccountID() after Clear (post-failure): got %q, want \"\"", got)
	}
	if err := store.Err(); err != nil {
		t.Errorf("Err() after Clear (post-failure): got %v, want nil", err)
	}
}

// TestIdentityStore_ClearIdempotent pins that Clear() on a fresh store and
// twice in a row do not panic.
func TestIdentityStore_ClearIdempotent(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()
	store.Clear()
	store.Clear()

	store.Set("444444444444", nil)
	store.Clear()
	store.Clear()
}

// TestIdentityStore_ConcurrentSafe runs 100 goroutines mixing
// AccountID/Err/Set/Clear; the race detector validates no data race.
func TestIdentityStore_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	store := session.NewIdentityStore()
	failure := errors.New("boom")

	const workers = 100
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		go func(n int) {
			defer wg.Done()
			switch n % 5 {
			case 0:
				store.Set("555555555555", nil)
			case 1:
				_ = store.AccountID()
			case 2:
				_ = store.Err()
			case 3:
				store.Set("", failure)
			case 4:
				store.Clear()
			}
		}(i)
	}

	wg.Wait()
}
