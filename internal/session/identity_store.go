// identity_store.go — session-scoped caller identity store interface and
// thread-safe implementation.
//
// Wired by PR-02c (replaces the package-globals previously in
// internal/aws/identity_cache.go).
package session

import "sync"

// IdentityStore is a session-scoped cache for the AWS caller's account ID.
// Pattern C related-checkers (e.g. Backup ListRecoveryPointsByResource, Glue
// GetTags) need the caller's account to construct ARNs; this store memoizes
// the STS GetCallerIdentity result for the lifetime of one Session.
//
// The store records BOTH success (cached AccountID) and failure (cached
// non-nil Err). A cached failure suppresses retry — callers see Err() != nil
// and skip the STS call rather than thrashing on a permission error every
// related-check pass. Session.Rotate() clears both on profile/region switch.
//
// Implementations must be safe for concurrent use.
type IdentityStore interface {
	// AccountID returns the cached AWS account ID, or "" if no successful
	// fetch has been recorded.
	AccountID() string

	// Err returns the cached error from the last fetch attempt. Non-nil
	// means a prior call failed AND the failure is sticky — callers must
	// not retry until Clear() is invoked (e.g. via Session.Rotate).
	Err() error

	// Set records the result of a fetch. id == "" + err == nil is invalid
	// (use Clear() instead). On success: id non-empty, err nil. On failure:
	// id empty, err non-nil. Last-write-wins under contention.
	Set(id string, err error)

	// Clear resets the cache so the next call falls through to a fresh
	// fetch. Called by Session.Rotate on profile/region switch.
	Clear()
}

type identityStore struct {
	mu        sync.RWMutex
	accountID string
	err       error
}

// NewIdentityStore returns a new thread-safe IdentityStore.
func NewIdentityStore() IdentityStore {
	return &identityStore{}
}

func (s *identityStore) AccountID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accountID
}

func (s *identityStore) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (s *identityStore) Set(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountID = id
	s.err = err
}

func (s *identityStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountID = ""
	s.err = nil
}
