package runtime

import (
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// Event is the typed marker interface for inputs the runtime accepts from an
// adapter — an alias for messages.Event (the runtime → UI event family, each
// implementing isEvent()). Tying HandleEvent to this interface instead of an
// empty `any` gives compile-time assurance that only event types reach the
// orchestrator; adapters translate their native messages into a messages.Event
// before dispatching.
type Event = messages.Event

// Core is the platform-agnostic app-core orchestrator. It owns session
// state and the catalog of resource types, dispatches inbound events to
// the appropriate handler, and returns a list of UI intents plus task
// requests that adapters apply to their renderer-specific state.
type Core struct {
	session *session.Session
	types   []catalog.ResourceTypeDef
	isDemo  bool

	// saveColumns resolves the list column set a resource short name persists
	// under (task #17 wave 1 stage 4: single materializer for both the
	// list-open and sweep save lanes). Set once via SetSaveColumns by the
	// renderer-neutral Controller constructor (app.New), which is
	// view-config-aware; nil means no renderer has registered one yet (e.g. a
	// bare Core built directly in a runtime-package test), in which case
	// SaveTypeRows falls back to resolveSaveColumns' built-in-defaults-only
	// cascade.
	saveColumns func(shortName string) []config.ListColumn
}

// New constructs a Core bound to the given session and catalog snapshot.
// The catalog slice is borrowed, not copied — callers must treat
// the installed catalog as immutable for the lifetime of the Core, which
// matches the existing static-catalog contract.
func New(s *session.Session, types []catalog.ResourceTypeDef) *Core {
	return &Core{session: s, types: types}
}

// SetIsDemo sets the demo-mode flag. Demo mode skips Wave-2 enrichment probes
// that require real AWS credentials against synthetic fakes.
func (c *Core) SetIsDemo(v bool) { c.isDemo = v }

// IsDemo reports whether the runtime is operating in demo mode.
func (c *Core) IsDemo() bool { return c.isDemo }

// Session returns the runtime-owned session handle. Adapters need this
// during the migration to read state the per-handler PRs have not yet
// migrated; the field becomes private-only once handler moves complete.
func (c *Core) Session() *session.Session { return c.session }

// Types returns the catalog snapshot the Core was constructed with.
func (c *Core) Types() []catalog.ResourceTypeDef { return c.types }

// SetSaveColumns registers the view-config-aware column resolver SaveTypeRows
// uses to materialize Path-backed columns before persisting a type's rows.
// Called once by app.New so a user's per-session column overrides are
// honored at save time, matching what the render path already does via
// resolveListColumnsForBuild.
func (c *Core) SetSaveColumns(fn func(shortName string) []config.ListColumn) {
	c.saveColumns = fn
}

// HandleEvent is the single entry point adapters call to deliver an
// inbound event. It returns the UI intents to apply and the background
// tasks to start.
//
// Unrecognised event types fall through to the nil, nil default.
//
// Messages NOT wired here (skipped — double-dispatch risk):
//
//	messages.ResourcesLoaded      — TUI shim handleResourcesLoaded calls
//	                                Core.HandleResourcesLoaded directly after
//	                                adapter-side derive + updateActiveView.
//	messages.RelatedCheckResult   — TUI shim handleRelatedCheckResult resolves
//	                                sourceID from the active detail view before
//	                                calling Core.HandleRelatedCheckResult.
//	messages.EnrichDetailResult   — TUI shim handleEnrichDetailResult does
//	                                adapter-side staleness drop and derive
//	                                before calling Core.HandleEnrichDetailResult.
//	messages.ValueRevealed        — TUI shim handleValueRevealed requires the
//	                                adapter's flash.gen for staleness surface.
//	messages.ClientsReady         — TUI shim handleClientsReady passes
//	                                StackDepth and HasActiveRL (renderer state)
//	                                into Core.HandleClientsReady.
//	messages.Flash                — TUI shim handleFlash bumps flash.gen before
//	                                calling Core.HandleFlash.
//	messages.ClearFlash           — TUI shim handleClearFlash passes flash.gen
//	                                and flash.isError (adapter state) into Core.
//	messages.ThemeFileRead        — TUI shim handleThemeFileRead runs
//	                                styles.ThemeFromYAML (renderer package) to
//	                                produce ParseErr before calling Core.
//	profilesLoadedMsg             — TUI-private type; no messages.* counterpart.
func (c *Core) HandleEvent(ev Event) ([]UIIntent, []TaskRequest) {
	if g, ok := ev.(messages.GenStamped); ok && messages.IsStale(g, c.session) {
		return nil, nil
	}
	switch msg := ev.(type) {
	case messages.AvailabilityCacheLoaded:
		return c.handleAvailabilityCacheLoaded(msg)
	case messages.AvailabilityPrefetched:
		return c.handleAvailabilityPrefetched(msg)
	case messages.AvailabilityChecked:
		return c.handleAvailabilityChecked(msg)
	case messages.EnrichmentChecked:
		return c.handleEnrichmentChecked(msg)
	case messages.ResourcesLoaded:
		// Row-store dual-write ONLY (task #17 wave 1): this case must never
		// return the intents/tasks HandleResourcesLoaded computes — the TUI
		// adapter calls Core.HandleResourcesLoaded directly (bypassing
		// HandleEvent entirely, see runtime_adapter_resources.go) and
		// Controller.Handle (internal/app/handle.go) already runs its own,
		// separate ResourcesLoaded pipeline (handleResourcesLoadedEvent /
		// applyResourcesLoaded). Applying HandleResourcesLoaded's intents here
		// too would double-apply PatchResourceCache/ClearFlash for every
		// Controller.Handle caller (web/headless/tests) — a real internal/app
		// behavior change this stage must not make. Feed RowStore the same
		// canonicalization + Fetch-origin write HandleResourcesLoaded performs,
		// then return nil, nil exactly like the pre-existing default case.
		c.observeResourcesLoadedRows(msg)
		return nil, nil
	case messages.RelatedCheckResult:
		// Row-store dual-write ONLY — same double-dispatch hazard as
		// ResourcesLoaded above (Controller.Handle applies its own
		// PatchRelatedCache/PatchResourceCache/PatchLazyResourceCache intents
		// for this message via a path outside HandleEvent).
		c.observeRelatedCheckResultRows(msg)
		return nil, nil
	case messages.IdentityLoaded:
		return c.HandleIdentityLoaded(IdentityLoadedEvent{Identity: msg.Identity})
	case messages.IdentityError:
		return c.HandleIdentityError(IdentityErrorEvent{Err: msg.Err})
	case messages.APIError:
		// DEF-5/C4: a headless/web caller feeding a failed KindFetchResources
		// execution's messages.APIError straight through HandleEvent (rather
		// than the TUI shim's bump-then-call path) must still get the
		// classification + ClearActiveListLoadingIntent(Err) intent — a web
		// caller has no adapter-owned flash.gen, so ConnectGen serves as the
		// stable stand-in (same pattern Controller.Handle already uses for
		// this event). The FlashTick task is dropped: it is only meaningful
		// to a running event loop (TUI/web timer), and a headless/orchestrator
		// caller has no loop to process it.
		intents, _ := c.HandleAPIError(APIErrorEvent{Err: msg.Err, NewGen: c.session.ConnectGen})
		return intents, nil
	}
	return nil, nil
}
