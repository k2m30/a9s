// app_related.go contains related-resource navigation and check dispatch handlers.
package tui

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// maxConcurrentProbes is the maximum number of related-resource checker goroutines
// that may run concurrently for a single detail view. Matches the architecture spec.
const maxConcurrentProbes = 4

// handleRelatedCheckStarted dispatches one async tea.Cmd per registered RelatedDef
// for the given resource type. In demo mode it calls the registered demo checker;
// in live mode it calls def.Checker with a 10-second timeout.
func (m Model) handleRelatedCheckStarted(msg messages.RelatedCheckStartedMsg) (tea.Model, tea.Cmd) {
	defs := resource.GetRelated(msg.ResourceType)
	if len(defs) == 0 {
		return m, nil
	}

	cache := m.buildResourceCacheSnapshot()
	gen := m.RelatedGen

	// Build a set of keys from the main ResourceCache (scope-filtered,
	// authoritative first-page results). This set is used for the prefetch
	// decision inside each checker goroutine so that:
	//   (a) lazy-only entries (IsTruncated=true in snapshot, not in mainCacheKeys)
	//       still trigger a prefetch even though they appear in the snapshot, and
	//   (b) real first-page entries (in mainCacheKeys) suppress the prefetch.
	// Captures the map by value into each closure; the map is read-only after
	// construction so concurrent access is safe without a mutex.
	mainCacheKeys := make(map[string]struct{}, len(m.ResourceCache))
	for k := range m.ResourceCache {
		mainCacheKeys[k] = struct{}{}
	}

	// Per-call semaphore: cap concurrent probes to maxConcurrentProbes so a
	// resource type with many defs (e.g., EC2 with 10) doesn't saturate the
	// goroutine pool. Created fresh per call so each detail-view open gets its
	// own independent budget.
	sem := make(chan struct{}, maxConcurrentProbes)
	cmds := make([]tea.Cmd, 0, len(defs))

	for _, def := range defs {
		// Capture per-closure copies so concurrent goroutines cannot race on the
		// shared outer variables.
		localCache := cache
		cmds = append(cmds, func() (out tea.Msg) {
			sem <- struct{}{}
			defer func() { <-sem }()
			// Defense in depth: a panic inside a checker or paginated fetcher (e.g.
			// from a buggy fake, an SDK regression, or a nil-typed concrete client
			// during partial migrations) must not kill the entire TUI. Surface as a
			// Count=-1 error sentinel so the related panel renders an error state.
			defer func() {
				if r := recover(); r != nil {
					out = messages.RelatedCheckResultMsg{
						ResourceType:     msg.ResourceType,
						SourceResourceID: msg.SourceResource.ID,
						DefDisplayName:   def.DisplayName,
						Result:           resource.RelatedCheckResult{TargetType: def.TargetType, Count: -1},
						Generation:       gen,
					}
				}
			}()

			if def.Checker == nil {
				return messages.RelatedCheckResultMsg{
					ResourceType:     msg.ResourceType,
					SourceResourceID: msg.SourceResource.ID,
					DefDisplayName:   def.DisplayName,
					Result:           resource.RelatedCheckResult{TargetType: def.TargetType, Count: -1},
					Generation:       gen,
				}
			}

			ctx, cancel := context.WithTimeout(m.appCtx, 10*time.Second)
			defer cancel()

			// Only pre-fetch the target type if this checker actually reads it from
			// the cache (NeedsTargetCache=true). Field-only checkers (e.g., checkEC2EBS)
			// ignore the cache entirely, so fetching would be wasted AWS API calls.
			var cachedPages map[string]resource.ResourceCacheEntry
			if def.NeedsTargetCache {
				// Prefetch fires when the target is absent from the main resourceCache.
				// Use mainCacheKeys (scope-filtered, authoritative first-page results) for
				// the decision — after fix 5, lazy-only entries appear in the snapshot
				// with IsTruncated=true, but they are sparse and still need a real prefetch.
				if _, inMainCache := mainCacheKeys[def.TargetType]; !inMainCache {
					if pf := resource.GetPaginatedFetcher(def.TargetType); pf != nil {
						if fr, err := pf(ctx, m.clients, ""); err == nil {
							isTrunc := fr.Pagination != nil && fr.Pagination.IsTruncated
							// If the snapshot already had a lazy-only entry (IsTruncated=true),
							// preserve that signal: the prefetch fetched a real first page but
							// the lazy entry told us the type was sparse. Keep IsTruncated=true
							// so the checker knows there may be more data beyond this first page.
							if prev, hasPrev := localCache[def.TargetType]; hasPrev && prev.IsTruncated {
								isTrunc = true
							}
							entry := resource.ResourceCacheEntry{
								Resources:   fr.Resources,
								IsTruncated: isTrunc,
								Pagination:  fr.Pagination,
							}
							// Enrich this closure's snapshot; never write back to the outer variable.
							enriched := make(resource.ResourceCache, len(localCache)+1)
							maps.Copy(enriched, localCache)
							enriched[def.TargetType] = entry
							localCache = enriched
							cachedPages = map[string]resource.ResourceCacheEntry{def.TargetType: entry}
						}
					}
				}
			}

			result := def.Checker(ctx, m.clients, msg.SourceResource, localCache)
			result.TargetType = def.TargetType

			// Lazy-add: if the checker emitted IDs not in the target cache,
			// ask the target type to fetch them by ID. Resolves the
			// filtered-target drill-to-empty bug class (kms customer-managed
			// filter, ami owners=self, ebs-snap owners=self, iam policy
			// scope=local). Without this, a checker can emit an AWS-managed
			// KMS key or a public AMI ID, the count renders > 0, but the
			// drill lands on an empty list because the top-level list fetcher
			// filters those targets out.
			//
			// Lazy-added rows travel on LazyAddedResources, not CachedPages —
			// they are a sparse set of IDs and must NOT replace or masquerade
			// as a complete first page.
			var lazyAdded map[string][]resource.Resource
			var lazyAddError error
			if len(result.ResourceIDs) > 0 {
				if ff := resource.GetFetchByIDs(def.TargetType); ff != nil {
					missing := missingFromCache(localCache, def.TargetType, result.ResourceIDs)
					if len(missing) > 0 {
						extra, fetchErr := ff(ctx, m.clients, missing)
						if fetchErr != nil {
							lazyAddError = fetchErr
						}
						if len(extra) > 0 {
							entry := localCache[def.TargetType]
							entry.Resources = append(append([]resource.Resource(nil), entry.Resources...), extra...)
							enriched := make(resource.ResourceCache, len(localCache)+1)
							maps.Copy(enriched, localCache)
							enriched[def.TargetType] = entry
							localCache = enriched
							lazyAdded = map[string][]resource.Resource{def.TargetType: extra}
						}
					}
				}
			}

			return messages.RelatedCheckResultMsg{
				ResourceType:       msg.ResourceType,
				SourceResourceID:   msg.SourceResource.ID,
				DefDisplayName:     def.DisplayName,
				Result:             result,
				Generation:         gen,
				CachedPages:        cachedPages,
				LazyAddedResources: lazyAdded,
				LazyAddError:       lazyAddError,
			}
		})
	}

	return m, tea.Batch(cmds...)
}


// missingFromCache returns the subset of ids that are not already present as
// Resource.ID entries in cache[targetType]. Empty-string IDs are filtered
// out (a result never identifies an empty ID). Used by the lazy-add path in
// handleRelatedCheckStarted to avoid a FetchByIDs call when every emitted ID
// is already covered by the cache.
func missingFromCache(cache resource.ResourceCache, targetType string, ids []string) []string {
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

// relatedListOpts configures a related-navigation resource list.
type relatedListOpts struct {
	pendingFilter        string
	relatedIDs           []string
	autoOpenSingleDetail bool
	reapplyChecker       resource.RelatedChecker // carried forward for m-loads-more re-apply
}

// newRelatedList creates a ResourceListModel configured for related-resource navigation,
// pushes it onto the view stack, and returns the init command.
// The caller decides whether to batch the fetch command (cache-hit branches skip it).
func (m *Model) newRelatedList(rt resource.ResourceTypeDef, src resource.Resource, opts relatedListOpts) tea.Cmd {
	rl := views.NewResourceList(rt, m.viewConfig, m.keys)
	rl.SetTitleSuffix(relatedTitleSuffix(src))
	if opts.pendingFilter != "" {
		rl.SetPendingFilter(opts.pendingFilter)
	}
	if len(opts.relatedIDs) > 0 {
		rl.SetRelatedIDFilter(opts.relatedIDs)
	}
	if opts.reapplyChecker != nil {
		rl.SetReapplyChecker(opts.reapplyChecker, src)
	}
	if opts.autoOpenSingleDetail {
		rl.SetAutoOpenSingleDetail(true)
	}
	rl.SetEscPops(true)
	rl.SetSize(m.innerSize())
	rl, initCmd := rl.Init()
	m.pushView(&rl)
	return initCmd
}

func relatedTitleSuffix(src resource.Resource) string {
	if src.ID == "" {
		return ""
	}
	if src.Name != "" {
		return fmt.Sprintf(" -- %s (%s)", src.ID, src.Name)
	}
	return " -- " + src.ID
}

// buildResourceCacheSnapshot returns a read-only snapshot of currently-loaded
// resource lists, keyed by resource short name. Merges resourceCache and
// lazyResourceCache so related checkers see the full set (including
// out-of-scope entries pulled via FetchByIDs). On ID collision, resourceCache
// wins (it is the scope-filtered authoritative source).
func (m *Model) buildResourceCacheSnapshot() resource.ResourceCache {
	snap := make(resource.ResourceCache, len(m.ResourceCache)+len(m.LazyResourceCache)+len(m.ProbeResources))
	// Seed from LazyResourceCache first; ResourceCache entries will overwrite.
	// Lazy-only entries are sparse (FetchByIDs, not a full first page), so mark
	// IsTruncated=true — the next top-level navigation will still fetch
	// authoritatively instead of treating this sparse set as complete.
	for shortName, rows := range m.LazyResourceCache {
		snap[shortName] = resource.ResourceCacheEntry{
			Resources:   rows,
			IsTruncated: true,
		}
	}
	// Then merge ProbeResources — first-page rows retained by the
	// availability/Wave-1 probe pass that runs at app start, BEFORE the
	// user opens any list view. Without this seeding, cross-ref enrichers
	// running at probe time (e.g. dbi-snap → dbi cache) would see an empty
	// snapshot until the user navigates into another list. ProbeResources
	// pages are first-page-only — mark IsTruncated=true so the orphan
	// rule (in cross-ref enrichers) treats parent-not-found as
	// "unknown, skip" rather than "definitively deleted" per spec §3.1.
	for shortName, rows := range m.ProbeResources {
		probeTrunc := m.ProbeTruncated[shortName] // false when absent (complete single page)
		if existing, ok := snap[shortName]; ok {
			// LazyResourceCache already seeded — merge probe rows for any
			// new IDs (probe is first-page authoritative; lazy is sparse).
			// IsTruncated: either source being truncated makes the merged entry truncated.
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
			snap[shortName] = resource.ResourceCacheEntry{
				Resources:   merged,
				IsTruncated: existing.IsTruncated || probeTrunc,
			}
		} else {
			snap[shortName] = resource.ResourceCacheEntry{
				Resources:   rows,
				IsTruncated: probeTrunc,
			}
		}
	}
	for shortName, entry := range m.ResourceCache {
		// ResourceCache is authoritative — overwrite anything from lazy/probe.
		// Carry the entry's pagination state (IsTruncated) verbatim so
		// callers can distinguish "complete cache" from "first page only".
		// Preserve the probe's IsTruncated signal via OR: if the probe reported
		// truncation for this type but the ResourceCache entry carries nil
		// pagination (seeded before the probe fix landed), honour the probe.
		cacheIsTruncated := (entry.Pagination != nil && entry.Pagination.IsTruncated) || m.ProbeTruncated[shortName]
		if existing, ok := snap[shortName]; ok {
			// Merge: ResourceCache rows win on collision; append non-cache IDs.
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
			snap[shortName] = resource.ResourceCacheEntry{
				Resources:   merged,
				IsTruncated: cacheIsTruncated,
			}
		} else {
			snap[shortName] = resource.ResourceCacheEntry{
				Resources:   entry.Resources,
				IsTruncated: cacheIsTruncated,
			}
		}
	}
	return snap
}

// enterChildForResource returns the ChildViewDef registered under Key="enter"
// for a resource type, or nil if none is registered or its DrillCondition
// vetoes the given row. Mirror of (ResourceListModel).enterChildFor — used
// when related-navigation takes the cache-hit fast path (KindDetail) and
// must replicate manual-Enter behavior without instantiating a list view.
func enterChildForResource(td *resource.ResourceTypeDef, r resource.Resource) *resource.ChildViewDef {
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

// buildChildContextForResource resolves ContextKeys for a ChildViewDef given
// the selected resource. Mirror of (ResourceListModel).buildChildContext for
// the KindDetail fast path, without parent-context chaining ("@parent.*"
// sources collapse to empty because related-navigation starts from a fresh
// detail drill, not a nested child stack).
func buildChildContextForResource(child resource.ChildViewDef, r resource.Resource) map[string]string {
	ctx := make(map[string]string, len(child.ContextKeys))
	for param, source := range child.ContextKeys {
		switch {
		case source == "ID":
			ctx[param] = r.ID
		case source == "Name":
			ctx[param] = r.Name
		case strings.HasPrefix(source, "@parent."):
			// Unreachable from a related-navigation KindDetail entry — the
			// source resource is not a child view. Leave empty rather than
			// reading uninitialised parent context.
		default:
			ctx[param] = r.Fields[source]
		}
	}
	return ctx
}
