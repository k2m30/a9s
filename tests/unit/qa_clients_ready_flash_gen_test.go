// The `handleClientsReady` flash.gen
// gate on `internal/tui/app_session.go`.
//
// The gate is `hasFlashWork(intents, tasks)`: `m.flash.gen` is bumped only
// when a FlashIntent is in intents OR a FlashTickPayload is in tasks. A
// broader gate (`len(intents) > 0 || len(tasks) > 0`) would silently
// invalidate any in-flight `ClearFlashMsg` for the current flash, because
// the normal success path returns non-flash tasks (`FetchIdentity`,
// `LoadAvailCache`) even when no flash work is emitted. These tests exercise
// the adapter directly (via `m.Update(ClientsReadyMsg)`).
package unit

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestHandleClientsReady_SuccessNoPendingRefresh_FlashGenUnchanged: a
// non-stale ClientsReadyMsg success path with `PendingRefresh=false` triggers
// `FetchIdentity` + `LoadAvailCache` tasks but no flash work, so the adapter
// must not advance `m.flash.gen` — an in-flight `ClearFlashMsg` for the
// current flash would otherwise stop matching.
func TestHandleClientsReady_SuccessNoPendingRefresh_FlashGenUnchanged(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "first flash"})
	genBefore := m.FlashGen()
	if genBefore == 0 {
		t.Fatalf("baseline flash.gen should be >0 after FlashMsg, got %d", genBefore)
	}

	// Dispatch a non-stale success ClientsReadyMsg. PendingRefresh stays
	// false (the session default), so Core returns only FetchIdentity +
	// LoadAvailCache tasks with no FlashIntent / FlashTickPayload.
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Clients: &awsclient.ServiceClients{},
		Region:  "us-east-1",
		Gen:     m.Core().Session().ConnectGen, // matches session → non-stale
	})

	if got := m.FlashGen(); got != genBefore {
		t.Errorf("handleClientsReady advanced flash.gen on success-no-pending-refresh path: was %d, now %d — broad-gate regression would silently invalidate any in-flight ClearFlashMsg for the current flash", genBefore, got)
	}
}

// TestHandleClientsReady_StaleGen_FlashGenUnchanged: when `msg.Gen` does not
// match the session's `ConnectGen`, Core returns (nil, nil) and the adapter
// leaves `flash.gen` alone.
func TestHandleClientsReady_StaleGen_FlashGenUnchanged(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "first flash"})
	genBefore := m.FlashGen()
	if genBefore == 0 {
		t.Fatalf("baseline flash.gen should be >0 after FlashMsg, got %d", genBefore)
	}

	// Dispatch with Gen far ahead of ConnectGen so the stale guard fires.
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Clients: &awsclient.ServiceClients{},
		Region:  "us-east-1",
		Gen:     m.Core().Session().ConnectGen + 5,
	})

	if got := m.FlashGen(); got != genBefore {
		t.Errorf("handleClientsReady advanced flash.gen on stale-gen path: was %d, now %d", genBefore, got)
	}
}
