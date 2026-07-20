// qa_console_open_test.go — key wiring for the "o = open in AWS console /
// O = copy console URL" feature (spec: console-url-spec.md, "Behavior
// contract (TUI)"). Follows the established Copy-key ('c') wiring pattern in
// tui_wiring_test.go and the filter/command-mode suppression pattern in
// qa_filtering_test.go / qa_mainmenu_nav_test.go.
//
// Demo mode is used throughout for the "o" (open) path so no real process is
// ever exec'd: per spec, "o" in demo mode returns the disabled-link Flash
// directly instead of calling openBrowserCmd. "O" (copy) is exercised the
// same tolerant way tui_wiring_test.go already exercises 'c' — clipboard
// access may fail in a headless CI environment, and that is an accepted,
// logged outcome, not a test failure.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// consoleDemoDisabledFlashText is the exact spec-mandated flash text
// (console-url-spec.md, "Behavior contract (TUI)": `o` -> Flash "demo mode
// — console link disabled (O still copies)"). Kept as the literal contract
// string — if the implementation's wording ever drifts from this, that is a
// real regression this test must catch, not something to paper over here.
const consoleDemoDisabledFlashText = "demo mode — console link disabled (O still copies)"

// newDemoConsoleModel builds a sized, demo-mode root model with clients
// wired, mirroring demo_app_test.go's TestDemoMode_* constructor pattern.
// The window is intentionally wide (200 cols) so the flash-text truncation
// in app_view.go's header render (maxFlash = width-40) never kicks in —
// consoleDemoDisabledFlashText is 50 runes and would otherwise be clipped
// with a trailing "..." at the default 80-column width, making an exact
// Contains() assertion width-dependent instead of contract-dependent.
func newDemoConsoleModel() tui.Model {
	m := tui.New(demo.DemoProfile, demo.DemoRegion,
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-0abc123", Name: "web-server", Fields: map[string]string{"instance_id": "i-0abc123", "state": "running"}},
			{ID: "i-0def456", Name: "worker", Fields: map[string]string{"instance_id": "i-0def456", "state": "running"}},
		},
	})
	return m
}

// ─── "o" on a resource list, demo mode ───────────────────────────────────────

func TestConsoleOpen_ResourceList_DemoMode_FlashesDisabledMessage(t *testing.T) {
	m := newDemoConsoleModel()
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

// ─── "O" on a resource list — copy path (tolerant of clipboard failure, same
// convention as TestWiring_CopyInResourceList_ReturnsFlashMsg) ──────────────

func TestConsoleOpen_ResourceList_UppercaseO_CopiesConsoleURL(t *testing.T) {
	m := newDemoConsoleModel()
	m = loadDemoEC2List(m)

	_, cmd := rootApplyMsg(m, rootKeyPress("O"))
	if cmd == nil {
		t.Fatal("pressing 'O' on a resource list should return a command for the console-URL copy")
	}

	msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
		switch msg.(type) {
		case messages.Flash, messages.Copied:
			return true
		default:
			return false
		}
	})
	switch v := msg.(type) {
	case messages.Flash:
		if v.IsError {
			t.Logf("clipboard copy returned error flash: %s (expected in headless env)", v.Text)
		}
	case messages.Copied:
		if !strings.Contains(v.Content, "console.aws.amazon.com") {
			t.Errorf("Copied.Content should be a console URL, got %q", v.Content)
		}
	default:
		t.Errorf("expected messages.Flash or messages.Copied, got %T", msg)
	}
}

// ─── filter mode suppresses "o" / "O" — they type into the filter ──────────

func TestConsoleOpen_FilterModeActive_OTypesIntoFilterInsteadOfTriggering(t *testing.T) {
	m := newDemoConsoleModel()
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
	m := newDemoConsoleModel()
	m = loadDemoEC2List(m)

	m = typeFilter(m, "O")

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/O") {
		t.Errorf("with filter mode active, 'O' should be typed into the filter (expected '/O' in header), got:\n%s", plain)
	}
}

// ─── command mode suppresses "o" / "O" — they type into the command bar ────

func TestConsoleOpen_CommandModeActive_OTypesIntoCommandBarInsteadOfTriggering(t *testing.T) {
	m := newDemoConsoleModel()
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
	m := newDemoConsoleModel()
	m = loadDemoEC2List(m)

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	m, _ = rootApplyMsg(m, rootKeyPress("O"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":O") {
		t.Errorf("with command mode active, 'O' should be typed into the command bar (expected ':O' in header), got:\n%s", plain)
	}
}

// ─── main menu: no-op, no crash ─────────────────────────────────────────────

func TestConsoleOpen_MainMenu_NoOpNoCrash(t *testing.T) {
	m := newDemoConsoleModel()

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
	m := newDemoConsoleModel()

	before := stripANSI(rootViewContent(m))
	newM, _ := rootApplyMsg(m, rootKeyPress("O"))
	after := stripANSI(rootViewContent(newM))

	if before != after {
		t.Errorf("'O' on the main menu should be a no-op; view changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// ─── detail view: "o"/"O" resolve the detailed resource ────────────────────

func TestConsoleOpen_DetailView_DemoMode_FlashesDisabledMessage(t *testing.T) {
	m := newDemoConsoleModel()
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

func TestConsoleOpen_DetailView_UppercaseO_CopiesConsoleURL(t *testing.T) {
	m := newDemoConsoleModel()
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

	_, cmd := rootApplyMsg(m, rootKeyPress("O"))
	if cmd == nil {
		t.Fatal("pressing 'O' in detail view should return a command for the console-URL copy")
	}

	msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
		switch msg.(type) {
		case messages.Flash, messages.Copied:
			return true
		default:
			return false
		}
	})
	switch v := msg.(type) {
	case messages.Flash:
		if v.IsError {
			t.Logf("clipboard copy returned error flash: %s (expected in headless env)", v.Text)
		}
	case messages.Copied:
		if !strings.Contains(v.Content, "console.aws.amazon.com") {
			t.Errorf("Copied.Content should be a console URL, got %q", v.Content)
		}
	default:
		t.Errorf("expected messages.Flash or messages.Copied, got %T", msg)
	}
}
