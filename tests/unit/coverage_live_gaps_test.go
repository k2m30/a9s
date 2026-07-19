// coverage_live_gaps_test.go — black-box tests for live, production-reachable
// exported/semi-exported surface that carried low coverage: handleCostsKeyMsg
// and handleDetailKeyMsg (driven through the real key-routing chain via
// tui.Model.Update), WithActiveTheme, ActiveDetailResource, RawYAML,
// RawYAMLFromResource, SetReapplyChecker, and SelectedTypeName.
//
// Reuses the chain* helpers from ec2_stories_nav_chains_test.go (same package)
// for navigation into a live EC2 detail screen.
package unit_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

// livegapStep sends msg through tui.Model.Update and type-asserts the result
// back to tui.Model.
func livegapStep(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tui.Model), cmd
}

// livegapKey builds a printable-character key press.
func livegapKey(ch string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: ch}
}

// livegapSpecialKey builds a non-printable key press (e.g. tea.KeyEscape).
func livegapSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// livegapModel builds a sized, isolated tui.Model (its own A9S_CONFIG_FOLDER)
// wired with real demo fake clients (demo.NewServiceClients(), the same
// shared demo-fake harness newChainDemoModel/newPreviewDemoModel use) rather
// than a clientless model. A clientless model never sets ClientsReady, so any
// live path gated on it (costs data fetch, resource lookups) would silently
// go unexercised by every test built on this helper; wiring real (fake, in-
// process) clients closes that gap while staying fully hermetic — no network
// calls, no real AWS credentials.
func livegapModel(t *testing.T) tui.Model {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	m := tui.New(demo.DemoProfile, demo.DemoRegion,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	t.Cleanup(m.CloseController)
	m, _ = livegapStep(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// livegapThemesModel is livegapModel plus a themes/ directory pre-populated
// with the given theme file names, for driving the theme selector. activeTheme
// == "" omits WithActiveTheme so the default takes effect. Also wired with
// real demo fake clients (see livegapModel's doc comment) instead of a
// clientless model; both call expressions keep literal tui.WithNoCache(true)/
// tui.WithIsDemo(true) arguments (rather than building the option slice
// dynamically) to match the construction-discipline gate's literal-detection
// exemption (qa_controller_construction_discipline_test.go).
func livegapThemesModel(t *testing.T, activeTheme string, themeFiles ...string) tui.Model {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll themes dir: %v", err)
	}
	for _, name := range themeFiles {
		if err := os.WriteFile(filepath.Join(themesDir, name), []byte("name: "+name+"\n"), 0o644); err != nil {
			t.Fatalf("writing theme file %s: %v", name, err)
		}
	}
	var m tui.Model
	if activeTheme != "" {
		m = tui.New(demo.DemoProfile, demo.DemoRegion,
			tui.WithClients(demo.NewServiceClients()),
			tui.WithIsDemo(true),
			tui.WithActiveTheme(activeTheme),
			tui.WithNoCache(true),
			tui.WithProfile(demo.DemoProfile),
			tui.WithRegion(demo.DemoRegion))
	} else {
		m = tui.New(demo.DemoProfile, demo.DemoRegion,
			tui.WithClients(demo.NewServiceClients()),
			tui.WithIsDemo(true),
			tui.WithNoCache(true),
			tui.WithProfile(demo.DemoProfile),
			tui.WithRegion(demo.DemoRegion))
	}
	t.Cleanup(m.CloseController)
	m, _ = livegapStep(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// ---------------------------------------------------------------------------
// handleCostsKeyMsg (app_costs.go)
// ---------------------------------------------------------------------------

// wantCostsFrameTitle asserts the exact "Costs: by <pivot> · <metric> ·
// <granularity>" breadcrumb (costsFrameTitle, core/app/costs_body.go) appears
// verbatim in the rendered screen — not just "something changed" — so a
// wrong action (e.g. a pivot key accidentally cycling the metric) fails.
func wantCostsFrameTitle(t *testing.T, plain, want string) {
	t.Helper()
	if !strings.Contains(plain, want) {
		t.Errorf("rendered costs screen missing frame title %q, got:\n%s", want, plain)
	}
}

func TestLiveGap_HandleCostsKeyMsg_MetricKeyChangesRenderedScreen(t *testing.T) {
	m := livegapModel(t)
	m, _ = livegapStep(m, messages.Navigate{Target: messages.TargetCosts})

	before := stripAnsi(m.View().Content)
	wantCostsFrameTitle(t, before, "Costs: by service · invoice · month")

	m, _ = livegapStep(m, livegapKey("b")) // CostMetric: cycle invoice -> unblended
	after := stripAnsi(m.View().Content)
	wantCostsFrameTitle(t, after, "Costs: by service · unblended · month")
}

func TestLiveGap_HandleCostsKeyMsg_PivotDigitChangesRenderedScreen(t *testing.T) {
	m := livegapModel(t)
	m, _ = livegapStep(m, messages.Navigate{Target: messages.TargetCosts})

	before := stripAnsi(m.View().Content)
	wantCostsFrameTitle(t, before, "Costs: by service · invoice · month")

	m, _ = livegapStep(m, livegapKey("2")) // CostPivot[1] -> pivot region
	after := stripAnsi(m.View().Content)
	wantCostsFrameTitle(t, after, "Costs: by region · invoice · month")
}

func TestLiveGap_HandleCostsKeyMsg_ZoomInChangesRenderedScreen(t *testing.T) {
	m := livegapModel(t)
	m, _ = livegapStep(m, messages.Navigate{Target: messages.TargetCosts})

	before := stripAnsi(m.View().Content)
	wantCostsFrameTitle(t, before, "Costs: by service · invoice · month")

	m, _ = livegapStep(m, livegapKey("+")) // CostZoomIn: month -> week (granularityChain)
	after := stripAnsi(m.View().Content)
	wantCostsFrameTitle(t, after, "Costs: by service · invoice · week")
}

// Edge case: movement/scroll keys on an empty grid (no CostsLoaded delivered
// yet) must not panic and must leave the costs screen rendered.
func TestLiveGap_HandleCostsKeyMsg_MovementKeysOnEmptyGrid_NoCrash(t *testing.T) {
	m := livegapModel(t)
	m, _ = livegapStep(m, messages.Navigate{Target: messages.TargetCosts})

	for _, k := range []tea.KeyPressMsg{
		livegapSpecialKey(tea.KeyDown),
		livegapSpecialKey(tea.KeyUp),
		livegapKey("h"),
		livegapKey("l"),
	} {
		m, _ = livegapStep(m, k)
	}

	plain := stripAnsi(m.View().Content)
	if !strings.Contains(plain, "Costs") {
		t.Errorf("costs screen should still be rendered after movement keys on an empty grid, got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// handleDetailKeyMsg (app_stack.go)
// ---------------------------------------------------------------------------

func TestLiveGap_HandleDetailKeyMsg_YAMLKey_NavigatesToYAMLTarget(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	_, cmd := chainApplyMsg(m, livegapKey("y"))
	if cmd == nil {
		t.Fatal("'y' on the detail screen returned a nil cmd; want messages.Navigate to TargetYAML")
	}
	nav, ok := cmd().(messages.Navigate)
	if !ok {
		t.Fatalf("cmd() = %T, want messages.Navigate", nav)
	}
	if nav.Target != messages.TargetYAML {
		t.Errorf("Navigate.Target = %v, want TargetYAML", nav.Target)
	}
	if nav.Resource == nil || nav.Resource.ID != ec2TestResource().ID {
		t.Errorf("Navigate.Resource = %+v, want the active EC2 detail resource (ID=%q)", nav.Resource, ec2TestResource().ID)
	}
}

func TestLiveGap_HandleDetailKeyMsg_JSONKey_NavigatesToJSONTarget(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	_, cmd := chainApplyMsg(m, livegapKey("J"))
	if cmd == nil {
		t.Fatal("'J' on the detail screen returned a nil cmd; want messages.Navigate to TargetJSON")
	}
	nav, ok := cmd().(messages.Navigate)
	if !ok || nav.Target != messages.TargetJSON {
		t.Errorf("cmd() = %#v, want messages.Navigate{Target: TargetJSON}", nav)
	}
}

func TestLiveGap_HandleDetailKeyMsg_CloudTrailKey_NavigatesWithResourceNameFilter(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	_, cmd := chainApplyMsg(m, livegapKey("t"))
	if cmd == nil {
		t.Fatal("'t' on the detail screen returned a nil cmd; want messages.RelatedNavigate for ct-events")
	}
	nav, ok := cmd().(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("cmd() = %T, want messages.RelatedNavigate", nav)
	}
	if nav.TargetType != "ct-events" {
		t.Errorf("RelatedNavigate.TargetType = %q, want %q", nav.TargetType, "ct-events")
	}
	if nav.FetchFilter["ResourceName"] != ec2TestResource().ID {
		t.Errorf("RelatedNavigate.FetchFilter[\"ResourceName\"] = %q, want %q", nav.FetchFilter["ResourceName"], ec2TestResource().ID)
	}
}

// Edge case: Escape while the detail search is active (confirmed, not typing)
// must clear the search WITHOUT popping the detail screen — a second Escape
// (search now cleared) pops it, proving the first one really cleared it
// rather than being silently swallowed.
func TestLiveGap_HandleDetailKeyMsg_EscapeWithActiveSearch_ClearsSearchWithoutPoppingScreen(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	m, _ = chainApplyMsg(m, livegapKey("/"))                 // activate search (input mode)
	m, _ = chainApplyMsg(m, livegapKey("r"))                 // type a query character
	m, _ = chainApplyMsg(m, livegapSpecialKey(tea.KeyEnter)) // confirm: exits input mode, search becomes "active"

	newM, cmd := chainApplyMsg(m, livegapSpecialKey(tea.KeyEscape))
	if cmd != nil {
		t.Errorf("Escape while search is active should return a nil cmd, got %#v", cmd())
	}
	plain := stripAnsi(chainViewContent(newM))
	if !strings.Contains(plain, ec2TestResource().Name) {
		t.Fatalf("first Escape (clearing an active search) must NOT pop the detail screen, got:\n%s", plain)
	}

	newM2, _ := chainApplyMsg(newM, livegapSpecialKey(tea.KeyEscape))
	plain2 := stripAnsi(chainViewContent(newM2))
	if strings.Contains(plain2, ec2TestResource().Name) {
		t.Errorf("second Escape (search already cleared) should pop the detail screen, got:\n%s", plain2)
	}
}

// ---------------------------------------------------------------------------
// WithActiveTheme (app_options.go)
// ---------------------------------------------------------------------------

func TestLiveGap_WithActiveTheme_MarksNamedThemeAsCurrentInSelector(t *testing.T) {
	m := livegapThemesModel(t, "dracula.yaml", "tokyo-night.yaml", "dracula.yaml")
	m, _ = livegapStep(m, messages.Navigate{Target: messages.TargetTheme})

	plain := stripAnsi(m.View().Content)
	var draculaLine, tokyoLine string
	for _, l := range strings.Split(plain, "\n") {
		switch {
		case strings.Contains(l, "dracula.yaml"):
			draculaLine = l
		case strings.Contains(l, "tokyo-night.yaml"):
			tokyoLine = l
		}
	}
	if draculaLine == "" {
		t.Fatalf("theme selector does not list dracula.yaml, got:\n%s", plain)
	}
	if !strings.Contains(draculaLine, "(current)") {
		t.Errorf("WithActiveTheme(%q): expected \"(current)\" marker on its line, got:\n%s", "dracula.yaml", draculaLine)
	}
	if tokyoLine != "" && strings.Contains(tokyoLine, "(current)") {
		t.Errorf("tokyo-night.yaml must NOT be marked current when WithActiveTheme(%q) was set, got:\n%s", "dracula.yaml", tokyoLine)
	}
}

func TestLiveGap_WithActiveTheme_DefaultsToTokyoNightWhenOptionOmitted(t *testing.T) {
	m := livegapThemesModel(t, "", "tokyo-night.yaml", "dracula.yaml") // WithActiveTheme not passed
	m, _ = livegapStep(m, messages.Navigate{Target: messages.TargetTheme})

	plain := stripAnsi(m.View().Content)
	var tokyoLine string
	for _, l := range strings.Split(plain, "\n") {
		if strings.Contains(l, "tokyo-night.yaml") {
			tokyoLine = l
			break
		}
	}
	if tokyoLine == "" || !strings.Contains(tokyoLine, "(current)") {
		t.Errorf("default active theme (no WithActiveTheme option) should mark tokyo-night.yaml \"(current)\", got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// ActiveDetailResource (app_accessors.go)
// ---------------------------------------------------------------------------

func TestLiveGap_ActiveDetailResource_ReturnsResourceOnDetailScreen(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	res, ok := m.ActiveDetailResource()
	if !ok {
		t.Fatal("ActiveDetailResource() ok=false while a detail screen is active")
	}
	if res.ID != ec2TestResource().ID {
		t.Errorf("ActiveDetailResource().ID = %q, want %q", res.ID, ec2TestResource().ID)
	}
}

func TestLiveGap_ActiveDetailResource_FalseWhenNotOnDetailScreen(t *testing.T) {
	m := newChainDemoModel(t) // still on the main menu

	if _, ok := m.ActiveDetailResource(); ok {
		t.Error("ActiveDetailResource() ok=true while on the main menu; want false")
	}
}

// ---------------------------------------------------------------------------
// RawYAML / RawYAMLFromResource (detail_render.go / transient.go)
// ---------------------------------------------------------------------------

type livegapRawStruct struct {
	Name string
	N    int
}

// TestLiveGap_RawYAML_MarshalsRawStructWhenPresent and
// TestLiveGap_RawYAML_EmptyWhenNoDataAtAll deleted (round 5,
// specs/022-codebase-cleanup, DetailModel core cleanup): both drove the
// retired views.NewDetail(...).RawYAML() construction path (NewDetail is
// dead) to exercise the exact struct-present/empty-data cases already
// pinned below via views.RawYAMLFromResource (a live, construction-free
// package function RawYAML delegates to — see
// TestLiveGap_RawYAMLFromResource_MirrorsRawYAML).

// TestLiveGap_RawYAML_MarshalsFieldsWhenNoRawStruct is the live-seam
// replacement for the retired views.NewDetail(...).RawYAML() call: the
// Fields-only (no RawStruct) case isn't covered by the RawYAMLFromResource
// tests below, so it's ported onto that construction-free function instead.
func TestLiveGap_RawYAML_MarshalsFieldsWhenNoRawStruct(t *testing.T) {
	res := resource.Resource{ID: "res-2", Fields: map[string]string{"Alpha": "one"}}

	got := views.RawYAMLFromResource(res)
	if !strings.Contains(got, "Alpha") || !strings.Contains(got, "one") {
		t.Errorf("RawYAMLFromResource() with Fields set (no RawStruct) = %q, want it to contain the Fields map", got)
	}
}

func TestLiveGap_RawYAMLFromResource_MirrorsRawYAML(t *testing.T) {
	res := resource.Resource{ID: "res-4", RawStruct: &livegapRawStruct{Name: "gadget", N: 3}}

	got := views.RawYAMLFromResource(res)
	if !strings.Contains(got, "gadget") || !strings.Contains(got, "3") {
		t.Errorf("RawYAMLFromResource() = %q, want it to contain the marshaled struct fields", got)
	}
}

func TestLiveGap_RawYAMLFromResource_EmptyWhenNoDataAtAll(t *testing.T) {
	res := resource.Resource{ID: "res-5"}

	if got := views.RawYAMLFromResource(res); got != "" {
		t.Errorf("RawYAMLFromResource() with no data = %q, want empty string", got)
	}
}

// ---------------------------------------------------------------------------
// SetReapplyChecker (resourcelist_helpers.go)
// ---------------------------------------------------------------------------

// livegapReapplyResources are the two rows fed to the checker/list body — IDs
// match the checker's own filter below ("i-match" survives, "i-other" is
// dropped) and carry realistic ec2 Fields (mirrors ec2TestResources,
// qa_pagination_root_test.go) so RenderList renders a real row, not an empty
// placeholder.
func livegapReapplyResources() []resource.Resource {
	mk := func(id string) resource.Resource {
		return resource.Resource{
			ID:   id,
			Name: id,
			Fields: map[string]string{
				"instance_id":   id,
				"instance_type": "t3.micro",
				"state":         "running",
				"name":          id,
			},
		}
	}
	return []resource.Resource{mk("i-match"), mk("i-other")}
}

// livegapRenderVisibleList renders the top list screen's current ListBody
// through the real production RenderList seam (views.NewTransientResourceList
// + RenderList, mirrored from renderListRaw in phase03_view_reads_test.go),
// stripped of ANSI — the "what the user sees" surface findings #2/#7/#15
// require instead of asserting on internal Controller state.
func livegapRenderVisibleList(t *testing.T, ctrl *app.Controller, td resource.ResourceTypeDef) string {
	t.Helper()
	lb := ctrl.Snapshot().Body.List
	if lb == nil {
		t.Fatal("expected a non-nil list body")
	}
	rm := views.NewTransientResourceList(td, 120, 30)
	return stripAnsi(rm.RenderList(*lb))
}

func TestLiveGap_SetReapplyChecker_RegistersCheckerAndActivatesZeroMatchFilter(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 type def not registered — update this test if the short name changed")
	}
	ctrl := newTestController(t)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	m := views.NewResourceList(*td, nil, keys.Default(), ctrl)

	resources := livegapReapplyResources()
	ctrl.ApplyResourcesLoaded("ec2", resources, nil, false)

	before := livegapRenderVisibleList(t, ctrl, *td)
	if !strings.Contains(before, "i-match") || !strings.Contains(before, "i-other") {
		t.Fatalf("precondition: both i-match and i-other should render before any related filter, got:\n%s", before)
	}

	checker := func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
		var ids []string
		for _, r := range cache["ec2"].Resources {
			if r.ID == "i-match" {
				ids = append(ids, r.ID)
			}
		}
		return resource.RelatedCheckResult{TargetType: "ec2", ResourceIDs: ids}
	}

	m.SetReapplyChecker(checker, resource.Resource{ID: "vpc-source"})

	// SetReapplyChecker activates a zero-match filter before the checker has
	// actually run: every row must be hidden from the rendered list.
	zeroMatch := livegapRenderVisibleList(t, ctrl, *td)
	if strings.Contains(zeroMatch, "i-match") || strings.Contains(zeroMatch, "i-other") {
		t.Errorf("zero-match filter (checker armed, not yet run) must hide every row from the rendered list, got:\n%s", zeroMatch)
	}

	ctrl.ApplyReapplyCheckerAgainst(resources)

	after := livegapRenderVisibleList(t, ctrl, *td)
	if !strings.Contains(after, "i-match") {
		t.Errorf("i-match must still render after the reapply filter activates, got:\n%s", after)
	}
	if strings.Contains(after, "i-other") {
		t.Errorf("i-other must be filtered out of the rendered list after the reapply filter activates, got:\n%s", after)
	}
}

// TestLiveGap_SetReapplyChecker_NilCheckerLeavesRelatedIDSetUntouched first
// activates a real related-ID filter (only i-match passes) so the assertion
// below can distinguish "nil is a no-op" from "nil silently clears the
// active filter" — both resources start visible before any filter, so
// asserting on that unfiltered state would pass even if the nil setter
// wrongly cleared ls.RelatedIDSet.
func TestLiveGap_SetReapplyChecker_NilCheckerLeavesRelatedIDSetUntouched(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 type def not registered — update this test if the short name changed")
	}
	ctrl := newTestController(t)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	m := views.NewResourceList(*td, nil, keys.Default(), ctrl)

	resources := livegapReapplyResources()
	ctrl.ApplyResourcesLoaded("ec2", resources, nil, false)

	checker := func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
		var ids []string
		for _, r := range cache["ec2"].Resources {
			if r.ID == "i-match" {
				ids = append(ids, r.ID)
			}
		}
		return resource.RelatedCheckResult{TargetType: "ec2", ResourceIDs: ids}
	}
	m.SetReapplyChecker(checker, resource.Resource{ID: "vpc-source"})
	ctrl.ApplyReapplyCheckerAgainst(resources)

	filtered := livegapRenderVisibleList(t, ctrl, *td)
	if !strings.Contains(filtered, "i-match") || strings.Contains(filtered, "i-other") {
		t.Fatalf("precondition: reapply filter should leave only i-match visible, got:\n%s", filtered)
	}

	m.SetReapplyChecker(nil, resource.Resource{})

	got := livegapRenderVisibleList(t, ctrl, *td)
	if !strings.Contains(got, "i-match") {
		t.Errorf("SetReapplyChecker(nil, ...) must leave the active filter's matched row (i-match) visible, got:\n%s", got)
	}
	if strings.Contains(got, "i-other") {
		t.Errorf("SetReapplyChecker(nil, ...) must not clear the active related-ID filter (i-other should stay hidden), got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// SelectedTypeName (rightcolumn.go)
// ---------------------------------------------------------------------------

func TestLiveGap_SelectedTypeName_ReturnsDisplayNameOfSelectedRow(t *testing.T) {
	defs := []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups"},
		{TargetType: "subnet", DisplayName: "Subnets"},
	}
	m := views.NewRightColumn(defs, resource.Resource{ID: "i-1"}, "ec2")

	if got := m.SelectedTypeName(); got != "Security Groups" {
		t.Errorf("SelectedTypeName() = %q, want %q (first row, default cursor)", got, "Security Groups")
	}
}

func TestLiveGap_SelectedTypeName_EmptyWhenNoRows(t *testing.T) {
	m := views.NewRightColumn(nil, resource.Resource{ID: "i-1"}, "ec2")

	if got := m.SelectedTypeName(); got != "" {
		t.Errorf("SelectedTypeName() with no related defs = %q, want empty string", got)
	}
}
