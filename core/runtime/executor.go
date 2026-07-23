// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// executor.go — renderer-neutral task executor for runtime.Core.
//
// PR-B0 Pass A: adds Core.ExecuteTask as the single entry point for a
// non-Bubble-Tea host (web, CLI, test harness) to run a TaskRequest
// synchronously and receive the result as a messages.Event.
//
// Adapter-only tasks — kinds that are inherently renderer concerns —
// return (nil, ErrAdapterOnlyTask):
//
//   - flash-tick        : delay is a renderer concern (tea.Tick / sleep).
//   - emit-navigate     : messages.Navigate is a renderer-routing directive.
//   - emit-api-error    : re-dispatches into the render loop.
//   - read-theme-file   : produces messages.ThemeFileRead for the renderer.
//   - save-theme-config : persists a theme choice with no data event result.
//   - fetch-profiles    : result is a TUI-private profilesLoadedMsg type.
//
// All other TaskKinds call the existing Core methods and wrap their output
// in the appropriate messages.Event.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ErrAdapterOnlyTask is returned by ExecuteTask for TaskKind values that are
// inherently renderer concerns and cannot be executed in a renderer-neutral
// context. See the executor.go package-doc for the complete list.
var ErrAdapterOnlyTask = errors.New("task kind is adapter-only and cannot be executed by Core.ExecuteTask")

// DispatchSnapshot captures the session state ExecuteTask reads, taken at
// DISPATCH time (synchronously, before the async command goroutine runs).
type DispatchSnapshot struct {
	Clients           *awsclient.ServiceClients
	AvailabilityGen   domain.Gen
	EnrichmentGen     domain.Gen
	ConnectGen        domain.Gen
	EnrichmentTypeGen map[string]domain.Gen
	Profile           string
	Region            string
	NoCache           bool
}

// CaptureDispatch snapshots the session generations and clients. Call it
// synchronously at task-dispatch time (NOT inside a goroutine).
func (c *Core) CaptureDispatch() DispatchSnapshot {
	profile, region := c.session.CurrentPair()
	return DispatchSnapshot{
		Clients:           c.session.Clients,
		AvailabilityGen:   c.session.AvailabilityGen,
		EnrichmentGen:     c.session.EnrichmentGen,
		ConnectGen:        c.session.ConnectGen,
		EnrichmentTypeGen: c.session.EnrichmentTypeGenSnapshot(),
		Profile:           profile,
		Region:            region,
		NoCache:           c.session.NoCache,
	}
}

// ExecuteTask runs a task using a snapshot captured now. Synchronous callers
// (DrainSync, non-TUI hosts) have no dispatch/execute gap. Async callers (the
// TUI's executeTaskCmd) MUST capture via CaptureDispatch at dispatch time and
// call ExecuteTaskAt instead.
func (c *Core) ExecuteTask(ctx context.Context, req TaskRequest) (messages.Event, error) {
	return c.ExecuteTaskAt(ctx, req, c.CaptureDispatch())
}

// ExecuteTaskAt executes req synchronously using the existing Core methods and
// returns the result as a messages.Event. snap must be captured at dispatch
// time via CaptureDispatch so that a concurrent session.Rotate cannot corrupt
// the generation/client values read during execution.
//
// ctx is forwarded to every blocking AWS call; callers should supply a
// context with an appropriate timeout.
//
// Adapter-only kinds return (nil, ErrAdapterOnlyTask). A nil Event with a
// nil error means the task completed with no result to dispatch (e.g.
// probe-enrich skipped in demo mode, save-cache with nothing to persist).
func (c *Core) ExecuteTaskAt(ctx context.Context, req TaskRequest, snap DispatchSnapshot) (messages.Event, error) {
	switch req.Key.Kind {

	// --- availability probe ---
	case TaskKindProbeAvailability:
		shortName := req.Key.Scope
		gen := snap.AvailabilityGen
		start := time.Now()
		r := c.ProbeResourceAvailability(ctx, snap.Clients, shortName)
		return messages.AvailabilityChecked{
			ResourceType: shortName,
			HasResources: r.HasResources,
			Count:        r.Count,
			Truncated:    r.Truncated,
			Issues:       r.Issues,
			Resources:    r.Resources,
			Err:          r.Err,
			Gen:          gen,
			Duration:     time.Since(start),
		}, nil

	// --- enrichment probe (Wave 2) ---
	// Demo clients are real *awsclient.ServiceClients backed by typed fakes
	// (core/demo.NewServiceClients), so Wave-2 enrichers run against them
	// exactly as they run against live AWS clients — no demo-mode skip here.
	case TaskKindProbeEnrich:
		shortName := req.Key.Scope
		if !c.HasIssueEnricher(shortName) {
			return nil, nil
		}
		gen := snap.EnrichmentGen
		typeGen := snap.EnrichmentTypeGen[shortName]
		start := time.Now()
		r := c.ProbeEnrichment(ctx, snap.Clients, shortName)
		return messages.EnrichmentChecked{
			ResourceType:     shortName,
			Truncated:        r.Truncated,
			Findings:         r.Findings,
			AttentionDetails: r.AttentionDetails,
			FieldUpdates:     r.FieldUpdates,
			TruncatedIDs:     r.TruncatedIDs,
			Gen:              gen,
			TypeGen:          typeGen,
			Err:              r.Err,
			Duration:         time.Since(start),
		}, nil

	// --- save availability cache ---
	// Single save path for both renderers: TUI and web both route
	// TaskKindSaveCache through this executor case, so availability counts
	// and per-type rows/findings persist identically regardless of host.
	// Entries derive from RowStore rather than any renderer-local model
	// state (e.g. MainMenuModel), keeping this path renderer-neutral.
	case TaskKindSaveCache:
		if snap.NoCache {
			return nil, nil
		}
		var flashErr error
		// Per C7/C8: an availability-sweep + Wave-2 enrichment completion
		// must persist that type's per-row rows/findings (SaveResourceListCache)
		// WITHOUT requiring any list screen to have been opened — mirrors the
		// list-open persistence path (app.Controller.maybeSaveResourceListCache)
		// but is driven from the dispatch-time snapshot the caller captured via
		// SaveCachePayload (see its doc comment for why dispatch-time capture,
		// not a live session read, is required), falling back to a live
		// RowStore read (task #17 wave 1 stage 2 — replaces the removed
		// session.ProbeResources/ProbeTruncated live-read fallback) for any
		// nil-Payload dispatch. C6 scope: RowStore's retained rows ARE this
		// session's canonical top-level population for each type — the same
		// rows a fresh list-open would seed from.
		//
		// Runs BEFORE SaveAvailabilityCache (order matters): both calls write
		// tf.Issues for the same type, and SaveAvailabilityCache's issueKnown
		// branch is the one with access to the fuller ResourceCache-derived
		// aggregate — it must run second so it either overwrites with that
		// exact aggregate (issueKnown) or carries forward the rows/issues this
		// call just persisted via existing.Issues/existing.Rows (not known).
		// The reverse order let the row-derived, potentially-incomplete count
		// computed here unconditionally clobber a more accurate aggregate.
		saveResources, saveTruncated := c.rowStoreResourcesAndTruncated()
		var wave2Complete bool
		if p, ok := req.Payload.(*SaveCachePayload); ok && p != nil {
			saveResources, saveTruncated = p.Resources, p.Truncated
			wave2Complete = p.Wave2Complete
		}
		// A nil-Payload dispatch is never the tagged Wave-2-completion save
		// (wave2Complete stays false — C6b): only handleEnrichmentChecked's
		// "all done" branch sets Wave2Complete, and it always carries a
		// SaveCachePayload.
		if err := c.saveProbeResourcesToTypeFiles(saveResources, saveTruncated, wave2Complete); err != nil {
			flashErr = err
		}
		entries, truncated, issueCounts, issueTruncated, issueKnown := c.availabilityFromResourceCache()
		if entries != nil {
			if err := c.SaveAvailabilityCache(
				entries, truncated, issueCounts, issueTruncated, issueKnown,
			); err != nil && flashErr == nil {
				flashErr = err
			}
		}
		if flashErr != nil {
			return messages.Flash{Text: fmt.Sprintf("cache save: %v", flashErr), IsError: true}, nil
		}
		return nil, nil

	// --- AWS connect ---
	case TaskKindConnect:
		p, ok := req.Payload.(ConnectPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing ConnectPayload", req.Key.Kind)
		}
		result, err := c.ConnectAWS(ctx, p.Profile, p.Region)
		return messages.ClientsReady{
			Clients: result.Clients,
			Region:  result.Region,
			Gen:     p.Gen,
			Err:     err,
		}, nil

	// --- fetch caller identity ---
	case TaskKindFetchIdentity:
		gen := snap.ConnectGen
		identity, err := c.FetchIdentity(ctx, snap.Clients)
		if err != nil {
			return messages.IdentityError{Err: err.Error(), Gen: gen}, nil
		}
		return messages.IdentityLoaded{Identity: identity, Gen: gen}, nil

	// --- load on-disk availability cache ---
	case TaskKindLoadAvailCache:
		store := c.LoadAvailabilityCache()
		if store == nil {
			return messages.AvailabilityCacheLoaded{
				Entries: make(map[string]int),
			}, nil
		}
		return cacheStoreToEvent(store), nil

	// --- demo prefetch ---
	case TaskKindDemoPrefetchCounts:
		gen := snap.AvailabilityGen
		r := c.DemoPrefetchCounts(ctx, snap.Clients)
		return messages.AvailabilityPrefetched{
			Entries:         r.Entries,
			Truncated:       r.Truncated,
			IssueCounts:     r.IssueCounts,
			IssueTruncated:  r.IssueTruncated,
			Resources:       r.Resources,
			Pagination:      r.Pagination,
			Gen:             gen,
			PrefetchErr:     r.PrefetchErr,
			PrefetchSoftErr: r.PrefetchSoftErr,
		}, nil

	// --- related-check fan-out ---
	// Bounded concurrent fan-out (MaxConcurrentProbes) over RunRelatedDef —
	// the exact per-def logic the TUI's own fan-out uses
	// (internal/tui/runtime_adapter_related.go), so the two lanes can never
	// diverge on timeout, panic recovery, NeedsTargetCache prefetch, or
	// lazy-add behavior.
	case KindRelatedCheck:
		p, ok := req.Payload.(RelatedCheckPayload)
		if !ok {
			return nil, nil
		}
		op := p.Op
		resourceType := op.ResourceType
		if resourceType == "" {
			resourceType, _ = splitScope(req.Key.Scope)
			op.ResourceType = resourceType
		}
		defs := resource.GetRelated(resourceType)
		if len(defs) == 0 {
			return nil, nil
		}
		cacheSnap := c.BuildResourceCacheSnapshot()
		fetchKeys := c.FetchOriginCacheKeys()
		mainCacheKeys := make(map[string]struct{}, len(fetchKeys))
		for _, k := range fetchKeys {
			mainCacheKeys[k] = struct{}{}
		}
		return c.runRelatedCheckers(ctx, op, cacheSnap, mainCacheKeys, defs), nil

	// --- enrich detail ---
	case KindEnrichDetail:
		p, ok := req.Payload.(EnrichDetailPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing EnrichDetailPayload", req.Key.Kind)
		}
		enricher := resource.GetDetailEnricher(p.Op.ResourceType)
		if enricher == nil {
			return nil, fmt.Errorf("ExecuteTask %s: no detail enricher for %s", req.Key.Kind, p.Op.ResourceType)
		}
		if p.DetailCtx == nil {
			return nil, fmt.Errorf("ExecuteTask %s: nil DetailCtx", req.Key.Kind)
		}
		enriched, err := enricher(ctx, p.DetailCtx, p.Op.Resource)
		return messages.EnrichDetailResult{
			ResourceType: p.Op.ResourceType,
			ResourceID:   p.Op.Resource.ID,
			EnrichedRes:  enriched,
			Err:          err,
			OperationID:  p.Op.ID,
		}, nil

	// --- fetch resources (top-level) ---
	case KindFetchResources:
		resourceType := req.Key.Scope
		gen := snap.AvailabilityGen
		res, err := c.FetchResources(ctx, snap.Clients, resourceType)
		if err != nil && len(res.Resources) == 0 {
			return messages.APIError{ResourceType: resourceType, Err: err, Gen: gen}, nil
		}
		// C1: a verify-refetch must verify the content actually being shown,
		// not just page 1 — so page up to the previously-cached depth. C5: a
		// truncated first page must never downgrade a stored exact total;
		// without this loop a 55-row cached/exact list would silently swap
		// down to a 50-row truncated one. Bounded by CachedListDepth so this
		// never fetches deeper than what was already shown.
		for err == nil && res.Pagination != nil && res.Pagination.IsTruncated &&
			res.Pagination.NextToken != "" && len(res.Resources) < c.CachedListDepth(resourceType) {
			var more resource.FetchResult
			more, err = c.FetchMoreResources(ctx, snap.Clients, FetchMoreParams{
				ResourceType: resourceType,
				Token:        res.Pagination.NextToken,
			})
			if err != nil {
				break
			}
			if len(more.Resources) == 0 {
				// A zero-progress page (hostile/buggy pagination: a NextToken
				// that keeps returning empty pages) must terminate the loop —
				// len(res.Resources) never grows past this point, so the
				// CachedListDepth bound above would otherwise spin forever.
				res.Pagination = more.Pagination
				break
			}
			res.Resources = append(res.Resources, more.Resources...)
			res.Pagination = more.Pagination
		}
		return messages.ResourcesLoaded{
			ResourceType: resourceType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Err:          err,
			Gen:          gen,
		}, nil

	// --- fetch filtered resources ---
	case KindFetchFiltered:
		p, ok := req.Payload.(FetchFilteredPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing FetchFilteredPayload", req.Key.Kind)
		}
		resourceType := req.Key.Scope
		gen := snap.AvailabilityGen
		res, err := c.FetchResourcesFiltered(ctx, snap.Clients, resourceType, p.Filter)
		if err != nil && len(res.Resources) == 0 {
			return messages.APIError{ResourceType: resourceType, Err: err, Gen: gen}, nil
		}
		return messages.ResourcesLoaded{
			ResourceType: resourceType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Err:          err,
			Gen:          gen,
		}, nil

	// --- fetch more (pagination) ---
	case KindFetchMore:
		p, ok := req.Payload.(FetchMorePayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing FetchMorePayload", req.Key.Kind)
		}
		resourceType := req.Key.Scope
		gen := snap.AvailabilityGen
		res, err := c.FetchMoreResources(ctx, snap.Clients, FetchMoreParams{
			ResourceType: resourceType,
			Token:        p.ContinuationToken,
			ParentCtx:    p.ParentContext,
			FetchFilter:  p.FetchFilter,
		})
		if err != nil && len(res.Resources) == 0 {
			return messages.APIError{ResourceType: resourceType, Err: err, Gen: gen}, nil
		}
		return messages.ResourcesLoaded{
			ResourceType: resourceType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Append:       true,
			Err:          err,
			Gen:          gen,
		}, nil

	// --- fetch child resources ---
	case TaskKindFetchChildResources:
		p, ok := req.Payload.(FetchChildResourcesPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing FetchChildResourcesPayload", req.Key.Kind)
		}
		gen := snap.AvailabilityGen
		res, err := c.FetchChildResources(ctx, snap.Clients, p.ChildType, p.ParentContext)
		if err != nil && len(res.Resources) == 0 {
			return messages.APIError{ResourceType: p.ChildType, Err: err, Gen: gen}, nil
		}
		return messages.ResourcesLoaded{
			ResourceType: p.ChildType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Err:          err,
			Gen:          gen,
		}, nil

	// --- fetch reveal value ---
	case KindFetchReveal:
		p, ok := req.Payload.(FetchRevealPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing FetchRevealPayload", req.Key.Kind)
		}
		gen := snap.ConnectGen
		value, err := c.FetchRevealValue(ctx, snap.Clients, p.ResourceType, p.ResourceID)
		return messages.ValueRevealed{
			ResourceType: p.ResourceType,
			ResourceID:   p.ResourceID,
			Value:        value,
			Err:          err,
			Gen:          gen,
		}, nil

	// --- fetch-by-id-detail ---
	// The TUI adapter navigates directly to the detail view after fetching.
	// The executor returns the resource as ResourcesLoaded so non-TUI callers
	// can observe the fetched data; navigation is a renderer concern.
	case KindFetchByIDDetail:
		p, ok := req.Payload.(FetchByIDDetailPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing FetchByIDDetailPayload", req.Key.Kind)
		}
		fn := resource.GetFetchByIDs(p.TargetType)
		if fn == nil {
			return messages.ByIDFetchFailed{TargetType: p.TargetType, ID: p.ID, Reason: fmt.Sprintf("no by-id fetcher for %s", p.TargetType)}, nil
		}
		res, err := fn(ctx, snap.Clients, []string{p.ID})
		if err != nil {
			return messages.ByIDFetchFailed{TargetType: p.TargetType, ID: p.ID, Reason: err.Error()}, nil
		}
		if len(res) == 0 {
			return messages.ByIDFetchFailed{TargetType: p.TargetType, ID: p.ID, Reason: fmt.Sprintf("%s %s not found", p.TargetType, p.ID)}, nil
		}
		gen := snap.AvailabilityGen
		return messages.ResourcesLoaded{
			ResourceType: p.TargetType,
			Resources:    res,
			Gen:          gen,
		}, nil

	// --- fetch costs (Cost Explorer) ---
	case KindFetchCosts:
		p, ok := req.Payload.(FetchCostsPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing FetchCostsPayload", req.Key.Kind)
		}
		if snap.Clients == nil || snap.Clients.CostExplorer == nil {
			return messages.CostsLoaded{
				Query:  p.Query,
				Window: p.Window,
				Err:    fmt.Errorf("cost explorer: no client configured for this session"),
				Gen:    snap.ConnectGen,
			}, nil
		}
		// The grid query and the anomaly overlay (FR-014) depend only on the
		// payload, never on each other's result — they run concurrently
		// rather than back to back. Joined below: the grid's own error wins
		// (Err), an anomaly-fetch failure degrades to no marks rather than
		// failing the grid data it accompanies. Each half is independently
		// skippable — SkipAnomalies when the store's cached anomaly snapshot
		// is still within its TTL, SkipGrid when the grid shape is already
		// fully covered and only the anomaly slot needs refreshing (X3,
		// screen.FetchPlan's two independent booleans) — so neither a
		// grid-only nor an anomalies-only dispatch ever bills for the half
		// it doesn't need. Each half's own request count folds into
		// Requests: a separate billed CE call, not free.
		var (
			result          awsclient.CostFetchResult
			err             error
			anomalies       []costs.AnomalyMark
			anomalyRequests int
			wg              sync.WaitGroup
		)
		if !p.SkipGrid {
			wg.Go(func() {
				if slices.Contains(p.Query.GroupBy, costs.DimensionResourceID) {
					// GetCostAndUsageWithResources requires the query to isolate
					// exactly one SERVICE — the drill query that reaches
					// RESOURCE_ID always pins it (costs.ResourceDrillAllowed
					// gates the drill before this dispatch exists), so a
					// missing pin here means the gate was bypassed: fail loud
					// rather than silently falling back to the plain,
					// unfiltered call.
					if len(p.Query.Filter.Equals[costs.DimensionService]) != 1 {
						err = awsclient.ErrCostsResourceDrillMissingServiceFilter
					} else {
						result, err = awsclient.FetchCostAndUsageWithResources(ctx, snap.Clients.CostExplorer, p.Query)
					}
				} else {
					result, err = awsclient.FetchCostAndUsage(ctx, snap.Clients.CostExplorer, p.Query)
				}
			})
		}
		if !p.SkipAnomalies {
			wg.Go(func() {
				marks, n, aErr := awsclient.FetchCostAnomaliesCounted(ctx, snap.Clients.CostExplorer, p.Query.Range)
				anomalyRequests = n
				if aErr == nil {
					// A genuinely successful fetch that found zero anomalies
					// still returns a NON-NIL slice — messages.CostsLoaded's
					// Anomalies field carries no separate "was this actually
					// requested" flag, so ApplyCostsLoaded's own discriminator
					// (ev.Anomalies != nil) must never see the same nil value
					// a skipped/errored fetch also leaves it at.
					if marks == nil {
						marks = []costs.AnomalyMark{}
					}
					anomalies = marks
				}
			})
		}
		wg.Wait()
		return messages.CostsLoaded{
			Query:     p.Query,
			Window:    p.Window,
			Grid:      costs.GridResult{Fetched: !p.SkipGrid, Records: result.Records, Err: err},
			Attrs:     result.Attrs,
			Anomalies: anomalies,
			Requests:  result.RequestCount + anomalyRequests,
			Err:       err,
			Gen:       snap.ConnectGen,
		}, nil

	// --- adapter-only kinds ---
	case TaskKindFlashTick,
		TaskKindEmitNavigate,
		TaskKindEmitAPIError,
		TaskKindReadThemeFile,
		TaskKindSaveThemeConfig,
		KindFetchProfiles:
		return nil, ErrAdapterOnlyTask

	default:
		return nil, fmt.Errorf("ExecuteTask: unknown task kind %q", req.Key.Kind)
	}
}

// availabilityFromResourceCache derives availability entries from RowStore's
// retained full (non-Partial) entries. Used by the save-cache executor path
// as a renderer-neutral alternative to reading counts from MainMenuModel.
func (c *Core) availabilityFromResourceCache() (
	entries map[string]int,
	truncated map[string]bool,
	issueCounts map[string]int,
	issueTruncated map[string]bool,
	issueKnown map[string]bool,
) {
	all := c.session.RowStore.SnapshotAll(false)
	if len(all) == 0 {
		return nil, nil, nil, nil, nil
	}
	entries = make(map[string]int, len(all))
	truncated = make(map[string]bool)
	issueCounts = make(map[string]int)
	issueTruncated = make(map[string]bool)
	issueKnown = make(map[string]bool)
	typeCache := make(map[string]*resource.ResourceTypeDef)
	for rt, tr := range all {
		entries[rt] = len(tr.Rows)
		// C5: a nil Pagination means this entry's truncation state was never
		// observed (e.g. a partial/legacy cache write) — treat as unknown,
		// which must NOT be conflated with a genuine "not truncated"
		// observation. Unknown truncation is conservatively truncated so a
		// downstream Exact-count derivation (SaveAvailabilityCache) never
		// promotes an unobserved page-1-shaped count to Exact (the
		// false-exact half of D14: a false Exact=true silently downgraded a
		// real exact 55 to a false exact 50 and then dropped the stored Rows).
		isTrunc := tr.Pagination == nil || tr.Pagination.IsTruncated
		if isTrunc {
			truncated[rt] = true
			issueTruncated[rt] = true
		}
		td, ok := typeCache[rt]
		if !ok {
			td = resource.FindResourceType(rt)
			typeCache[rt] = td
		}
		issues := 0
		for _, r := range tr.Rows {
			if td != nil && !td.ExcludeFromIssueBadge && td.ResolveColor(r).IsIssue() {
				issues++
			}
		}
		issueCounts[rt] = issues
		issueKnown[rt] = true
	}
	return
}

// saveProbeResourcesToTypeFiles persists probeResources — a snapshot (or, for
// a nil-Payload dispatch, a live read) of the availability sweep's (and
// Wave-2 enrichment's) retained per-type rows, findings included — to each
// type's on-disk file via SaveResourceListCache. Per C7/C8: this is the
// sweep-completion counterpart to app.Controller.maybeSaveResourceListCache,
// which only runs when a list screen has been opened; this path lets that
// same per-type persistence happen from a background sweep alone, so a
// corrupt/missing type file self-heals on the next sweep rather than only on
// the next list visit.
//
// No-op when probeResources is empty (nothing retained — e.g. mid-sweep with
// no completions yet, or every probe failed). Best-effort per type: a save
// failure for one type does not prevent the others from being attempted: the
// first error is returned to the caller (mirrors SaveAvailabilityCache's
// firstErr convention), matching every other cache-write call site's
// best-effort posture.
//
// wave2Complete distinguishes the two TaskKindSaveCache dispatch sites that
// both route through this function (C6b): the Wave-1 sweep-completion save
// (handleAvailabilityChecked) passes false — a bare rows-carrying observation
// that must carry forward any Wave-2 data the on-disk rows already have
// (reconcileTypeFile's carry step). The Wave-2-completion save
// (handleEnrichmentChecked) passes true — this observation IS the fresh
// enrichment result and must supersede carried data wholesale so a
// healed/resolved issue can clear.
func (c *Core) saveProbeResourcesToTypeFiles(probeResources map[string][]resource.Resource, probeTruncated map[string]bool, wave2Complete bool) error {
	if len(probeResources) == 0 {
		return nil
	}
	var firstErr error
	for shortName, resources := range probeResources {
		truncated := probeTruncated[shortName]
		exact := !truncated
		td := resource.FindResourceType(shortName)
		issuesKnown := td != nil && !td.ExcludeFromIssueBadge
		issues := 0
		if issuesKnown {
			issues = unifiedIssueCount(resources, *td, nil)
		}
		err := c.SaveTypeRows(shortName, resources, len(resources), exact, issues, issuesKnown, truncated, wave2Complete)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// CacheStoreToEvent converts a *cache.Store's loaded per-type files into a
// messages.AvailabilityCacheLoaded event carrying the counts-only projection
// (Entries/Truncated/IssueCounts/IssueTruncated/IssueKnown). Row/Findings data
// stays in the Store itself — handleAvailabilityCacheLoaded reads it directly
// via Core.CacheStore()/EnsureCacheStore() to seed session.ProbeResources, so
// it is not duplicated onto this event. Exported so renderer adapters (e.g.
// the TUI's tea.Cmd-based loadAvailabilityCache) share this single
// conversion instead of re-deriving it.
func CacheStoreToEvent(store *cache.Store) messages.AvailabilityCacheLoaded {
	if store == nil {
		// Mirrors the TaskKindLoadAvailCache case above: a nil store (no
		// cache loaded for this pair) is "no knowledge yet", not a panic.
		return messages.AvailabilityCacheLoaded{
			Entries: make(map[string]int),
		}
	}
	return cacheStoreToEvent(store)
}

func cacheStoreToEvent(store *cache.Store) messages.AvailabilityCacheLoaded {
	types := store.Types()
	entries := make(map[string]int, len(types))
	truncated := make(map[string]bool)
	issueCounts := make(map[string]int)
	issueTruncated := make(map[string]bool)
	issueKnown := make(map[string]bool)
	for name, tf := range types {
		// A completely zero-value TypeFile (never Put with any real
		// probe/fetch data — HasResources false, Count 0, no issues known, no
		// rows) carries no observation to report. Excluding it here mirrors
		// the pre-round-2 cache.Entry.Error-string exclusion and C1's "never
		// 0" placeholder rule: a genuinely-observed empty type still reports
		// Count=0 through this same path, but only once something has
		// actually Put it (HasResources/Count/IssuesKnown/Rows all zero at
		// once is the "nothing was ever recorded" signature).
		if !tf.HasResources && tf.Count == 0 && !tf.IssuesKnown && len(tf.Rows) == 0 {
			continue
		}
		entries[name] = tf.Count
		if !tf.Exact {
			truncated[name] = true
		}
		if tf.IssuesKnown {
			issueCounts[name] = tf.Issues
			issueKnown[name] = true
			if tf.IssuesTruncated {
				issueTruncated[name] = true
			}
		}
	}
	return messages.AvailabilityCacheLoaded{
		Entries:        entries,
		Truncated:      truncated,
		IssueCounts:    issueCounts,
		IssueTruncated: issueTruncated,
		IssueKnown:     issueKnown,
	}
}

// runRelatedCheckers runs every registered RelatedDef checker for op
// concurrently, bounded by MaxConcurrentProbes, via RunRelatedDef — the same
// per-def logic the TUI's own fan-out uses. Returns a RelatedCheckBatch
// carrying one entry per def; results is written by index (never appended),
// so concurrent completion order never reorders the returned slice.
func (c *Core) runRelatedCheckers(
	ctx context.Context,
	op DetailOperation,
	cacheSnap resource.ResourceCache,
	mainCacheKeys map[string]struct{},
	defs []resource.RelatedDef,
) messages.RelatedCheckBatch {
	results := make([]messages.RelatedCheckResult, len(defs))
	sem := make(chan struct{}, MaxConcurrentProbes)
	var wg sync.WaitGroup
	for i, def := range defs {
		wg.Add(1)
		go func(i int, def resource.RelatedDef) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = RunRelatedDef(ctx, op, cacheSnap, mainCacheKeys, def)
		}(i, def)
	}
	wg.Wait()

	return messages.RelatedCheckBatch{
		ResourceType:     op.ResourceType,
		SourceResourceID: op.Resource.ID,
		Results:          results,
		OperationID:      op.ID,
	}
}

// RunRelatedDef runs one RelatedDef's checker for op.Resource and returns
// the per-def result stamped with op.ID. Shared by both renderer lanes: the
// TUI's per-def tea.Cmd fan-out (internal/tui/runtime_adapter_related.go,
// progressive rendering — scheduling stays a renderer concern) and this
// file's runRelatedCheckers (bounded concurrent fan-out) call this exact
// function, so a checker's NeedsTargetCache prefetch, timeout, panic
// recovery, and lazy-add behavior can never diverge between them.
//
// ctx is wrapped with awsclient.WithDetailOp(ctx, op.ID) once here so every
// AWS call this def's checker (and any NeedsTargetCache prefetch or
// lazy-add FetchByIDs call) makes shares one coalescing namespace scoped to
// this operation (core/aws/coalesce.go) — an operation's enricher and every
// one of its related checkers coalesce concurrent identical calls with each
// other; a later operation (fresh open or refresh) can never join them.
func RunRelatedDef(ctx context.Context, op DetailOperation, cacheSnap resource.ResourceCache, mainCacheKeys map[string]struct{}, def resource.RelatedDef) (result messages.RelatedCheckResult) {
	defer func() {
		if r := recover(); r != nil {
			result = messages.RelatedCheckResult{
				ResourceType:     op.ResourceType,
				SourceResourceID: op.Resource.ID,
				DefDisplayName:   def.DisplayName,
				Result:           resource.UnknownRelated(def.TargetType),
				OperationID:      op.ID,
				LazyAddError:     fmt.Errorf("related checker for %s panicked: %v", def.TargetType, r),
			}
		}
	}()

	if def.Checker == nil {
		// No checker: unknown state so the row shows "?" not 0.
		return messages.RelatedCheckResult{
			ResourceType:     op.ResourceType,
			SourceResourceID: op.Resource.ID,
			DefDisplayName:   def.DisplayName,
			Result:           resource.UnknownRelated(def.TargetType),
			OperationID:      op.ID,
		}
	}

	checkCtx, cancel := context.WithTimeout(awsclient.WithDetailOp(ctx, op.ID), RelatedCheckerTimeout)
	defer cancel()

	localCache := cacheSnap
	var cachedPages map[string]resource.ResourceCacheEntry
	if def.NeedsTargetCache {
		if _, inMain := mainCacheKeys[def.TargetType]; !inMain {
			if pf := resource.GetPaginatedFetcher(def.TargetType); pf != nil {
				// E5 partial success: rows may arrive alongside a composite
				// error (listed-but-denied resources). Seed whatever rows
				// came — a partially-visible target cache beats an unknown
				// "?" row.
				if fr, err := pf(checkCtx, op.Clients, ""); err == nil || len(fr.Resources) > 0 {
					isTrunc := fr.Pagination != nil && fr.Pagination.IsTruncated
					if prev, hasPrev := localCache[def.TargetType]; hasPrev && prev.IsTruncated {
						isTrunc = true
					}
					entry := resource.ResourceCacheEntry{
						Resources:   fr.Resources,
						IsTruncated: isTrunc,
						Pagination:  fr.Pagination,
					}
					enriched := make(resource.ResourceCache, len(localCache)+1)
					maps.Copy(enriched, localCache)
					enriched[def.TargetType] = entry
					localCache = enriched
					cachedPages = map[string]resource.ResourceCacheEntry{def.TargetType: entry}
				}
			}
		}
	}

	checkResult := def.Checker(checkCtx, op.Clients, op.Resource, localCache)
	checkResult.TargetType = def.TargetType

	var lazyAdded map[string][]resource.Resource
	var lazyAddError error
	// CloudTrail-event pivots are event-derived: the ids come straight from
	// the event body, so the count needs no fetch. Skip the eager prefetch —
	// the drill fetches on demand (KindFetchByIDDetail) — so a cross-account
	// target (an AssumeRole role in another account) does not surface a
	// "FetchByIDs failed" header error at detail open.
	if op.Resource.Type != "ct-events" && len(checkResult.ResourceIDs) > 0 {
		if ff := resource.GetFetchByIDs(def.TargetType); ff != nil {
			missing := MissingFromCache(localCache, def.TargetType, checkResult.ResourceIDs)
			if len(missing) > 0 {
				extra, fetchErr := ff(checkCtx, op.Clients, missing)
				if fetchErr != nil {
					lazyAddError = fetchErr
				}
				if len(extra) > 0 {
					lazyAdded = map[string][]resource.Resource{def.TargetType: extra}
				}
			}
		}
	}

	return messages.RelatedCheckResult{
		ResourceType:       op.ResourceType,
		SourceResourceID:   op.Resource.ID,
		DefDisplayName:     def.DisplayName,
		Result:             checkResult,
		OperationID:        op.ID,
		CachedPages:        cachedPages,
		LazyAddedResources: lazyAdded,
		LazyAddError:       lazyAddError,
	}
}

// splitScope splits a "type/id" TaskKey.Scope into its two components.
func splitScope(scope string) (resourceType, id string) {
	for i := 0; i < len(scope); i++ {
		if scope[i] == '/' {
			return scope[:i], scope[i+1:]
		}
	}
	return scope, ""
}

// FetchFilteredPayload is the typed payload for KindFetchFiltered tasks.
// The TUI adapter derives the filter from its own message fields at dispatch
// time; headless callers (web, DrainSync) attach this payload so ExecuteTask
// can invoke the filtered fetcher without reaching into renderer state.
type FetchFilteredPayload struct {
	Filter map[string]string
}

func (FetchFilteredPayload) isTaskPayload() {}
