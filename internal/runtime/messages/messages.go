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

import "github.com/k2m30/a9s/v3/internal/domain"

// Cmd is the marker interface every UI-originated command implements.
type Cmd interface{ isCmd() }

// Event is the marker interface every core-originated event implements.
type Event interface{ isEvent() }

// GenStamped is implemented by every Event that carries a dispatch generation
// counter. A handler that switches on Event can read GenStamp() through the
// interface and discard stale results without knowing the concrete type. This
// closes the bug class where a handler forgets to gen-check a result delivered
// after a profile/region switch.
type GenStamped interface {
	Event
	GenStamp() domain.Gen
}
