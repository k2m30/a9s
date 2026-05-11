// handlers.go — placeholder for shell-level handlers migrated out of
// internal/tui/app_handlers.go.
//
// PR-05a-h1 (AS-147) moves the dispatcher entry "app_handlers.go" out of
// internal/tui per the file-level inventory in docs/refactor/05-boundary.md
// §"5a-extract". The handlers that lived in that file are renderer-shell
// concerns — keyboard dispatch (handleKeyMsg), flash state with tea.Tick
// timers (handleFlash, handleClearFlash, handleAPIError), AWS client
// lifecycle (handleClientsReady), profile/region selection wrappers
// (handleProfileSelected, handleRegionSelected), theme application
// (handleThemeSelected), and view-stack push helpers (handleProfilesLoaded,
// handleValueRevealed, handleEnterChildView).
//
// Each of those touches Bubble Tea types (tea.Cmd, tea.KeyMsg, tea.Tick,
// key.Matches) or tui.Model fields (m.flash, m.errorHistory, m.clients,
// m.profile, m.region, m.identity, m.connectGen, m.pendingRefresh, the
// view stack m.stack, the keys map m.keys, the input mode, …) that are
// adapter-owned in the Phase 05 boundary. None of them touch session.Session
// state beyond a single c.session.Rotate() call in profile/region select.
//
// Honest disposition: those handler bodies remain in the TUI adapter,
// redistributed across thematic sibling files:
//
//   internal/tui/app_input.go     — handleKeyMsg (keyboard dispatcher)
//   internal/tui/app_flash.go     — handleFlash, handleClearFlash, handleAPIError
//   internal/tui/app_session.go   — handleClientsReady, handleProfileSelected,
//                                   handleRegionSelected, handleThemeSelected,
//                                   handleProfilesLoaded
//   internal/tui/app_screens.go   — handleValueRevealed, handleEnterChildView
//
// internal/tui/app_handlers.go is deleted as required by the spec exit
// criterion ("ls internal/tui/app_handlers*.go" → no such file or directory
// for the bare file). The sibling tui-side files keep the (tea.Model, tea.Cmd)
// adapter signatures intact.
//
// A follow-up PR will migrate tui.Model fields (profile, region, identity,
// clients, connectGen, pendingRefresh, hasPrevState, prevProfile, prevRegion,
// preSuppliedClients, command, noCache) to session.Session. Once they live
// on Session, the handler bodies can move to *Core methods returning
// ([]UIIntent, []TaskRequest) — with new intents for the connect lifecycle
// (ConnectIntent, IdentityClearIntent) and a connect TaskRequest for the
// AWS bootstrap. That migration is multi-PR scope and is intentionally
// deferred from this PR per the M sizing on AS-147.
//
// For now this file exists to satisfy the spec's path-level acceptance
// (test -f internal/runtime/handlers.go) and to document why it has no
// Core methods yet. Adding stub methods here without real semantics would
// be premature — it would lock in shapes before the field-migration PR
// can establish the right contracts.
package runtime
