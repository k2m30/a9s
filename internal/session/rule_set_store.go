// rule_set_store.go — session-scoped SES receipt rule set cache.
//
// Wired by PR-02d (replaces the package-level globals previously in
// internal/aws/ses_related.go: sesRuleSetCacheMu, sesRuleSetCaches map keyed
// by *ServiceClients pointer).
package session

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

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

	// GetOrFetch returns the cached value if present; otherwise calls fetcher
	// once across all concurrent callers (single-flight) and caches its result.
	// The "active" key is fixed because this store has a single slot.
	//
	// The fetcher receives a detached context built from ctx so that leader
	// cancellation does not abort in-flight upstream calls on behalf of
	// followers whose own ctx remains alive. Each waiting caller selects on
	// its own ctx.Done() and may bail early without cancelling the fetch.
	GetOrFetch(ctx context.Context, fetcher func(context.Context) (any, error)) (any, error)
}

type ruleSetStore struct {
	mu      sync.RWMutex
	ruleSet any
	ok      bool
	gen     uint64 // bumped on Clear() to detect stale writes from in-flight fetches
	sf      singleflight.Group
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
	s.gen++
}

// GetOrFetch returns the cached value if present; otherwise coalesces all
// concurrent callers into a single upstream fetch via singleflight.
//
// Key design decisions:
//
//  1. DoChan — the caller selects on the result channel OR its own ctx.Done().
//     A follower with a live ctx waits for the channel; a follower whose ctx
//     fires bails early with its own error WITHOUT aborting the in-flight fetch.
//
//  2. Detached context — the fetcher runs with context.WithoutCancel(ctx) so
//     the leader's cancellation does not propagate to the upstream API call.
//     A 30-second timeout is applied so a stuck upstream eventually releases.
//
//  3. Generation counter — if Clear() is called while a fetch is in flight,
//     the post-fetch Set is skipped (gen mismatch) so stale data from an old
//     profile never poisons the active cache.
func (s *ruleSetStore) GetOrFetch(ctx context.Context, fetcher func(context.Context) (any, error)) (any, error) {
	if v, ok := s.Get(); ok {
		return v, nil
	}

	// Snapshot generation before entering the flight so we can detect
	// a Clear() that happens while the fetch is in progress.
	s.mu.RLock()
	startGen := s.gen
	s.mu.RUnlock()

	ch := s.sf.DoChan("active", func() (any, error) {
		// Re-check inside the singleflight in case another caller filled it
		// between our miss and acquiring the flight. Cheap.
		if v, ok := s.Get(); ok {
			return v, nil
		}
		// Detach from the leader's context so its cancellation does not abort
		// the upstream API call on behalf of waiting followers.
		fetchCtx := context.WithoutCancel(ctx)
		fetchCtx, cancel := context.WithTimeout(fetchCtx, 30*time.Second)
		defer cancel()
		v, err := fetcher(fetchCtx)
		if err != nil {
			return nil, err
		}
		// Only commit the value if the store has not been cleared since we
		// started — a bumped generation means a profile/region switch happened
		// and this result is stale.
		s.mu.Lock()
		if s.gen == startGen {
			s.ruleSet = v
			s.ok = true
		}
		s.mu.Unlock()
		return v, nil
	})

	select {
	case res := <-ch:
		return res.Val, res.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
