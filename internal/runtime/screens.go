// Package runtime owns the platform-agnostic app core: session ownership,
// app-core dispatch, fetcher invocation, selectors, queries, tasks, and
// generation stamping. It MUST NOT import Bubble Tea, Lipgloss, Bubbles, or
// any other renderer toolkit. Renderer adapters (today: internal/tui)
// translate runtime events/intents to and from their native message types.
//
// This file defines the screen-descriptor contract used by adapters to
// render multi-screen workflows (logs, CloudTrail, cost views, …) without
// growing central shell switches. Phase 05 PR-05a-scaffold creates the
// types only; per-handler PRs (AS-72-h1..h8) wire them to live behavior.
// PR-05a-h4-a (AS-769) adds the typed ScreenPayload contract and the
// concrete selector/reveal/child-list payloads used by the four ported
// view-stack-pushing handlers.
package runtime

import "github.com/k2m30/a9s/v3/internal/domain"

// ScreenID is the stable identifier for a registered screen. Adapters use
// it to look up the renderer-specific builder; the shared core never
// constructs renderer types itself.
type ScreenID string

// Screen IDs emitted by PR-05a-h4-a handlers and the headless controller
// (PR-B). Capability screens (logs, ct.scan, cost) reuse the existing
// ScreenContext-only PushScreen path and are not enumerated here.
const (
	ScreenProfileSelector ScreenID = "profile-selector"
	ScreenReveal          ScreenID = "reveal"
	ScreenChildList       ScreenID = "child-list"

	// ScreenHelp, ScreenRegion, ScreenTheme, ScreenIdentity are used by the
	// headless controller's applyNavResult to push navigate-target screens that
	// the TUI adapter constructs directly from key-handling context. Introduced
	// in PR-B so the headless controller has stable ScreenIDs for these screens
	// without depending on renderer-specific view types.
	ScreenHelp     ScreenID = "help"
	ScreenRegion   ScreenID = "region"
	ScreenTheme    ScreenID = "theme"
	ScreenIdentity ScreenID = "identity"

	// ScreenResourceList is the live or cached top-level resource list
	// pushed by NavigateKindPushResourceList / NavigateKindPushResourceListCached.
	// Row data (ListState) is populated in PR-C once the result lane lands.
	ScreenResourceList ScreenID = "resource-list"

	// ScreenMenu is the root main-menu screen. The controller pushes it on
	// startup so Snapshot() returns BodyKindMenu from the first call.
	ScreenMenu ScreenID = "menu"

	// ScreenYAML and ScreenJSON are the YAML/JSON text viewer screens pushed
	// when the user presses y or j on a resource-list or detail row.
	ScreenYAML ScreenID = "yaml"
	ScreenJSON ScreenID = "json"

	// ScreenDetail is the resource key-value detail screen pushed when the
	// user selects a resource from the list (Enter/d). The headless controller
	// uses this ID; the TUI adapter constructs a DetailModel from the
	// associated DetailState.Resource.
	ScreenDetail ScreenID = "detail"

	// ScreenErrorLog is the session error-log text viewer pushed when the user
	// presses '!' and at least one error has been recorded this session.
	// Its body is a TextBody with one line per error entry (newest-first).
	ScreenErrorLog ScreenID = "error-log"
)

// ScreenContext is the input handed to an adapter when the runtime asks
// it to push, replace, or otherwise materialize a screen. Renderer-free.
type ScreenContext struct {
	ResourceType string
	ResourceID   string
	Capability   domain.CapabilityID
	// Query is the zero value when the screen is not query-driven.
	Query domain.QuerySpec
}

// ScreenPayload is the marker interface for typed per-Screen payload
// structs. PushScreen.Payload carries one of these; adapters type-switch
// on the concrete type to recover the payload fields. nil Payload is
// permitted when ScreenContext.Capability alone suffices for routing.
type ScreenPayload interface {
	isScreenPayload()
}

// ProfileSelectorPayload carries the profile list + the currently-active
// profile name for the profile selector screen.
type ProfileSelectorPayload struct {
	Profiles []string
	Current  string
}

func (ProfileSelectorPayload) isScreenPayload() {}

// RevealPayload carries the resource ID and decrypted value for the
// reveal screen (`x` key over a secret-bearing resource).
type RevealPayload struct {
	ResourceID string
	Value      string
}

func (RevealPayload) isScreenPayload() {}

// ChildListPayload carries the child-type short name, the parent context
// map used by the child fetcher, and the human-readable display name
// rendered as the child view's frame title.
type ChildListPayload struct {
	ChildType     string
	ParentContext map[string]string
	DisplayName   string
}

func (ChildListPayload) isScreenPayload() {}

// ScreenDescriptor declaratively describes one registrable screen. Used by
// the adapter to wire its builder map and by the runtime to validate that
// a capability resolves to a known screen.
type ScreenDescriptor struct {
	ID    ScreenID
	Title string
}

// ScreenRegistry is the runtime-side registry of declared screens. The
// renderer adapter owns the parallel map from ScreenID to its concrete
// builder type (e.g. tea.Model factory). Capability dispatch flows
// CapabilityID -> ScreenID -> renderer-specific builder.
type ScreenRegistry interface {
	Register(ScreenDescriptor)
	Get(ScreenID) (ScreenDescriptor, bool)
	ScreenForCapability(domain.CapabilityID) (ScreenID, bool)
}
