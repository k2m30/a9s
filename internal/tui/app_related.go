// app_related.go contains related-resource navigation and check dispatch handlers.
package tui

import (
	"context"
	"fmt"
	"maps"
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
		cmds = append(cmds, func() tea.Msg {
			sem <- struct{}{}
			defer func() { <-sem }()

			if m.demoMode {
				demoFn := resource.GetRelatedDemo(msg.ResourceType)
				if demoFn != nil {
					for _, r := range demoFn(msg.SourceResource) {
						if r.TargetType == def.TargetType {
							return messages.RelatedCheckResultMsg{
								ResourceType:     msg.ResourceType,
								SourceResourceID: msg.SourceResource.ID,
								Result:           r,
								Generation:       gen,
							}
						}
					}
				}
				return messages.RelatedCheckResultMsg{
					ResourceType:     msg.ResourceType,
					SourceResourceID: msg.SourceResource.ID,
					Result:           resource.RelatedCheckResult{TargetType: def.TargetType, Count: -1},
					Generation:       gen,
				}
			}

			if def.Checker == nil {
				return messages.RelatedCheckResultMsg{
					ResourceType:     msg.ResourceType,
					SourceResourceID: msg.SourceResource.ID,
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
			return messages.RelatedCheckResultMsg{
				ResourceType:     msg.ResourceType,
				SourceResourceID: msg.SourceResource.ID,
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
func (m Model) handleRelatedNavigate(msg messages.RelatedNavigateMsg) (tea.Model, tea.Cmd) {
	rt := resource.FindResourceType(msg.TargetType)
	if rt == nil {
		return m, func() tea.Msg {
			return messages.FlashMsg{
				Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
				IsError: true,
			}
		}
	}

	// FetchFilter path: use server-side filtered fetcher — skip frozen relatedIDSet entirely.
	if len(msg.FetchFilter) > 0 {
		rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
		rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
		rl.SetFetchFilter(msg.FetchFilter)
		rl.SetEscPops(true)
		rl.SetSize(m.innerSize())
		rl, initCmd := rl.Init()
		m.pushView(&rl)
		return m, tea.Batch(initCmd, m.fetchResourcesFiltered(msg.TargetType, msg.FetchFilter))
	}

	// Bug 1 fix: TargetID set → find resource in cache and push detail directly.
	if msg.TargetID != "" {
		if entry, ok := m.resourceCache[msg.TargetType]; ok {
			for _, r := range entry.resources {
				if r.ID == msg.TargetID {
					detail := views.NewDetail(r, msg.TargetType, m.viewConfig, m.keys)
					detail.SetSize(m.innerSize())
					m.pushView(&detail)
					if detail.NeedsRelatedCheck() {
						ck := relatedCacheKey(msg.TargetType, r.ID)
						if cached, ok := m.relatedCache.get(ck); ok && len(cached) > 0 {
							detail.ApplyRelatedResults(cached)
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
		}
		// Exact AMI navigation should fetch by image ID in live mode instead of
		// falling back to the owned-AMI list, which misses public and third-party images.
		// In demo/tests, preserve the list-load flow so synthetic ResourcesLoadedMsg
		// can still auto-open the exact target without requiring AWS clients.
		if msg.TargetType == "ami" && m.clients != nil && !m.demoMode {
			cmd := m.fetchAMIDetail(msg.TargetID)
			return m, cmd
		}
		// Resource not in cache — fetch target list and preserve exact-ID filtering.
		m.flash = flashState{
			text:    fmt.Sprintf("Resource %s not in cache; loading %s list", msg.TargetID, msg.TargetType),
			isError: false,
			active:  true,
		}
		initCmd := m.newRelatedList(*rt, msg.SourceResource, relatedListOpts{
			pendingFilter:        msg.TargetID,
			relatedIDs:           []string{msg.TargetID},
			autoOpenSingleDetail: true,
		})
		return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
	}

	// Bug 2 fix: single RelatedID → push detail directly (same as TargetID).
	if len(msg.RelatedIDs) == 1 {
		targetID := msg.RelatedIDs[0]
		if entry, ok := m.resourceCache[msg.TargetType]; ok {
			for _, r := range entry.resources {
				if r.ID == targetID {
					detail := views.NewDetail(r, msg.TargetType, m.viewConfig, m.keys)
					detail.SetSize(m.innerSize())
					m.pushView(&detail)
					if detail.NeedsRelatedCheck() {
						ck := relatedCacheKey(msg.TargetType, r.ID)
						if cached, ok := m.relatedCache.get(ck); ok && len(cached) > 0 {
							detail.ApplyRelatedResults(cached)
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
		}
		// Not in cache — fall through to list with pending filter
		initCmd := m.newRelatedList(*rt, msg.SourceResource, relatedListOpts{
			pendingFilter:        targetID,
			relatedIDs:           []string{targetID},
			autoOpenSingleDetail: true,
		})
		return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
	}

	// Bug 3 fix: multiple RelatedIDs → filter cache to only matching resources.
	if len(msg.RelatedIDs) > 1 {
		if entry, ok := m.resourceCache[msg.TargetType]; ok {
			idSet := make(map[string]bool, len(msg.RelatedIDs))
			for _, id := range msg.RelatedIDs {
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
			if len(filtered) < len(msg.RelatedIDs) && entry.pagination != nil && entry.pagination.IsTruncated {
				rl := views.NewResourceListFromCache(
					*rt, m.viewConfig, m.keys,
					filtered, entry.pagination,
					"",
					entry.sortField, entry.sortAsc,
					0, 0,
					false,
				)
				rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
				rl.SetRelatedIDFilter(msg.RelatedIDs)
				rl.SetEscPops(true)
				rl.SetSize(m.innerSize())
				m.pushView(&rl)
				// Use fetchMoreResources with the stored token to resume from
				// the correct page, not fetchResources which resets to page 1.
				fetchCmd := m.fetchMoreResources(messages.LoadMoreMsg{
					ResourceType:      msg.TargetType,
					ContinuationToken: entry.pagination.NextToken,
				})
				return m, fetchCmd
			}
			rl := views.NewResourceListFromCache(
				*rt, m.viewConfig, m.keys,
				filtered, entry.pagination,
				"", // no text filter needed, already filtered by ID
				entry.sortField, entry.sortAsc,
				0, 0,
				false,
			)
			rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
			rl.SetRelatedIDFilter(msg.RelatedIDs)
			rl.SetEscPops(true)
			rl.SetSize(m.innerSize())
			m.pushView(&rl)
			return m, nil
		}
		// Cache miss: fetch and preserve exact-ID filtering.
		initCmd := m.newRelatedList(*rt, msg.SourceResource, relatedListOpts{
			relatedIDs: msg.RelatedIDs,
		})
		return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
	}

	// Fallback: no IDs specified — push unfiltered list.
	initCmd := m.newRelatedList(*rt, msg.SourceResource, relatedListOpts{})
	return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
}

// relatedListOpts configures a related-navigation resource list.
type relatedListOpts struct {
	pendingFilter        string
	relatedIDs           []string
	autoOpenSingleDetail bool
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

