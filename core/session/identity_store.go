// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// identity_store.go — session-scoped caller identity store interface and
// thread-safe implementation.
//
// Replaces the package-globals that previously lived in
// core/aws/identity_cache.go.
package session

import "sync"

// identityStore is a session-scoped cache for the AWS caller's account ID.
// Pattern C related-checkers (e.g. Backup ListRecoveryPointsByResource, Glue
// GetTags) need the caller's account to construct ARNs; this store memoizes
// the STS GetCallerIdentity result for the lifetime of one Session.
//
// The store records BOTH success (cached AccountID) and failure (cached
// non-nil Err). A cached failure suppresses retry — callers see Err() != nil
// and skip the STS call rather than thrashing on a permission error every
// related-check pass. Session.Rotate() clears both on profile/region switch.
//
// Safe for concurrent use. core/aws consumes it via its own local
// structural interface (identityStore in core/aws/client.go) rather than
// importing this type, so the method set below is the real contract.
type identityStore struct {
	mu        sync.RWMutex
	accountID string
	err       error
}

// NewIdentityStore returns a new thread-safe identityStore.
func NewIdentityStore() *identityStore {
	return &identityStore{}
}

// AccountID returns the cached AWS account ID, or "" if no successful fetch
// has been recorded.
func (s *identityStore) AccountID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accountID
}

// Err returns the cached error from the last fetch attempt. Non-nil means a
// prior call failed AND the failure is sticky — callers must not retry until
// Clear() is invoked (e.g. via Session.Rotate).
func (s *identityStore) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

// Set records the result of a fetch. id == "" + err == nil is invalid (use
// Clear() instead). On success: id non-empty, err nil. On failure: id empty,
// err non-nil.
func (s *identityStore) Set(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Anti-poison: refuse to overwrite an already-cached successful
	// AccountID with a later failure. Two concurrent Pattern-C checks may
	// both observe an empty store and both call STS.GetCallerIdentity;
	// if the slower one times out (10s checker context) AFTER the faster
	// one cached a good account ID, the naive write would erase the good
	// value and leave the session stuck reporting RelatedUnknown. See store godoc.
	if id == "" && err != nil && s.accountID != "" {
		return
	}
	s.accountID = id
	s.err = err
}

// Clear resets the cache so the next call falls through to a fresh fetch.
// Called by Session.Rotate on profile/region switch.
func (s *identityStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountID = ""
	s.err = nil
}
