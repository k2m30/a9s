// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
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
	// under (single materializer for both the list-open and sweep save
	// lanes). Set once via SetSaveColumns by the
	// renderer-neutral Controller constructor (app.New), which is
	// view-config-aware; nil means no renderer has registered one yet (e.g. a
	// bare Core built directly in a runtime-package test), in which case
	// SaveTypeRows falls back to resolveSaveColumns' built-in-defaults-only
	// cascade.
	saveColumns func(shortName string) []config.ListColumn
}

// New constructs a Core bound to the given session and catalog snapshot.
// The catalog slice is borrowed, not copied — callers must treat
// the installed catalog as immutable for the lifetime of the Core, per the
// static-catalog contract.
func New(s *session.Session, types []catalog.ResourceTypeDef) *Core {
	return &Core{session: s, types: types}
}

// SetIsDemo sets the demo-mode flag. A demo session's resources exist in no
// account, so its console links are not opened.
func (c *Core) SetIsDemo(v bool) { c.isDemo = v }

// IsDemo reports whether the runtime is operating in demo mode.
func (c *Core) IsDemo() bool { return c.isDemo }

// Session returns the runtime-owned session handle.
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
	case messages.RowEnriched:
		return c.handleRowEnriched(msg)
	case messages.EnrichmentChecked:
		return c.handleEnrichmentChecked(msg)
	case messages.ResourcesLoaded:
		// Forwards HandleResourcesLoaded's tasks (the list-open Wave-2 probe),
		// its FlashIntent and its verdict (filterResourcesLoadedIntents) — but
		// NEVER its ClearFlash/PatchResourceCache intents. The TUI adapter
		// calls Core.HandleResourcesLoaded directly (see
		// runtime_adapter_resources.go) and Controller.Handle
		// (core/app/handle.go) runs its own ResourcesLoaded pipeline
		// (handleResourcesLoadedEvent / applyResourcesLoaded); applying those
		// intents here too would double-apply them for every
		// Controller.Handle caller (web/headless/tests). A partial-fetch
		// composite error's FlashIntent has no other producer on this lane:
		// applyResourcesLoaded only installs the screen's own LastFetchError
		// marker, it never flashes. A TaskRequest is never double-applied by
		// Controller.Handle (only applyIntents(intents) is), so the tasks are
		// forwarded unfiltered: this is the one shared producer for the
		// Wave-2 list-open dispatch on the web/headless lane, and the TUI
		// reaches the same producer via its own direct call, so the task is
		// emitted exactly once per lane per list load.
		//
		// core/app.Controller.handleResourcesLoadedEvent owns the RowStore
		// write for every screen state, including a canonical result whose
		// screen is gone: Controller.Handle is the only caller that reaches
		// this case with a ResourcesLoaded, and its pipeline observes the
		// same rows materialized, the shape every reader wants. The
		// supersession question is answered once, by HandleResourcesLoaded,
		// as a ListResultVerdict Controller.Handle stamps onto the message
		// before its own pipeline sees it.
		intents, tasks := c.HandleResourcesLoaded(ResourcesLoadedEvent{
			ResourceType: msg.ResourceType,
			Resources:    msg.Resources,
			Pagination:   msg.Pagination,
			Append:       msg.Append,
			TypeGen:      msg.TypeGen,
			ListSeq:      msg.ListSeq,
			ScreenID:     msg.ScreenID,
			Err:          msg.Err,
			Provenance:   msg.Provenance,
		})
		return filterResourcesLoadedIntents(intents), tasks
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
		// A headless/web caller feeding a failed KindFetchResources
		// execution's messages.APIError straight through HandleEvent (rather
		// than the TUI shim's bump-then-call path) must still get the
		// classification + ClearActiveListLoadingIntent(Err) intent — a web
		// caller has no adapter-owned flash.gen, so ConnectGen serves as the
		// stable stand-in (the same pattern Controller.Handle uses for
		// this event). The FlashTick task is dropped: it is only meaningful
		// to a running event loop (TUI/web timer), and a headless/orchestrator
		// caller has no loop to process it.
		msg = c.StampListFailure(msg)
		intents, _ := c.HandleAPIError(APIErrorEvent{
			Err:          msg.Err,
			NewGen:       c.session.ConnectGen,
			Append:       msg.Append,
			LoadingMore:  msg.LoadingMore,
			ResourceType: msg.ResourceType,
			Provenance:   msg.Provenance,
			ScreenID:     msg.ScreenID,
			ListSeq:      msg.ListSeq,
			Superseded:   msg.Superseded,
		})
		return intents, nil
	}
	return nil, nil
}

// filterResourcesLoadedIntents keeps the FlashIntent and the ListResultVerdict
// entries from HandleResourcesLoaded's intents, dropping every other kind.
// HandleEvent's messages.ResourcesLoaded case uses it to forward the
// partial-fetch-error flash without forwarding the ClearFlash/
// PatchResourceCache intents Controller.Handle's own ResourcesLoaded pipeline
// owns (see that case's comment). The verdict is not applied to anything —
// it is how the canonical short name and the supersession answer reach the
// pipeline that consumes the message (StampListResult).
func filterResourcesLoadedIntents(intents []UIIntent) []UIIntent {
	var out []UIIntent
	for _, in := range intents {
		switch in.(type) {
		case FlashIntent, ListResultVerdict:
			out = append(out, in)
		}
	}
	return out
}
