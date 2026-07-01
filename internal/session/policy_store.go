// policy_store.go — session-scoped IAM policy resource cache with per-phase
// build memoization.
//
// Replaces the package-level globals that previously lived in
// internal/aws/iam_policies.go.
package session

import (
	"sync"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// PolicyStore caches IAM policy resources keyed by both PolicyName and ARN.
// The cache is built lazily on first lookup that misses, then memoized until
// Clear (typically called by Session.Rotate on profile/region switch).
//
// PolicyStore tracks two independent build phases — managed (ListPolicies
// Scope=All) and inline (ListGroups + ListGroupPolicies) — so a transient
// failure during inline enumeration does NOT poison managed-policy lookups.
type PolicyStore interface {
	// Lookup returns the cached resource for the given key (PolicyName or
	// ARN). Returns (zero, false) if the key isn't present in the cache.
	Lookup(key string) (resource.Resource, bool)

	// Set inserts a resource under the given key. Used by the build phase to
	// populate the cache. Concurrent Set calls overwrite each other.
	Set(key string, r resource.Resource)

	// ManagedBuilt reports whether the managed-policy ListPolicies(Scope=All)
	// build has completed successfully.
	ManagedBuilt() bool
	// MarkManagedBuilt sets the managed-built flag. Call after a successful
	// ListPolicies(Scope=All) walk.
	MarkManagedBuilt()

	// InlineBuilt reports whether the inline-group-policy build has completed
	// successfully. Stays false on partial failure so the next call retries.
	InlineBuilt() bool
	// MarkInlineBuilt sets the inline-built flag. Only call after a fully
	// successful ListGroups + ListGroupPolicies walk.
	MarkInlineBuilt()

	// Clear empties the cache and resets both build flags.
	Clear()
}

type policyStore struct {
	mu           sync.RWMutex
	byID         map[string]resource.Resource
	managedBuilt bool
	inlineBuilt  bool
}

// NewPolicyStore returns a new thread-safe PolicyStore implementation.
func NewPolicyStore() PolicyStore {
	return &policyStore{byID: map[string]resource.Resource{}}
}

func (s *policyStore) Lookup(key string) (resource.Resource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byID[key]
	return r, ok
}

func (s *policyStore) Set(key string, r resource.Resource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID == nil {
		s.byID = map[string]resource.Resource{}
	}
	s.byID[key] = r
}

func (s *policyStore) ManagedBuilt() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.managedBuilt
}

func (s *policyStore) MarkManagedBuilt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.managedBuilt = true
}

func (s *policyStore) InlineBuilt() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inlineBuilt
}

func (s *policyStore) MarkInlineBuilt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inlineBuilt = true
}

func (s *policyStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID = map[string]resource.Resource{}
	s.managedBuilt = false
	s.inlineBuilt = false
}
