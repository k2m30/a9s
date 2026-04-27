// rule_set_store.go — session-scoped SES receipt rule set cache.
//
// Wired by PR-02d (replaces the package-level globals previously in
// internal/aws/ses_related.go: sesRuleSetCacheMu, sesRuleSetCaches map keyed
// by *ServiceClients pointer).
package session

import "sync"

// RuleSetStore is a session-scoped, single-slot cache for the SES v1
// DescribeActiveReceiptRuleSet response. Each Session owns one store; the
// keying-by-pointer that the legacy globals required for per-clients
// isolation is no longer necessary because Sessions ARE the isolation unit.
//
// The store records ONLY successful responses — errors are not cached so
// transient ListReceiptRules failures retry on the next call rather than
// locking the session for its lifetime. Session.Rotate() Clears the store on
// profile/region switch.
//
// The stored value is `any` so internal/session does not import the AWS
// SES SDK; the consumer (internal/aws/ses_related.go) does the type
// assertion at the call site.
//
// Implementations must be safe for concurrent use.
type RuleSetStore interface {
	// Get returns the cached rule set and ok=true if a successful Set has
	// been recorded; ("", false)-like semantics otherwise (typed as any).
	Get() (ruleSet any, ok bool)

	// Set caches the rule set. ok becomes true after this call.
	Set(ruleSet any)

	// Clear empties the cache. Called by Session.Rotate on profile/region
	// switch and by Ctrl+R on the SES detail view (so receipt-rule changes
	// are picked up without waiting for a full reconnect).
	Clear()
}

type ruleSetStore struct {
	mu      sync.RWMutex
	ruleSet any
	ok      bool
}

// NewRuleSetStore returns a new thread-safe RuleSetStore.
func NewRuleSetStore() RuleSetStore {
	return &ruleSetStore{}
}

func (s *ruleSetStore) Get() (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ruleSet, s.ok
}

func (s *ruleSetStore) Set(ruleSet any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ruleSet = ruleSet
	s.ok = true
}

func (s *ruleSetStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ruleSet = nil
	s.ok = false
}
