package runtime

import (
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
//	messages.APIError             — TUI shim handleAPIError bumps flash.gen
//	                                before calling Core.HandleAPIError.
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
	case messages.IdentityLoaded:
		return c.HandleIdentityLoaded(IdentityLoadedEvent{Identity: msg.Identity})
	case messages.IdentityError:
		return c.HandleIdentityError(IdentityErrorEvent{Err: msg.Err})
	}
	return nil, nil
}
