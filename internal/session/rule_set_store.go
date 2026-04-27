// rule_set_store.go — session-scoped SES receipt rule set store interface and
// thread-safe map-backed implementation.
//
// NOT load-bearing in PR-02a; wired in PR-02d.
package session

import "sync"

// RuleSetStore is a session-scoped store for SES receipt rule set caches.
// Implementations must be safe for concurrent use.
type RuleSetStore interface {
	Get(key string) (ruleSet any, ok bool)
	Set(key string, ruleSet any)
	Delete(key string)
	Clear()
}

type ruleSetStore struct {
	mu sync.RWMutex
	m  map[string]any
}

// NewRuleSetStore returns a new thread-safe RuleSetStore implementation.
func NewRuleSetStore() RuleSetStore {
	return &ruleSetStore{m: map[string]any{}}
}

func (s *ruleSetStore) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

func (s *ruleSetStore) Set(key string, ruleSet any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = ruleSet
}

func (s *ruleSetStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}

func (s *ruleSetStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = map[string]any{}
}
