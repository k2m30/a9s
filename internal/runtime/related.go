package runtime

import (
	"maps"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// KindRelatedCheck is the TaskKind for fan-out related-resource checker probes.
const KindRelatedCheck TaskKind = "related-check"

// MaxConcurrentProbes caps concurrent checker goroutines per detail view open.
const MaxConcurrentProbes = 4

// RelatedCheckStartedEvent carries the resource type and source resource.
type RelatedCheckStartedEvent struct {
	ResourceType   string
	SourceResource resource.Resource
}

// HandleRelatedCheckStarted dispatches a KindRelatedCheck TaskRequest when
// defs are registered for the event's resource type.
func (c *Core) HandleRelatedCheckStarted(ev RelatedCheckStartedEvent) ([]UIIntent, []TaskRequest) {
	defs := resource.GetRelated(ev.ResourceType)
	if len(defs) == 0 {
		return nil, nil
	}
	return nil, []TaskRequest{{
		Key:   TaskKey{Kind: KindRelatedCheck, Scope: ev.ResourceType + "/" + ev.SourceResource.ID},
		Cache: CacheNone,
	}}
}

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

// EnterChildForResource returns the ChildViewDef registered under Key="enter",
// or nil when absent or DrillCondition vetoes the row.
func EnterChildForResource(td *resource.ResourceTypeDef, r resource.Resource) *resource.ChildViewDef {
	if td == nil {
		return nil
	}
	for i := range td.Children {
		c := &td.Children[i]
		if c.Key != "enter" {
			continue
		}
		if c.DrillCondition != nil && !c.DrillCondition(r) {
			return nil
		}
		return c
	}
	return nil
}

// BuildChildContextForResource resolves ContextKeys for a ChildViewDef.
func BuildChildContextForResource(child resource.ChildViewDef, r resource.Resource) map[string]string {
	ctx := make(map[string]string, len(child.ContextKeys))
	for param, source := range child.ContextKeys {
		switch {
		case source == "ID":
			ctx[param] = r.ID
		case source == "Name":
			ctx[param] = r.Name
		case strings.HasPrefix(source, "@parent."):
			// no parent stack in related-navigation KindDetail entry
		default:
			ctx[param] = r.Fields[source]
		}
	}
	return ctx
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

// BuildResourceCacheSnapshot returns a merged snapshot of session caches for
// related checkers. ResourceCache wins on ID collision.
func (c *Core) BuildResourceCacheSnapshot() resource.ResourceCache {
	s := c.session
	snap := make(resource.ResourceCache, len(s.ResourceCache)+len(s.LazyResourceCache)+len(s.ProbeResources))
	for shortName, rows := range s.LazyResourceCache {
		snap[shortName] = resource.ResourceCacheEntry{Resources: rows, IsTruncated: true}
	}
	for shortName, rows := range s.ProbeResources {
		pt := s.ProbeTruncated[shortName]
		if existing, ok := snap[shortName]; ok {
			known := make(map[string]struct{}, len(existing.Resources))
			for _, r := range existing.Resources {
				known[r.ID] = struct{}{}
			}
			merged := append([]resource.Resource(nil), existing.Resources...)
			for _, r := range rows {
				if _, dup := known[r.ID]; !dup {
					merged = append(merged, r)
				}
			}
			snap[shortName] = resource.ResourceCacheEntry{Resources: merged, IsTruncated: existing.IsTruncated || pt}
		} else {
			snap[shortName] = resource.ResourceCacheEntry{Resources: rows, IsTruncated: pt}
		}
	}
	for shortName, entry := range s.ResourceCache {
		if entry == nil {
			continue
		}
		cacheIsTrunc := (entry.Pagination != nil && entry.Pagination.IsTruncated) || s.ProbeTruncated[shortName]
		if existing, ok := snap[shortName]; ok {
			known := make(map[string]struct{}, len(entry.Resources))
			for _, r := range entry.Resources {
				known[r.ID] = struct{}{}
			}
			merged := append([]resource.Resource(nil), entry.Resources...)
			for _, r := range existing.Resources {
				if _, dup := known[r.ID]; !dup {
					merged = append(merged, r)
				}
			}
			snap[shortName] = resource.ResourceCacheEntry{Resources: merged, IsTruncated: cacheIsTrunc}
		} else {
			snap[shortName] = resource.ResourceCacheEntry{Resources: entry.Resources, IsTruncated: cacheIsTrunc}
		}
	}
	return snap
}

// SnapshotCache returns a flat map snapshot combining ResourceCache and
// LazyResourceCache. ResourceCache wins on ID collision.
func (c *Core) SnapshotCache() map[string][]resource.Resource {
	s := c.session
	snap := make(map[string][]resource.Resource, len(s.ResourceCache)+len(s.LazyResourceCache))
	maps.Copy(snap, s.LazyResourceCache)
	for shortName, entry := range s.ResourceCache {
		if entry == nil {
			continue
		}
		if existing, ok := snap[shortName]; ok {
			known := make(map[string]struct{}, len(entry.Resources))
			for _, r := range entry.Resources {
				known[r.ID] = struct{}{}
			}
			merged := append([]resource.Resource(nil), entry.Resources...)
			for _, r := range existing {
				if _, dup := known[r.ID]; !dup {
					merged = append(merged, r)
				}
			}
			snap[shortName] = merged
		} else {
			snap[shortName] = entry.Resources
		}
	}
	return snap
}
