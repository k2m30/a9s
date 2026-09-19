// Key wiring for "o" (open in AWS console) and
// "O" (copy console URL).
//
// Demo mode is used for the "o" path so no real process is ever exec'd: in
// demo mode "o" returns the disabled-link Flash instead of calling
// openBrowserCmd.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// consoleDemoDisabledFlashText is the exact flash text `o` shows in demo mode.
const consoleDemoDisabledFlashText = "demo mode — console link disabled (O still copies)"

// newDemoConsoleModel builds a sized, demo-mode root model with clients wired.
// The window is intentionally wide (200 cols) so the flash-text truncation
// in app_view.go's header render (maxFlash = width-40) never kicks in —
// consoleDemoDisabledFlashText is 50 runes and would otherwise be clipped
// with a trailing "..." at the default 80-column width, making an exact
// Contains() assertion width-dependent instead of contract-dependent.
func newDemoConsoleModel(t testing.TB) tui.Model {
	m := newBlessedModel(t, demo.DemoProfile, demo.DemoRegion,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 200, Height: 40})
	m, _ = rootApplyMsg(m, demoClientsReadyMsg())
	return m
}

func loadDemoEC2List(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-0abc123", Name: "web-server", Fields: map[string]string{"instance_id": "i-0abc123", "state": "running"}},
			{ID: "i-0def456", Name: "worker", Fields: map[string]string{"instance_id": "i-0def456", "state": "running"}},
		},
	})
	return m
}

func TestConsoleOpen_ResourceList_DemoMode_FlashesDisabledMessage(t *testing.T) {
	m := newDemoConsoleModel(t)
	m = loadDemoEC2List(m)

	newM, cmd := rootApplyMsg(m, rootKeyPress("o"))
	if cmd == nil {
		t.Fatal("pressing 'o' on a resource list should return a command (the flash auto-clear timer)")
	}

	// The flash text is applied synchronously during Update() (see
	// applyIntent's runtime.FlashIntent case in internal/tui/runtime_adapter.go)
	// — the returned cmd is only the auto-clear tick (resolves to
	// messages.ClearFlash later), not a carrier of the flash text itself. The
	// header render is the one place the text is observable from this package.
	plain := stripANSI(rootViewContent(newM))
	if !strings.Contains(plain, consoleDemoDisabledFlashText) {
		t.Errorf("demo-mode 'o' flash text missing from header; want %q, got:\n%s", consoleDemoDisabledFlashText, plain)
	}
}

// TestConsoleOpen_ResourceList_UppercaseO_CopiesConsoleURL drives the real
// clipboard write and reads it back — handleOpenConsole(copyOnly=true) ->
// copyToClipboard returns messages.Flash, a constant success/failure label,
// never the copied content itself.
func TestConsoleOpen_ResourceList_UppercaseO_CopiesConsoleURL(t *testing.T) {
	m := newDemoConsoleModel(t)
	m = loadDemoEC2List(m)

	got := ReadClipboardAfter(t, func() {
		_, cmd := rootApplyMsg(m, rootKeyPress("O"))
		if cmd == nil {
			t.Fatal("pressing 'O' on a resource list should return a command for the console-URL copy")
		}
		msg := cmd()
		flash, ok := msg.(messages.Flash)
		if !ok {
			t.Fatalf("pressing 'O' should produce messages.Flash, got %T", msg)
		}
		// The copy is captured at the seam, which cannot fail, so an error
		// flash is the copy path reporting a real failure.
		if flash.IsError {
			t.Fatalf("the console-URL copy reported a failure: %s", flash.Text)
		}
	})
	want := "https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-0abc123"
	if got != want {
		t.Errorf("clipboard content after 'O' = %q, want %q", got, want)
	}
}

func TestConsoleOpen_FilterModeActive_OTypesIntoFilterInsteadOfTriggering(t *testing.T) {
	m := newDemoConsoleModel(t)
	m = loadDemoEC2List(m)

	m = typeFilter(m, "o")

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/o") {
		t.Errorf("with filter mode active, 'o' should be typed into the filter (expected '/o' in header), got:\n%s", plain)
	}
	if strings.Contains(plain, consoleDemoDisabledFlashText) {
		t.Error("'o' typed while filtering must not trigger the console-open action")
	}
}

func TestConsoleOpen_FilterModeActive_UppercaseOTypesIntoFilterInsteadOfTriggering(t *testing.T) {
	m := newDemoConsoleModel(t)
	m = loadDemoEC2List(m)

	m = typeFilter(m, "O")

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/O") {
		t.Errorf("with filter mode active, 'O' should be typed into the filter (expected '/O' in header), got:\n%s", plain)
	}
}

func TestConsoleOpen_CommandModeActive_OTypesIntoCommandBarInsteadOfTriggering(t *testing.T) {
	m := newDemoConsoleModel(t)
	m = loadDemoEC2List(m)

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	m, _ = rootApplyMsg(m, rootKeyPress("o"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":o") {
		t.Errorf("with command mode active, 'o' should be typed into the command bar (expected ':o' in header), got:\n%s", plain)
	}
	if strings.Contains(plain, consoleDemoDisabledFlashText) {
		t.Error("'o' typed while in command mode must not trigger the console-open action")
	}
}

func TestConsoleOpen_CommandModeActive_UppercaseOTypesIntoCommandBarInsteadOfTriggering(t *testing.T) {
	m := newDemoConsoleModel(t)
	m = loadDemoEC2List(m)

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	m, _ = rootApplyMsg(m, rootKeyPress("O"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":O") {
		t.Errorf("with command mode active, 'O' should be typed into the command bar (expected ':O' in header), got:\n%s", plain)
	}
}

func TestConsoleOpen_MainMenu_NoOpNoCrash(t *testing.T) {
	m := newDemoConsoleModel(t)

	before := stripANSI(rootViewContent(m))

	newM, cmd := rootApplyMsg(m, rootKeyPress("o"))
	after := stripANSI(rootViewContent(newM))

	if cmd != nil {
		msg := cmd()
		if flash, ok := msg.(messages.Flash); ok && flash.Text == consoleDemoDisabledFlashText {
			t.Error("'o' on the main menu (not a resource screen) should not produce the resource-list disabled-link flash")
		}
	}
	if before != after {
		t.Errorf("'o' on the main menu should be a no-op; view changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestConsoleOpen_MainMenu_UppercaseO_NoOpNoCrash(t *testing.T) {
	m := newDemoConsoleModel(t)

	before := stripANSI(rootViewContent(m))
	newM, _ := rootApplyMsg(m, rootKeyPress("O"))
	after := stripANSI(rootViewContent(newM))

	if before != after {
		t.Errorf("'O' on the main menu should be a no-op; view changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestConsoleOpen_DetailView_DemoMode_FlashesDisabledMessage(t *testing.T) {
	m := newDemoConsoleModel(t)
	res := &resource.Resource{
		ID:     "i-0abc123",
		Name:   "web-server",
		Fields: map[string]string{"instance_id": "i-0abc123"},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     res,
	})

	newM, cmd := rootApplyMsg(m, rootKeyPress("o"))
	if cmd == nil {
		t.Fatal("pressing 'o' in detail view should return a command (the flash auto-clear timer)")
	}

	plain := stripANSI(rootViewContent(newM))
	if !strings.Contains(plain, consoleDemoDisabledFlashText) {
		t.Errorf("demo-mode 'o' flash text missing from header in detail view; want %q, got:\n%s", consoleDemoDisabledFlashText, plain)
	}
}

// TestConsoleOpen_DetailView_UppercaseO_CopiesConsoleURL reads the real
// clipboard back, since copyToClipboard returns only messages.Flash.
func TestConsoleOpen_DetailView_UppercaseO_CopiesConsoleURL(t *testing.T) {
	m := newDemoConsoleModel(t)
	res := &resource.Resource{
		ID:     "i-0abc123",
		Name:   "web-server",
		Fields: map[string]string{"instance_id": "i-0abc123"},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     res,
	})

	got := ReadClipboardAfter(t, func() {
		_, cmd := rootApplyMsg(m, rootKeyPress("O"))
		if cmd == nil {
			t.Fatal("pressing 'O' in detail view should return a command for the console-URL copy")
		}
		msg := cmd()
		flash, ok := msg.(messages.Flash)
		if !ok {
			t.Fatalf("pressing 'O' should produce messages.Flash, got %T", msg)
		}
		// The copy is captured at the seam, which cannot fail, so an error
		// flash is the copy path reporting a real failure.
		if flash.IsError {
			t.Fatalf("the console-URL copy reported a failure: %s", flash.Text)
		}
	})
	want := "https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-0abc123"
	if got != want {
		t.Errorf("clipboard content after 'O' = %q, want %q", got, want)
	}
}

// assertOverlayDismissedByConsoleOpenKey: OpenConsole/CopyConsoleURL are only
// intercepted on resource list (incl. child list) and detail screens. On
// every other screen kind — help, identity, selectors, costs, etc. — "o"/"O"
// reach that screen's own key handling like any other unbound key, so on
// help/identity they close the overlay (app_stack.go's updateActiveRS) and
// return to the main menu.
func assertOverlayDismissedByConsoleOpenKey(t *testing.T, openKey, dismissKey, label string) {
	t.Helper()
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, rootKeyPress(openKey))

	m, cmd := rootApplyMsg(m, rootKeyPress(dismissKey))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("%s: pressing %q should close the overlay (any-key-dismiss), not be silently consumed by the console-open feature; view:\n%s", label, dismissKey, plain)
	}
}

func TestConsoleOpen_HelpOverlay_OKeyClosesHelpInsteadOfBeingConsumed(t *testing.T) {
	assertOverlayDismissedByConsoleOpenKey(t, "?", "o", "help overlay")
}

func TestConsoleOpen_HelpOverlay_UppercaseOKeyClosesHelpInsteadOfBeingConsumed(t *testing.T) {
	assertOverlayDismissedByConsoleOpenKey(t, "?", "O", "help overlay")
}

func TestConsoleOpen_IdentityOverlay_OKeyClosesIdentityInsteadOfBeingConsumed(t *testing.T) {
	assertOverlayDismissedByConsoleOpenKey(t, "i", "o", "identity overlay")
}

func TestConsoleOpen_IdentityOverlay_UppercaseOKeyClosesIdentityInsteadOfBeingConsumed(t *testing.T) {
	assertOverlayDismissedByConsoleOpenKey(t, "i", "O", "identity overlay")
}

// TestConsoleOpen_TUIPathAndHeadlessSnapshot_AgreeOnSameConsoleURL: both
// surfaces resolve the console URL through
// consolelink.Resolve(Controller.ConsoleTarget()). The TUI's Update() loop
// via 'O' and an independently-built headless Controller's
// Snapshot().ConsoleURL, driven against equivalent state, must agree.
//
// 'O' (copy) is used rather than 'o' (open) because 'o' in demo mode
// short-circuits to the disabled-link flash before resolving a URL.
func TestConsoleOpen_TUIPathAndHeadlessSnapshot_AgreeOnSameConsoleURL(t *testing.T) {
	// ApplyResourcesLoaded on a top-level canonical list triggers a real
	// disk-cache save (maybeSaveResourceListCache) — redirect it into a
	// throwaway dir so this test never touches the real ~/.a9s/cache.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	const ec2ID = "i-0abc123"
	ec2Row := resource.Resource{ID: ec2ID, Name: "web-server", Fields: map[string]string{"instance_id": ec2ID, "state": "running"}}
	wantURL := "https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=" + ec2ID

	// Headless side: an independently-built Controller with equivalent state
	// (same profile/region as newDemoConsoleModel, same ec2 row).
	ctrl := newParityHeadlessController(t, demo.DemoProfile, demo.DemoRegion)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	ctrl.ApplyResourcesLoaded("ec2", []resource.Resource{ec2Row}, nil, false)
	headlessURL := ctrl.Snapshot().ConsoleURL
	if headlessURL != wantURL {
		t.Fatalf("setup: headless Snapshot().ConsoleURL = %q, want %q", headlessURL, wantURL)
	}

	m := newDemoConsoleModel(t)
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{ec2Row}})

	// handleOpenConsole(copyOnly=true) -> copyToClipboard always returns
	// messages.Flash (a constant success/failure label, never the copied
	// content — see text_ports_test.go's wave3CopyAndReadClipboard doc
	// comment for the same contract on the 'c' key). The only way to verify
	// what was actually copied is a real clipboard read-back; skip (not
	// fail) when clipboard access is unavailable in this environment.
	got := ReadClipboardAfter(t, func() {
		_, cmd := rootApplyMsg(m, rootKeyPress("O"))
		if cmd == nil {
			t.Fatal("pressing 'O' should return a command for the console-URL copy")
		}
		msg := cmd()
		flash, ok := msg.(messages.Flash)
		if !ok {
			t.Fatalf("pressing 'O' should produce messages.Flash, got %T", msg)
		}
		// Captured at the seam: an error flash is the copy path failing,
		// not a machine without a pasteboard.
		if flash.IsError {
			t.Fatalf("the TUI console-URL copy reported a failure: %s (headless produced %q)", flash.Text, headlessURL)
		}
	})
	if got != wantURL {
		t.Errorf("TUI 'O' copied clipboard content = %q, want %q", got, wantURL)
	}
	if got != headlessURL {
		t.Errorf("TUI 'O' copied %q, headless Snapshot().ConsoleURL = %q — the two console-link surfaces disagree", got, headlessURL)
	}
}
