// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_doc_cache.go provides a session-scoped, concurrency-safe cache for
// on-demand detail documents (currently: CloudFormation stack templates).
// Owned by the session runtime and passed to detail enrichers via
// DetailEnrichmentCtx. Cache lifetime is tied to the session; profile/region
// rotation rebuilds the cache so entries from a previous account are not
// returned to the next one.
//
// Step Functions definitions are deliberately UNCACHED by design (see
// enrichSfn's doc comment: the ARN survives a redeploy, so caching would
// serve a stale definition exactly while an operator watches a deploy) — this
// cache currently holds only cfn entries.
package aws

import (
	"strings"
	"sync"

	"github.com/k2m30/a9s/v3/core/domain"
)

// opAwareDocStore is the shared op-aware document-cache implementation
// embedded by both DetailDocCache and PolicyDocumentCache — the state and
// Get/Set/SetIfNewer bodies used to be duplicated verbatim between the two
// (this repo's no-duplicate-truth rule); both exported types keep their own
// distinct key namespaces and public method signatures unchanged.
type opAwareDocStore struct {
	mu       sync.RWMutex
	m        map[string]any
	writerOp map[string]domain.Gen
	// latestKey tracks the current versioned key per logical resource
	// identity, for callers that pass one (see setIfNewer's resource
	// parameter). Only DetailDocCache uses this; PolicyDocumentCache's keys
	// are stable and always pass "" (no bookkeeping, no eviction).
	latestKey map[string]string
}

func (s *opAwareDocStore) get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.m == nil {
		return nil
	}
	return s.m[key]
}

func (s *opAwareDocStore) set(key string, doc any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = make(map[string]any)
	}
	s.m[key] = doc
}

// setIfNewer stores doc under key unless opID is strictly older than the
// opID a prior setIfNewer call recorded for that key, in which case the
// write is refused (a stale, already-superseded detail-refresh's write must
// never land after a newer operation's fresh write and silently resurrect
// stale content). opID == 0 is a special "no operation" writer: the write is
// always accepted but never recorded as the key's writer, so it can never
// raise the bar against a later write.
//
// When resource is non-empty, key is treated as one version-stamped
// generation of that logical resource: any previously accepted key recorded
// for the same resource is evicted (its doc and writerOp entry both
// removed), so a resource whose key changes on every update (e.g. a
// CloudFormation stack's "cfn:<stackId>:<version>" key) does not grow this
// cache without bound for the life of the session. resource == "" skips this
// bookkeeping entirely.
//
// Returns whether the write was accepted.
func (s *opAwareDocStore) setIfNewer(key, resource string, doc any, opID domain.Gen) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if opID != 0 {
		if recorded, ok := s.writerOp[key]; ok && opID < recorded {
			return false
		}
	}
	if s.m == nil {
		s.m = make(map[string]any)
	}
	s.m[key] = doc
	if opID != 0 {
		if s.writerOp == nil {
			s.writerOp = make(map[string]domain.Gen)
		}
		s.writerOp[key] = opID
	}
	if resource != "" {
		if s.latestKey == nil {
			s.latestKey = make(map[string]string)
		}
		if prev, ok := s.latestKey[resource]; ok && prev != key {
			delete(s.m, prev)
			delete(s.writerOp, prev)
		}
		s.latestKey[resource] = key
	}
	return true
}

// DetailDocCache is a concurrency-safe cache for on-demand detail documents.
// Zero value is ready to use — no constructor needed.
//
// Cache keys use an explicit prefix plus a version stamp:
//   - CloudFormation templates: "cfn:<stackId or stackName>:<versionUnixSeconds>"
type DetailDocCache struct {
	opAwareDocStore
}

// Get returns the cached document for the given key, or nil if absent.
func (c *DetailDocCache) Get(key string) any { return c.get(key) }

// Set stores a document in the cache, unconditionally — the op-oblivious
// write path every caller outside the on-demand detail-enrichment engine
// uses unchanged.
func (c *DetailDocCache) Set(key string, doc any) { c.set(key, doc) }

// SetIfNewer stores doc under key per opAwareDocStore.setIfNewer's contract,
// deriving the "cfn:<id>" logical-resource identity from key (everything
// before its final ":<version>" segment) so accepting a new version evicts
// the stack's previous version-stamped entry instead of leaving it to grow
// the cache forever.
func (c *DetailDocCache) SetIfNewer(key string, doc any, opID domain.Gen) bool {
	return c.setIfNewer(key, versionedKeyResource(key), doc, opID)
}

// versionedKeyResource returns the logical-resource portion of a
// "<prefix>:<id>:<version>" cache key (everything before the final colon),
// or "" when the key has no version segment to strip. Safe even when id
// itself contains colons (e.g. a CloudFormation StackId ARN): the version
// stamp is always a plain decimal string with no colon of its own, so the
// LAST colon in the key is always the separator enrichCfn's cacheKey
// introduced, never one from inside id.
func versionedKeyResource(key string) string {
	i := strings.LastIndex(key, ":")
	if i < 0 {
		return ""
	}
	return key[:i]
}
