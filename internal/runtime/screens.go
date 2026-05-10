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
package runtime

import "github.com/k2m30/a9s/v3/internal/domain"

// ScreenID is the stable identifier for a registered screen. Adapters use
// it to look up the renderer-specific builder; the shared core never
// constructs renderer types itself.
type ScreenID string

// ScreenContext is the input handed to an adapter when the runtime asks
// it to push, replace, or otherwise materialize a screen. Renderer-free.
type ScreenContext struct {
	ResourceType string
	ResourceID   string
	Capability   domain.CapabilityID
	// Query is the zero value when the screen is not query-driven.
	Query domain.QuerySpec
}

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
