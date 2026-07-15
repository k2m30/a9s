// filtered_rows_cache.go — session home for server-side-filtered
// related-drill results (cache contract C6).
//
// A related-panel row backed by a FetchFilter (e.g. a KMS key's "CloudTrail
// Events" pivot) always fetches with Cache:CacheNone and lands only on the
// pushed screen's local ListState.Rows — never in RowStore, because a
// filtered subset stored under the type's canonical RowStore key would
// poison that type's global row set for every other consumer of the type
// (the plain resource list, other pivots, availability counts). Without a
// session home, re-entering the same drill re-fetches behind a bare
// full-screen Loading every time, even though the previous result is still
// valid.
//
// FilteredRowsLRU gives these results a session-scoped home keyed by
// (resource type + fetch filter) so the next entry into the same drill
// replays instantly while a background fetch verifies/refreshes the
// content. Same LRU idiom as RelatedCacheLRU: container/list + index map,
// no mutex — all Model updates run on the single Bubble Tea goroutine.
package session

import (
	"container/list"
	"sort"
	"strings"

	"github.com/k2m30/a9s/v3/core/resource"
)

// MaxFilteredRowsEntries bounds FilteredRowsLRU.
const MaxFilteredRowsEntries = 100

// FilteredRowsEntry is the cached, accumulated result of one filtered
// related drill.
type FilteredRowsEntry struct {
	Rows      []resource.Resource
	Truncated bool
	Cursor    string
}

type filteredRowsItem struct {
	key   string
	entry FilteredRowsEntry
}

// FilteredRowsLRU is a bounded LRU cache for filtered related-drill results.
type FilteredRowsLRU struct {
	cap   int
	index map[string]*list.Element
	order *list.List
}

// NewFilteredRowsLRU constructs a new FilteredRowsLRU with the given capacity.
func NewFilteredRowsLRU(cap int) *FilteredRowsLRU {
	return &FilteredRowsLRU{
		cap:   cap,
		index: make(map[string]*list.Element),
		order: list.New(),
	}
}

// Get retrieves the cached entry for the given key, promoting it to
// most-recently-used.
func (c *FilteredRowsLRU) Get(key string) (FilteredRowsEntry, bool) {
	el, ok := c.index[key]
	if !ok {
		return FilteredRowsEntry{}, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*filteredRowsItem).entry, true
}

// Set stores the entry for the given key, evicting the LRU entry if at
// capacity.
func (c *FilteredRowsLRU) Set(key string, e FilteredRowsEntry) {
	if el, ok := c.index[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*filteredRowsItem).entry = e
		return
	}
	el := c.order.PushFront(&filteredRowsItem{key: key, entry: e})
	c.index[key] = el
	if c.order.Len() > c.cap {
		back := c.order.Back()
		if back != nil {
			c.order.Remove(back)
			delete(c.index, back.Value.(*filteredRowsItem).key)
		}
	}
}

// Clear removes all entries from the cache.
func (c *FilteredRowsLRU) Clear() {
	c.index = make(map[string]*list.Element)
	c.order.Init()
}

// Len returns the number of entries currently in the cache.
func (c *FilteredRowsLRU) Len() int {
	return c.order.Len()
}

// FilteredRowsKey builds a deterministic FilteredRowsLRU key from a resource
// type and its fetch filter. Filter keys are sorted so equal maps always
// produce the same key regardless of iteration order.
func FilteredRowsKey(resourceType string, filter map[string]string) string {
	keys := make([]string, 0, len(filter))
	for k := range filter {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(resourceType)
	for _, k := range keys {
		b.WriteByte(0)
		b.WriteString(k)
		b.WriteByte(1)
		b.WriteString(filter[k])
	}
	return b.String()
}
