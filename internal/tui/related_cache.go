package tui

// related_cache.go — LRU cache and replay helpers for related-resource check
// results, keyed by `<resourceType>:<resourceID>`. Hits replay each row's
// RelatedCheckResultMsg into the rightcolumn view on detail re-entry so the
// pivot table re-paints without re-issuing AWS describe calls.
//
// Split from app.go to keep that file under the 500-line file-size budget.

import (
	"container/list"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// relatedCacheKey builds the map key for relatedCache lookups.
func relatedCacheKey(resourceType, resourceID string) string {
	return resourceType + ":" + resourceID
}

// relatedCacheReplay converts cached related-check results into the
// RelatedCheckResultMsg form the detail view expects, preserving both the
// resourceType and the per-row DefDisplayName so rightcolumn replay can
// match the correct row on detail re-entry.
func relatedCacheReplay(resourceType string, cached []relatedCacheResult) []messages.RelatedCheckResultMsg {
	out := make([]messages.RelatedCheckResultMsg, len(cached))
	for i, c := range cached {
		out[i] = messages.RelatedCheckResultMsg{
			ResourceType:   resourceType,
			DefDisplayName: c.DefDisplayName,
			Result:         c.Result,
		}
	}
	return out
}

// relatedCacheLRU is a simple LRU cache for related-resource check results.
// It caps at maxRelatedCacheEntries entries; the least-recently-used entry
// is evicted when the cap is exceeded. Thread-safety is not required because
// all Model updates run on the single Bubble Tea goroutine.
const maxRelatedCacheEntries = 500

type relatedCacheLRU struct {
	cap   int
	index map[string]*list.Element
	order *list.List
}

// relatedCacheResult bundles the per-row DisplayName with the checker result
// so that cache replay can reconstruct the full RelatedCheckResultMsg — the
// rightcolumn view disambiguates multiple rows sharing a TargetType (e.g.
// ct-events' 4 self-pivots) by DefDisplayName, and losing it on the first
// replay would leave those rows stuck loading forever.
type relatedCacheResult struct {
	DefDisplayName string
	Result         resource.RelatedCheckResult
}

type relatedCacheItem struct {
	key     string
	results []relatedCacheResult
}

func newRelatedCacheLRU(cap int) *relatedCacheLRU {
	return &relatedCacheLRU{
		cap:   cap,
		index: make(map[string]*list.Element),
		order: list.New(),
	}
}

func (c *relatedCacheLRU) get(key string) ([]relatedCacheResult, bool) {
	el, ok := c.index[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*relatedCacheItem).results, true
}

func (c *relatedCacheLRU) set(key string, results []relatedCacheResult) {
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

func (c *relatedCacheLRU) delete(key string) {
	if el, ok := c.index[key]; ok {
		c.order.Remove(el)
		delete(c.index, key)
	}
}

func (c *relatedCacheLRU) clear() {
	c.index = make(map[string]*list.Element)
	c.order.Init()
}

func (c *relatedCacheLRU) len() int {
	return c.order.Len()
}
