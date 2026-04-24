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
	gen := m.relatedGen
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
				if _, inCache := localCache[def.TargetType]; !inCache {
					if pf := resource.GetPaginatedFetcher(def.TargetType); pf != nil {
						if fr, err := pf(ctx, m.clients, ""); err == nil {
							isTrunc := fr.Pagination != nil && fr.Pagination.IsTruncated
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
			if len(result.ResourceIDs) > 0 {
				if ff := resource.GetFetchByIDs(def.TargetType); ff != nil {
					missing := missingFromCache(localCache, def.TargetType, result.ResourceIDs)
					if len(missing) > 0 {
						if extra, err := ff(ctx, m.clients, missing); err == nil && len(extra) > 0 {
							entry := localCache[def.TargetType]
							entry.Resources = append(append([]resource.Resource(nil), entry.Resources...), extra...)
							enriched := make(resource.ResourceCache, len(localCache)+1)
							maps.Copy(enriched, localCache)
							enriched[def.TargetType] = entry
							localCache = enriched
							if cachedPages == nil {
								cachedPages = map[string]resource.ResourceCacheEntry{}
							}
							cachedPages[def.TargetType] = entry
						}
					}
				}
			}

			return messages.RelatedCheckResultMsg{
				ResourceType:     msg.ResourceType,
				SourceResourceID: msg.SourceResource.ID,
				DefDisplayName:   def.DisplayName,
				Result:           result,
				Generation:       gen,
				CachedPages:      cachedPages,
			}
		})
	}

	return m, tea.Batch(cmds...)
}

// handleRelatedNavigate pushes a resource list view for the related target type.
// Pre-filters the list when a specific target ID or related IDs are available.
// It delegates type-resolution and branch logic to ResolveRelatedNavigate, then
// dispatches to the appropriate view-stack push based on the result Kind.
func (m Model) handleRelatedNavigate(msg messages.RelatedNavigateMsg) (tea.Model, tea.Cmd) {
	result := ResolveRelatedNavigate(msg, m.snapshotCache())

	switch result.Kind {
	case KindFlash:
		return m, func() tea.Msg {
			return messages.FlashMsg{
				Text:    result.FlashMessage,
				IsError: result.FlashIsError,
			}
		}

	case KindEnterChildView:
		return m.handleRelatedNavigateChild(msg)

	case KindFilteredList:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
					IsError: true,
				}
			}
		}

		// FetchFilter path: use server-side filtered fetcher.
		if len(result.FetchFilter) > 0 {
			rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
			rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
			rl.SetFetchFilter(result.FetchFilter)
			rl.SetEscPops(true)
			rl.SetSize(m.innerSize())
			rl, initCmd := rl.Init()
			m.pushView(&rl)
			return m, tea.Batch(initCmd, m.fetchResourcesFiltered(msg.TargetType, result.FetchFilter))
		}

		// TargetID-based filtered list (cache miss).
		if result.TargetID != "" {
			// Exact AMI navigation should fetch by image ID instead of
			// falling back to the owned-AMI list, which misses public and third-party images.
			if msg.TargetType == "ami" && m.clients != nil {
				cmd := m.fetchAMIDetail(result.TargetID)
				return m, cmd
			}
			m.flash = flashState{
				text:    fmt.Sprintf("Resource %s not in cache; loading %s list", result.TargetID, msg.TargetType),
				isError: false,
				active:  true,
			}
			initCmd := m.newRelatedList(*rt, msg.SourceResource, relatedListOpts{
				pendingFilter:        result.TargetID,
				relatedIDs:           []string{result.TargetID},
				autoOpenSingleDetail: true,
				reapplyChecker:       msg.Checker,
			})
			return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
		}

		// RelatedIDs-based filtered list (multi or single cache miss).
		if len(result.RelatedIDs) > 0 {
			if entry, ok := m.resourceCache[msg.TargetType]; ok {
				idSet := make(map[string]bool, len(result.RelatedIDs))
				for _, id := range result.RelatedIDs {
					idSet[id] = true
				}
				var filtered []resource.Resource
				for _, r := range entry.resources {
					if idSet[r.ID] {
						filtered = append(filtered, r)
					}
				}
				// If some IDs are missing and cache may have more pages, fetch the rest.
				// Pre-populate the list with already-cached filtered rows so they remain
				// visible when subsequent pages arrive via Append:true ResourcesLoadedMsg.
				if len(filtered) < len(result.RelatedIDs) && entry.pagination != nil && entry.pagination.IsTruncated {
					rl := views.NewResourceListFromCache(
						*rt, m.viewConfig, m.keys,
						filtered, entry.pagination,
						"",
						entry.sortColIdx, entry.sortAsc,
						0, 0,
						false,
					)
					rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
					rl.SetRelatedIDFilter(result.RelatedIDs)
					if msg.Checker != nil {
						rl.SetReapplyChecker(msg.Checker, msg.SourceResource)
					}
					rl.SetEscPops(true)
					rl.SetSize(m.innerSize())
					m.pushView(&rl)
					fetchCmd := m.fetchMoreResources(messages.LoadMoreMsg{
						ResourceType:      msg.TargetType,
						ContinuationToken: entry.pagination.NextToken,
					})
					return m, fetchCmd
				}
				// All RelatedIDs matched — no more pages can contribute to this
				// filter, so strip IsTruncated on the pagination we hand to the
				// view. Otherwise the list inherits the upstream cache's
				// truncation flag and renders a misleading "m: load more"
				// footer for a fully-resolved exact-ID filter.
				paginationForView := entry.pagination
				if paginationForView != nil && paginationForView.IsTruncated {
					clone := *paginationForView
					clone.IsTruncated = false
					clone.NextToken = ""
					paginationForView = &clone
				}
				rl := views.NewResourceListFromCache(
					*rt, m.viewConfig, m.keys,
					filtered, paginationForView,
					"", // no text filter needed, already filtered by ID
					entry.sortColIdx, entry.sortAsc,
					0, 0,
					false,
				)
				rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
				rl.SetRelatedIDFilter(result.RelatedIDs)
				if msg.Checker != nil {
					rl.SetReapplyChecker(msg.Checker, msg.SourceResource)
				}
				rl.SetEscPops(true)
				rl.SetSize(m.innerSize())
				m.pushView(&rl)
				return m, nil
			}
			// Cache miss: fetch and preserve exact-ID filtering.
			var opts relatedListOpts
			if len(result.RelatedIDs) == 1 {
				opts = relatedListOpts{
					pendingFilter:        result.RelatedIDs[0],
					relatedIDs:           result.RelatedIDs,
					autoOpenSingleDetail: true,
					reapplyChecker:       msg.Checker,
				}
			} else {
				opts = relatedListOpts{relatedIDs: result.RelatedIDs, reapplyChecker: msg.Checker}
			}
			initCmd := m.newRelatedList(*rt, msg.SourceResource, opts)
			return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
		}

	case KindDetail:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
					IsError: true,
				}
			}
		}

		// Find resource in cache.
		targetID := result.TargetID
		if targetID == "" && len(result.RelatedIDs) == 1 {
			targetID = result.RelatedIDs[0]
		}
		if entry, ok := m.resourceCache[msg.TargetType]; ok {
			for _, r := range entry.resources {
				if r.ID != targetID {
					continue
				}
				// Mirror manual Enter on this row: if the target type
				// registers a child under Key="enter" and its DrillCondition
				// (if any) admits the row, jump straight into the child
				// view. Otherwise push the generic detail. This keeps
				// single-result auto-drill consistent with what Enter does
				// in the target list — a pivot that narrows to one row
				// must not strand the operator on bucket metadata when
				// Enter would have opened bucket contents.
				if enterChild := enterChildForResource(rt, r); enterChild != nil {
					ctx := buildChildContextForResource(*enterChild, r)
					displayName := ctx[enterChild.DisplayNameKey]
					childType := enterChild.ChildType
					return m, func() tea.Msg {
						return messages.EnterChildViewMsg{
							ChildType:     childType,
							ParentContext: ctx,
							DisplayName:   displayName,
						}
					}
				}
				detail := views.NewDetail(r, msg.TargetType, m.viewConfig, m.keys)
				detail.SetSize(m.innerSize())
				m.pushView(&detail)
				if detail.NeedsRelatedCheck() {
					ck := relatedCacheKey(msg.TargetType, r.ID)
					if cached, ok := m.relatedCache.get(ck); ok && len(cached) > 0 {
						detail.ApplyRelatedResults(relatedCacheReplay(msg.TargetType, cached))
						return m, nil
					}
					srcRes := r
					return m, func() tea.Msg {
						return messages.RelatedCheckStartedMsg{
							ResourceType:   msg.TargetType,
							SourceResource: srcRes,
						}
					}
				}
				return m, nil
			}
		}

	case KindResourceList:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
					IsError: true,
				}
			}
		}
		// Approximate-zero (0+) path: zero known IDs but the reverse-scan
		// cache was truncated. Navigate with the checker so each loaded page
		// re-applies the predicate and matches accumulate.
		initCmd := m.newRelatedList(*rt, msg.SourceResource, relatedListOpts{
			reapplyChecker: msg.Checker,
		})
		return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
	}

	return m, nil
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

// snapshotCache returns a flat map[string][]resource.Resource snapshot of the
// current resource cache, suitable for passing to pure resolver functions.
func (m *Model) snapshotCache() map[string][]resource.Resource {
	snap := make(map[string][]resource.Resource, len(m.resourceCache))
	for shortName, entry := range m.resourceCache {
		snap[shortName] = entry.resources
	}
	return snap
}

// handleRelatedNavigateChild handles navigation to a child resource type from
// the related panel. It dispatches an EnterChildViewMsg so that the existing
// child-view machinery handles the push and fetch.
func (m Model) handleRelatedNavigateChild(msg messages.RelatedNavigateMsg) (tea.Model, tea.Cmd) {
	childDef := resource.GetChildType(msg.TargetType)
	if childDef == nil {
		return m, func() tea.Msg {
			return messages.FlashMsg{
				Text:    fmt.Sprintf("unknown child type: %s", msg.TargetType),
				IsError: true,
			}
		}
	}

	// Extract parent context from related IDs using the child type's extractor.
	var parentCtx map[string]string
	if childDef.RelatedContextFromIDs != nil {
		parentCtx = childDef.RelatedContextFromIDs(msg.RelatedIDs)
	}
	if parentCtx == nil {
		parentCtx = map[string]string{}
	}

	displayName := msg.TargetType
	if childDef.Name != "" {
		displayName = childDef.Name
	}

	return m, func() tea.Msg {
		return messages.EnterChildViewMsg{
			ChildType:     msg.TargetType,
			ParentContext: parentCtx,
			DisplayName:   displayName,
		}
	}
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
// resource lists, keyed by resource short name. Used by related checkers.
func (m *Model) buildResourceCacheSnapshot() resource.ResourceCache {
	snap := make(resource.ResourceCache, len(m.resourceCache))
	for shortName, entry := range m.resourceCache {
		snap[shortName] = resource.ResourceCacheEntry{
			Resources:   entry.resources,
			IsTruncated: entry.pagination != nil && entry.pagination.IsTruncated,
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
