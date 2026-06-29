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

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
	return DispatchSnapshot{
		Clients:           c.session.Clients,
		AvailabilityGen:   c.session.AvailabilityGen,
		EnrichmentGen:     c.session.EnrichmentGen,
		ConnectGen:        c.session.ConnectGen,
		EnrichmentTypeGen: c.session.EnrichmentTypeGen,
		Profile:           c.session.Profile,
		Region:            c.session.Region,
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
		gen := snap.EnrichmentGen
		typeGen := snap.EnrichmentTypeGen[shortName]
		r := c.ProbeEnrichment(ctx, snap.Clients, shortName)
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
		if snap.NoCache {
			return nil, nil
		}
		entries, truncated, issueCounts, issueTruncated, issueKnown := c.availabilityFromResourceCache()
		if entries == nil {
			return nil, nil
		}
		err := c.SaveAvailabilityCache(
			snap.Profile, snap.Region,
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
		gen := snap.ConnectGen
		identity, err := c.FetchIdentity(ctx, snap.Clients)
		if err != nil {
			return messages.IdentityError{Err: err.Error(), Gen: gen}, nil
		}
		return messages.IdentityLoaded{Identity: identity, Gen: gen}, nil

	// --- load on-disk availability cache ---
	case TaskKindLoadAvailCache:
		cf, err := c.LoadAvailabilityCache(snap.Profile, snap.Region)
		if err != nil || cf == nil {
			return messages.AvailabilityCacheLoaded{
				Entries: make(map[string]int),
				Expired: true,
			}, nil
		}
		return cacheFileToEvent(cf), nil

	// --- demo prefetch ---
	case TaskKindDemoPrefetchCounts:
		gen := snap.AvailabilityGen
		r := c.DemoPrefetchCounts(ctx, snap.Clients)
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
		p, ok := req.Payload.(RelatedCheckPayload)
		if !ok {
			// No payload means this task was dispatched without a source resource
			// (e.g. from HandleRelatedCheckStarted in the TUI path). The headless
			// executor cannot invoke checkers without the resource; skip gracefully.
			return nil, nil
		}
		resourceType := p.ResourceType
		if resourceType == "" {
			resourceType, _ = splitScope(req.Key.Scope)
		}
		defs := resource.GetRelated(resourceType)
		if len(defs) == 0 {
			return nil, nil
		}
		cacheSnap := c.SnapshotCache()
		mainCacheKeys := make(map[string]struct{}, len(c.ResourceCacheKeys()))
		for _, k := range c.ResourceCacheKeys() {
			mainCacheKeys[k] = struct{}{}
		}
		gen := c.RelatedGen()
		return c.runRelatedCheckers(ctx, cacheSnap, mainCacheKeys, defs, p.Resource, resourceType, gen), nil

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
		gen := snap.AvailabilityGen
		res, err := c.FetchResources(ctx, snap.Clients, resourceType)
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
			return messages.Flash{Text: fmt.Sprintf("no by-id fetcher for %s", p.TargetType), IsError: true}, nil
		}
		res, err := fn(ctx, snap.Clients, []string{p.ID})
		if err != nil {
			return messages.Flash{Text: err.Error(), IsError: true}, nil
		}
		if len(res) == 0 {
			return messages.Flash{Text: fmt.Sprintf("%s %s not found", p.TargetType, p.ID), IsError: true}, nil
		}
		gen := snap.AvailabilityGen
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

// runRelatedCheckers runs all RelatedDef checkers for res sequentially,
// mirroring the TUI adapter's relatedCheckCmd fan-out logic (same
// NeedsTargetCache handling, same self-pivot-zero suppression, same
// result shape). Returns a RelatedCheckBatch carrying one entry per def.
//
// Differences from the TUI path (intentional):
//   - Sequential, not concurrent: the headless executor has no goroutine
//     budget or semaphore — DrainSync is synchronous by design.
//   - No lazy-add (FetchByIDs): the lazy-add path enriches the resource cache
//     for navigation UX (so clicking a related row finds the resource). In the
//     headless path the cache is not used for navigation, so the extra AWS
//     call is omitted for now. The per-def result still carries ResourceIDs
//     so callers can use them if needed.
//   - Returns a single RelatedCheckBatch instead of N individual
//     RelatedCheckResult messages so DrainSync can route all results in one
//     Handle call.
//
// snap and mainCacheKeys must be pre-built by the caller via c.SnapshotCache()
// and c.ResourceCacheKeys() — no renderer state is read here.
func (c *Core) runRelatedCheckers(
	ctx context.Context,
	snap map[string][]resource.Resource,
	mainCacheKeys map[string]struct{},
	defs []resource.RelatedDef,
	res resource.Resource,
	resourceType string,
	gen domain.Gen,
) messages.Event {
	results := make([]messages.RelatedCheckResult, 0, len(defs))

	for _, def := range defs {
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

		var checkResult resource.RelatedCheckResult
		if def.Checker == nil {
			// No checker: unknown count (-1) so the row shows "?" not 0.
			checkResult = resource.RelatedCheckResult{
				TargetType: def.TargetType,
				Count:      -1,
			}
		} else {
			checkResult = def.Checker(ctx, c.session.Clients, res, resource.ResourceCache(localSnapToCache(localSnap)))
			checkResult.TargetType = def.TargetType
		}

		// Self-pivot-zero: when the checker reports 0 and the target type is
		// the same as the source type, the row is meaningless (a resource
		// cannot be its own related resource). Mirror the TUI adapter's guard.
		if checkResult.Count == 0 && def.TargetType == resourceType {
			checkResult.Count = 0 // stays zero — ApplyDetailRelatedResult will render it; isSelfPivotZero hides it in the UI
		}

		results = append(results, messages.RelatedCheckResult{
			ResourceType:     resourceType,
			SourceResourceID: res.ID,
			DefDisplayName:   def.DisplayName,
			Result:           checkResult,
			Generation:       gen,
		})
	}

	if len(results) == 0 {
		return nil
	}
	return messages.RelatedCheckBatch{
		ResourceType:     resourceType,
		SourceResourceID: res.ID,
		Results:          results,
		Generation:       gen,
	}
}

// localSnapToCache converts the flat snap map (type→[]Resource) used by the
// executor into the ResourceCache map type the RelatedChecker signature expects.
func localSnapToCache(snap map[string][]resource.Resource) map[string]resource.ResourceCacheEntry {
	if len(snap) == 0 {
		return nil
	}
	out := make(map[string]resource.ResourceCacheEntry, len(snap))
	for k, v := range snap {
		out[k] = resource.ResourceCacheEntry{Resources: v}
	}
	return out
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
