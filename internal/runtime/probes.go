// probes.go — platform-agnostic availability and Wave-2 enrichment probes.
//
// These are (c *Core) methods reading session state via c.session. The
// Bubble Tea adapter in internal/tui/probe_adapter.go wraps them in tea.Cmd
// closures for the TUI runtime.
//
// No Bubble Tea, Lipgloss, or Bubbles imports are permitted in this file.
package runtime

import (
	"context"
	"fmt"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ProbeAvailabilityResult carries the outcome of a single Wave-1 resource
// availability probe. Adapters convert this into a platform-specific message
// (e.g. messages.AvailabilityChecked for the Bubble Tea adapter).
type ProbeAvailabilityResult struct {
	ResourceType string
	HasResources bool
	Count        int
	Truncated    bool
	Issues       int
	Resources    []resource.Resource
	Err          error
}

// ProbeEnrichmentResult carries the outcome of a single Wave-2 issue
// enrichment probe. Adapters convert this into a platform-specific message
// (e.g. messages.EnrichmentChecked for the Bubble Tea adapter).
//
// Findings and AttentionDetails are both keyed by Resource.ID. The fold layer
// (runtime.Core.applyEnrichment) flips AttentionDetails to FindingCode against
// the matching r.Findings entry when writing onto cached rows.
type ProbeEnrichmentResult struct {
	ResourceType     string
	Issues           int
	Truncated        bool
	Findings         map[string]domain.Finding
	AttentionDetails map[string]domain.AttentionDetail
	FieldUpdates     map[string]map[string]string
	TruncatedIDs     map[string]bool
	Err              error
}

// DemoPrefetchResult carries the combined outcome of a synchronous demo
// prefetch of all registered resource types. Adapters convert this into
// a platform-specific message (e.g. messages.AvailabilityPrefetched for
// the Bubble Tea adapter).
type DemoPrefetchResult struct {
	Entries        map[string]int
	Truncated      map[string]bool
	IssueCounts    map[string]int
	IssueTruncated map[string]bool
	Resources      map[string][]resource.Resource
	Pagination     map[string]*resource.PaginationMeta
	PrefetchErr    error
}

// LoadAvailabilityCache loads (or reuses the already-loaded) per-type disk
// cache for profile/region via EnsureCacheStore and returns it converted to
// the counts-only *cache.File-equivalent shape callers expect: a map of
// per-type TypeFile snapshots. Returns the Store directly — callers read it
// via (*cache.Store).Type/Types. NoCache=true (or a profile/region mismatch
// with the currently-loaded pair) is handled by EnsureCacheStore itself.
//
// profile/region are accepted for signature stability with the pre-existing
// call sites; the Store loaded reflects c.session.Profile/Region (the caller
// is expected to have already set them via SetProfile/SetRegion or Rotate).
func (c *Core) LoadAvailabilityCache(profile, region string) *cache.Store {
	_ = profile
	_ = region
	return c.EnsureCacheStore()
}

// SaveAvailabilityCache persists the supplied availability state to disk, one
// type file per resource type (C7: per-type files, no merge logic). Returns
// nil immediately when entries is nil or caching is disabled (NoCache). The
// per-type Store is obtained via EnsureCacheStore so a save can never precede
// that pair's own load (C7 hard invariant).
//
// Row/Findings persistence (C6, all loaded pages) is intentionally NOT done
// here — this method only carries the counts-only availability-probe shape
// callers historically populated it with. Full-row persistence for a type's
// canonical top-level list happens via Core.SaveResourceListCache, called
// from the list-fetch-completion seam (applyResourcesLoaded).
func (c *Core) SaveAvailabilityCache(
	profile, region string,
	entries map[string]int,
	truncated map[string]bool,
	issueCounts map[string]int,
	issueTruncated map[string]bool,
	issueKnown map[string]bool,
) error {
	_ = profile
	_ = region
	if entries == nil || c.session.NoCache {
		return nil
	}
	store := c.EnsureCacheStore()
	if store == nil {
		return nil
	}
	var firstErr error
	for name, count := range entries {
		trunc := false
		if truncated != nil {
			trunc = truncated[name]
		}
		existing, _ := store.Type(name)
		tf := cache.TypeFile{
			HasResources: count > 0,
			Count:        count,
			// C5: a truncated first-page probe never downgrades a stored
			// exact total — only replace Exact when this observation is
			// itself untruncated (a genuine exact observation).
			Exact: existing.Exact || !trunc,
			Rows:  existing.Rows,
		}
		if existing.Exact && trunc && existing.Count > count {
			// Preserve the previously-observed exact count/rows rather than
			// letting a smaller truncated lower-bound regress it.
			tf.Count = existing.Count
			tf.HasResources = existing.Count > 0
		}
		if issueKnown[name] {
			tf.Issues = issueCounts[name]
			tf.IssuesKnown = true
			tf.IssuesTruncated = issueTruncated[name]
		} else {
			tf.Issues = existing.Issues
			tf.IssuesKnown = existing.IssuesKnown
			tf.IssuesTruncated = existing.IssuesTruncated
		}
		store.Put(name, tf)
		if err := store.SaveType(name); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// SaveResourceListCache persists rows for one resource type's canonical
// top-level, unfiltered list (C6): every loaded page's rows (ID/Name/Fields/
// Findings — colors/glyphs/status are derived at render time and never
// persisted), the current count, and the exact flag. Callers are responsible
// for the C6 scope gate (only calling this for a top-level unfiltered list,
// never a child/related/filtered view) and for skipping the call entirely
// when NoCache is set (mirrored here defensively).
func (c *Core) SaveResourceListCache(shortName string, rows []cache.Row, count int, exact bool, issues int, issuesKnown, issuesTruncated bool) error {
	if c.session.NoCache {
		return nil
	}
	store := c.EnsureCacheStore()
	if store == nil {
		return nil
	}
	existing, _ := store.Type(shortName)
	tf := cache.TypeFile{
		HasResources: count > 0,
		Count:        count,
		Rows:         rows,
	}
	// C5: exactness only ever advances — a truncated observation never
	// downgrades an already-exact stored total. DEF-4b: when a truncated
	// refetch's smaller row set would otherwise regress an already-exact,
	// fuller stored state, the PRIOR (fuller, exact) row set is kept in its
	// entirety — never a Count taken from `existing` paired with a smaller
	// `rows` slice, which would leave len(Rows) != Count inconsistent. The
	// fuller row set is only ever replaced by a NEW exact observation (the
	// `case exact` branch above, which always takes the fresh rows/count as
	// a matched pair).
	switch {
	case exact:
		tf.Exact = true
	case existing.Exact:
		tf.Exact = true
		if existing.Count > count {
			tf.Count = existing.Count
			tf.HasResources = existing.Count > 0
			tf.Rows = existing.Rows
		}
	}
	if issuesKnown {
		tf.Issues = issues
		tf.IssuesKnown = true
		tf.IssuesTruncated = issuesTruncated
	} else {
		tf.Issues = existing.Issues
		tf.IssuesKnown = existing.IssuesKnown
		tf.IssuesTruncated = existing.IssuesTruncated
	}
	store.Put(shortName, tf)
	return store.SaveType(shortName)
}

// ProbeResourceAvailability calls the registered paginated fetcher for
// shortName with a 10-second timeout, applies Wave-1 issue counting, and
// returns the result. Adapters wrap this in platform-specific async
// machinery (e.g. a tea.Cmd for the Bubble Tea adapter).
//
// Paginated fetchers are tried so that truncation can be detected and
// reported as "(N+)" in the main menu.
func (c *Core) ProbeResourceAvailability(ctx context.Context, clients *awsclient.ServiceClients, shortName string) ProbeAvailabilityResult {
	if clients == nil {
		return ProbeAvailabilityResult{
			ResourceType: shortName,
			Err:          fmt.Errorf("AWS clients not initialized"),
		}
	}
	pf := resource.GetPaginatedFetcher(shortName)
	if pf == nil {
		return ProbeAvailabilityResult{
			ResourceType: shortName,
			Err:          fmt.Errorf("no fetcher for %s", shortName),
		}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := awsclient.RetryOnThrottle(probeCtx, awsclient.DefaultRetryConfig(), func() (resource.FetchResult, error) {
		return pf(probeCtx, clients, "")
	})

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
			return ProbeAvailabilityResult{
				ResourceType: shortName,
				Err:          err,
			}
		}
		// Partial success: some resources returned alongside an error.
		// Surface both so the menu shows what was found and the flash log
		// records the failure (never-silent-skip contract).
		return ProbeAvailabilityResult{
			ResourceType: shortName,
			HasResources: true,
			Count:        len(result.Resources),
			Truncated:    truncated,
			Issues:       issues,
			Resources:    result.Resources,
			Err:          err,
		}
	}
	return ProbeAvailabilityResult{
		ResourceType: shortName,
		HasResources: len(result.Resources) > 0,
		Count:        len(result.Resources),
		Truncated:    truncated,
		Issues:       issues,
		Resources:    result.Resources,
	}
}

// DemoPrefetchCounts synchronously calls all registered paginated fetchers
// and returns a combined result. Used when pre-supplied clients are present
// and no-cache is active so the main menu shows counts immediately without
// the async probe pipeline.
func (c *Core) DemoPrefetchCounts(ctx context.Context, clients *awsclient.ServiceClients) DemoPrefetchResult {
	allNames := resource.AllShortNames()
	entries := make(map[string]int, len(allNames))
	truncated := make(map[string]bool)
	issueCounts := make(map[string]int, len(allNames))
	issueTruncated := make(map[string]bool)
	retainedResources := make(map[string][]resource.Resource, len(allNames))
	pagination := make(map[string]*resource.PaginationMeta, len(allNames))
	var failures []string
	attempted := 0

	for _, shortName := range allNames {
		// Stop early if the app context is done (shutdown or profile/region switch).
		if ctx.Err() != nil {
			break
		}
		pf := resource.GetPaginatedFetcher(shortName)
		if pf == nil {
			continue
		}
		attempted++
		perFetchCtx, perFetchCancel := context.WithTimeout(ctx, 5*time.Second)
		result, err := pf(perFetchCtx, clients, "")
		perFetchCancel()
		// Partial-success: a per-item composite error MAY accompany a non-empty
		// result. Hard failure (no resources) → skip and record. Soft failure
		// (some resources) → record the failure AND count the resources so the
		// main menu badge isn't blanked by a single per-item timeout.
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", shortName, err))
			if len(result.Resources) == 0 {
				continue
			}
		}
		entries[shortName] = len(result.Resources)
		// Preserve full pagination meta so the seeded ResourceCache entry's
		// pagination state is authoritative — a later load-more or navigate
		// must be able to advance past page 1.
		if result.Pagination != nil {
			pagination[shortName] = result.Pagination
		}
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

	return DemoPrefetchResult{
		Entries:        entries,
		Truncated:      truncated,
		IssueCounts:    issueCounts,
		IssueTruncated: issueTruncated,
		Resources:      retainedResources,
		Pagination:     pagination,
		PrefetchErr:    awsclient.AggregateFailures("availability-prefetch", failures, attempted),
	}
}

// buildEnrichQueue returns resource types that have a registered Wave-2 issue
// enricher AND retained probe resources, in dispatch order. Dispatch order
// (priority ascending, then alphabetical) is owned by awsclient.AllWave2 so
// this function only filters by ProbeResources membership.
func (c *Core) BuildEnrichQueue() []string {
	all := awsclient.AllWave2()
	queue := make([]string, 0, len(all))
	for _, e := range all {
		if _, ok := c.session.ProbeResources[e.ShortName]; !ok {
			continue
		}
		queue = append(queue, e.ShortName)
	}
	return queue
}

// ProbeEnrichment runs the registered Wave-2 enricher for shortName and
// returns the enrichment result. The typeGen captured from
// c.session.EnrichmentTypeGen at call time is the caller's responsibility;
// the caller embeds it in the adapter message for stale-result rejection.
//
// Builds a ResourceCache snapshot via buildResourceCacheSnapshot — it merges
// c.session.ProbeResources (first-page rows retained by the availability
// probe) AND c.session.LazyResourceCache AND c.session.ResourceCache.
// On the normal startup path c.session.ResourceCache is empty until the user
// opens a list, so building from ResourceCache alone would leave the first
// enrichment pass blind to siblings.
// Regression pin: TestProbeEnrichment_CacheSnapshotMergesProbeResources.
func (c *Core) ProbeEnrichment(ctx context.Context, clients *awsclient.ServiceClients, shortName string) ProbeEnrichmentResult {
	if clients == nil {
		return ProbeEnrichmentResult{
			ResourceType: shortName,
			Err:          fmt.Errorf("AWS clients not initialized"),
		}
	}
	e, ok := awsclient.Wave2EnricherFor(shortName)
	if !ok {
		return ProbeEnrichmentResult{ResourceType: shortName}
	}
	resources := c.session.ProbeResources[shortName]
	cacheSnap := c.BuildResourceCacheSnapshot()

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := awsclient.RetryOnThrottle(probeCtx, awsclient.DefaultRetryConfig(), func() (awsclient.IssueEnricherResult, error) {
		return e.Fn(probeCtx, clients, resources, cacheSnap)
	})
	// Always populate fields from result regardless of err. RetryOnThrottle
	// preserves partial result on non-retryable errors (partial-success
	// contract: never-silent-skip).
	return ProbeEnrichmentResult{
		ResourceType:     shortName,
		Issues:           result.IssueCount,
		Truncated:        result.Truncated,
		Findings:         result.Findings,
		AttentionDetails: result.AttentionDetails,
		FieldUpdates:     result.FieldUpdates,
		TruncatedIDs:     result.TruncatedIDs,
		Err:              err,
	}
}

// BuildResourceCacheSnapshot returns a read-only snapshot of currently-loaded
// resource lists, keyed by resource short name. Merges ResourceCache,
// LazyResourceCache, and ProbeResources so enrichers see the full set
// (including out-of-scope entries pulled via FetchByIDs). On ID collision,
// ResourceCache wins (it is the scope-filtered authoritative source).
//
// LazyResourceCache entries are marked IsTruncated=true because they are
// sparse (FetchByIDs, not a full first page). ProbeResources pages are
// first-page-only — also marked IsTruncated=true so the orphan rule in
// cross-ref enrichers treats parent-not-found as "unknown, skip" rather
// than "definitively deleted" per spec §3.1.
func (c *Core) BuildResourceCacheSnapshot() resource.ResourceCache {
	s := c.session
	snap := make(resource.ResourceCache, len(s.ResourceCache)+len(s.LazyResourceCache)+len(s.ProbeResources))

	// Seed from LazyResourceCache first; ResourceCache entries will overwrite.
	for shortName, rows := range s.LazyResourceCache {
		snap[shortName] = resource.ResourceCacheEntry{
			Resources:   rows,
			IsTruncated: true,
		}
	}
	// Merge ProbeResources — first-page rows retained by the Wave-1 probe pass.
	for shortName, rows := range s.ProbeResources {
		probeTrunc := s.ProbeTruncated[shortName]
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
	// ResourceCache is authoritative — overwrite anything from lazy/probe.
	for shortName, entry := range s.ResourceCache {
		cacheIsTruncated := (entry.Pagination != nil && entry.Pagination.IsTruncated) || s.ProbeTruncated[shortName]
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
