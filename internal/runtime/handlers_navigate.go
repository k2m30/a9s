// handlers_navigate.go — NavigateEvent dispatch.
//
// PR-05a-h3 (AS-149) moves the unified-navigation entry point out of
// internal/tui per the Phase 05 boundary contract
// (docs/refactor/05-boundary.md §"5a-extract").
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

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
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
)

// NavigateResult is the pure-function output of HandleNavigate. Fields are
// conditionally populated depending on Kind.
type NavigateResult struct {
	Kind            NavigateKind
	ResolvedType    string                      // canonical short name
	DisplayAlias    string                      // empty when same as ResolvedType
	ReplaceCurrent  bool                        // mirrors NavigateEvent.ReplaceCurrent
	Resource        *resource.Resource          // for Push{Detail,YAML,JSON} and FetchReveal
	CachedEntry     *session.ResourceCacheEntry // for PushResourceListCached
	DispatchEnrich  bool                        // for Push{Detail,YAML,JSON}
	DispatchRelated bool                        // for PushDetail
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
)

// FetchRevealPayload carries the typed inputs for KindFetchReveal.
type FetchRevealPayload struct {
	ResourceType string
	ResourceID   string
}

// isTaskPayload satisfies the TaskPayload marker interface.
func (FetchRevealPayload) isTaskPayload() {}

// HandleNavigate resolves the navigation kind for ev, mutating session
// state the runtime owns (EnrichGen / EnrichResKey bumps for detail
// enrichment dispatch), and returns the decision plus any fetch tasks the
// adapter should start.
//
// Receiver migrated from *Model to *Core per docs/refactor/05-boundary.md.
// Session fields (ResourceCache, EnrichGen, EnrichResKey) are accessed
// through c.session instead of the previously-embedded model fields.
//
// View construction and Bubble Tea specifics remain in the TUI adapter so
// this handler is platform-agnostic and testable without standing up
// Bubble Tea.
func (c *Core) HandleNavigate(ev NavigateEvent) (NavigateResult, []TaskRequest) {
	switch ev.Target {
	case NavigateTargetMainMenu:
		return NavigateResult{Kind: NavigateKindPopAll}, nil

	case NavigateTargetResourceList:
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
		if entry, ok := c.session.ResourceCache[canon]; ok {
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
		return NavigateResult{
				Kind:         NavigateKindPushResourceList,
				ResolvedType: canon,
				DisplayAlias: alias,
			}, []TaskRequest{{
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
	}
	return NavigateResult{Kind: NavigateKindNoop}, nil
}
