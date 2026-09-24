// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// relatedcache.go — related-cache helpers.
//
// RelatedCacheKey is a free function that lives here (rather than
// core/session) so renderer adapters can resolve cache keys via
// core/runtime instead of importing core/session. RelatedCacheResult is
// re-exported as a type alias so callers can construct cache entries using
// the runtime name; the underlying type lives in core/session
// because *session.RelatedCacheLRU stores it and session.Session.New
// initialises the cache field.
package runtime

import (
	"github.com/k2m30/a9s/v3/core/session"
)

// RelatedCacheResult is the per-row payload stored in the related-cache
// LRU. Alias for session.RelatedCacheResult so renderer adapters need not
// import core/session to construct cache entries.
type RelatedCacheResult = session.RelatedCacheResult

// RelatedCacheKey builds the map key for RelatedCache lookups of a resource
// read in the session's Region, `<resourceType>:<resourceID>`. Lives here so
// renderer adapters can resolve cache keys without importing core/session.
func RelatedCacheKey(resourceType, resourceID string) string {
	return RelatedCacheKeyIn(resourceType, "", resourceID)
}

// RelatedCacheKeyIn is RelatedCacheKey for a resource read in region ("" for
// the session's), `<resourceType>@<region>:<resourceID>`: a name-keyed type
// can hold the same ID in two Regions.
func RelatedCacheKeyIn(resourceType, region, resourceID string) string {
	if region != "" {
		resourceType += "@" + region
	}
	return resourceType + ":" + resourceID
}
