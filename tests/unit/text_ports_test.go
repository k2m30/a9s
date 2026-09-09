// text_ports_test.go — text-viewer family unique behavior pins ported onto
// the live controller/renderer-adapter path, per
// specs/022-codebase-cleanup/wave3-map-text.md. The dying legacy-model tests
// these port from (qa_yaml_test.go, qa_json_test.go, qa_error_log_test.go,
// ct_events_t_key_test.go's YAML case, qa_view_switching_test.go,
// scroll_state_test.go) stay untouched — nothing is deleted here, only new
// live-path pins are added so the behavior survives their eventual removal.
package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ── Shared helper: press Copy and read back what was copied ────────────────
//
// The 'c' key routes through Model.handleCopy() -> copyToClipboard(content,
// label). On rsKindText/Reveal/Identity the returned messages.Flash.Text
// carries a constant label (e.g. "Copied YAML to clipboard"), never the
// copied content itself — so content correctness (uncolored body, exact ARN,
// ...) can be verified only by capturing what the copy handed to the
// clipboard, which is what ReadClipboardAfter does.
func wave3CopyAndReadClipboard(t *testing.T, m tui.Model) string {
	t.Helper()
	return ReadClipboardAfter(t, func() {
		_, cmd := rootApplyMsg(m, rootKeyPress("c"))
		if cmd == nil {
			t.Fatal("copy key must return a non-nil cmd")
		}
		msg := cmd()
		flash, ok := msg.(messages.Flash)
		if !ok {
			t.Fatalf("expected messages.Flash from copy, got %T", msg)
		}
		// The capture seam cannot fail, so an error flash here is the copy
		// path itself reporting a failure, not a machine without a
		// pasteboard.
		if flash.IsError || strings.HasPrefix(flash.Text, "Copy failed:") {
			t.Fatalf("copy reported a failure: %q", flash.Text)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// Item 1 + 11 (JSON) — rsKindText copy strips color for JSON.
// Verify-then-port: no existing test drives handleCopy's rsKindText branch
// for JSON at all (qa_json_test.go's RawContent/CopyContent tests all use the
// dead NewJSON()/JSONModel path). PORTED.
// ═══════════════════════════════════════════════════════════════════════════

func TestPort_JSONCopy_UncoloredContent(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	res := resource.Resource{
		ID: "i-jsonport1", Name: "json-port-server",
		Fields: map[string]string{"instance_id": "i-jsonport1", "state": "running"},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{res}})

	// Navigate to the JSON view (mirrors qa_copy_test.go's YAML case).
	m, cmd := rootApplyMsg(m, rootKeyPress("J"))
	if cmd == nil {
		t.Fatal("J on resource list should return a navigate command")
	}
	navMsg := cmd()
	m, _ = rootApplyMsg(m, navMsg)

	content := wave3CopyAndReadClipboard(t, m)
	if strings.Contains(content, "\x1b[") {
		t.Errorf("JSON copy must strip ANSI color codes (rawContentFromTextBody), got: %q", content)
	}
	if !strings.Contains(content, "i-jsonport1") {
		t.Errorf("JSON copy content must include the resource's real field value, got: %q", content)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Item 1 (YAML) — strengthens the existing TestQA_Copy_YAML_CopiesFullYAML
// (tests/unit/qa_copy_test.go), which only checks the constant Flash label
// ("...YAML..."), not that the copied body is actually uncolored / correct.
// PORTED.
// ═══════════════════════════════════════════════════════════════════════════

func TestPort_YAMLCopy_UncoloredContent(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	res := resource.Resource{
		ID: "i-yamlport1", Name: "yaml-port-server",
		Fields: map[string]string{"instance_id": "i-yamlport1", "state": "stopped"},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{res}})

	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd == nil {
		t.Fatal("y on resource list should return a navigate command")
	}
	navMsg := cmd()
	m, _ = rootApplyMsg(m, navMsg)

	content := wave3CopyAndReadClipboard(t, m)
	if strings.Contains(content, "\x1b[") {
		t.Errorf("YAML copy must strip ANSI color codes (rawContentFromTextBody), got: %q", content)
	}
	if !strings.Contains(content, "i-yamlport1") || !strings.Contains(content, "stopped") {
		t.Errorf("YAML copy content must include the resource's real field values, got: %q", content)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Item 4 — error-log copy label + uncolored content. qa_error_log_test.go's
// TestTextViewer_CopyContentReturnsRawText/RawContentReturnsRawText assert on
// the dead NewTextViewer().CopyContent()/RawContent(); the live error-log
// screen is ctrl-backed via newErrorLogRS+EnsureTextState and shares
// handleCopy's rsKindText branch, which had zero coverage. PORTED.
// ═══════════════════════════════════════════════════════════════════════════

func TestPort_ErrorLogCopy_UncoloredContent(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Seed error history via a real APIError flow (mirrors qa_error_log_test.go).
	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "ec2",
		Err:          errors.New("wave3-port-error: AccessDenied on DescribeInstances"),
	})

	// '!' opens the error log.
	m, cmd := rootApplyMsg(m, rootKeyPress("!"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "wave3-port-error") {
		t.Fatalf("error log view must show the seeded error, got: %s", plain)
	}

	content := wave3CopyAndReadClipboard(t, m)
	if strings.Contains(content, "\x1b[") {
		t.Errorf("error-log copy must strip ANSI color codes, got: %q", content)
	}
	if !strings.Contains(content, "wave3-port-error") {
		t.Errorf("error-log copy content must include the logged error text, got: %q", content)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Item 3 — colorizeYAML/colorizeJSON exact-line golden, ported onto the live
// ContentLines() path (NewYAMLWithCtrl / NewJSONWithCtrl are LIVE per the
// map). text_render_parity_test.go only proves the legacy View() and the
// live RenderText() agree with EACH OTHER (both call the same colorize
// functions) — it is not an independent pin that a specific field renders
// with real color codes. qa_yaml_test.go's SyntaxColoring / qa_json_test.go's
// ColorizeStructuralTokens tests all build via the dead NewYAML()/NewJSON()
// constructors. PORTED.
// ═══════════════════════════════════════════════════════════════════════════

func TestPort_YAML_ColorizeGolden_LiveContentLines(t *testing.T) {
	res := resource.Resource{
		ID: "i-goldenyaml", Name: "golden-yaml",
		Fields: map[string]string{"state": "running"},
	}
	m := views.NewYAMLWithCtrl(res, "ec2", keys.Default(), nil)
	m.SetSize(120, 40)
	lines := m.ContentLines()

	var target string
	for _, line := range lines {
		if strings.Contains(stripANSI(line), "state: running") {
			target = line
			break
		}
	}
	if target == "" {
		t.Fatalf("expected a ContentLines() entry whose stripped form is exactly %q, got lines: %v", "state: running", lines)
	}
	if stripANSI(target) != "state: running" {
		t.Errorf("golden line mismatch after stripping ANSI: got %q, want %q", stripANSI(target), "state: running")
	}
	if !strings.Contains(target, "\x1b[") {
		t.Errorf("live YAML ContentLines() must colorize the key/value line with ANSI codes, got: %q", target)
	}
}

func TestPort_JSON_ColorizeGolden_LiveContentLines(t *testing.T) {
	res := resource.Resource{
		ID: "i-goldenjson", Name: "golden-json",
		Fields: map[string]string{"state": "running"},
	}
	m := views.NewJSONWithCtrl(res, "ec2", keys.Default(), nil)
	m.SetSize(120, 40)
	lines := m.ContentLines()

	var found bool
	for _, line := range lines {
		if strings.Contains(stripANSI(line), `"state"`) && strings.Contains(stripANSI(line), `"running"`) {
			if !strings.Contains(line, "\x1b[") {
				t.Errorf("live JSON ContentLines() must colorize the state field with ANSI codes, got: %q", line)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a ContentLines() entry containing the state field, got lines: %v", lines)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Item 5 — YAML text-screen 't' key -> ct-events RelatedNavigate on the LIVE
// path. ct_events_t_key_test.go's TestYAML_TKey_EmitsRelatedNavigateMsg
// drives the dead views.NewYAML()+Update() directly. The live text-screen
// handler was uncovered. PORTED. (The Detail 't'-key case belongs to a
// different agent's scope; this pins ONLY the YAML/text case.)
//
// CONFIRMED REGRESSION, intentionally left RED: internal/tui/app_stack.go's
// handleTextKeyMsg (the rsKindText key router) has no case for m.keys.
// CloudTrail at all — only Search/SearchNext/SearchPrev/Escape/ToggleWrap/
// Up/Down/Top/Bottom/PageUp/PageDown are handled; unmatched keys fall
// through to the stored viewport. Pressing 't' while already on a live
// YAML/JSON screen is therefore currently a no-op, contradicting
// docs/shared/keybindings.md ("t: Jump to CloudTrail Events for the
// selected resource (all resource types)") and the legacy pin this ports.
// This is a genuine gap for a coder to wire, not a test defect — do not
// weaken this assertion to match the current no-op behavior.
// ═══════════════════════════════════════════════════════════════════════════

func TestPort_YAML_TKey_LiveCTEventsNavigate(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    []resource.Resource{ctEventsEC2Resource()},
	})

	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd == nil {
		t.Fatal("y on resource list should return a navigate command")
	}
	navMsg := cmd()
	m, _ = rootApplyMsg(m, navMsg)
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "i-test") && !strings.Contains(plain, "test-instance") {
		t.Fatalf("expected to be on the YAML view for i-test, got: %s", plain)
	}

	_, cmd = rootApplyMsg(m, rootKeyPress("t"))
	if cmd == nil {
		t.Fatal("pressing 't' on the live YAML text screen must return a non-nil cmd")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("pressing 't' on the live YAML text screen must emit RelatedNavigate; got %T", msg)
	}
	if nav.TargetType != "ct-events" {
		t.Errorf("RelatedNavigate.TargetType = %q, want %q", nav.TargetType, "ct-events")
	}
	if nav.FetchFilter["ResourceName"] != "i-test" {
		t.Errorf("FetchFilter[ResourceName] = %q, want %q", nav.FetchFilter["ResourceName"], "i-test")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Item 7 — ScrollState.VisibleWindow centered-window cases, copied verbatim
// from scroll_state_test.go (which is slated for a later partial deletion:
// cursor-arithmetic cases removed, VisibleWindow cases kept). Copying now
// means the pin survives regardless of when/how that trim lands. ScrollState
// itself is untouched production code (LIVE via RenderSelector) — these are
// duplicated test cases, not a "port" of dead-path behavior. PORTED (insurance
// copy).
// ═══════════════════════════════════════════════════════════════════════════

func TestPort_ScrollState_VisibleWindow_AllFit(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(2)
	start, end := s.VisibleWindow(10)
	if start != 0 || end != 5 {
		t.Errorf("when all items fit, expected (0,5), got (%d,%d)", start, end)
	}
}

func TestPort_ScrollState_VisibleWindow_CursorCentered(t *testing.T) {
	s := views.NewScrollState(20)
	s.SetCursor(10)
	start, end := s.VisibleWindow(5)
	if start != 8 || end != 13 {
		t.Errorf("expected cursor=10 centered in (8,13), got (%d,%d)", start, end)
	}
}

func TestPort_ScrollState_VisibleWindow_CursorNearTop(t *testing.T) {
	s := views.NewScrollState(20)
	s.SetCursor(1)
	start, end := s.VisibleWindow(5)
	if start != 0 || end != 5 {
		t.Errorf("expected (0,5) when cursor near top, got (%d,%d)", start, end)
	}
}

func TestPort_ScrollState_VisibleWindow_CursorNearBottom(t *testing.T) {
	s := views.NewScrollState(20)
	s.SetCursor(19)
	start, end := s.VisibleWindow(5)
	if start != 15 || end != 20 {
		t.Errorf("expected (15,20) when cursor near bottom, got (%d,%d)", start, end)
	}
}

func TestPort_ScrollState_VisibleWindow_ZeroTotal(t *testing.T) {
	s := views.NewScrollState(0)
	start, end := s.VisibleWindow(5)
	if start != 0 || end != 0 {
		t.Errorf("expected (0,0) with zero total, got (%d,%d)", start, end)
	}
}

func TestPort_ScrollState_VisibleWindow_ViewHeightOne(t *testing.T) {
	s := views.NewScrollState(10)
	s.SetCursor(5)
	start, end := s.VisibleWindow(1)
	if end-start != 1 {
		t.Errorf("with viewHeight=1, should show exactly 1 row, got %d", end-start)
	}
	if s.CursorForTest() < start || s.CursorForTest() >= end {
		t.Errorf("cursor %d should be within [%d,%d)", s.CursorForTest(), start, end)
	}
}

func TestPort_ScrollState_VisibleWindow_ExactFit(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(2)
	start, end := s.VisibleWindow(5)
	if start != 0 || end != 5 {
		t.Errorf("when total == viewHeight, expected (0,5), got (%d,%d)", start, end)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Item 10 — YAML<->JSON 'y'/'J' toggle on the LIVE path. qa_view_switching_test.go
// drives the dead JSONModel/YAMLModel.Update() directly (jsonModel() ->
// views.NewJSON()). No existing test presses 'J' at all on the live path.
// PORTED.
//
// CONFIRMED REGRESSION, intentionally left RED: same root cause as the
// TestPort_YAML_TKey_LiveCTEventsNavigate note above — handleTextKeyMsg
// has no case for m.keys.YAML or m.keys.JSON either, so pressing 'J' while
// on YAML (or 'y' while on JSON) is currently a no-op instead of toggling,
// contradicting docs/shared/keybindings.md's unscoped "y"/"J" actions and
// the legacy toggle pins these tests port. Do not weaken these assertions.
// ═══════════════════════════════════════════════════════════════════════════

func TestPort_YAMLToJSON_LiveToggle(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	res := resource.Resource{ID: "i-toggle1", Name: "toggle-server", Fields: map[string]string{"state": "running"}}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{res}})

	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd == nil {
		t.Fatal("y on resource list should return a navigate command")
	}
	m, _ = rootApplyMsg(m, cmd())

	m, cmd = rootApplyMsg(m, rootKeyPress("J"))
	if cmd == nil {
		t.Fatal("J on YAML view should return a navigate-to-JSON command")
	}
	m, _ = rootApplyMsg(m, cmd())

	content := wave3CopyAndReadClipboard(t, m)
	if !strings.Contains(content, `"state"`) {
		t.Errorf("expected to be on the JSON view after y->J toggle, copied content: %q", content)
	}
}

func TestPort_JSONToYAML_LiveToggle(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	res := resource.Resource{ID: "i-toggle2", Name: "toggle-server-2", Fields: map[string]string{"state": "stopped"}}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{res}})

	m, cmd := rootApplyMsg(m, rootKeyPress("J"))
	if cmd == nil {
		t.Fatal("J on resource list should return a navigate command")
	}
	m, _ = rootApplyMsg(m, cmd())

	m, cmd = rootApplyMsg(m, rootKeyPress("y"))
	if cmd == nil {
		t.Fatal("y on JSON view should return a navigate-to-YAML command")
	}
	m, _ = rootApplyMsg(m, cmd())

	content := wave3CopyAndReadClipboard(t, m)
	if !strings.Contains(content, "state:") {
		t.Errorf("expected to be on the YAML view after J->y toggle, copied content: %q", content)
	}
}

// TestPort_YAMLToggle_SurvivesAsyncEnrichment pins that a field added by
// async detail enrichment (messages.EnrichDetailResult) survives a y/J
// toggle. handleTextKeyMsg's YAML/JSON cases (app_stack.go) resolve the
// resource to re-marshal via Controller.GetTextResource ->
// findCachedResourceByID — the row cache — not the enriched TextState that
// UpdateTextLines wrote (runtime_adapter_resources.go's
// handleEnrichDetailResult). ApplyDetailEnrichmentForResource only mutates a
// matching ScreenDetail's state, never the row cache, so a resource with no
// Detail screen on the stack (as here, navigated straight to YAML) has no
// path back to the enriched Fields at all: the toggle re-marshals the
// pre-enrichment row and silently drops the field.
func TestPort_YAMLToggle_SurvivesAsyncEnrichment(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	res := resource.Resource{ID: "i-enrichtoggle1", Name: "enrich-toggle-server", Fields: map[string]string{"state": "running"}}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{res}})

	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd == nil {
		t.Fatal("y on resource list should return a navigate command")
	}
	m, _ = rootApplyMsg(m, cmd())

	enriched := resource.Resource{
		ID:   res.ID,
		Name: res.Name,
		Fields: map[string]string{
			"state":           "running",
			"iam_profile_arn": "arn:aws:iam::123456789012:instance-profile/enriched-role",
		},
	}
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "ec2",
		ResourceID:   res.ID,
		EnrichedRes:  enriched,
	})

	before := wave3CopyAndReadClipboard(t, m)
	if !strings.Contains(before, "iam_profile_arn") {
		t.Fatalf("precondition: enriched field must be visible before the toggle, got: %q", before)
	}

	m, cmd = rootApplyMsg(m, rootKeyPress("J"))
	if cmd == nil {
		t.Fatal("J on YAML view should return a navigate-to-JSON command")
	}
	m, _ = rootApplyMsg(m, cmd())

	after := wave3CopyAndReadClipboard(t, m)
	if !strings.Contains(after, "iam_profile_arn") {
		t.Errorf("enriched field iam_profile_arn dropped by the y/J toggle — toggle re-marshals the stale row-cache resource instead of the enriched TextState, got: %q", after)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Item 11 — copy label/payload per remaining text-family view.
// ═══════════════════════════════════════════════════════════════════════════

// Selector and Help are not handled by handleCopy's switch at all (only
// rsKindList/Detail/Reveal/Text/Identity are), so 'c' must be a true no-op
// there. This ports qa_view_methods_test.go's dead
// TestSelector_CopyContentReturnsEmpty / TestCopyContent_Help_ReturnsEmpty
// pins onto the live path.
func TestPort_SelectorCopy_IsNoOp(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "aws-regions") {
		t.Fatal("expected to be on the region selector view")
	}
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))
	if cmd != nil {
		t.Error("c on a selector screen should be a no-op (nil cmd) — handleCopy has no rsKindSelector case")
	}
}

func TestPort_HelpCopy_IsNoOp(t *testing.T) {
	m := newRootSizedModel()
	m, cmd := rootApplyMsg(m, rootKeyPress("?"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	_, cmd = rootApplyMsg(m, rootKeyPress("c"))
	if cmd != nil {
		t.Error("c on the help screen should be a no-op (nil cmd) — handleCopy has no rsKindHelp case")
	}
}

// Identity copy: press 'i' after a live IdentityLoaded event so rs.identityData
// is seeded synchronously (app_input.go), then verify 'c' copies the exact ARN.
// No prior test drove handleCopy's rsKindIdentity branch. PORTED.
func TestPort_IdentityCopy_CopiesExactARN(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	wantARN := "arn:aws:iam::123456789012:user/wave3-port-identity"
	m, _ = rootApplyMsg(m, messages.IdentityLoaded{
		Identity: &awsclient.CallerIdentity{
			AccountID: "123456789012",
			Arn:       wantARN,
			UserName:  "wave3-port-identity",
		},
		Gen: 0,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("i"))
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "123456789012") {
		t.Fatalf("expected to be on the identity view showing the account ID, got: %s", plain)
	}

	content := wave3CopyAndReadClipboard(t, m)
	if content != wantARN {
		t.Errorf("identity copy content = %q, want exact ARN %q", content, wantARN)
	}
}

// Identity copy while still loading (no IdentityLoaded event delivered yet)
// must be a no-op — handleCopy's rsKindIdentity branch gates on
// !rs.identityLoading. Ports tui_identity_test.go's dead-path
// TestIdentityView_CopyContent_EmptyWhenLoading (m.CopyContent()) onto the
// live seam.
func TestPort_IdentityCopy_NoOpWhileLoading(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// 'i' pushes the identity screen; with no prior IdentityLoaded event,
	// m.core.Identity() is nil so idRS.identityLoading stays true.
	m, _ = rootApplyMsg(m, rootKeyPress("i"))

	_, cmd := rootApplyMsg(m, rootKeyPress("c"))
	if cmd != nil {
		t.Error("c on the identity screen while still loading should be a no-op (nil cmd)")
	}
}

// Reveal copy: register a test reveal fetcher over "secrets" (restored via
// t.Cleanup), drive the real 'x' -> fetchRevealValue -> ValueRevealed chain
// with demo clients, then verify 'c' copies the exact revealed value. No
// prior test drove handleCopy's rsKindReveal branch. PORTED.
func TestPort_RevealCopy_CopiesExactValue(t *testing.T) {
	const shortName = "secrets"
	origFetcher := resource.GetRevealFetcher(shortName)
	wantValue := "wave3-port-secret-value-xyz"
	resource.SetRevealFetcherForTest(shortName, func(_ context.Context, _ any, _ string) (string, error) {
		return wantValue, nil
	})
	t.Cleanup(func() { resource.SetRevealFetcherForTest(shortName, origFetcher) })

	tui.Version = "test"
	clients := demo.NewServiceClients()
	m := newBlessedModel(t, "test", "us-east-1", tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	// WithClients only pre-supplies the session's PreSuppliedClients; the
	// fetch path reads session.Clients, populated by a live ClientsReady.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: clients, Region: "us-east-1", Gen: 1})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: shortName})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: shortName,
		Resources:    []resource.Resource{{ID: "wave3-secret-arn", Name: "wave3-secret"}},
	})

	m, cmd := rootApplyMsg(m, rootKeyPress("x"))
	if cmd == nil {
		t.Fatal("x on secrets list should return a reveal fetch command")
	}
	revealedMsg := cmd()
	m, _ = rootApplyMsg(m, revealedMsg)

	content := wave3CopyAndReadClipboard(t, m)
	if content != wantValue {
		t.Errorf("reveal copy content = %q, want exact revealed value %q", content, wantValue)
	}
}

// Reveal copy of a JSON secret must stay RAW (compact), not the pretty-printed
// form RevealModel.displayValue() renders for View(). Ports the one unique
// pin from qa_reveal_test.go's dead-path TestQA_Reveal_JSONValue_CopyReturnsRaw
// (m.CopyContent()) onto the live handleCopy/rsKindReveal seam, which copies
// rs.revealValue directly — the raw fetched string, never the display-only
// json.MarshalIndent output.
func TestPort_RevealCopy_JSONValueStaysRaw(t *testing.T) {
	const shortName = "secrets"
	origFetcher := resource.GetRevealFetcher(shortName)
	wantValue := `{"api_key":"sk-123456","endpoint":"https://api.example.com"}`
	resource.SetRevealFetcherForTest(shortName, func(_ context.Context, _ any, _ string) (string, error) {
		return wantValue, nil
	})
	t.Cleanup(func() { resource.SetRevealFetcherForTest(shortName, origFetcher) })

	tui.Version = "test"
	clients := demo.NewServiceClients()
	m := newBlessedModel(t, "test", "us-east-1", tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: clients, Region: "us-east-1", Gen: 1})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: shortName})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: shortName,
		Resources:    []resource.Resource{{ID: "wave3-json-secret-arn", Name: "wave3-json-secret"}},
	})

	m, cmd := rootApplyMsg(m, rootKeyPress("x"))
	if cmd == nil {
		t.Fatal("x on secrets list should return a reveal fetch command")
	}
	revealedMsg := cmd()
	m, _ = rootApplyMsg(m, revealedMsg)

	content := wave3CopyAndReadClipboard(t, m)
	if content != wantValue {
		t.Errorf("JSON reveal copy content = %q, want raw compact JSON %q (not pretty-printed)", content, wantValue)
	}
}

// Reveal copy of an EMPTY secret value is a silent no-op on the live path:
// handleCopy's rsKindReveal branch sets content=rs.revealValue unconditionally
// (internal/tui/runtime_adapter_navigate.go), but the shared
// `if content == "" { return m, nil }` guard AFTER the switch applies to
// every rsKind including Reveal. This contradicts the dead
// RevealModel.CopyContent(), which unconditionally returned
// ("", "Secret copied to clipboard") — qa_reveal_test.go's
// TestQA_Reveal_EmptyValue asserted behavior the live path does not actually
// have. Ports that pin onto the live seam with the CORRECTED expectation.
func TestPort_RevealCopy_EmptyValue(t *testing.T) {
	const shortName = "secrets"
	origFetcher := resource.GetRevealFetcher(shortName)
	resource.SetRevealFetcherForTest(shortName, func(_ context.Context, _ any, _ string) (string, error) {
		return "", nil
	})
	t.Cleanup(func() { resource.SetRevealFetcherForTest(shortName, origFetcher) })

	tui.Version = "test"
	clients := demo.NewServiceClients()
	m := newBlessedModel(t, "test", "us-east-1", tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: clients, Region: "us-east-1", Gen: 1})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: shortName})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: shortName,
		Resources:    []resource.Resource{{ID: "wave3-empty-secret-arn", Name: "wave3-empty-secret"}},
	})

	m, cmd := rootApplyMsg(m, rootKeyPress("x"))
	if cmd == nil {
		t.Fatal("x on secrets list should return a reveal fetch command")
	}
	revealedMsg := cmd()
	m, _ = rootApplyMsg(m, revealedMsg)

	_, copyCmd := rootApplyMsg(m, rootKeyPress("c"))
	if copyCmd != nil {
		t.Errorf("copy key on an empty-value reveal screen should be a silent no-op (nil cmd) — "+
			"handleCopy's post-switch `content == \"\"` guard applies to rsKindReveal too, got cmd=%v", copyCmd)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// qa_view_switching_test.go's TestJSONView_PressD_EmitsNavigateToDetail /
// TestYAMLView_PressD_EmitsNavigateToDetail ('d' from JSON/YAML -> Detail)
// ported onto the live path.
//
// CONFIRMED REGRESSION, intentionally left RED: unlike keys.YAML/JSON/
// CloudTrail (wired into handleTextKeyMsg, app_stack.go), there is NO
// keys.Detail case in handleTextKeyMsg at all — pressing 'd' on a live
// YAML/JSON text screen is currently a no-op instead of navigating to
// Detail, contradicting the legacy pin these tests port and
// docs/shared/keybindings.md's unscoped "d: Detail view". Do not weaken.
// ═══════════════════════════════════════════════════════════════════════════

func TestPort_YAML_DKey_LiveNavigateToDetail(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	res := resource.Resource{ID: "i-dkey1", Name: "dkey-server", Fields: map[string]string{"state": "running"}}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{res}})

	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd == nil {
		t.Fatal("y on resource list should return a navigate command")
	}
	m, _ = rootApplyMsg(m, cmd())

	_, cmd = rootApplyMsg(m, rootKeyPress("d"))
	if cmd == nil {
		t.Fatal("pressing 'd' on the live YAML text screen must return a non-nil cmd")
	}
	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("pressing 'd' on the live YAML text screen must emit Navigate; got %T", msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("Navigate.Target = %v, want TargetDetail", nav.Target)
	}
	if !nav.ReplaceCurrent {
		t.Error("Navigate.ReplaceCurrent = false, want true")
	}
}

func TestPort_JSON_DKey_LiveNavigateToDetail(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	res := resource.Resource{ID: "i-dkey2", Name: "dkey-server-2", Fields: map[string]string{"state": "stopped"}}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{res}})

	m, cmd := rootApplyMsg(m, rootKeyPress("J"))
	if cmd == nil {
		t.Fatal("J on resource list should return a navigate command")
	}
	m, _ = rootApplyMsg(m, cmd())

	_, cmd = rootApplyMsg(m, rootKeyPress("d"))
	if cmd == nil {
		t.Fatal("pressing 'd' on the live JSON text screen must return a non-nil cmd")
	}
	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("pressing 'd' on the live JSON text screen must emit Navigate; got %T", msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("Navigate.Target = %v, want TargetDetail", nav.Target)
	}
	if !nav.ReplaceCurrent {
		t.Error("Navigate.ReplaceCurrent = false, want true")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Round 4, item 4 — text_ctrl_interaction_test.go verify-then-port-then-delete.
//
// That file's own header claimed it drove "keys through YAMLModel/JSONModel
// .Update() with ctrl wired" to close a gap where Update() might mutate
// model-local state without reaching the controller. Verified false:
// internal/tui/app_stack.go's handleTextKeyMsg (the real rsKindText key
// router) never calls YAMLModel/JSONModel.Update() at all — search input
// mode routes through rs.search, and Search/SearchNext/SearchPrev/Escape/
// ToggleWrap/Up/Down/Top/Bottom/PageUp/PageDown all call m.ctrl.Apply(...)
// directly (app_stack.go:594-755). YAMLModel/JSONModel.Update() is
// unreachable from production — the file's "ViaUpdate" pins exercised dead
// code under a false-confidence banner, not the live dispatch.
//
// The controller-level pins (non-"ViaUpdate": ToggleWrap, Search,
// SearchNextPrev, Scroll) test real, live mechanism (ctrl.Apply + RenderText
// are exactly what handleTextKeyMsg and the renderer use) with useful
// precision (exact ScrollY arithmetic across 5 different actions, exact
// SearchCursor index) that a single key-press integration test does not
// practically reproduce — ported below with only the dead-constructor setup
// fixed (NewYAML/NewJSON -> NewYAMLWithCtrl/NewJSONWithCtrl), no behavior
// lost.
//
// The "ViaUpdate" pins are replaced by NEW pins below that drive the REAL
// key path (root model -> handleTextKeyMsg -> ctrl.Apply), proving the
// wiring the old file only assumed. handleTextKeyMsg's switch does not
// branch on YAML vs JSON for any of these keys (verified by reading it) —
// wrap/search/scroll dispatch is identical for both screen kinds, so each
// mechanism is ported ONCE against YAML with a single JSON parity
// spot-check, rather than duplicating all 5 mechanisms across both kinds
// (duplicating a proven-identical dispatch path is padding, not coverage).
// ═══════════════════════════════════════════════════════════════════════════

// newTextScreenController builds a Controller with a YAML or JSON screen on
// the stack, mirroring text_ctrl_interaction_test.go's retired
// newTextController. Blessed via knownConstructionDebt in
// qa_controller_construction_discipline_test.go (no ResourcesLoaded/
// EnrichmentChecked/AvailabilityChecked event is ever driven through it).
func newTextScreenController(t testing.TB, screenID runtime.ScreenID, lines []string) *app.Controller {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	ctrl := newBlessedController(t, core)
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: screenID}})
	ctrl.EnsureTextState(lines)
	return ctrl
}

// wave3CtrlInteractionResource mirrors text_ctrl_interaction_test.go's
// retired textInteractionResource: enough fields (plus one deliberately long
// value) that wrap, search, and scroll are all observable at a narrow
// viewport.
func wave3CtrlInteractionResource() resource.Resource {
	return resource.Resource{
		ID:   "wave3-ctrl-i-0abc123",
		Name: "wave3-ctrl-instance",
		Fields: map[string]string{
			"instance_id":   "i-0abc123def456",
			"instance_type": "t3.medium",
			"state":         "running",
			"launch_time":   "2024-01-15T10:30:00Z",
			"public_ip":     "203.0.113.42",
			"private_ip":    "10.0.1.100",
			"vpc_id":        "vpc-0abc12345",
			"subnet_id":     "subnet-0def67890",
			"field_a":       "value-match-one",
			"field_b":       "value-match-two",
			"field_c":       "value-match-three",
			"long_field":    strings.Repeat("padding-for-wrap ", 10),
		},
	}
}

func wave3TextInteractionLines(t *testing.T) []string {
	t.Helper()
	m := views.NewYAMLWithCtrl(wave3CtrlInteractionResource(), "ec2", keys.Default(), nil)
	m.SetSize(80, 24)
	return m.ContentLines()
}

// ── ctrl.Apply-direct precision pins (live mechanism; only the dead-
// constructor setup needed fixing) ─────────────────────────────────────────

func TestPort_YAML_ToggleWrap_CtrlPrecision(t *testing.T) {
	tuitest.NoColor(t)
	lines := wave3TextInteractionLines(t)
	ctrl := newTextScreenController(t, runtime.ScreenYAML, lines)

	before := textSnapshotHelper(t, ctrl)
	if before.Wrap {
		t.Fatal("expected Wrap=false before toggle")
	}
	ctrl.Apply(app.Action{Kind: app.ActionToggleWrap})
	after := textSnapshotHelper(t, ctrl)
	if !after.Wrap {
		t.Fatal("expected Wrap=true after ActionToggleWrap")
	}
	ctrl.Apply(app.Action{Kind: app.ActionToggleWrap})
	final := textSnapshotHelper(t, ctrl)
	if final.Wrap {
		t.Fatal("expected Wrap=false after second toggle")
	}
}

func TestPort_YAML_Search_CtrlPrecision(t *testing.T) {
	tuitest.NoColor(t)
	lines := wave3TextInteractionLines(t)
	ctrl := newTextScreenController(t, runtime.ScreenYAML, lines)

	before := textSnapshotHelper(t, ctrl)
	if before.Search != "" {
		t.Fatalf("expected empty Search before action, got %q", before.Search)
	}
	ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: "instance"})
	after := textSnapshotHelper(t, ctrl)
	if after.Search != "instance" {
		t.Fatalf("expected Search=%q after ActionSearch, got %q", "instance", after.Search)
	}
	if len(after.SearchMatches) == 0 {
		t.Fatal("expected SearchMatches to be populated after ActionSearch with a matching query")
	}

	var scratch views.YAMLModel
	scratch.SetSize(80, 24)
	withSearch := scratch.RenderText(*after)
	withoutSearch := scratch.RenderText(app.TextBody{Lines: lines})
	if withSearch == withoutSearch {
		t.Error("RenderText with active search must differ from no-search output (highlights expected)")
	}
}

func TestPort_YAML_SearchNextPrev_CtrlPrecision(t *testing.T) {
	tuitest.NoColor(t)
	lines := wave3TextInteractionLines(t)
	ctrl := newTextScreenController(t, runtime.ScreenYAML, lines)

	ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: "e"})
	snap0 := textSnapshotHelper(t, ctrl)
	if len(snap0.SearchMatches) < 2 {
		t.Skipf("need >=2 matches to test next/prev, got %d", len(snap0.SearchMatches))
	}
	if snap0.SearchCursor != 0 {
		t.Fatalf("expected SearchCursor=0 after initial search, got %d", snap0.SearchCursor)
	}

	ctrl.Apply(app.Action{Kind: app.ActionSearchNext})
	snap1 := textSnapshotHelper(t, ctrl)
	if snap1.SearchCursor != 1 {
		t.Fatalf("expected SearchCursor=1 after SearchNext, got %d", snap1.SearchCursor)
	}

	ctrl.Apply(app.Action{Kind: app.ActionSearchPrev})
	snap2 := textSnapshotHelper(t, ctrl)
	if snap2.SearchCursor != 0 {
		t.Fatalf("expected SearchCursor=0 after SearchPrev, got %d", snap2.SearchCursor)
	}
}

func TestPort_YAML_Scroll_CtrlPrecision(t *testing.T) {
	tuitest.NoColor(t)
	m := views.NewYAMLWithCtrl(wave3CtrlInteractionResource(), "ec2", keys.Default(), nil)
	m.SetSize(80, 5)
	lines := m.ContentLines()
	ctrl := newTextScreenController(t, runtime.ScreenYAML, lines)

	before := textSnapshotHelper(t, ctrl)
	if before.ScrollY != 0 {
		t.Fatalf("expected ScrollY=0 initially, got %d", before.ScrollY)
	}

	ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
	ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
	ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
	after := textSnapshotHelper(t, ctrl)
	if after.ScrollY != 3 {
		t.Fatalf("expected ScrollY=3 after 3 MoveDown, got %d", after.ScrollY)
	}

	ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
	snap := textSnapshotHelper(t, ctrl)
	if snap.ScrollY != 2 {
		t.Fatalf("expected ScrollY=2 after MoveUp, got %d", snap.ScrollY)
	}

	ctrl.Apply(app.Action{Kind: app.ActionMoveTop})
	top := textSnapshotHelper(t, ctrl)
	if top.ScrollY != 0 {
		t.Fatalf("expected ScrollY=0 after MoveTop, got %d", top.ScrollY)
	}

	ctrl.Apply(app.Action{Kind: app.ActionPageDown, N: 3})
	pd := textSnapshotHelper(t, ctrl)
	if pd.ScrollY != 3 {
		t.Fatalf("expected ScrollY=3 after PageDown(3), got %d", pd.ScrollY)
	}

	ctrl.Apply(app.Action{Kind: app.ActionPageUp, N: 2})
	pu := textSnapshotHelper(t, ctrl)
	if pu.ScrollY != 1 {
		t.Fatalf("expected ScrollY=1 after PageUp(2), got %d", pu.ScrollY)
	}
}

func TestPort_JSON_Search_CtrlPrecision(t *testing.T) {
	tuitest.NoColor(t)
	m := views.NewJSONWithCtrl(wave3CtrlInteractionResource(), "ec2", keys.Default(), nil)
	m.SetSize(80, 24)
	lines := m.ContentLines()
	ctrl := newTextScreenController(t, runtime.ScreenJSON, lines)

	ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: "instance"})
	snap := textSnapshotHelper(t, ctrl)
	if snap.Search != "instance" {
		t.Fatalf("expected Search=%q, got %q", "instance", snap.Search)
	}
	if len(snap.SearchMatches) == 0 {
		t.Fatal("expected SearchMatches populated after search in JSON")
	}

	var scratch views.JSONModel
	scratch.SetSize(80, 24)
	withSearch := scratch.RenderText(*snap)
	withoutSearch := scratch.RenderText(app.TextBody{Lines: lines})
	if withSearch == withoutSearch {
		t.Error("RenderText with search must differ from no-search output (JSON)")
	}
}

// textSnapshotHelper returns Snapshot().Body.Text, failing the test if nil.
func textSnapshotHelper(t *testing.T, ctrl *app.Controller) *app.TextBody {
	t.Helper()
	snap := ctrl.Snapshot()
	if snap.Body.Text == nil {
		t.Fatal("Snapshot().Body.Text is nil — screen was not pushed or EnsureTextState was not called")
	}
	return snap.Body.Text
}

// ── Real key-path pins: root model -> handleTextKeyMsg -> ctrl.Apply ───────

// wave3EnterYAML navigates a fresh root model to the live YAML screen for
// wave3CtrlInteractionResource via the real 'y' key from an ec2 list.
func wave3EnterYAML(t *testing.T, w, h int) tui.Model {
	t.Helper()
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: w, Height: h})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{wave3CtrlInteractionResource()}})
	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd == nil {
		t.Fatal("y on resource list should return a navigate command")
	}
	m, _ = rootApplyMsg(m, cmd())
	return m
}

// wave3EnterJSON is wave3EnterYAML's JSON counterpart, via the real 'J' key.
func wave3EnterJSON(t *testing.T, w, h int) tui.Model {
	t.Helper()
	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: w, Height: h})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{wave3CtrlInteractionResource()}})
	m, cmd := rootApplyMsg(m, rootKeyPress("J"))
	if cmd == nil {
		t.Fatal("J on resource list should return a navigate command")
	}
	m, _ = rootApplyMsg(m, cmd())
	return m
}

// wave3TypeSearch presses '/' then each rune of query then Enter, mirroring
// how a real user commits a search query on a text screen.
func wave3TypeSearch(m tui.Model, query string) tui.Model {
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range query {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	return m
}

func TestPort_YAML_ToggleWrap_LiveKeyPath(t *testing.T) {
	// 60 columns is the app's minimum width (below it every screen renders
	// the same "Terminal too narrow" message regardless of wrap state).
	m := wave3EnterYAML(t, 65, 15)

	before := stripANSI(rootViewContent(m))
	m, _ = rootApplyMsg(m, rootKeyPress("w"))
	after := stripANSI(rootViewContent(m))
	if before == after {
		t.Error("pressing 'w' on the live YAML screen must change the rendered view — " +
			"the long field should reflow once ActionToggleWrap reaches the controller")
	}
}

func TestPort_JSON_ToggleWrap_LiveKeyPath(t *testing.T) {
	m := wave3EnterJSON(t, 65, 15)

	before := stripANSI(rootViewContent(m))
	m, _ = rootApplyMsg(m, rootKeyPress("w"))
	after := stripANSI(rootViewContent(m))
	if before == after {
		t.Error("pressing 'w' on the live JSON screen must change the rendered view — " +
			"same handleTextKeyMsg ToggleWrap case as YAML, no kind-specific branch")
	}
}

func TestPort_YAML_SearchCommit_LiveKeyPath(t *testing.T) {
	m := wave3EnterYAML(t, 120, 40)

	before := stripANSI(rootViewContent(m))
	m = wave3TypeSearch(m, "match")
	after := stripANSI(rootViewContent(m))
	if before == after {
		t.Error("committing a search query on the live YAML screen must change the rendered view " +
			"(match highlighting) once ActionSearch reaches the controller")
	}
}

func TestPort_YAML_SearchNextPrev_LiveKeyPath(t *testing.T) {
	m := wave3EnterYAML(t, 120, 40)
	m = wave3TypeSearch(m, "match")
	afterCommit := stripANSI(rootViewContent(m))

	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	afterNext := stripANSI(rootViewContent(m))
	if afterNext == afterCommit {
		t.Error("pressing 'n' after a multi-match search on the live YAML screen must change the " +
			"rendered view once ActionSearchNext reaches the controller")
	}

	m, _ = rootApplyMsg(m, rootKeyPress("N"))
	afterPrev := stripANSI(rootViewContent(m))
	if afterPrev == afterNext {
		t.Error("pressing 'N' after 'n' on the live YAML screen must change the rendered view " +
			"once ActionSearchPrev reaches the controller")
	}
}

func TestPort_YAML_SearchClear_LiveKeyPath(t *testing.T) {
	m := wave3EnterYAML(t, 120, 40)
	before := stripANSI(rootViewContent(m))

	m = wave3TypeSearch(m, "match")
	withSearch := stripANSI(rootViewContent(m))
	if withSearch == before {
		t.Fatal("search commit did not change the view — cannot test clearing it")
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	cleared := stripANSI(rootViewContent(m))
	if cleared != before {
		t.Error("pressing Esc after a committed search on the live YAML screen must restore the " +
			"pre-search view once ActionSearchClear reaches the controller")
	}
}

func TestPort_YAML_Scroll_LiveKeyPath(t *testing.T) {
	m := wave3EnterYAML(t, 80, 8)

	before := stripANSI(rootViewContent(m))
	m, _ = rootApplyMsg(m, rootKeyPress("j"))
	m, _ = rootApplyMsg(m, rootKeyPress("j"))
	m, _ = rootApplyMsg(m, rootKeyPress("j"))
	after := stripANSI(rootViewContent(m))
	if before == after {
		t.Error("pressing 'j' 3 times on the live YAML screen must change the rendered view " +
			"once ActionMoveDown reaches the controller and the content window shifts")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// CONFIRMED REGRESSION, intentionally left RED: colorizeYAML (internal/tui/
// views/yaml.go) corrupts a quoted map key that itself contains a colon.
//
// yaml.Marshal correctly quotes a map key like "aws:autoscaling:groupName"
// (found via a real EC2 tag key while migrating qa_yaml_all_test.go this
// round — verified NOT a marshal bug: reverting to the dead RawContent(),
// which never colorizes, parses fine). colorizeYAML's line-coloring pass
// then mangles that quoted "key: value"-shaped key, and the stripped
// (ANSI-free) result is no longer valid YAML:
//
//	aws: autoscaling:groupName: acme-web-prod-asg
//
// instead of the marshaled:
//
//	"aws:autoscaling:groupName": acme-web-prod-asg
//
// This is exactly what a user sees on the live YAML screen (ContentLines()
// is the same colorizeYAML output View() renders) — cosmetic (the value
// isn't lost) but genuinely broken syntax highlighting/copy-paste fidelity.
// A coder fixes colorizeYAML's key-detection regex next; do not weaken this
// assertion to match the current corrupted output.
// ═══════════════════════════════════════════════════════════════════════════

func TestPort_ColorizeYAML_ColonInQuotedKey_Regression(t *testing.T) {
	res := resource.Resource{
		ID:   "wave3-colon-key-i-1",
		Name: "wave3-colon-key-instance",
		Fields: map[string]string{
			"aws:autoscaling:groupName": "acme-web-prod-asg",
		},
	}
	m := views.NewYAMLWithCtrl(res, "ec2", keys.Default(), nil)
	m.SetSize(80, 24)
	raw := stripANSI(strings.Join(m.ContentLines(), "\n"))

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("colorized YAML failed to re-parse: %v\n---\n%s", err, raw)
	}
	if got, want := parsed["aws:autoscaling:groupName"], "acme-web-prod-asg"; got != want {
		t.Errorf("colon-bearing key did not survive colorizing verbatim: got %v, want %q", got, want)
	}
}

// splitYAMLKeyValue must treat a `"` as escaped by backslash-run PARITY (odd
// = escaped, even = real close), not by a one-char lookback. The trigger needs
// a control character forcing yaml.v3 double-quoting — synthetic for AWS data,
// but the contract stripANSI(colorize(x)) == x must hold for all inputs.

func TestPort_ColorizeYAML_EvenBackslashRunBeforeQuote_Regression(t *testing.T) {
	res := resource.Resource{
		ID:   "wave3-backslash-key-i-1",
		Name: "wave3-backslash-key-instance",
		Fields: map[string]string{
			"foo\t\\": "bar",
		},
	}
	m := views.NewYAMLWithCtrl(res, "ec2", keys.Default(), nil)
	m.SetSize(80, 24)
	raw := stripANSI(strings.Join(m.ContentLines(), "\n"))

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("colorized YAML failed to re-parse: %v\n---\n%s", err, raw)
	}
	if got, want := parsed["foo\t\\"], "bar"; got != want {
		t.Errorf("key ending in an escaped literal backslash did not survive colorizing verbatim: got %v, want %q\n---\n%s", got, want, raw)
	}
}
