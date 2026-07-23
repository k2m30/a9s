// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_doc_cache.go provides a session-scoped, concurrency-safe cache for
// on-demand detail documents (Step Functions state-machine definitions,
// CloudFormation stack templates). Owned by the session runtime and passed
// to detail enrichers via DetailEnrichmentCtx. Cache lifetime is tied to the
// session; profile/region rotation rebuilds the cache so entries from a
// previous account are not returned to the next one.
package aws

import "sync"

// DetailDocCache is a concurrency-safe cache for on-demand detail documents.
// Zero value is ready to use — no constructor needed.
//
// Cache keys use explicit prefixes to namespace by resource type:
//   - Step Functions definitions: "sfn:<stateMachineArn>"
//   - CloudFormation templates:   "cfn:<stackId or stackName>"
type DetailDocCache struct {
	mu sync.RWMutex
	m  map[string]any
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

// Set stores a document in the cache.
func (c *DetailDocCache) Set(key string, doc any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]any)
	}
	c.m[key] = doc
}
