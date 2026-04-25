package tui

// app_probes.go — main-menu availability + Wave-2 enrichment pipeline.
//
// Functions in this file implement the menu-discovery cycle:
//   loadAvailabilityCache  — read disk cache (TTL'd) before any AWS calls.
//   probeResourceAvailability — per-type fetcher probe to populate menu badges.
//   demoPrefetchCounts     — synchronous probe variant used in demo / no-cache mode.
//   saveAvailabilityCache  — persist menu state back to disk.
//   buildEnrichQueue       — order Wave-2 enricher dispatch by registered priority.
//   probeEnrichment        — per-type Wave-2 enricher probe.
//   refreshResourceListWithEnrichmentRerun — Ctrl+R hook so a re-fetched list
//                                            re-arms its enricher.
//
// Split from app_fetchers.go (which now holds list/detail fetchers only) to
// keep both files under the 500-line file-size budget.

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// loadAvailabilityCache returns a tea.Cmd that reads the availability cache from disk.
func (m *Model) loadAvailabilityCache() tea.Cmd {
	profile := m.profile
	region := m.region
	return func() tea.Msg {
		cf, err := cache.Load(profile, region)
		if err != nil || cf == nil {
			// No cache or error — return empty entries, will trigger full re-check
			return messages.AvailabilityCacheLoadedMsg{
				Entries: make(map[string]int),
				Expired: true,
			}
		}
		entries := make(map[string]int, len(cf.Resources))
		truncated := make(map[string]bool)
		issueCounts := make(map[string]int)
		issueTruncated := make(map[string]bool)
		issueKnown := make(map[string]bool)
		for name, entry := range cf.Resources {
			if entry.Error == "" {
				entries[name] = entry.Count
				if entry.Truncated {
					truncated[name] = true
				}
				if entry.IssuesKnown {
					issueCounts[name] = entry.Issues
					issueKnown[name] = true
					if entry.IssuesTruncated {
						issueTruncated[name] = true
					}
				}
			}
		}
		return messages.AvailabilityCacheLoadedMsg{
			Entries:        entries,
			Truncated:      truncated,
			Expired:        cf.IsExpired(cache.DefaultTTL),
			IssueCounts:    issueCounts,
			IssueTruncated: issueTruncated,
			IssueKnown:     issueKnown,
		}
	}
}

// probeResourceAvailability returns a tea.Cmd that checks if a resource type
// has any resources by calling its registered fetcher with a timeout.
// Paginated fetchers are tried first so that truncation can be detected and
// reported as "(N+)" in the main menu.
func (m *Model) probeResourceAvailability(shortName string, gen int) tea.Cmd {
	clients := m.clients
	appCtx := m.appCtx
	return func() tea.Msg {
		if clients == nil {
			return messages.AvailabilityCheckedMsg{
				ResourceType: shortName,
				Err:          fmt.Errorf("AWS clients not initialized"),
				Gen:          gen,
			}
		}
		ctx, cancel := context.WithTimeout(appCtx, 10*time.Second)
		defer cancel()

		pf := resource.GetPaginatedFetcher(shortName)
		if pf == nil {
			return messages.AvailabilityCheckedMsg{
				ResourceType: shortName,
				Err:          fmt.Errorf("no fetcher for %s", shortName),
				Gen:          gen,
			}
		}

		result, err := awsclient.RetryOnThrottle(ctx, awsclient.DefaultRetryConfig(), func() (resource.FetchResult, error) {
			return pf(ctx, clients, "")
		})
		// Hoist truncated+issues calculation so both the error and success
		// branches can use them (partial-success: err non-nil but Resources present).
		truncated := result.Pagination != nil && result.Pagination.IsTruncated
		// Count issue-status resources (red/yellow only, not green/dim).
		issues := 0
		td := resource.FindResourceType(shortName)
		for _, r := range result.Resources {
			if td != nil && !td.ExcludeFromIssueBadge && td.ResolveColor(r).IsIssue() {
				issues++
			}
		}
		if err != nil {
			if len(result.Resources) == 0 {
				// Full failure: no resources recovered — treat as unknown.
				return messages.AvailabilityCheckedMsg{
					ResourceType: shortName,
					Err:          err,
					Gen:          gen,
				}
			}
			// Partial success: some resources returned alongside an error.
			// Surface both so the menu shows what was found and the flash log
			// records the failure (never-silent-skip contract).
			return messages.AvailabilityCheckedMsg{
				ResourceType: shortName,
				HasResources: true,
				Count:        len(result.Resources),
				Truncated:    truncated,
				Gen:          gen,
				Issues:       issues,
				Resources:    result.Resources,
				Err:          err,
			}
		}
		return messages.AvailabilityCheckedMsg{
			ResourceType: shortName,
			HasResources: len(result.Resources) > 0,
			Count:        len(result.Resources),
			Truncated:    truncated,
			Gen:          gen,
			Issues:       issues,
			Resources:    result.Resources,
		}
	}
}

// saveAvailabilityCache returns a tea.Cmd that persists the current availability state to disk.
// No-op when caching is disabled (e.g. demo mode or --no-cache).
func (m *Model) saveAvailabilityCache() tea.Cmd {
	if m.noCache {
		return nil
	}
	profile := m.profile
	region := m.region

	// Collect availability, truncation, and issue counts from main menu.
	var entries map[string]int
	var truncatedMap map[string]bool
	var issueCounts map[string]int
	var issueTruncated map[string]bool
	var issueKnown map[string]bool
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		entries = menu.GetAvailability()
		truncatedMap = menu.GetTruncated()
		issueCounts = menu.GetIssueCounts()
		issueTruncated = menu.GetIssueTruncated()
		issueKnown = menu.GetIssueKnown()
	}
	if entries == nil {
		return nil
	}

	return func() tea.Msg {
		cf := &cache.File{
			Profile:   profile,
			Region:    region,
			CheckedAt: time.Now(),
			Resources: make(map[string]cache.Entry, len(entries)),
		}
		for name, count := range entries {
			trunc := false
			if truncatedMap != nil {
				trunc = truncatedMap[name]
			}
			e := cache.Entry{HasResources: count > 0, Count: count, Truncated: trunc}
			if issueKnown[name] {
				e.Issues = issueCounts[name]
				e.IssuesKnown = true
				e.IssuesTruncated = issueTruncated[name]
			}
			cf.Resources[name] = e
		}
		// Best-effort save — don't flash errors for cache write failures
		_ = cache.Save(cf)
		return nil
	}
}

// demoPrefetchCounts returns a tea.Cmd that synchronously calls all registered
// paginated fetchers and returns AvailabilityPrefetchedMsg with all counts
// pre-filled. Used when pre-supplied clients are present and no-cache is active,
// so the main menu shows counts immediately without the async probe pipeline.
func (m *Model) demoPrefetchCounts() tea.Cmd {
	clients := m.clients
	appCtx := m.appCtx
	gen := m.availabilityGen
	return func() tea.Msg {
		allNames := resource.AllShortNames()
		entries := make(map[string]int, len(allNames))
		truncated := make(map[string]bool)
		issueCounts := make(map[string]int, len(allNames))
		issueTruncated := make(map[string]bool)
		retainedResources := make(map[string][]resource.Resource, len(allNames))
		var failures []string
		attempted := 0
		for _, shortName := range allNames {
			// Stop early if the app context is done (shutdown or profile/region switch).
			if appCtx.Err() != nil {
				break
			}
			pf := resource.GetPaginatedFetcher(shortName)
			if pf == nil {
				continue
			}
			attempted++
			perFetchCtx, perFetchCancel := context.WithTimeout(appCtx, 5*time.Second)
			result, err := pf(perFetchCtx, clients, "")
			perFetchCancel()
			// Partial-success: a per-item composite error MAY accompany a
			// non-empty result. Hard failure (no resources) → skip and record.
			// Soft failure (some resources) → record the failure AND count
			// the resources so the main menu badge isn't blanked by a single
			// per-item timeout.
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", shortName, err))
				if len(result.Resources) == 0 {
					continue
				}
			}
			entries[shortName] = len(result.Resources)
			isTrunc := result.Pagination != nil && result.Pagination.IsTruncated
			if isTrunc {
				truncated[shortName] = true
				issueTruncated[shortName] = true
			}
			// Count issue-status resources (red/yellow only).
			issues := 0
			td := resource.FindResourceType(shortName)
			for _, r := range result.Resources {
				if td != nil && !td.ExcludeFromIssueBadge && td.ResolveColor(r).IsIssue() {
					issues++
				}
			}
			issueCounts[shortName] = issues
			// Retain first-page resources for Wave 2 enricher consumption.
			retainedResources[shortName] = result.Resources
		}
		return messages.AvailabilityPrefetchedMsg{
			Entries:        entries,
			Truncated:      truncated,
			IssueCounts:    issueCounts,
			IssueTruncated: issueTruncated,
			Resources:      retainedResources,
			Gen:            gen,
			PrefetchErr:    awsclient.AggregateFailures("availability-prefetch", failures, attempted),
		}
	}
}

// refreshResourceListWithEnrichmentRerun wraps the ordinary refresh fetch for
// a top-level list so that the ResourcesLoadedMsg it produces carries an
// enrichment-rerun token. The token is captured at Ctrl+R dispatch time and
// stamped into the message; the ResourcesLoadedMsg handler in app.go checks
// TypeGen in its tail branch to decide whether to seed probeResources and
// dispatch probeEnrichment. APIErrorMsg and any other message pass through
// unchanged.
func (m *Model) refreshResourceListWithEnrichmentRerun(
	rl views.ResourceListModel, tok int,
) tea.Cmd {
	inner := m.refreshResourceList(rl)
	return func() tea.Msg {
		msg := inner()
		if loaded, ok := msg.(messages.ResourcesLoadedMsg); ok {
			loaded.TypeGen = tok
			return loaded
		}
		return msg
	}
}

// buildEnrichQueue returns resource types that have registered Wave 2 issue
// enrichers AND have retained probe resources, sorted by declarative priority
// from IssueEnricherRegistry[name].Priority (lower values first), then
// alphabetically within the same priority tier. Priority is metadata on the
// registry entry: 10 = batchable (cheap, run first), 100 = default
// per-resource issue enricher.
func (m *Model) buildEnrichQueue() []string {
	type pair struct {
		name     string
		priority int
	}

	var ps []pair
	for name, e := range awsclient.IssueEnricherRegistry {
		if _, ok := m.probeResources[name]; !ok {
			continue
		}
		ps = append(ps, pair{name: name, priority: e.Priority})
	}
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].priority != ps[j].priority {
			return ps[i].priority < ps[j].priority
		}
		return ps[i].name < ps[j].name // stable: alphabetical within priority
	})
	queue := make([]string, len(ps))
	for i, p := range ps {
		queue[i] = p.name
	}
	return queue
}

// probeEnrichment returns a tea.Cmd that runs the registered enricher for a
// resource type and returns an EnrichmentCheckedMsg.
// typeGen is the per-type generation counter captured at dispatch time; it is
// embedded in the message so handleEnrichmentChecked can drop stale results.
func (m *Model) probeEnrichment(shortName string, gen int) tea.Cmd {
	clients := m.clients
	appCtx := m.appCtx
	resources := m.probeResources[shortName]
	enricherFn := awsclient.IssueEnricherRegistry[shortName].Fn
	typeGen := m.enrichmentTypeGen[shortName]
	if enricherFn == nil {
		return nil
	}
	// Build a resource.ResourceCache snapshot from the TUI cache so cross-ref
	// enrichers (e.g. rds-snap) can read sibling resource lists without extra API calls.
	cacheSnap := make(resource.ResourceCache, len(m.resourceCache))
	for k, entry := range m.resourceCache {
		if entry != nil {
			isTruncated := entry.pagination != nil && entry.pagination.IsTruncated
			cacheSnap[k] = resource.ResourceCacheEntry{
				Resources:   entry.resources,
				IsTruncated: isTruncated,
			}
		}
	}
	return func() tea.Msg {
		if clients == nil {
			return messages.EnrichmentCheckedMsg{
				ResourceType: shortName,
				Err:          fmt.Errorf("AWS clients not initialized"),
				Gen:          gen,
				TypeGen:      typeGen,
			}
		}
		ctx, cancel := context.WithTimeout(appCtx, 10*time.Second)
		defer cancel()

		result, err := awsclient.RetryOnThrottle(ctx, awsclient.DefaultRetryConfig(), func() (awsclient.IssueEnricherResult, error) {
			return enricherFn(ctx, clients, resources, cacheSnap)
		})
		// Always populate fields from result regardless of err. RetryOnThrottle
		// preserves partial result on non-retryable errors (partial-success
		// contract: never-silent-skip). The message handler decides what to
		// surface from the partial data.
		return messages.EnrichmentCheckedMsg{
			ResourceType: shortName,
			Issues:       result.IssueCount,
			Truncated:    result.Truncated,
			Findings:     result.Findings,
			FieldUpdates: result.FieldUpdates,
			IssueAppends: result.IssueAppends,
			TruncatedIDs: result.TruncatedIDs,
			Gen:          gen,
			TypeGen:      typeGen,
			Err:          err,
		}
	}
}
