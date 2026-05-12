// qa_clients_ready_flash_gen_test.go — adapter-level regression for the
// `handleClientsReady` flash.gen gate, covering the CXR/Architect Stage 5
// R3+R4 finding on `internal/tui/app_session.go`.
//
// The R2 rework introduced a broad gate (`len(intents) > 0 ||
// len(tasks) > 0`) that bumped `m.flash.gen` on any non-stale Core result.
// That silently invalidated any in-flight `ClearFlashMsg` for the current
// flash because the normal success path returns non-flash tasks
// (`FetchIdentity`, `LoadAvailCache`) even when no FlashIntent or
// FlashTickPayload is emitted.
//
// R3 narrowed the gate to `hasFlashWork(intents, tasks)`. These tests
// exercise the adapter directly (via `m.Update(ClientsReadyMsg)`) and
// fail if the gate is reverted to anything broader than "FlashIntent in
// intents OR FlashTickPayload in tasks".
package unit

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// TestHandleClientsReady_SuccessNoPendingRefresh_FlashGenUnchanged
// exercises the regression that the R3 fix addresses: a non-stale
// ClientsReadyMsg success path with `PendingRefresh=false` triggers
// `FetchIdentity` + `LoadAvailCache` tasks but no flash work — the
// adapter must not advance `m.flash.gen` because any in-flight
// `ClearFlashMsg` for the current flash would otherwise stop matching.
//
// Reverting `internal/tui/app_session.go` to the prior broad gate
// (`len(intents) > 0 || len(tasks) > 0`) makes this test fail because
// the success path emits two non-flash tasks.
func TestHandleClientsReady_SuccessNoPendingRefresh_FlashGenUnchanged(t *testing.T) {
	m := newRootSizedModel()

	// Establish a non-zero baseline flash.gen via a normal FlashMsg.
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
		Gen:     m.Session.ConnectGen, // matches session → non-stale
	})

	if got := m.FlashGen(); got != genBefore {
		t.Errorf("handleClientsReady advanced flash.gen on success-no-pending-refresh path: was %d, now %d — broad-gate regression would silently invalidate any in-flight ClearFlashMsg for the current flash", genBefore, got)
	}
}

// TestHandleClientsReady_StaleGen_FlashGenUnchanged pins the symmetric
// stale-gen invariant: when `msg.Gen` does not match the session's
// `ConnectGen`, Core returns (nil, nil) and the adapter must leave
// `flash.gen` alone. The broad gate would also pass this test, but
// keeping the assertion makes the invariant explicit at the adapter
// level so future changes that only tweak the success path cannot
// inadvertently regress the stale path.
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
		Gen:     m.Session.ConnectGen + 5,
	})

	if got := m.FlashGen(); got != genBefore {
		t.Errorf("handleClientsReady advanced flash.gen on stale-gen path: was %d, now %d", genBefore, got)
	}
}
