// costs_nav_test.go — Cost Explorer Phase 2: navigation wiring
// (specs/021-cost-explorer/data-model.md §"Controller & runtime additions",
// last paragraph: executeCommand "costs"/"ce", the synthetic main-menu
// entry, and NavigateKindPushCosts).
//
// Colon-command driving follows tui_root_test.go's TestRootExecuteCommand_*
// convention (":" enters command mode, characters are typed, Enter submits)
// combined with qa_cli_command_flag_test.go's extractMsg/findNavigateMsg
// batch-walking helpers — both already live in this package (unit) and are
// reused here rather than re-implemented.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// newCostsNavController builds a bare headless Controller (menu screen root,
// nothing else pushed) for the main-menu synthetic-entry assertions below.
func newCostsNavController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	return c
}

// typeColonCommand drives the root model through ":" + cmd + Enter, exactly
// like tui_root_test.go's TestRootExecuteCommand_* tests.
func typeColonCommand(m tui.Model, cmd string) (tui.Model, tea.Cmd) {
	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, r := range cmd {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}
	return rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
}

// ---------------------------------------------------------------------------
// executeCommand "costs" / "ce"
// ---------------------------------------------------------------------------

func TestQA_Costs_ColonCommand_Costs_EmitsNavigateTargetCosts(t *testing.T) {
	m := newRootSizedModel()

	_, cmd := typeColonCommand(m, "costs")

	nav := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.Navigate)
		return ok
	})
	navMsg, ok := nav.(messages.Navigate)
	if !ok {
		t.Fatalf("expected messages.Navigate, got %T", nav)
	}
	if navMsg.Target != messages.TargetCosts {
		t.Errorf(":costs command: Navigate.Target got %v want messages.TargetCosts", navMsg.Target)
	}
}

func TestQA_Costs_ColonCommand_CEAlias_EmitsNavigateTargetCosts(t *testing.T) {
	m := newRootSizedModel()

	_, cmd := typeColonCommand(m, "ce")

	nav := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.Navigate)
		return ok
	})
	navMsg, ok := nav.(messages.Navigate)
	if !ok {
		t.Fatalf("expected messages.Navigate, got %T", nav)
	}
	if navMsg.Target != messages.TargetCosts {
		t.Errorf(":ce command: Navigate.Target got %v want messages.TargetCosts", navMsg.Target)
	}
}

// ---------------------------------------------------------------------------
// NavigateKindPushCosts pushes ScreenCosts (+ seeds CostsState via
// EnsureCostsState) — observed through the rendered view, since the runtime
// adapter's push logic is unexported.
// ---------------------------------------------------------------------------

func TestQA_Costs_Navigate_PushesCostsScreen(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Costs") {
		t.Errorf("after navigating to TargetCosts, rendered view should show the costs screen, got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Main-menu synthetic "costs" entry
// ---------------------------------------------------------------------------

func TestQA_Costs_MainMenu_SyntheticEntryPresent(t *testing.T) {
	c := newCostsNavController(t)

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindMenu || vs.Body.Menu == nil {
		t.Fatalf("expected the root screen to be the menu, got Body.Kind=%q", vs.Body.Kind)
	}

	var found *app.MenuEntry
	for i := range vs.Body.Menu.Entries {
		if vs.Body.Menu.Entries[i].ShortName == "costs" {
			found = &vs.Body.Menu.Entries[i]
			break
		}
	}
	if found == nil {
		t.Fatal("main menu is missing the synthetic \"costs\" entry")
	}
	if found.Category != "Cost" {
		t.Errorf("synthetic costs entry Category: got %q want %q", found.Category, "Cost")
	}
}

func TestQA_Costs_SyntheticEntry_AbsentFromAllResourceTypes(t *testing.T) {
	for _, rt := range resource.AllResourceTypes() {
		if rt.ShortName == "costs" {
			t.Fatalf("resource.AllResourceTypes() must NOT contain a real \"costs\" resource type — it is a synthetic main-menu entry, not a fetchable resource type (found: %+v)", rt)
		}
	}
}

// ---------------------------------------------------------------------------
// Main-menu Enter on the synthetic "costs" entry
// ---------------------------------------------------------------------------

func TestQA_Costs_MainMenu_Enter_EmitsNavigateTargetCosts(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)

	// "costs" does not collide with any real resource ShortName/Alias/
	// Display substring, so filtering to it isolates exactly one entry
	// regardless of where the synthetic entry is inserted.
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "costs"})
	c.Apply(app.Action{Kind: app.ActionMoveTop})

	menu := views.NewMainMenu(keys.Default(), c)
	menu.SetSize(80, 24)

	_, cmd := menu.Update(rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter on the synthetic costs entry produced no command")
	}
	msg := cmd()
	navMsg, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("expected messages.Navigate, got %T", msg)
	}
	if navMsg.Target != messages.TargetCosts {
		t.Errorf("main-menu Enter on \"costs\": Navigate.Target got %v want messages.TargetCosts", navMsg.Target)
	}
}
