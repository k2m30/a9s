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
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// livegapStep sends msg through tui.Model.Update and type-asserts the result
// back to tui.Model.
func livegapStep(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	return tuitest.Step(m, msg)
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
// wired with in-process demo fake clients (demo.NewServiceClients()): a
// clientless model never sets ClientsReady, so paths gated on it (costs data
// fetch, resource lookups) would never run.
func livegapModel(t *testing.T) tui.Model {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	m := newBlessedModel(t, demo.DemoProfile, demo.DemoRegion,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	t.Cleanup(m.CloseController)
	m, _ = livegapStep(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// livegapThemesModel is livegapModel plus a themes/ directory pre-populated
// with the given theme file names, for driving the theme selector. activeTheme
// == "" omits WithActiveTheme so the default takes effect. The literal
// tui.WithNoCache(true)/tui.WithIsDemo(true) arguments match the
// construction-discipline gate's literal-detection exemption
// (qa_controller_construction_discipline_test.go).
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
		m = newBlessedModel(t, demo.DemoProfile, demo.DemoRegion,
			tui.WithClients(demo.NewServiceClients()),
			tui.WithIsDemo(true),
			tui.WithActiveTheme(activeTheme),
			tui.WithNoCache(true),
			tui.WithProfileForTest(demo.DemoProfile),
			tui.WithRegionForTest(demo.DemoRegion))
	} else {
		m = newBlessedModel(t, demo.DemoProfile, demo.DemoRegion,
			tui.WithClients(demo.NewServiceClients()),
			tui.WithIsDemo(true),
			tui.WithNoCache(true),
			tui.WithProfileForTest(demo.DemoProfile),
			tui.WithRegionForTest(demo.DemoRegion))
	}
	t.Cleanup(m.CloseController)
	m, _ = livegapStep(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

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

type livegapRawStruct struct {
	Name string
	N    int
}

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
// stripped of ANSI.
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
		return resource.KnownRelated("ec2", ids, false)
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

// A related-ID filter (only i-match passes) is active first: both resources
// are visible before any filter, so only a filtered state shows whether a
// nil setter cleared ls.RelatedIDSet.
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
		return resource.KnownRelated("ec2", ids, false)
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

func TestLiveGap_HandleDetailKeyMsg_RelatedPanelEnter_OnActionableRow_NavigatesUsingControllerRow(t *testing.T) {
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	// Resolve exactly one EC2 related row ("Security Groups") to an
	// actionable, drillable state; every other row (InitDetailRelatedRows
	// seeds all of them Loading on navigate) stays non-drillable, so
	// focusing the right column below auto-lands the cursor here
	// (ActionToggleFocus's detailSkipToDrillable).
	m, _ = chainApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: ec2TestResource().ID,
		DefDisplayName:   "Security Groups",
		Result:           resource.KnownRelated("sg", []string{"sg-0aaa111111111111a"}, false),
	})

	// 'l' (ScrollRight) focuses the right column via ActionToggleFocus.
	m, _ = chainApplyMsg(m, livegapKey("l"))

	_, cmd := chainApplyMsg(m, livegapSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter on the focused, actionable related row returned a nil cmd, want messages.RelatedNavigate")
	}
	nav, ok := cmd().(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("cmd() = %T, want messages.RelatedNavigate", nav)
	}
	if nav.TargetType != "sg" {
		t.Errorf("RelatedNavigate.TargetType = %q, want %q", nav.TargetType, "sg")
	}
	if len(nav.RelatedIDs) != 1 || nav.RelatedIDs[0] != "sg-0aaa111111111111a" {
		t.Errorf("RelatedNavigate.RelatedIDs = %v, want [sg-0aaa111111111111a]", nav.RelatedIDs)
	}
	if nav.TargetID != "sg-0aaa111111111111a" {
		t.Errorf("RelatedNavigate.TargetID = %q, want %q (single-ID fast path)", nav.TargetID, "sg-0aaa111111111111a")
	}
	if nav.SourceResource.ID != ec2TestResource().ID {
		t.Errorf("RelatedNavigate.SourceResource.ID = %q, want %q", nav.SourceResource.ID, ec2TestResource().ID)
	}
	if nav.SourceType != "ec2" {
		t.Errorf("RelatedNavigate.SourceType = %q, want %q", nav.SourceType, "ec2")
	}
}

// While the RELATED filter is active, Down moves DetailState.RelatedCursor.
// The first Enter only confirms the filter text and stays on the detail
// screen (the list-filter convention, also driven by
// TestDemoScenarioHarness_DetailRelatedAndYAMLSearch); a second Enter
// navigates to the row under the cursor.

// relatedFilterGapModel builds an EC2 detail with two RELATED rows
// ("Target Groups" and "Auto Scaling Groups") resolved to a single
// actionable result each, focuses the RELATED panel, and activates the
// filter with a query ("groups") that matches both plus three still-Loading
// (non-actionable, hence skipped-over) rows — "EKS Node Groups", "Security
// Groups", "Log Groups". extraKeys are sent after the filter text (e.g. a
// Down press), before the caller drives Enter itself.
func relatedFilterGapModel(t *testing.T, extraKeys ...tea.KeyMsg) tui.Model {
	t.Helper()
	m := newChainDemoModel(t)
	m = chainNavigateToEC2Detail(t, m)

	m, _ = chainApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: ec2TestResource().ID,
		DefDisplayName:   "Target Groups",
		Result:           resource.KnownRelated("tg", []string{"tg-web-prod-01"}, false),
	})
	m, _ = chainApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: ec2TestResource().ID,
		DefDisplayName:   "Auto Scaling Groups",
		Result:           resource.KnownRelated("asg", []string{"asg-web-prod-01"}, false),
	})

	m, _ = chainApplyMsg(m, livegapKey("l")) // ScrollRight: focuses the right column
	m, _ = chainApplyMsg(m, livegapKey("/")) // Search: activates the filter
	for _, ch := range "groups" {
		m, _ = chainApplyMsg(m, livegapKey(string(ch)))
	}
	for _, k := range extraKeys {
		m, _ = chainApplyMsg(m, k)
	}
	return m
}

func TestLiveGap_HandleDetailKeyMsg_RelatedPanelFilterMode_EnterConfirmsFilterWithoutNavigating(t *testing.T) {
	m := relatedFilterGapModel(t)

	m, cmd := chainApplyMsg(m, livegapSpecialKey(tea.KeyEnter))
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if nav, ok := msg.(messages.RelatedNavigate); ok {
				t.Errorf("Enter #1 while filtering returned messages.RelatedNavigate{TargetType:%q}, want no navigation — Enter must only confirm the filter and stay on the detail screen", nav.TargetType)
			}
			m, _ = chainApplyMsg(m, msg)
		}
	}

	res, ok := m.ActiveDetailResource()
	if !ok {
		t.Fatal("expected the detail screen to still be active after the filter-confirm Enter, got no active detail resource")
	}
	if res.ID != ec2TestResource().ID {
		t.Errorf("ActiveDetailResource().ID = %q, want %q (filter-confirm Enter must not navigate away from the source EC2 detail)", res.ID, ec2TestResource().ID)
	}
}

// The first Enter's cmd is not run here, so the test isolates cursor
// survival across the confirm from navigation by the first Enter.
func TestLiveGap_HandleDetailKeyMsg_RelatedPanelFilterMode_SecondEnterNavigatesToSurvivingCursorRow(t *testing.T) {
	cases := []struct {
		name           string
		down           bool
		wantTargetType string
		wantID         string
	}{
		{name: "NoDown_FirstMatch", down: false, wantTargetType: "tg", wantID: "tg-web-prod-01"},
		{name: "AfterDown_SecondMatch", down: true, wantTargetType: "asg", wantID: "asg-web-prod-01"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m tui.Model
			if tc.down {
				m = relatedFilterGapModel(t, livegapSpecialKey(tea.KeyDown))
			} else {
				m = relatedFilterGapModel(t)
			}

			m, _ = chainApplyMsg(m, livegapSpecialKey(tea.KeyEnter)) // Enter #1: confirm

			_, cmd := chainApplyMsg(m, livegapSpecialKey(tea.KeyEnter)) // Enter #2: navigate
			if cmd == nil {
				t.Fatal("Enter #2 (no longer filtering) returned a nil cmd, want messages.RelatedNavigate for the row under the surviving cursor")
			}
			nav, ok := cmd().(messages.RelatedNavigate)
			if !ok {
				t.Fatalf("cmd() = %T, want messages.RelatedNavigate", nav)
			}
			if nav.TargetType != tc.wantTargetType {
				t.Errorf("RelatedNavigate.TargetType = %q, want %q", nav.TargetType, tc.wantTargetType)
			}
			if len(nav.RelatedIDs) != 1 || nav.RelatedIDs[0] != tc.wantID {
				t.Errorf("RelatedNavigate.RelatedIDs = %v, want [%s]", nav.RelatedIDs, tc.wantID)
			}
			if nav.TargetID != tc.wantID {
				t.Errorf("RelatedNavigate.TargetID = %q, want %q", nav.TargetID, tc.wantID)
			}
		})
	}
}

// Production always sets DefDisplayName; without it, mergeDetailRelatedRow
// (core/app/handle.go) binds by TargetType only when exactly one row
// carries it.

func TestLiveGap_ApplyDetailRelatedResultForResource_AmbiguousTargetTypeWithoutDisplayName_NoBind(t *testing.T) {
	res := resource.Resource{ID: "evt-livegap-ambiguous-0001", Name: "evt-livegap-ambiguous-0001"}
	c := newDetailController(t, res, "ct-events")

	rows := []app.DetailRelatedRow{
		{TargetType: "ct-events", DisplayName: "CT events by AccessKeyId", State: domain.RelatedLoading, Loading: true},
		{TargetType: "ct-events", DisplayName: "CT events by Username", State: domain.RelatedLoading, Loading: true},
	}
	c.ApplyDetailRelated(rows)

	c.ApplyDetailRelatedResultForResource("ct-events", res.ID, "", "ct-events", domain.RelatedResolved, 5, false, "", false, nil, nil, "")

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	if len(body.Related) != 2 {
		t.Fatalf("expected exactly 2 related rows, got %d", len(body.Related))
	}
	for _, rb := range body.Related {
		if !rb.Loading {
			t.Errorf("row %q resolved to non-loading after an ambiguous DefDisplayName-less result; the ambiguous match must be refused", rb.Name)
		}
		if rb.Count != 0 {
			t.Errorf("row %q Count = %d, want 0 (unbound, untouched)", rb.Name, rb.Count)
		}
	}
}

func TestLiveGap_ApplyDetailRelatedResultForResource_UnambiguousTargetTypeFallback_Binds(t *testing.T) {
	res := resource.Resource{ID: "i-livegap-unambiguous-0001", Name: "i-livegap-unambiguous-0001"}
	c := newDetailController(t, res, "ec2")

	rows := []app.DetailRelatedRow{
		{TargetType: "tg", DisplayName: "Target Groups", State: domain.RelatedLoading, Loading: true},
	}
	c.ApplyDetailRelated(rows)

	// DefDisplayName omitted, but exactly one row carries TargetType "tg" —
	// the tight fallback must bind it.
	c.ApplyDetailRelatedResultForResource("ec2", res.ID, "", "tg", domain.RelatedResolved, 7, false, "", false, []string{"tg-1"}, nil, "")

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	if len(body.Related) != 1 {
		t.Fatalf("expected exactly 1 related row, got %d", len(body.Related))
	}
	rb := body.Related[0]
	if rb.Loading {
		t.Error("row still Loading after the unique-TargetType fallback should have resolved it")
	}
	if rb.Count != 7 {
		t.Errorf("row.Count = %d, want 7", rb.Count)
	}
}

func TestLiveGap_RelatedCursor_FilterMode_UpDown_MoveWithinFilteredSet(t *testing.T) {
	res := resource.Resource{ID: "i-livegap-filtercursor-0001", Name: "i-livegap-filtercursor-0001"}
	c := newDetailController(t, res, "ec2")

	rows := []app.DetailRelatedRow{
		{TargetType: "tg", DisplayName: "Target Groups A", State: domain.RelatedResolved, Count: 1},
		{TargetType: "tg2", DisplayName: "Target Groups B", State: domain.RelatedResolved, Count: 1},
		{TargetType: "sg", DisplayName: "Security Groups", State: domain.RelatedResolved, Count: 1},
	}
	c.SetDetailRelatedVisible(true, false)
	c.ApplyDetailRelated(rows)
	c.Apply(app.Action{Kind: app.ActionToggleFocus})
	vs, _ := c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "Target"})

	if len(vs.Body.Detail.Related) != 2 {
		t.Fatalf("precondition: filtering on %q should leave 2 rows visible, got %d", "Target", len(vs.Body.Detail.Related))
	}
	if got := vs.Body.Detail.Related[vs.Body.Detail.RelatedCursor].Name; got != "Target Groups A" {
		t.Fatalf("precondition: RelatedCursor after ActionSetFilter = %q, want %q", got, "Target Groups A")
	}

	vs, _ = c.Apply(app.Action{Kind: app.ActionMoveDown})
	if got := vs.Body.Detail.Related[vs.Body.Detail.RelatedCursor].Name; got != "Target Groups B" {
		t.Errorf("MoveDown inside filter mode = %q, want %q", got, "Target Groups B")
	}

	// A further Down must stay clamped to the filtered set (2 rows), never
	// leaking into the filtered-out "Security Groups" row.
	vs, _ = c.Apply(app.Action{Kind: app.ActionMoveDown})
	if got := vs.Body.Detail.Related[vs.Body.Detail.RelatedCursor].Name; got != "Target Groups B" {
		t.Errorf("MoveDown past the last filtered row = %q, want to stay clamped at %q (Security Groups is filtered out)", got, "Target Groups B")
	}

	vs, _ = c.Apply(app.Action{Kind: app.ActionMoveUp})
	if got := vs.Body.Detail.Related[vs.Body.Detail.RelatedCursor].Name; got != "Target Groups A" {
		t.Errorf("MoveUp inside filter mode = %q, want %q", got, "Target Groups A")
	}
}
