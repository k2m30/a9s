// relatedcache.go — PR-05a-h4-c (AS-963) related-cache helpers.
//
// RelatedCacheKey and RelatedCacheReplay are free functions moved verbatim
// from internal/session/related_cache.go so renderer adapters can call them
// via internal/runtime instead of internal/session. RelatedCacheResult is
// re-exported as a type alias so callers can construct cache entries using
// the runtime name; the underlying type still lives in internal/session
// because *session.RelatedCacheLRU stores it and session.Session.New
// initialises the cache field.
package runtime

import (
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// RelatedCacheResult is the per-row payload stored in the related-cache
// LRU. Alias for session.RelatedCacheResult so renderer adapters need not
// import internal/session to construct cache entries.
type RelatedCacheResult = session.RelatedCacheResult

// RelatedCacheKey builds the map key for RelatedCache lookups, using the
// same `<resourceType>:<resourceID>` format as the session-side helper.
// Moved here so renderer adapters can resolve cache keys without importing
// internal/session.
func RelatedCacheKey(resourceType, resourceID string) string {
	return resourceType + ":" + resourceID
}

// RelatedCacheReplay converts cached related-check results into the
// RelatedCheckResultMsg form the detail view expects, preserving both the
// resourceType and the per-row DefDisplayName so rightcolumn replay can
// match the correct row on detail re-entry.
func RelatedCacheReplay(resourceType string, cached []RelatedCacheResult) []messages.RelatedCheckResult {
	out := make([]messages.RelatedCheckResult, len(cached))
	for i, c := range cached {
		out[i] = messages.RelatedCheckResult{
			ResourceType:   resourceType,
			DefDisplayName: c.DefDisplayName,
			Result:         c.Result,
		}
	}
	return out
}
