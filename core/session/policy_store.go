// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// policy_store.go — session-scoped IAM policy resource cache with per-phase
// build memoization.
//
// Replaces the package-level globals that previously lived in
// core/aws/iam_policies.go.
package session

import (
	"sync"

	"github.com/k2m30/a9s/v3/core/resource"
)

// policyStore caches IAM policy resources keyed by both PolicyName and ARN.
// The cache is built lazily on first lookup that misses, then memoized until
// Clear (typically called by Session.Rotate on profile/region switch).
//
// policyStore tracks two independent build phases — managed (ListPolicies
// Scope=All) and inline (ListGroups + ListGroupPolicies) — so a transient
// failure during inline enumeration does NOT poison managed-policy lookups.
//
// Safe for concurrent use. core/aws consumes it via its own local
// structural interface (iamPolicyStore in core/aws/iam_policies.go)
// rather than importing this type, so the method set below is the real
// contract.
type policyStore struct {
	mu           sync.RWMutex
	byID         map[string]resource.Resource
	managedBuilt bool
	inlineBuilt  bool
}

// NewPolicyStore returns a new thread-safe policyStore.
func NewPolicyStore() *policyStore {
	return &policyStore{byID: map[string]resource.Resource{}}
}

// Lookup returns the cached resource for the given key (PolicyName or ARN).
// Returns (zero, false) if the key isn't present in the cache.
func (s *policyStore) Lookup(key string) (resource.Resource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byID[key]
	return r, ok
}

// Set inserts a resource under the given key. Used by the build phase to
// populate the cache. Concurrent Set calls overwrite each other.
func (s *policyStore) Set(key string, r resource.Resource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID == nil {
		s.byID = map[string]resource.Resource{}
	}
	s.byID[key] = r
}

// ManagedBuilt reports whether the managed-policy ListPolicies(Scope=All)
// build has completed successfully.
func (s *policyStore) ManagedBuilt() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.managedBuilt
}

// MarkManagedBuilt sets the managed-built flag. Call after a successful
// ListPolicies(Scope=All) walk.
func (s *policyStore) MarkManagedBuilt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.managedBuilt = true
}

// InlineBuilt reports whether the inline-group-policy build has completed
// successfully. Stays false on partial failure so the next call retries.
func (s *policyStore) InlineBuilt() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inlineBuilt
}

// MarkInlineBuilt sets the inline-built flag. Only call after a fully
// successful ListGroups + ListGroupPolicies walk.
func (s *policyStore) MarkInlineBuilt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inlineBuilt = true
}

// Clear empties the cache and resets both build flags.
func (s *policyStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID = map[string]resource.Resource{}
	s.managedBuilt = false
	s.inlineBuilt = false
}
