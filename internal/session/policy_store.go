// policy_store.go — session-scoped IAM policy document store interface and
// thread-safe map-backed implementation.
//
// NOT load-bearing in PR-02a; wired in PR-02b.
package session

import "sync"

// PolicyStore is a session-scoped store for IAM policy documents.
// Implementations must be safe for concurrent use.
type PolicyStore interface {
	Get(arn string) (policy any, ok bool)
	Set(arn string, policy any)
	Clear()
}

type policyStore struct {
	mu sync.RWMutex
	m  map[string]any
}

// NewPolicyStore returns a new thread-safe PolicyStore implementation.
func NewPolicyStore() PolicyStore {
	return &policyStore{m: map[string]any{}}
}

func (s *policyStore) Get(arn string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[arn]
	return v, ok
}

func (s *policyStore) Set(arn string, policy any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[arn] = policy
}

func (s *policyStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = map[string]any{}
}
