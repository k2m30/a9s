// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package messages defines the platform-agnostic command and event taxonomy
// shared by the app core and the renderer adapter.
//
// Commands (Cmd) flow UI → app core: "do this". Events (Event) flow app core
// → adapter: "this happened". GenStamped is implemented by every Event that
// carries a dispatch generation counter; the type system therefore prevents a
// handler from forgetting to gen-check, because the Gen value is reachable
// through the interface without knowing the concrete event type.
//
// Renderer adapters translate between concrete Bubble Tea (or future Electron
// IPC, Wails, etc.) values and these types at the boundary. The shared core
// only ever sees Cmd / Event values.
package messages

import "github.com/k2m30/a9s/v3/core/domain"

// Cmd is the marker interface every UI-originated command implements.
type Cmd interface{ isCmd() }

// Event is the marker interface every core-originated event implements.
type Event interface{ isEvent() }

// Aspect identifies which session-wide generation counter a GenStamped event
// is stamped against. GenSource maps each Aspect to the concrete field on the
// session, so the central guard can resolve the correct counter without knowing
// the concrete session type.
type Aspect uint8

const (
	AspectInvalid      Aspect = iota
	AspectAvailability        // session.AvailabilityGen
	AspectEnrichment          // session.EnrichmentGen (Wave 2 batch enrichment)
	AspectConnect             // session.ConnectGen
	AspectDetailOp            // session.DetailOpGen (core/runtime.DetailOperation lifecycle)
)

// GenSource is implemented by the session (core/session.Session) and
// exposes current generation counters without the messages package importing
// the session package (which would create a cycle).
type GenSource interface {
	CurrentGenFor(Aspect) domain.Gen
}

// GenStamped is implemented by every Event that carries a dispatch generation
// counter. The three-method interface lets the central guard in Core.HandleEvent
// discard stale events without a type switch — the guard reads GenStamp(),
// GenAspect(), and AcceptZeroGen() through the interface.
//
// Implementing all three methods is enforced structurally: a GenStamped event
// that is missing GenAspect or AcceptZeroGen will not compile.
type GenStamped interface {
	Event
	GenStamp() domain.Gen
	// GenAspect returns the Aspect that identifies which session counter this
	// event is stamped against.
	GenAspect() Aspect
	// AcceptZeroGen returns true when a zero GenStamp should NOT be treated as
	// stale.
	//
	// The actual contract: every session generation counter
	// (core/session.Session.New seeds ConnectGen/AvailabilityGen/
	// EnrichmentGen/DetailOpGen at 1) starts nonzero, and every per-operation
	// OperationID is minted by domain.Gen.Bump(), which can never return 0
	// either — so in PRODUCTION, a real dispatch never carries Gen 0. Zero
	// only ever originates from a synthetic/unstamped construction (most
	// commonly a test or demo helper that builds an event literal without
	// setting the Gen/OperationID field).
	//
	// Most events return true: accepting zero costs nothing in production
	// (it can never arrive from a live dispatch) and lets those synthetic
	// callers skip round-tripping a live session gen — a deliberate
	// test-construction affordance, not a claim that zero is ever a
	// legitimate current value. AvailabilityChecked and AvailabilityPrefetched
	// return false instead: a mid-sweep rebuild can legitimately re-emit a
	// message stamped with a pre-rotation gen, and rejecting zero alongside
	// every other stale value closes that specific hazard (see
	// AvailabilityPrefetched's own doc comment) — an exception grounded in a
	// real production race, not a convenience choice, so treat it as the
	// template for any future false-returning event.
	AcceptZeroGen() bool
}

// IsStale reports whether ev should be discarded because its generation stamp
// no longer matches the session. It is the single staleness check used by both
// the central guard in Core.HandleEvent and any adapter-level guards that need
// the same logic.
func IsStale(ev GenStamped, src GenSource) bool {
	stamp := ev.GenStamp()
	if stamp == 0 && ev.AcceptZeroGen() {
		return false
	}
	return stamp != src.CurrentGenFor(ev.GenAspect())
}
