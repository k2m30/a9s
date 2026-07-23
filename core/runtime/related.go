// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	"time"

	"github.com/k2m30/a9s/v3/core/resource"
)

// KindRelatedCheck is the TaskKind for fan-out related-resource checker probes.
const KindRelatedCheck TaskKind = "related-check"

// MaxConcurrentProbes caps concurrent checker goroutines per detail view open.
const MaxConcurrentProbes = 4

// RelatedCheckerTimeout bounds a single RelatedDef checker call (including
// its NeedsTargetCache prefetch and lazy-add FetchByIDs call, if any) — the
// same per-checker budget the TUI's fan-out has always used, now shared with
// the headless executor's fan-out via RunRelatedDef (executor.go).
const RelatedCheckerTimeout = 10 * time.Second

// RelatedTitleSuffix returns the " -- id (name)" suffix for list titles.
func RelatedTitleSuffix(src resource.Resource) string {
	if src.ID == "" {
		return ""
	}
	if src.Name != "" {
		return " -- " + src.ID + " (" + src.Name + ")"
	}
	return " -- " + src.ID
}

// MissingFromCache returns ids absent from cache[targetType], excluding empty
// strings and duplicates. Used by the lazy-add path in the BT adapter.
func MissingFromCache(cache resource.ResourceCache, targetType string, ids []string) []string {
	known := make(map[string]struct{})
	if entry, ok := cache[targetType]; ok {
		for _, r := range entry.Resources {
			known[r.ID] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(ids))
	var missing []string
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, hit := known[id]; hit {
			continue
		}
		missing = append(missing, id)
	}
	return missing
}

// BuildResourceCacheSnapshot is defined in probes.go. Every RunRelatedDef
// caller — the TUI's per-def fan-out (runtime_adapter_related.go) and the
// executor's KindRelatedCheck case (executor.go) alike — builds its
// cacheSnap argument via Core.BuildResourceCacheSnapshot, so both lanes see
// the identical IsTruncated-aware snapshot.
