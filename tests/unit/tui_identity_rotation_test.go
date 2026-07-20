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

// identityFetchingPlaceholder is the exact loading string renderIdentity's
// live path shows (views/identity.go's renderLoading, reached whenever
// rs.identityLoading is true — internal/tui/renderer.go's renderIdentity).
const identityFetchingPlaceholder = "Fetching identity..."

// TestRoot_NoCacheRotation_IdentityScreenNotStuckLoading is a Codex round-5
// P2 finding, the last in this identity-rotation chain: core/runtime/
// handlers.go's no-cache success branch (handleClientsReadySuccess, "Demo /
// no-cache: synchronous prefetch instead of the async probe pipeline.
// Identity fetch is skipped in this mode (synthetic creds).") never
// dispatches TaskKindFetchIdentity. The ClearIdentityIntent case (internal/
// tui/runtime_adapter.go) sets rs.identityLoading=true on rotation
// expecting the reconnect's OWN identity fetch to resolve it — true on the
// live path (handlers.go dispatches TaskKindFetchIdentity itself there) but
// never true in --no-cache/demo mode, so a rotation while the identity
// screen is open leaves it showing "Fetching identity..." forever.
//
// Cheapest no-cache arrangement: tui.WithNoCache(true) (internal/tui/
// app_options.go's WithNoCache calls m.core.SetNoCache, the same session.
// NoCache flag handlers.go's "if s.NoCache" branches on) — the same option
// already used by the sibling tests above and by text_ports_test.go's reveal
// tests, paired with tui.WithClients so Init() doesn't need a live connect.
//
// The rotation's OWN Connect task cmd fails locally for the bogus
// "other-profile" (SharedConfigProfileNotExistError — no such profile in
// ~/.aws/config), so its raw messages.ClientsReady carries a real Err and
// would take the error branch, never reaching the no-cache success path
// this test targets. A successful reconnect is modeled instead by reusing
// that message's own Region/Gen (so the ConnectGen staleness check still
// matches) with Err cleared and the same demo Clients reinstalled — this is
// the "no-cache ClientsReady" a genuinely successful rotation would deliver.
func TestRoot_NoCacheRotation_IdentityScreenNotStuckLoading(t *testing.T) {
	clients := demo.NewServiceClients()
	tui.Version = "test"
	m := tui.New("test-profile", "us-east-1", tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: clients, Region: "us-east-1", Gen: 0})

	m, cmd := rootApplyMsg(m, rootKeyPress("i"))
	if cmd == nil {
		t.Fatal("pressing 'i' with clients ready must return a non-nil fetchIdentity cmd")
	}
	loadedMsg := cmd()
	loaded, ok := loadedMsg.(messages.IdentityLoaded)
	if !ok || loaded.Identity == nil {
		t.Fatalf("initial fetchIdentity did not produce messages.IdentityLoaded with a non-nil Identity (cannot reproduce via this harness): got %T: %+v", loadedMsg, loadedMsg)
	}
	loadedIdentity, ok := loaded.Identity.(*awsclient.CallerIdentity)
	if !ok {
		t.Fatalf("messages.IdentityLoaded.Identity is not a *awsclient.CallerIdentity: got %T", loaded.Identity)
	}
	staleARN := loadedIdentity.Arn
	if staleARN == "" {
		t.Fatal("precondition: demo STS transport returned an empty ARN — cannot pin staleness against an empty string")
	}
	m, _ = rootApplyMsg(m, loaded)

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, staleARN) {
		t.Fatalf("precondition: identity screen must show the fetched ARN %q before rotation, got:\n%s", staleARN, plain)
	}

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetProfile})
	m, cmd = rootApplyMsg(m, messages.ProfileSelected{Profile: "other-profile"})
	if cmd == nil {
		t.Fatal("ProfileSelected must return a non-nil cmd")
	}

	var rawReady *messages.ClientsReady
	for _, got := range wave5CollectCmdMsgs(cmd) {
		if cr, ok := got.(messages.ClientsReady); ok {
			rawReady = &cr
			break
		}
	}
	if rawReady == nil {
		t.Fatal("ProfileSelected's cmd tree produced no messages.ClientsReady to derive a successful reconnect from")
	}
	// Reinstall the same demo clients and clear Err — the local connect
	// attempt fails (no such profile), but the Region/Gen the framework
	// itself computed are reused so the no-cache success path is entered
	// under the exact ConnectGen the rotation armed.
	noCacheReady := messages.ClientsReady{Clients: clients, Region: rawReady.Region, Gen: rawReady.Gen}
	m, _ = rootApplyMsg(m, noCacheReady)

	final := stripANSI(rootViewContent(m))
	if strings.Contains(final, staleARN) {
		t.Errorf("identity screen still shows the stale pre-switch ARN %q after the no-cache reconnect settled; got:\n%s", staleARN, final)
	}
	if strings.Contains(final, identityFetchingPlaceholder) {
		t.Errorf("identity screen is stuck on the %q placeholder after the no-cache reconnect settled — "+
			"no-cache/demo mode never dispatches TaskKindFetchIdentity (core/runtime/handlers.go), so nothing "+
			"ever resolves the loading state ClearIdentityIntent set; got:\n%s", identityFetchingPlaceholder, final)
	}
}
