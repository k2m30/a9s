// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_doc_cache.go provides a session-scoped, concurrency-safe cache for
// on-demand detail documents (Step Functions state-machine definitions,
// CloudFormation stack templates). Owned by the session runtime and passed
// to detail enrichers via DetailEnrichmentCtx. Cache lifetime is tied to the
// session; profile/region rotation rebuilds the cache so entries from a
// previous account are not returned to the next one.
package aws

import (
	"sync"

	"github.com/k2m30/a9s/v3/core/domain"
)

// DetailDocCache is a concurrency-safe cache for on-demand detail documents.
// Zero value is ready to use — no constructor needed.
//
// Cache keys use explicit prefixes to namespace by resource type:
//   - Step Functions definitions: "sfn:<stateMachineArn>"
//   - CloudFormation templates:   "cfn:<stackId or stackName>"
type DetailDocCache struct {
	mu sync.RWMutex
	m  map[string]any
	// writerOp records the DetailOperation.ID that most recently wrote each
	// key via SetIfNewer with opID != 0 — the "bar" a later SetIfNewer call
	// for the same key must not fall strictly below to be accepted.
	writerOp map[string]domain.Gen
}

// Get returns the cached document for the given key, or nil if absent.
func (c *DetailDocCache) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.m == nil {
		return nil
	}
	return c.m[key]
}

// Set stores a document in the cache, unconditionally — the op-oblivious
// write path every caller outside the on-demand detail-enrichment engine
// uses unchanged.
func (c *DetailDocCache) Set(key string, doc any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]any)
	}
	c.m[key] = doc
}

// SetIfNewer stores doc under key unless opID is strictly older than the
// opID a prior SetIfNewer call recorded for that key, in which case the
// write is refused (a stale, already-superseded detail-refresh's write must
// never land after a newer operation's fresh write and silently resurrect
// stale content). Returns whether the write was accepted.
//
// opID == 0 is a special "no operation" writer: the write is always
// accepted but never recorded as the key's writer, so it can never raise
// the bar against a later write — mirroring Set's exact semantics for
// non-operation callers, just routed through this same entry point so the
// on-demand detail-enrichment engine (core/aws/detail_enrich_engine.go) has
// one call for both the operation-aware and op-oblivious cases.
func (c *DetailDocCache) SetIfNewer(key string, doc any, opID domain.Gen) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if opID != 0 {
		if recorded, ok := c.writerOp[key]; ok && opID < recorded {
			return false
		}
	}
	if c.m == nil {
		c.m = make(map[string]any)
	}
	c.m[key] = doc
	if opID != 0 {
		if c.writerOp == nil {
			c.writerOp = make(map[string]domain.Gen)
		}
		c.writerOp[key] = opID
	}
	return true
}
