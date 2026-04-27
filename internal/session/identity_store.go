// identity_store.go — session-scoped caller identity store interface and
// thread-safe implementation.
//
// NOT load-bearing in PR-02a; wired in PR-02c.
package session

import "sync"

// IdentityStore is a session-scoped store for the AWS caller identity.
// Implementations must be safe for concurrent use.
type IdentityStore interface {
	Get() (identity any, ok bool)
	Set(identity any)
	Clear()
}

type identityStore struct {
	mu       sync.RWMutex
	identity any
	ok       bool
}

// NewIdentityStore returns a new thread-safe IdentityStore implementation.
func NewIdentityStore() IdentityStore {
	return &identityStore{}
}

func (s *identityStore) Get() (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identity, s.ok
}

func (s *identityStore) Set(identity any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identity = identity
	s.ok = true
}

func (s *identityStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identity = nil
	s.ok = false
}
