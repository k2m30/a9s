// text_ctrl_interaction_test.go — controller-driven interaction gate for YAML/JSON screens.
//
// This test is the gate that catches the hollow-view bug: if Update() mutates
// model-local wrap/search/scroll state but never calls ctrl.Apply(), then
// View() reads the stale controller snapshot and the live interactions are
// invisible. This test drives interactions through the controller (the correct
// path) and asserts:
//
//  1. ctrl.Apply(ActionToggleWrap)    → Snapshot().Body.Text.Wrap flips true/false.
//  2. ctrl.Apply(ActionSearch)        → Snapshot().Body.Text.Search is set, SearchMatches populated.
//  3. ctrl.Apply(ActionSearchNext/Prev) → Snapshot().Body.Text.SearchCursor advances/retreats.
//  4. ctrl.Apply(ActionMoveDown/Up)   → Snapshot().Body.Text.ScrollY changes.
//  5. ctrl.Apply(ActionPageDown/Up)   → Snapshot().Body.Text.ScrollY jumps by page.
//  6. RenderText(body) output differs between wrap-off and wrap-on states.
//  7. RenderText(body) output differs between no-search and search-active states.
//  8. RenderText(body) output differs between scroll=0 and scroll=N states.
//
// The test also drives keys through YAMLModel/JSONModel.Update() with ctrl wired
// and asserts the same controller mutations, closing the gap where Update() had
// model-local mutations that never reached the controller.
package unit_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Test infrastructure
// ---------------------------------------------------------------------------

// newTextController builds a Controller with a YAML or JSON screen on the stack.
// It uses ApplyIntents to push the screen (same as the TUI navigator does) and
// calls EnsureTextState with the provided content lines.
func newTextController(screenID runtime.ScreenID, lines []string) *app.Controller {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	ctrl := app.New(core)
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: screenID}})
	ctrl.EnsureTextState(lines)
	return ctrl
}

// textInteractionResource returns a resource with enough fields and lines to
// make wrap/search/scroll interactions observable. The content must be long
// enough that PageDown actually advances ScrollY past 0.
func textInteractionResource() resource.Resource {
	type netBlock struct {
		Protocol string `yaml:"protocol" json:"protocol"`
		Port     int    `yaml:"port"     json:"port"`
	}
	type data struct {
		InstanceID   string   `yaml:"instance_id"   json:"instance_id"`
		InstanceType string   `yaml:"instance_type" json:"instance_type"`
		State        string   `yaml:"state"         json:"state"`
		LaunchTime   string   `yaml:"launch_time"   json:"launch_time"`
		PublicIP     string   `yaml:"public_ip"     json:"public_ip"`
		PrivateIP    string   `yaml:"private_ip"    json:"private_ip"`
		VPCID        string   `yaml:"vpc_id"        json:"vpc_id"`
		SubnetID     string   `yaml:"subnet_id"     json:"subnet_id"`
		Monitoring   bool     `yaml:"monitoring"    json:"monitoring"`
		Tags         []string `yaml:"tags"          json:"tags"`
	}
	return resource.Resource{
		ID:   "i-0abc123def456",
		Name: "test-instance",
		RawStruct: data{
			InstanceID:   "i-0abc123def456",
			InstanceType: "t3.medium",
			State:        "running",
			LaunchTime:   "2024-01-15T10:30:00Z",
			PublicIP:     "203.0.113.42",
			PrivateIP:    "10.0.1.100",
			VPCID:        "vpc-0abc12345",
			SubnetID:     "subnet-0def67890",
			Monitoring:   true,
			Tags:         []string{"prod", "backend"},
		},
	}
}

// textSnapshot returns Snapshot().Body.Text, failing the test if nil.
func textSnapshot(t *testing.T, ctrl *app.Controller) *app.TextBody {
	t.Helper()
	snap := ctrl.Snapshot()
	if snap.Body.Text == nil {
		t.Fatal("Snapshot().Body.Text is nil — screen was not pushed or EnsureTextState was not called")
	}
	return snap.Body.Text
}

// ---------------------------------------------------------------------------
// YAML controller-path interaction tests
// ---------------------------------------------------------------------------

func TestTextCtrlInteraction_YAML_ToggleWrap(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 24)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenYAML, lines)
	m2 := views.NewYAMLWithCtrl(res, "ec2", keys.Default(), ctrl)
	m2.SetSize(80, 24)

	// Before: Wrap is false.
	before := textSnapshot(t, ctrl)
	if before.Wrap {
		t.Fatal("expected Wrap=false before toggle")
	}

	// Toggle wrap via controller.
	ctrl.Apply(app.Action{Kind: app.ActionToggleWrap})

	after := textSnapshot(t, ctrl)
	if !after.Wrap {
		t.Fatal("expected Wrap=true after ActionToggleWrap")
	}

	// Toggle back.
	ctrl.Apply(app.Action{Kind: app.ActionToggleWrap})
	final := textSnapshot(t, ctrl)
	if final.Wrap {
		t.Fatal("expected Wrap=false after second toggle")
	}
}

func TestTextCtrlInteraction_YAML_ToggleWrap_ViaUpdate(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 24)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenYAML, lines)
	m2 := views.NewYAMLWithCtrl(res, "ec2", keys.Default(), ctrl)
	m2.SetSize(80, 24)

	// Send "w" key (ToggleWrap) through Update with ctrl wired.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "w"})

	body := textSnapshot(t, ctrl)
	if !body.Wrap {
		t.Fatal("expected ctrl.TextState.Wrap=true after w key through Update")
	}

	// RenderText with Wrap=true must differ from Wrap=false for a long line.
	// Build a long-line body to make wrap observable.
	longLine := strings.Repeat("instance_id: i-0abc123def456  ", 5)
	longLines := []string{longLine, "state: running", "vpc_id: vpc-0abc12345"}
	bodyOn := app.TextBody{Lines: longLines, Wrap: true}
	bodyOff := app.TextBody{Lines: longLines, Wrap: false}

	var scratch views.YAMLModel
	scratch.SetSize(40, 10)
	renderedOn := scratch.RenderText(bodyOn)
	renderedOff := scratch.RenderText(bodyOff)
	if renderedOn == renderedOff {
		t.Error("RenderText with Wrap=true must differ from Wrap=false for a long line at narrow width")
	}
}

func TestTextCtrlInteraction_YAML_Search(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 24)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenYAML, lines)

	// No search active initially.
	before := textSnapshot(t, ctrl)
	if before.Search != "" {
		t.Fatalf("expected empty Search before action, got %q", before.Search)
	}

	// Apply search.
	ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: "instance"})

	after := textSnapshot(t, ctrl)
	if after.Search != "instance" {
		t.Fatalf("expected Search=%q after ActionSearch, got %q", "instance", after.Search)
	}
	if len(after.SearchMatches) == 0 {
		t.Fatal("expected SearchMatches to be populated after ActionSearch with a matching query")
	}

	// RenderText with search active must differ from no search.
	bodySearch := *after
	bodyNoSearch := app.TextBody{Lines: lines}

	var scratch views.YAMLModel
	scratch.SetSize(80, 24)
	withSearch := scratch.RenderText(bodySearch)
	withoutSearch := scratch.RenderText(bodyNoSearch)
	if withSearch == withoutSearch {
		t.Error("RenderText with active search must differ from no-search output (highlights expected)")
	}
}

func TestTextCtrlInteraction_YAML_SearchNextPrev(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 24)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenYAML, lines)

	// Search for "e" which appears in many lines — ensures multiple matches.
	ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: "e"})
	snap0 := textSnapshot(t, ctrl)
	if len(snap0.SearchMatches) < 2 {
		t.Skipf("need >=2 matches to test next/prev, got %d", len(snap0.SearchMatches))
	}
	if snap0.SearchCursor != 0 {
		t.Fatalf("expected SearchCursor=0 after initial search, got %d", snap0.SearchCursor)
	}

	// Next.
	ctrl.Apply(app.Action{Kind: app.ActionSearchNext})
	snap1 := textSnapshot(t, ctrl)
	if snap1.SearchCursor != 1 {
		t.Fatalf("expected SearchCursor=1 after SearchNext, got %d", snap1.SearchCursor)
	}

	// Prev — back to 0.
	ctrl.Apply(app.Action{Kind: app.ActionSearchPrev})
	snap2 := textSnapshot(t, ctrl)
	if snap2.SearchCursor != 0 {
		t.Fatalf("expected SearchCursor=0 after SearchPrev, got %d", snap2.SearchCursor)
	}
}

func TestTextCtrlInteraction_YAML_SearchNextPrev_ViaUpdate(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 24)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenYAML, lines)
	m2 := views.NewYAMLWithCtrl(res, "ec2", keys.Default(), ctrl)
	m2.SetSize(80, 24)

	// Activate search via "/" key.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
	// Type "e" (multiple matches expected).
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "e"})
	// Confirm with Enter.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	snap0 := textSnapshot(t, ctrl)
	if snap0.Search != "e" {
		t.Fatalf("expected Search=%q after typing through Update, got %q", "e", snap0.Search)
	}
	if len(snap0.SearchMatches) < 2 {
		t.Skipf("need >=2 matches, got %d", len(snap0.SearchMatches))
	}

	// Press "n" (SearchNext) through Update.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "n"})
	snap1 := textSnapshot(t, ctrl)
	if snap1.SearchCursor != 1 {
		t.Fatalf("expected SearchCursor=1 after n key through Update, got %d", snap1.SearchCursor)
	}

	// Press "N" (SearchPrev) through Update.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "N"})
	snap2 := textSnapshot(t, ctrl)
	if snap2.SearchCursor != 0 {
		t.Fatalf("expected SearchCursor=0 after N key through Update, got %d", snap2.SearchCursor)
	}

	_ = m2
}

func TestTextCtrlInteraction_YAML_Scroll(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	// Build a long resource so scrolling is observable.
	res := textInteractionResource()
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 5)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenYAML, lines)

	before := textSnapshot(t, ctrl)
	if before.ScrollY != 0 {
		t.Fatalf("expected ScrollY=0 initially, got %d", before.ScrollY)
	}

	// Move down.
	ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
	ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
	ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
	after := textSnapshot(t, ctrl)
	if after.ScrollY != 3 {
		t.Fatalf("expected ScrollY=3 after 3 MoveDown, got %d", after.ScrollY)
	}

	// Move up.
	ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
	snap := textSnapshot(t, ctrl)
	if snap.ScrollY != 2 {
		t.Fatalf("expected ScrollY=2 after MoveUp, got %d", snap.ScrollY)
	}

	// MoveTop.
	ctrl.Apply(app.Action{Kind: app.ActionMoveTop})
	top := textSnapshot(t, ctrl)
	if top.ScrollY != 0 {
		t.Fatalf("expected ScrollY=0 after MoveTop, got %d", top.ScrollY)
	}

	// PageDown.
	ctrl.Apply(app.Action{Kind: app.ActionPageDown, N: 3})
	pd := textSnapshot(t, ctrl)
	if pd.ScrollY != 3 {
		t.Fatalf("expected ScrollY=3 after PageDown(3), got %d", pd.ScrollY)
	}

	// PageUp.
	ctrl.Apply(app.Action{Kind: app.ActionPageUp, N: 2})
	pu := textSnapshot(t, ctrl)
	if pu.ScrollY != 1 {
		t.Fatalf("expected ScrollY=1 after PageUp(2), got %d", pu.ScrollY)
	}
}

func TestTextCtrlInteraction_YAML_Scroll_ViaUpdate(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 5)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenYAML, lines)
	m2 := views.NewYAMLWithCtrl(res, "ec2", keys.Default(), ctrl)
	m2.SetSize(80, 5)

	// Press j (Down) 3 times through Update.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "j"})

	snap := textSnapshot(t, ctrl)
	if snap.ScrollY != 3 {
		t.Fatalf("expected ctrl ScrollY=3 after 3 j presses through Update, got %d", snap.ScrollY)
	}

	// RenderText at scroll=0 vs scroll=3 must differ when content exceeds viewport.
	var scratch views.YAMLModel
	scratch.SetSize(80, 5)
	b0 := app.TextBody{Lines: lines, ScrollY: 0}
	b3 := app.TextBody{Lines: lines, ScrollY: 3}
	r0 := scratch.RenderText(b0)
	r3 := scratch.RenderText(b3)
	if r0 == r3 {
		t.Error("RenderText at ScrollY=0 must differ from ScrollY=3 when content exceeds viewport height")
	}

	_ = m2
}

func TestTextCtrlInteraction_YAML_SearchClear_ViaUpdate(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 24)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenYAML, lines)
	m2 := views.NewYAMLWithCtrl(res, "ec2", keys.Default(), ctrl)
	m2.SetSize(80, 24)

	// Activate and confirm search.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "i"})
	m2, _ = m2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if textSnapshot(t, ctrl).Search == "" {
		t.Fatal("expected search active in ctrl after search input")
	}

	// Press Esc to clear search.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	snap := textSnapshot(t, ctrl)
	if snap.Search != "" {
		t.Fatalf("expected Search cleared in ctrl after Esc, got %q", snap.Search)
	}

	_ = m2
}

// ---------------------------------------------------------------------------
// JSON controller-path interaction tests
// ---------------------------------------------------------------------------

func TestTextCtrlInteraction_JSON_ToggleWrap(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewJSON(res, "ec2", keys.Default())
	m.SetSize(80, 24)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenJSON, lines)
	m2 := views.NewJSONWithCtrl(res, "ec2", keys.Default(), ctrl)
	m2.SetSize(80, 24)

	before := textSnapshot(t, ctrl)
	if before.Wrap {
		t.Fatal("expected Wrap=false before toggle")
	}

	// Toggle via key.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "w"})

	after := textSnapshot(t, ctrl)
	if !after.Wrap {
		t.Fatal("expected Wrap=true after w key through Update (JSON)")
	}

	_ = m2
}

func TestTextCtrlInteraction_JSON_Search(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewJSON(res, "ec2", keys.Default())
	m.SetSize(80, 24)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenJSON, lines)

	ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: "instance"})

	snap := textSnapshot(t, ctrl)
	if snap.Search != "instance" {
		t.Fatalf("expected Search=%q, got %q", "instance", snap.Search)
	}
	if len(snap.SearchMatches) == 0 {
		t.Fatal("expected SearchMatches populated after search in JSON")
	}

	// RenderText differs with vs without search.
	var scratch views.JSONModel
	scratch.SetSize(80, 24)
	withSearch := scratch.RenderText(*snap)
	withoutSearch := scratch.RenderText(app.TextBody{Lines: lines})
	if withSearch == withoutSearch {
		t.Error("RenderText with search must differ from no-search output (JSON)")
	}
}

func TestTextCtrlInteraction_JSON_SearchNextPrev_ViaUpdate(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewJSON(res, "ec2", keys.Default())
	m.SetSize(80, 24)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenJSON, lines)
	m2 := views.NewJSONWithCtrl(res, "ec2", keys.Default(), ctrl)
	m2.SetSize(80, 24)

	// Search for "e".
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "e"})
	m2, _ = m2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	snap0 := textSnapshot(t, ctrl)
	if snap0.Search != "e" {
		t.Fatalf("expected Search=e in ctrl, got %q", snap0.Search)
	}
	if len(snap0.SearchMatches) < 2 {
		t.Skipf("need >=2 matches, got %d", len(snap0.SearchMatches))
	}

	// Next.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "n"})
	if textSnapshot(t, ctrl).SearchCursor != 1 {
		t.Fatalf("expected SearchCursor=1 after n, got %d", textSnapshot(t, ctrl).SearchCursor)
	}

	// Prev.
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "N"})
	if textSnapshot(t, ctrl).SearchCursor != 0 {
		t.Fatalf("expected SearchCursor=0 after N, got %d", textSnapshot(t, ctrl).SearchCursor)
	}

	_ = m2
}

func TestTextCtrlInteraction_JSON_Scroll_ViaUpdate(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textInteractionResource()
	m := views.NewJSON(res, "ec2", keys.Default())
	m.SetSize(80, 5)
	lines := m.ContentLines()

	ctrl := newTextController(runtime.ScreenJSON, lines)
	m2 := views.NewJSONWithCtrl(res, "ec2", keys.Default(), ctrl)
	m2.SetSize(80, 5)

	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "j"})

	snap := textSnapshot(t, ctrl)
	if snap.ScrollY != 2 {
		t.Fatalf("expected ctrl ScrollY=2 after 2 j presses (JSON), got %d", snap.ScrollY)
	}

	// RenderText at different scroll positions must differ.
	var scratch views.JSONModel
	scratch.SetSize(80, 5)
	b0 := app.TextBody{Lines: lines, ScrollY: 0}
	b2 := app.TextBody{Lines: lines, ScrollY: 2}
	r0 := scratch.RenderText(b0)
	r2 := scratch.RenderText(b2)
	if r0 == r2 {
		t.Error("RenderText at ScrollY=0 must differ from ScrollY=2 when content exceeds viewport height (JSON)")
	}

	_ = m2
}
