package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// TestRoot_IdentityScreen_StaleARN_ClearedAfterProfileRotation is a Codex
// round-3 P2 finding: ClearIdentityIntent (emitted by
// HandleProfileSelected/HandleRegionSelected on rotation) is forwarded by the
// TUI's applyIntent (internal/tui/runtime_adapter.go, ClearIdentityIntent
// case) to the CONTROLLER only. The identity SCREEN's own renderer state
// (rs.identityData/rs.identityLoading on the rendererState stack, populated
// via runtime.SetIdentityIntent's loop over m.stack in app_dispatch.go's
// applyIntents) is never reset — nothing clears it on a profile/region
// switch.
//
// Real flow driven here (confirmed empirically — see the stack-shape
// t.Logf below): press 'i' (pushes rsKindIdentity, loading=true, schedules a
// fetch) -> messages.IdentityLoaded lands while that rs is still on the
// stack (populates rs.identityData via the live SetIdentityIntent path,
// exactly as TestRoot_IdentityLoaded_UpdatesHeader/text_ports_test.go's
// TestPort_IdentityCopy_CopiesExactARN do) -> the ':' key is a GLOBAL key in
// app_input.go's key router (no rs.kind guard, unlike Identity/Help/ErrorLog
// which only special-case their OWN key), so colon-command mode works from
// the identity screen -> "profile"/"ctx" resolves to
// messages.Navigate{Target: messages.TargetProfile} (app_input.go's
// executeCommand), pushing the profile selector ON TOP of the still-present
// identity rs -> messages.ProfileSelected{Profile: ...} is exactly what the
// selector's Enter-confirm emits (core/runtime/messages/cmd.go), routed to
// Model.handleProfileSelected -> Core.HandleProfileSelected ->
// dispatchHandlerResult -> applyIntent per intent, including the buggy
// ClearIdentityIntent forward -> PopSelectorIntent (also emitted) pops the
// selector screen (it only pops a profile/region/theme selector — core/app/
// intents.go), REVEALING the identity screen again, still carrying the
// stale, pre-switch ARN.
func TestRoot_IdentityScreen_StaleARN_ClearedAfterProfileRotation(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("i"))

	staleARN := "arn:aws:iam::123456789012:user/stale-preswitch-operator"
	m, _ = rootApplyMsg(m, messages.IdentityLoaded{
		Identity: &awsclient.CallerIdentity{
			AccountID: "123456789012",
			Arn:       staleARN,
			UserName:  "stale-preswitch-operator",
		},
		Gen: 0,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, staleARN) {
		t.Fatalf("precondition: identity screen must show the loaded ARN before rotation, got:\n%s", plain)
	}

	// Open the profile selector on top of the identity screen via the same
	// colon-command flow a real user takes (':' is a global key even while
	// on the identity screen — app_input.go).
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetProfile})
	t.Logf("after Navigate(TargetProfile) while on identity screen, view contains 'identity'=%v 'profile'=%v",
		strings.Contains(strings.ToLower(stripANSI(rootViewContent(m))), "identity"),
		strings.Contains(strings.ToLower(stripANSI(rootViewContent(m))), "profile"))

	// Confirm a different profile — the exact message the selector's Enter
	// key emits (core/runtime/messages/cmd.go's ProfileSelected).
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "other-profile"})

	afterPlain := strings.ToLower(stripANSI(rootViewContent(m)))
	t.Logf("after ProfileSelected, view contains 'identity'=%v (stack should have popped the selector, revealing identity)",
		strings.Contains(afterPlain, "identity"))

	if strings.Contains(rootViewContent(m), staleARN) {
		t.Errorf("identity screen must NOT show the stale pre-switch ARN %q after a profile rotation "+
			"(a loading placeholder or blank is fine — the old ARN must be cleared); got:\n%s",
			staleARN, stripANSI(rootViewContent(m)))
	}
}

// wave5CollectCmdMsgs recursively executes cmd, flattening any nested
// tea.BatchMsg into the flat list of leaf tea.Msg values every terminal cmd
// in the tree produced. Nothing is fed back into any model — this is a pure
// structural inspection of what a dispatch WOULD deliver, used to assert on
// the shape of a cmd tree without executing side-effecting cmds (like a real
// AWS connect) any further than calling them once each.
func wave5CollectCmdMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var all []tea.Msg
		for _, c := range batch {
			all = append(all, wave5CollectCmdMsgs(c)...)
		}
		return all
	}
	return []tea.Msg{msg}
}

// TestRoot_ProfileRotation_DispatchesNoIdentityFetchBeforeReconnect is a
// Codex round-4 P2 finding, reworked into a structural contract test after
// the coder's fix landed: internal/tui/runtime_adapter.go's
// ClearIdentityIntent case used to return m.fetchIdentity(m.core.ConnectGen())
// IMMEDIATELY on rotation. But Session.Rotate() (core/session/session.go)
// deliberately keeps the OLD clients until a ClientsReady for the new
// profile lands (core/runtime/handlers.go's ClientsReady/"Live AWS path"
// always dispatches its OWN TaskKindFetchIdentity afterward with the NEW
// clients) — so that premature fetch resolved the OUTGOING profile's
// identity via the OLD clients, yet stamped it with the NEW ConnectGen,
// getting accepted as fresh once IdentityLoaded landed.
//
// A content-based "stale ARN absent" assertion cannot distinguish stale from
// fresh here: the demo STS fake returns the SAME identity for every profile,
// so the legitimate POST-connect refetch shows an identical ARN too. The
// only thing that actually distinguishes correct from broken is WHICH cmds
// a profile rotation dispatches — so this asserts the structural contract
// directly: messages.ProfileSelected's cmd tree must never contain an
// IdentityLoaded message. messages.ClientsReady (from the Connect task) and
// the flash tick are expected and are only inspected, never delivered back
// into the model (delivering ClientsReady would itself trigger the
// legitimate follow-up identity fetch, muddying what this test isolates).
func TestRoot_ProfileRotation_DispatchesNoIdentityFetchBeforeReconnect(t *testing.T) {
	clients := demo.NewServiceClients()
	tui.Version = "test"
	m := tui.New("test-profile", "us-east-1", tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: clients, Region: "us-east-1", Gen: 0})

	_, cmd := rootApplyMsg(m, messages.ProfileSelected{Profile: "other-profile"})
	if cmd == nil {
		t.Fatal("ProfileSelected must return a non-nil cmd")
	}

	msgs := wave5CollectCmdMsgs(cmd)
	if len(msgs) == 0 {
		t.Fatal("ProfileSelected's cmd tree produced no messages at all — harness did not actually exercise the dispatch")
	}
	t.Logf("ProfileSelected dispatched %d message(s): %#v", len(msgs), msgs)

	for _, got := range msgs {
		if _, ok := got.(messages.IdentityLoaded); ok {
			t.Errorf("ProfileSelected's dispatched cmds must NOT fetch identity before the reconnect lands "+
				"(identity may only arrive as a follow-up of ClientsReady with the NEW clients); "+
				"got messages.IdentityLoaded among the dispatched messages: %#v", msgs)
		}
	}
}
