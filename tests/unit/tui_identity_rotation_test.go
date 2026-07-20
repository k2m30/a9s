package unit

import (
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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
