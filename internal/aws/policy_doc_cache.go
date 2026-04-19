// policy_doc_cache.go provides a session-scoped, concurrency-safe cache for
// decoded IAM policy documents. Owned by the session runtime (tui.Model's
// embedded sessionRuntime) and passed to detail enrichers via
// DetailEnrichmentCtx. Cache lifetime is tied to the session; profile/region
// rotation rebuilds the cache so entries from a previous account are not
// returned to the next one.
package aws

import "sync"

// PolicyDocumentCache is a concurrency-safe cache for decoded IAM policy documents.
// Zero value is ready to use — no constructor needed.
//
// Cache keys use explicit prefixes to namespace by policy type:
//   - managed policies: "managed:<policyArn>"
//   - inline policies:  "inline:<roleName>/<policyName>"
type PolicyDocumentCache struct {
	mu sync.RWMutex
	m  map[string]any
}

// Get returns the cached document for the given key, or nil if absent.
func (c *PolicyDocumentCache) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.m == nil {
		return nil
	}
	return c.m[key]
}

// Set stores a document in the cache.
func (c *PolicyDocumentCache) Set(key string, doc any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]any)
	}
	c.m[key] = doc
}

// ManagedKey returns the cache key for a managed policy document.
func ManagedKey(policyArn string) string {
	return "managed:" + policyArn
}

// InlineKey returns the cache key for an inline policy document.
func InlineKey(roleName, policyName string) string {
	return "inline:" + roleName + "/" + policyName
}
