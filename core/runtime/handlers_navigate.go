// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// handlers_navigate.go — NavigateEvent dispatch.
//
//	HandleNavigate — resolves the navigation kind for the requested target,
//	                 mutates session state where the runtime owns it
//	                 (canonical-type resolution, EnrichGen / EnrichResKey
//	                 bumps for detail enrichment), and returns the decision
//	                 plus any TaskRequests the adapter should start.
//
// View construction, view-stack manipulation, and Bubble Tea specifics
// remain in the TUI adapter (internal/tui/runtime_adapter_navigate.go).
// The runtime owns only the platform-agnostic policy: type validation,
// cache lookups, enrichment-dispatch trigger, and fetch task emission.
package runtime

import (
	"fmt"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// NavigateTarget enumerates the navigation targets the runtime knows how
// to dispatch. It mirrors the TUI adapter's messages.ViewTarget; adapters
// translate from their native enum before calling HandleNavigate.
type NavigateTarget int

const (
	NavigateTargetUnknown NavigateTarget = iota
	NavigateTargetMainMenu
	NavigateTargetResourceList
	NavigateTargetDetail
	NavigateTargetYAML
	NavigateTargetJSON
	NavigateTargetReveal
	NavigateTargetProfile
	NavigateTargetRegion
	NavigateTargetTheme
	NavigateTargetHelp
	NavigateTargetCosts
)

// NavigateEvent is the runtime-side event for unified navigation.
// Adapters translate from their native message type before calling
// HandleNavigate.
type NavigateEvent struct {
	Target         NavigateTarget
	ResourceType   string             // alias allowed; runtime canonicalizes
	Resource       *resource.Resource // for Detail/YAML/JSON/Reveal
	ReplaceCurrent bool               // pop current view before pushing target
}

// NavigateKind enumerates the possible outcomes of HandleNavigate. The
// adapter switches on Kind and uses the populated NavigateResult fields
// appropriate for that kind.
type NavigateKind int

const (
	NavigateKindNoop                   NavigateKind = iota
	NavigateKindFlash                               // FlashMessage / FlashIsError populated
	NavigateKindPopAll                              // pop every view from the stack
	NavigateKindPushResourceList                    // ResolvedType + DisplayAlias populated; cache miss
	NavigateKindPushResourceListCached              // ResolvedType + DisplayAlias + CachedEntry populated
	NavigateKindPushDetail                          // ResolvedType + Resource populated; DispatchEnrich/DispatchRelated flags
	NavigateKindPushYAML                            // ResolvedType + Resource populated; DispatchEnrich flag
	NavigateKindPushJSON                            // ResolvedType + Resource populated; DispatchEnrich flag
	NavigateKindPushHelp                            // adapter constructs the help view
	NavigateKindPushRegion                          // adapter constructs the region view
	NavigateKindPushTheme                           // adapter constructs the theme view
	NavigateKindFetchProfiles                       // adapter starts the profile fetch task
	NavigateKindFetchReveal                         // ResolvedType + Resource populated; adapter starts reveal task
	NavigateKindPushCosts                           // adapter pushes ScreenCosts + seeds CostsState via EnsureCostsState
)

// NavigateResult is the pure-function output of HandleNavigate. Fields are
// conditionally populated depending on Kind.
type NavigateResult struct {
	Kind            NavigateKind
	ResolvedType    string                     // canonical short name
	DisplayAlias    string                     // empty when same as ResolvedType
	ReplaceCurrent  bool                       // mirrors NavigateEvent.ReplaceCurrent
	Resource        *resource.Resource         // for Push{Detail,YAML,JSON} and FetchReveal
	CachedEntry     *domain.ListViewCacheEntry // for PushResourceListCached
	DispatchEnrich  bool                       // for Push{Detail,YAML,JSON}
	DispatchRelated bool                       // for PushDetail
	FlashMessage    string
	FlashIsError    bool
}

// TaskKind constants for fetch operations emitted by HandleNavigate.
// Adapters type-switch on these in their TaskRequest-to-Cmd translators.
const (
	// KindFetchProfiles asks the adapter to load the local AWS profile list.
	// TaskKey.Scope is empty.
	KindFetchProfiles TaskKind = "fetch-profiles"

	// KindFetchReveal asks the adapter to call the registered reveal fetcher
	// for the resource named by FetchRevealPayload.
	KindFetchReveal TaskKind = "fetch-reveal"

	// KindFetchCosts asks the adapter to run a Cost Explorer GetCostAndUsage
	// fetch for the costs.Query carried by FetchCostsPayload.
	KindFetchCosts TaskKind = "fetch-costs"
)

// FetchRevealPayload carries the typed inputs for KindFetchReveal.
type FetchRevealPayload struct {
	ResourceType string
	ResourceID   string
}

// isTaskPayload satisfies the TaskPayload marker interface.
func (FetchRevealPayload) isTaskPayload() {}

// FetchCostsPayload carries the costs.Query the adapter must fetch for
// KindFetchCosts, plus the exact visible-window periods that Query.Range
// spans (Window) — the executor threads Window through onto the resulting
// messages.CostsLoaded event so ApplyCostsLoaded can stamp Store.MergeCoverage
// for precisely what was requested, independent of which periods CE actually
// returned records for (R2: a period with zero returned groups is a real,
// cacheable answer, not a gap).
type FetchCostsPayload struct {
	Query  costs.Query
	Window []costs.Period
	// SkipAnomalies tells the executor to skip the GetAnomalies call
	// riding alongside the main cost-and-usage fetch — set by
	// ensureCostsShapeFetched when the store's cached anomaly snapshot
	// (Store.Anomalies) is still within its TTL, so a shape-miss on the
	// main data never forces a redundant anomaly refetch too.
	SkipAnomalies bool
	// SkipGrid tells the executor to skip the GetCostAndUsage(WithResources)
	// call: the grid shape is already fully covered (screen.FetchPlan.Grid
	// false) and only the anomaly slot is absent/expired — an
	// anomalies-only dispatch (X3), never billed for cost data the store
	// already has.
	SkipGrid bool
}

func (FetchCostsPayload) isTaskPayload() {}

// HandleNavigate resolves the navigation kind for ev, mutating session
// state the runtime owns (EnrichGen / EnrichResKey bumps for detail
// enrichment dispatch), and returns the decision plus any fetch tasks the
// adapter should start.
//
// View construction and Bubble Tea specifics remain in the TUI adapter so
// this handler is platform-agnostic and testable without standing up
// Bubble Tea.
func (c *Core) HandleNavigate(ev NavigateEvent) (NavigateResult, []TaskRequest) {
	switch ev.Target {
	case NavigateTargetMainMenu:
		return NavigateResult{Kind: NavigateKindPopAll}, nil

	case NavigateTargetResourceList:
		// One-shot command disarm (D15): disarm the deferred one-shot -c navigation the instant any
		// resource-list navigation actually happens. CommandArmed/PendingCommand
		// (session.go) latch a REPLAY of this exact navigation, deferred until
		// handleAvailabilityCacheLoaded's seed lands (deferred -c navigation, D11) — but nothing
		// previously re-checked "has the user already navigated since arming"
		// at consumption time. A manually-typed navigation to the SAME or a
		// DIFFERENT resource type in the race window between arming (connect
		// time) and consumption (availability-cache-loaded time) left the flag
		// armed, so the later deferred emit fired a second, redundant
		// NavigateTargetResourceList for the (still-armed) PendingCommand type —
		// pushing a second ScreenChildList/ScreenResourceList of that type onto
		// an already-navigated stack. Two ListStates then existed for one
		// visible screen: the first fetch landed on the first push, the second
		// (armed-replay) fetch landed on the second push, and callers reading
		// topListState() (the load-more gate, the renderer) saw whichever push
		// was topmost — silently diverging from whichever push the frame last
		// rendered. Clearing the arming here (not at the push site) closes the
		// race at its source: by the time handleAvailabilityCacheLoaded checks
		// CommandArmed, any real navigation that already happened has disarmed
		// it, so the deferred emit becomes the intended no-op instead of a
		// duplicate push. The armed emit's own eventual HandleNavigate call
		// re-disarms harmlessly (CommandArmed is already false by then, cleared
		// synchronously by handleAvailabilityCacheLoaded before dispatch).
		// Guarded on the current value (not an unconditional write) so the
		// overwhelmingly common never-armed case costs only a bool read.
		if c.session.CommandArmed {
			c.session.CommandArmed = false
			c.session.PendingCommand = ""
		}

		rt := resource.FindResourceType(ev.ResourceType)
		if rt == nil {
			return NavigateResult{
				Kind:         NavigateKindFlash,
				FlashMessage: fmt.Sprintf("unknown resource type: %s", ev.ResourceType),
				FlashIsError: true,
			}, nil
		}
		// When navigated via an alias (e.g. "rds" → ShortName "dbi"), preserve
		// the alias so the adapter can render the user-requested display name.
		alias := ev.ResourceType
		if alias == rt.ShortName {
			alias = ""
		}
		canon := rt.ShortName
		if entry, ok := c.ResourceCache(canon); ok {
			// Cached resources already carry fetcher-emitted Findings; no
			// re-derive needed (W1.4b.3 dropped the legacy Status/Issues bridge).
			return NavigateResult{
				Kind:         NavigateKindPushResourceListCached,
				ResolvedType: canon,
				DisplayAlias: alias,
				CachedEntry:  entry,
			}, nil
		}
		// Cache miss: adapter pushes a fresh list and the fetch task loads it.
		// Scope keeps the user-supplied type (alias preserved) so the fetcher
		// resolves to the same registry entry the adapter chose for display.
		result := NavigateResult{
			Kind:         NavigateKindPushResourceList,
			ResolvedType: canon,
			DisplayAlias: alias,
		}
		// The seed rides the miss branch, not a NavigateKindPushResourceListCached
		// promotion: this check above (c.ResourceCache(canon)) only hits a FULL
		// (non-Partial, OriginFetch) RowStore entry (warm-open seeding, C1 + Goal 4) — an
		// OriginProbe/OriginDisk entry retained below is knowledge the probe
		// gathered, not a verified live fetch, so the fetch must still run to
		// confirm/replace what the probe retained; Kind and the
		// KindFetchResources task below are unchanged.
		//
		// task #17 wave 1 stage 3: a type's rows live in exactly one RowStore
		// entry regardless of which lane wrote them, so the free-then-disk-
		// fallback race this comment used to describe against a separate
		// session.ProbeResources map no longer exists; the store is read
		// directly instead.
		//
		// Observed-empty guard on the disk-store fallback: it fires only when this session never
		// observed canon at all. Gen (domain.Gen, zero value 0) is the
		// observed-at-all discriminator here, NOT Origin — TypeRows{}'s zero
		// value has Origin==OriginDisk (iota 0), so testing Origin alone cannot
		// distinguish "never observed" from "observed via disk with zero rows";
		// every RowStore.Observe call unconditionally increments Gen from its
		// prior value (0 on first observation), so Gen!=0 means "observed",
		// independent of which Origin produced the observation. A tr.Gen!=0
		// entry with a zero-length Rows slice means a live Wave-1 probe (or a
		// disk seed, or a fetch) already confirmed the type is empty this
		// session — that observed-empty result is fresher than any disk row
		// (C2), so it seeds a bare list rather than falling back to stale disk
		// rows.
		tr := c.session.RowStore.Snapshot(canon)
		if tr.Gen != 0 {
			if len(tr.Rows) > 0 {
				result.CachedEntry = &domain.ListViewCacheEntry{
					Resources: tr.Rows,
					Pagination: &resource.PaginationMeta{
						IsTruncated: tr.Pagination != nil && tr.Pagination.IsTruncated,
					},
				}
			}
		} else {
			_ = c.ReadCacheStore(func(store *cache.Store) error {
				if store == nil {
					return nil
				}
				if tf, ok := store.Type(canon); ok && len(tf.Rows) > 0 {
					result.CachedEntry = &domain.ListViewCacheEntry{
						Resources: rowsFromCacheRows(canon, tf.Rows),
						Pagination: &resource.PaginationMeta{
							IsTruncated: !tf.Exact,
						},
						// Seed-time provisional total: tf.Count is the authoritative total for the
						// C6a reconstructable pair (Count may exceed len(tf.Rows) — a
						// counts-only write never touches Rows). Carry it through so
						// the seeded list's title shows the real total, not the
						// last-known page count. Only set when it actually exceeds
						// what Rows would already report, matching TotalCount's
						// "unknown/not applicable" zero-value contract.
						TotalCount: max(tf.Count, len(tf.Rows)),
					}
				}
				return nil
			})
		}
		return result, []TaskRequest{{
			Key:   TaskKey{Kind: KindFetchResources, Scope: ev.ResourceType},
			Cache: CacheNone,
		}}

	case NavigateTargetDetail, NavigateTargetYAML, NavigateTargetJSON:
		if ev.Resource == nil {
			return NavigateResult{Kind: NavigateKindNoop}, nil
		}
		// Canonicalize alias to the registered ShortName so EnrichResKey,
		// HasDetailEnricher, and downstream Stage 2 message routing all use
		// the same key the registry returns. When ResourceType is empty the
		// adapter is responsible for resolving from the active view before
		// dispatch (the runtime has no view stack to consult).
		resType := ev.ResourceType
		if td := resource.FindResourceType(resType); td != nil {
			resType = td.ShortName
		}
		kind := NavigateKindPushDetail
		switch ev.Target {
		case NavigateTargetYAML:
			kind = NavigateKindPushYAML
		case NavigateTargetJSON:
			kind = NavigateKindPushJSON
		}
		result := NavigateResult{
			Kind:           kind,
			ResolvedType:   resType,
			ReplaceCurrent: ev.ReplaceCurrent,
			Resource:       ev.Resource,
		}
		// Detail-enrichment dispatch is the only state mutation HandleNavigate
		// performs: bump EnrichGen only when the resource identity changes,
		// so opening YAML/JSON for the same resource doesn't invalidate an
		// in-flight enrichment from the detail view open.
		if resType != "" && resource.HasDetailEnricher(resType) {
			key := resType + ":" + ev.Resource.ID
			if key != c.session.EnrichResKey {
				c.session.EnrichGen++
				c.session.EnrichResKey = key
			}
			result.DispatchEnrich = true
		}
		// Related-check is detail-only. The runtime decides *applicability*
		// (this is a detail navigation → DispatchRelated); the adapter applies
		// the *gate* (d.NeedsRelatedCheck() && RelatedCache miss). This split is
		// the boundary-correct end state, not a deferral: NeedsRelatedCheck is
		// true only when the right column auto-shows, which depends on terminal
		// width — inherently renderer-side state a platform-agnostic runtime
		// cannot (and should not) own. The RelatedCache short-circuit is an
		// adapter-side render optimization for the same reason.
		if ev.Target == NavigateTargetDetail {
			result.DispatchRelated = true
		}
		return result, nil

	case NavigateTargetHelp:
		return NavigateResult{Kind: NavigateKindPushHelp}, nil

	case NavigateTargetProfile:
		return NavigateResult{Kind: NavigateKindFetchProfiles}, []TaskRequest{{
			Key:   TaskKey{Kind: KindFetchProfiles},
			Cache: CacheNone,
		}}

	case NavigateTargetRegion:
		return NavigateResult{Kind: NavigateKindPushRegion}, nil

	case NavigateTargetTheme:
		return NavigateResult{Kind: NavigateKindPushTheme}, nil

	case NavigateTargetReveal:
		if ev.Resource == nil {
			return NavigateResult{Kind: NavigateKindNoop}, nil
		}
		return NavigateResult{
				Kind:         NavigateKindFetchReveal,
				ResolvedType: ev.ResourceType,
				Resource:     ev.Resource,
			}, []TaskRequest{{
				Key:     TaskKey{Kind: KindFetchReveal, Scope: ev.ResourceType + "/" + ev.Resource.ID},
				Cache:   CacheNone,
				Payload: FetchRevealPayload{ResourceType: ev.ResourceType, ResourceID: ev.Resource.ID},
			}}

	case NavigateTargetCosts:
		// No unconditional fetch here (SC-002): the adapter's PushCosts
		// handling seeds CostsState via EnsureCostsState, then the shared
		// ensureCostsShapeFetched decides — a warm cache opens with zero CE
		// calls, a cold one fetches. See core/app/navigate.go and
		// internal/tui/runtime_adapter_navigate.go's NavigateKindPushCosts cases.
		return NavigateResult{Kind: NavigateKindPushCosts}, nil
	}
	return NavigateResult{Kind: NavigateKindNoop}, nil
}
