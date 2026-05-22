// related_cache.go — LRU cache for related-resource check results.
//
// The bounded LRU and the per-row payload type live here because
// session.Session owns the cache instance (RelatedCacheLRU field). The
// `RelatedCacheKey` / `RelatedCacheReplay` free helpers used to live here
// too; PR-05a-h4-c (AS-963) moved them to internal/runtime so renderer
// adapters can resolve cache keys without importing internal/session.
// Tests and runtime code reach the helpers via runtime.RelatedCacheKey /
// runtime.RelatedCacheReplay; there is no session-side re-export to keep
// drift between two copies impossible.
//
// Moved from internal/tui/related_cache.go as part of Phase 02 session owner migration.
package session

import (
	"container/list"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// RelatedCacheLRU is a simple LRU cache for related-resource check results.
// It caps at MaxRelatedCacheEntries entries; the least-recently-used entry
// is evicted when the cap is exceeded. Thread-safety is not required because
// all Model updates run on the single Bubble Tea goroutine.
const MaxRelatedCacheEntries = 500

// RelatedCacheResult bundles the per-row DisplayName with the checker result
// so that cache replay can reconstruct the full RelatedCheckResultMsg — the
// rightcolumn view disambiguates multiple rows sharing a TargetType (e.g.
// ct-events' 4 self-pivots) by DefDisplayName, and losing it on the first
// replay would leave those rows stuck loading forever.
type RelatedCacheResult struct {
	DefDisplayName string
	Result         resource.RelatedCheckResult
}

type relatedCacheItem struct {
	key     string
	results []RelatedCacheResult
}

// RelatedCacheLRU is a bounded LRU cache for related-resource check results.
type RelatedCacheLRU struct {
	cap   int
	index map[string]*list.Element
	order *list.List
}

// NewRelatedCacheLRU constructs a new RelatedCacheLRU with the given capacity.
func NewRelatedCacheLRU(cap int) *RelatedCacheLRU {
	return &RelatedCacheLRU{
		cap:   cap,
		index: make(map[string]*list.Element),
		order: list.New(),
	}
}

// Get retrieves cached results for the given key, promoting the entry to most-recently-used.
func (c *RelatedCacheLRU) Get(key string) ([]RelatedCacheResult, bool) {
	el, ok := c.index[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*relatedCacheItem).results, true
}

// Set stores results for the given key, evicting the LRU entry if at capacity.
func (c *RelatedCacheLRU) Set(key string, results []RelatedCacheResult) {
	if el, ok := c.index[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*relatedCacheItem).results = results
		return
	}
	el := c.order.PushFront(&relatedCacheItem{key: key, results: results})
	c.index[key] = el
	if c.order.Len() > c.cap {
		back := c.order.Back()
		if back != nil {
			c.order.Remove(back)
			delete(c.index, back.Value.(*relatedCacheItem).key)
		}
	}
}

// Delete removes the entry for the given key if present.
func (c *RelatedCacheLRU) Delete(key string) {
	if el, ok := c.index[key]; ok {
		c.order.Remove(el)
		delete(c.index, key)
	}
}

// Clear removes all entries from the cache.
func (c *RelatedCacheLRU) Clear() {
	c.index = make(map[string]*list.Element)
	c.order.Init()
}

// Len returns the number of entries currently in the cache.
func (c *RelatedCacheLRU) Len() int {
	return c.order.Len()
}
