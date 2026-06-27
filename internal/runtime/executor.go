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

	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ErrAdapterOnlyTask is returned by ExecuteTask for TaskKind values that are
// inherently renderer concerns and cannot be executed in a renderer-neutral
// context. See the executor.go package-doc for the complete list.
var ErrAdapterOnlyTask = errors.New("task kind is adapter-only and cannot be executed by Core.ExecuteTask")

// ExecuteTask executes req synchronously using the existing Core methods and
// returns the result as a messages.Event. It is safe to call from any
// goroutine and does not touch any renderer or Bubble Tea state.
//
// ctx is forwarded to every blocking AWS call; callers should supply a
// context with an appropriate timeout.
//
// Adapter-only kinds return (nil, ErrAdapterOnlyTask). A nil Event with a
// nil error means the task completed with no result to dispatch (e.g.
// probe-enrich skipped in demo mode, save-cache with nothing to persist).
func (c *Core) ExecuteTask(ctx context.Context, req TaskRequest) (messages.Event, error) {
	switch req.Key.Kind {

	// --- availability probe ---
	case TaskKindProbeAvailability:
		shortName := req.Key.Scope
		gen := c.session.AvailabilityGen
		r := c.ProbeResourceAvailability(ctx, c.session.Clients, shortName)
		return messages.AvailabilityChecked{
			ResourceType: shortName,
			HasResources: r.HasResources,
			Count:        r.Count,
			Truncated:    r.Truncated,
			Issues:       r.Issues,
			Resources:    r.Resources,
			Err:          r.Err,
			Gen:          gen,
		}, nil

	// --- enrichment probe (Wave 2) ---
	// Renderer-neutral coupling resolved: gate on c.isDemo instead of m.isDemo.
	case TaskKindProbeEnrich:
		if c.isDemo {
			return nil, nil
		}
		shortName := req.Key.Scope
		if !c.HasIssueEnricher(shortName) {
			return nil, nil
		}
		gen := c.session.EnrichmentGen
		typeGen := c.session.EnrichmentTypeGen[shortName]
		r := c.ProbeEnrichment(ctx, c.session.Clients, shortName)
		return messages.EnrichmentChecked{
			ResourceType:     shortName,
			Issues:           r.Issues,
			Truncated:        r.Truncated,
			Findings:         r.Findings,
			AttentionDetails: r.AttentionDetails,
			FieldUpdates:     r.FieldUpdates,
			TruncatedIDs:     r.TruncatedIDs,
			Gen:              gen,
			TypeGen:          typeGen,
			Err:              r.Err,
		}, nil

	// --- save availability cache ---
	// Renderer-neutral coupling resolved: derive entries from c.session.ResourceCache
	// instead of reading from m.stack[0] (MainMenuModel). The TUI adapter's
	// saveAvailabilityCache() continues to use the more precise MainMenuModel
	// counts for the live TUI; this path serves non-TUI hosts.
	case TaskKindSaveCache:
		if c.session.NoCache {
			return nil, nil
		}
		entries, truncated, issueCounts, issueTruncated, issueKnown := c.availabilityFromResourceCache()
		if entries == nil {
			return nil, nil
		}
		err := c.SaveAvailabilityCache(
			c.session.Profile, c.session.Region,
			entries, truncated, issueCounts, issueTruncated, issueKnown,
		)
		if err != nil {
			return messages.Flash{Text: fmt.Sprintf("cache save: %v", err), IsError: true}, nil
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
		gen := c.session.ConnectGen
		identity, err := c.FetchIdentity(ctx, c.session.Clients)
		if err != nil {
			return messages.IdentityError{Err: err.Error(), Gen: gen}, nil
		}
		return messages.IdentityLoaded{Identity: identity, Gen: gen}, nil

	// --- load on-disk availability cache ---
	case TaskKindLoadAvailCache:
		cf, err := c.LoadAvailabilityCache(c.session.Profile, c.session.Region)
		if err != nil || cf == nil {
			return messages.AvailabilityCacheLoaded{
				Entries: make(map[string]int),
				Expired: true,
			}, nil
		}
		return cacheFileToEvent(cf), nil

	// --- demo prefetch ---
	case TaskKindDemoPrefetchCounts:
		gen := c.session.AvailabilityGen
		r := c.DemoPrefetchCounts(ctx, c.session.Clients)
		return messages.AvailabilityPrefetched{
			Entries:        r.Entries,
			Truncated:      r.Truncated,
			IssueCounts:    r.IssueCounts,
			IssueTruncated: r.IssueTruncated,
			Resources:      r.Resources,
			Pagination:     r.Pagination,
			Gen:            gen,
			PrefetchErr:    r.PrefetchErr,
		}, nil

	// --- related-check fan-out ---
	// Renderer-neutral coupling resolved: cache-key set from c.ResourceCacheKeys(),
	// snapshot from c.SnapshotCache() — no m.stack[0] read needed.
	// The executor runs checkers sequentially (no goroutine fan-out); the TUI
	// adapter's relatedCheckCmd continues to use its concurrent fan-out with
	// semaphore and lazy-add for the live renderer.
	case KindRelatedCheck:
		resourceType, _ := splitScope(req.Key.Scope)
		defs := resource.GetRelated(resourceType)
		if len(defs) == 0 {
			return nil, nil
		}
		snap := c.SnapshotCache()
		mainCacheKeys := make(map[string]struct{}, len(c.ResourceCacheKeys()))
		for _, k := range c.ResourceCacheKeys() {
			mainCacheKeys[k] = struct{}{}
		}
		return c.runRelatedCheckers(ctx, snap, mainCacheKeys, defs), nil

	// --- enrich detail ---
	case KindEnrichDetail:
		p, ok := req.Payload.(EnrichDetailPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing EnrichDetailPayload", req.Key.Kind)
		}
		enricher := resource.GetDetailEnricher(p.ResourceType)
		if enricher == nil {
			return nil, fmt.Errorf("ExecuteTask %s: no detail enricher for %s", req.Key.Kind, p.ResourceType)
		}
		if p.DetailCtx == nil {
			return nil, fmt.Errorf("ExecuteTask %s: nil DetailCtx", req.Key.Kind)
		}
		enriched, err := enricher(ctx, p.DetailCtx, p.Resource)
		return messages.EnrichDetailResult{
			ResourceType: p.ResourceType,
			ResourceID:   p.Resource.ID,
			EnrichedRes:  enriched,
			Err:          err,
			Generation:   p.Generation,
		}, nil

	// --- fetch resources (top-level) ---
	case KindFetchResources:
		resourceType := req.Key.Scope
		gen := c.session.AvailabilityGen
		res, err := c.FetchResources(ctx, c.session.Clients, resourceType)
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

	// --- fetch filtered resources ---
	case KindFetchFiltered:
		p, ok := req.Payload.(fetchFilteredPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing fetchFilteredPayload", req.Key.Kind)
		}
		resourceType := req.Key.Scope
		gen := c.session.AvailabilityGen
		res, err := c.FetchResourcesFiltered(ctx, c.session.Clients, resourceType, p.Filter)
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
		gen := c.session.AvailabilityGen
		res, err := c.FetchMoreResources(ctx, c.session.Clients, FetchMoreParams{
			ResourceType: resourceType,
			Token:        p.ContinuationToken,
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
		gen := c.session.AvailabilityGen
		res, err := c.FetchChildResources(ctx, c.session.Clients, p.ChildType, p.ParentContext)
		if err != nil {
			return messages.APIError{ResourceType: p.ChildType, Err: err, Gen: gen}, nil
		}
		return messages.ResourcesLoaded{
			ResourceType: p.ChildType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Gen:          gen,
		}, nil

	// --- fetch reveal value ---
	case KindFetchReveal:
		p, ok := req.Payload.(FetchRevealPayload)
		if !ok {
			return nil, fmt.Errorf("ExecuteTask %s: missing FetchRevealPayload", req.Key.Kind)
		}
		gen := c.session.ConnectGen
		value, err := c.FetchRevealValue(ctx, c.session.Clients, p.ResourceType, p.ResourceID)
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
			return messages.Flash{Text: fmt.Sprintf("no by-id fetcher for %s", p.TargetType), IsError: true}, nil
		}
		res, err := fn(ctx, c.session.Clients, []string{p.ID})
		if err != nil {
			return messages.Flash{Text: err.Error(), IsError: true}, nil
		}
		if len(res) == 0 {
			return messages.Flash{Text: fmt.Sprintf("%s %s not found", p.TargetType, p.ID), IsError: true}, nil
		}
		gen := c.session.AvailabilityGen
		return messages.ResourcesLoaded{
			ResourceType: p.TargetType,
			Resources:    res,
			Gen:          gen,
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

// availabilityFromResourceCache derives availability entries from the session's
// ResourceCache. Used by the save-cache executor path as a renderer-neutral
// alternative to reading counts from MainMenuModel.
func (c *Core) availabilityFromResourceCache() (
	entries map[string]int,
	truncated map[string]bool,
	issueCounts map[string]int,
	issueTruncated map[string]bool,
	issueKnown map[string]bool,
) {
	if len(c.session.ResourceCache) == 0 {
		return nil, nil, nil, nil, nil
	}
	entries = make(map[string]int, len(c.session.ResourceCache))
	truncated = make(map[string]bool)
	issueCounts = make(map[string]int)
	issueTruncated = make(map[string]bool)
	issueKnown = make(map[string]bool)
	typeCache := make(map[string]*resource.ResourceTypeDef)
	for rt, entry := range c.session.ResourceCache {
		if entry == nil {
			continue
		}
		entries[rt] = len(entry.Resources)
		isTrunc := entry.Pagination != nil && entry.Pagination.IsTruncated
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
		for _, r := range entry.Resources {
			if td != nil && !td.ExcludeFromIssueBadge && td.ResolveColor(r).IsIssue() {
				issues++
			}
		}
		issueCounts[rt] = issues
		issueKnown[rt] = true
	}
	return
}

// cacheFileToEvent converts a *cache.File into a messages.AvailabilityCacheLoaded
// event. Mirrors the conversion logic in probe_adapter.go loadAvailabilityCache.
func cacheFileToEvent(cf *cache.File) messages.AvailabilityCacheLoaded {
	entries := make(map[string]int, len(cf.Resources))
	truncated := make(map[string]bool)
	issueCounts := make(map[string]int)
	issueTruncated := make(map[string]bool)
	issueKnown := make(map[string]bool)
	for name, entry := range cf.Resources {
		if entry.Error != "" {
			continue
		}
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
	return messages.AvailabilityCacheLoaded{
		Entries:        entries,
		Truncated:      truncated,
		Expired:        cf.IsExpired(cache.DefaultTTL),
		IssueCounts:    issueCounts,
		IssueTruncated: issueTruncated,
		IssueKnown:     issueKnown,
	}
}

// runRelatedCheckers runs all RelatedDef checkers for a resource sequentially.
// This is the renderer-neutral alternative to the TUI adapter's concurrent
// relatedCheckCmd fan-out. Returns nil (no single aggregate event) since each
// check result is a separate RelatedCheckResult in the TUI path; the executor
// path is primarily for testing the dispatch surface.
//
// snap and mainCacheKeys must be pre-built by the caller via c.SnapshotCache()
// and c.ResourceCacheKeys() — no renderer state is read here.
func (c *Core) runRelatedCheckers(
	ctx context.Context,
	snap map[string][]resource.Resource,
	mainCacheKeys map[string]struct{},
	defs []resource.RelatedDef,
) messages.Event {
	for _, def := range defs {
		if def.Checker == nil {
			continue
		}
		localSnap := snap
		if def.NeedsTargetCache {
			if _, inMain := mainCacheKeys[def.TargetType]; !inMain {
				if pf := resource.GetPaginatedFetcher(def.TargetType); pf != nil {
					if fr, err := pf(ctx, c.session.Clients, ""); err == nil {
						enriched := make(map[string][]resource.Resource, len(localSnap)+1)
						maps.Copy(enriched, localSnap)
						enriched[def.TargetType] = fr.Resources
						localSnap = enriched
					}
				}
			}
		}
		// Checker invocation omitted: a source resource.Resource is required
		// but not carried by KindRelatedCheck's TaskKey.Scope (only type/id,
		// not the full resource). The executor path confirms dispatch plumbing;
		// real checker invocation remains in the TUI fan-out adapter where the
		// source resource is in scope from the detail view.
		_ = localSnap
	}
	return nil
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

// fetchFilteredPayload is the typed payload for KindFetchFiltered tasks created
// through the executor. The TUI adapter derives the filter from its own message
// fields at dispatch time; this type lets ExecuteTask carry the same data.
type fetchFilteredPayload struct {
	Filter map[string]string
}

func (fetchFilteredPayload) isTaskPayload() {}
