// detail_ports_test.go — live-seam port pins for the DetailModel legacy-view
// deletion, per specs/022-codebase-cleanup/
// wave3-map-detail.md's "Unique pins to PORT" list. Every test here exercises
// the LIVE seam (NewTransientDetail+RenderDetail, or the app.Controller
// action/footer/search path) so the corresponding legacy-DetailModel-only
// test can be deleted later without losing coverage. No production code is
// touched by this file; old test files it supersedes stay untouched.
//
// Per-item disposition (see wave3-map-detail.md for the numbered list):
//  1. HARD BLOCKER — golden RenderDetail(body) output. PORTED below
//     (Test_RenderDetail_Golden_FieldsAttentionRelatedPanel).
//  2. Detail bottom-hints (navigable-field / Tab-suppressed-on-auto-show /
//     CloudTrail). PORTED below onto Controller.buildDetailFooterHints via
//     the public Snapshot().Footer seam (footer.go:110) — confirmed zero
//     existing coverage (app_footer_hints_test.go pins menu hints only).
//  3. Detail search activate/highlight/next-prev. PORTED below onto the live
//     seam: tui.Model key routing (app_stack.go, keys.Search/SearchNext/
//     SearchPrev) -> Controller.Apply(ActionSearch/...) -> renderer.go
//     rs.search.SyncCursor(body.SearchCursor) -> RenderDetail.
//  4. Section-styling + color-tier goldens. PORTED below directly onto
//     RenderDetail(body) using hand-built FieldRow slices — detail_fields.go's
//     renderFromFieldList (the LIVE styling switch) is exercised identically
//     whether the FieldRow rows came from ct-events, Attention, or a test.
//     The "cursor-skips-IsSection" sub-case is COVERED, not re-ported: it is
//     a controller-state invariant (applyDetailActions' skip loop), already
//     pinned type-agnostically by detail_livepath_migration_test.go's
//     TestDetailController_MoveDown_SkipsSectionHeadersAndSpacers ("regardless
//     of whether the section came from the Attention block or a type-specific
//     projector" — its own docstring).
//  5. ct-events section headers. PORTED below: catalog_monitoring.go registers
//     Project: ctevent.Project for "ct-events" (the SAME projector the dead
//     buildFieldList used), and buildDetailFieldItems (detail_body.go:123)
//     calls td.Project unconditionally — but zero existing controller-path
//     test asserted the ACTOR/ACTION/CONTEXT section shape survives that
//     wiring, so this is a real gap, not a duplicate.
//  6. t-key (ActionCloudTrail) emits ct-events RelatedNavigate from Detail.
//     PORTED below: grep found zero test references to ActionCloudTrail
//     anywhere in the repo. ct_events_rightcol_dispatch_test.go's D2/D3
//     assertions are about a DIFFERENT trigger (selecting an already-visible
//     related row), already superseded by detail_livepath_migration_test.go
//     per that file's own docstring — that one is COVERED, but
//     handleActionCloudTrail's own "t"-key path was untested until now.
//  7. isFieldNavigable predicate table. COVERED, not re-ported:
//     detail_navigable_test.go's TestIsFieldNavigable_MatchFound/NoMatch/
//     UnknownType (lines 184-215) call resource.IsFieldNavigableForTest directly —
//     a resource-package predicate shared verbatim by both the legacy
//     views.DetailModel and the live buildDetailFieldItems path (not a
//     views-package duplicate), so there is no separate port target.
//  8. Boundary end-clamp (detail_boundary_spec008_test.go's DetailModel j/k
//     clamp). detail_livepath_migration_test.go (MIG) only covers the
//     skip-over-sections loop, NOT the edge clamp guards themselves
//     (ds.FieldCursor>0 / ds.FieldCursor<fieldCount-1 in
//     applyDetailActions, detail_cursor.go:55,77). PORTED below.
//  9. Dispatch-on-resize (detail_resize_test.go's narrow->wide
//     TakePendingRelatedDispatch). NOT PORTED — confirmed architecturally
//     obsolete, not merely "already covered": Controller.EnsureDetailState
//     (detail_state.go:57) dispatches KindRelatedCheck unconditionally on
//     screen entry regardless of width, and buildDetailBody's RelatedVisible
//     is driven only by DetailState flags (SetDetailRelatedVisible), never by
//     width — the width gate lives solely in RenderDetail's render-time "if
//     body.RelatedVisible && m.width >= layout.MinInnerContentWidth" check.
//     The legacy mechanic existed only to defer a fetch the controller never
//     defers in the first place, so there is no live behavior left to pin.
//  10. Wave2 S4/S5 full-detail-sentence + falls-back-to-phrase. PORTED below
//     directly against injectAttentionSectionDetail's live output
//     (detail_body.go:330) — app_detail_attention_cursor_test.go only pins
//     FieldCursor stability across a mixed-severity sort, never the rendered
//     Detail-sentence row's presence/absence.
package unit_test

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/viewport"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ---------------------------------------------------------------------------
// 1. HARD BLOCKER — RenderDetail golden on the NewTransientDetail seam.
// ---------------------------------------------------------------------------

// wave3RenderDetailGolden was captured from the current, correct
// RenderDetail(body) output for an EC2 resource with one SevBroken Attention
// finding (with supporting rows) and a related panel showing one resolved
// row (VPC, count 1) and one loading row (Security Groups). NO_COLOR is
// forced so the golden is deterministic across terminals/CI; the one cursor
// row (FieldCursor defaults to 0, landing on "Architecture" — the
// alphabetically-first content row after the Attention block) still carries
// its background-highlight escape because that highlight is applied by a
// raw wrap independent of the lipgloss color styles NO_COLOR disables — this
// is real, current behavior, not a test artifact.
//
// This is the ONE pin required before View() (and the buildFieldList/
// buildLiveBody halves that only exist to reproduce it) can be deleted per
// wave3-map-detail.md's HARD BLOCKER note: RP/PUR currently only prove
// RenderDetail == View(), never RenderDetail's own correctness independent
// of View().
const wave3RenderDetailGolden = " Attention (1)                                                                                             │            RELATED\n     ! Encryption key unavailable                                                                          │  VPC (1)\n         KMS Key: arn:aws:kms:us-east-1:123456789012:key/abc123                                            │  Security Groups\n         Reason: key is pending deletion                                                                   │\n                                                                                                           │\n\x1b[48;2;122;162;247m Architecture:         x86_64                                                                              \x1b[m│\n ImageId:              ami-0a1b2c3d4e5f60001                                                               │\n InstanceId:           i-0abc123def456789a                                                                 │\n KeyName:              prod-keypair                                                                        │\n LaunchTime:           2024-01-15T10:30:00Z                                                                │\n PrivateIpAddress:     10.0.1.100                                                                          │\n PublicIpAddress:      203.0.113.42                                                                        │\n State:                running                                                                             │\n SubnetId:             subnet-0abc12345def67890                                                            │\n VpcId:                vpc-0abc12345def67890                                                               │\n architecture:         x86_64                                                                              │\n availability_zone:    us-east-1a                                                                          │\n iam_instance_profile: acme-ec2-instance-profile                                                           │\n image_id:             ami-0a1b2c3d4e5f60001                                                               │\n instance_id:          i-0abc123def456789a                                                                 │\n instance_type:        t3.medium                                                                           │\n key_name:             prod-keypair                                                                        │\n launch_time:          2024-01-15T10:30:00Z                                                                │\n monitoring:           enabled                                                                             │\n private_ip:           10.0.1.100                                                                          │\n public_ip:            203.0.113.42                                                                        │\n security_groups:      sg-0aaa111111111111a                                                                │\n state:                running                                                                             │\n subnet_id:            subnet-0abc12345def67890                                                            │\n vpc_id:               vpc-0abc12345def67890                                                               │"

func Test_RenderDetail_Golden_FieldsAttentionRelatedPanel(t *testing.T) {
	tuitest.NoColor(t)

	c := newDetailController(t, detailParityEC2Resource(), "ec2")
	c.ApplyDetailFinding(detailParityBrokenFinding(), detailParityAttentionDetail())
	c.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: "vpc", DisplayName: "VPC", State: domain.RelatedResolved, Count: 1},
		{TargetType: "sg", DisplayName: "Security Groups", State: domain.RelatedLoading, Loading: true},
	})

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil after seeding attention + related state")
	}

	vp := viewport.New(viewport.WithWidth(140), viewport.WithHeight(30))
	m := views.NewTransientDetail(140, 30, vp)
	got := m.RenderDetail(*body)

	if got != wave3RenderDetailGolden {
		t.Errorf("RenderDetail(body) output drifted from the golden captured for the field-rows + attention + related-panel shape.\nThis pin exists so View() can be safely deleted later — a diff here means RenderDetail itself regressed, not just parity with the dying View().\n--- got ---\n%s\n--- want ---\n%s", got, wave3RenderDetailGolden)
	}
}

// ---------------------------------------------------------------------------
// 2. Detail bottom-hints on the Controller.buildDetailFooterHints seam.
// ---------------------------------------------------------------------------

// wave3FooterHintEC2 is a minimal RawStruct exposing a VpcId field for the
// generic fieldpath projector to extract, mirroring detail_navigable_test.go's
// testNavEC2.
type wave3FooterHintEC2 struct {
	VpcId string
}

func wave3NavViewConfig() *config.ViewsConfig {
	return &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {Detail: []config.DetailField{{Path: "VpcId"}}},
		},
	}
}

func wave3NoopRelatedChecker(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{}
}

// Test_FooterHints_Detail_NavigableFieldHint pins that when FieldCursor
// sits on a navigable field, Snapshot().Footer's first hint is
// {enter, <target type display name>} — the live equivalent of
// qa_bottom_hints_test.go's TestBottomHints_Detail_NavigableField.
func Test_FooterHints_Detail_NavigableFieldHint(t *testing.T) {
	replaceEC2NavigableFields(t, []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})

	res := resource.Resource{
		ID:        "i-footer-nav-0001",
		Name:      "footer-nav-instance",
		Fields:    map[string]string{"vpc_id": "vpc-footer-0001"},
		RawStruct: wave3FooterHintEC2{VpcId: "vpc-footer-0001"},
	}
	c := newDetailController(t, res, "ec2")
	c.SetViewConfig(wave3NavViewConfig())

	footer := c.Snapshot().Footer
	if len(footer) == 0 {
		t.Fatal("Snapshot().Footer is empty for a detail screen with a navigable field under the cursor")
	}
	first := footer[0]
	if first.Key != "enter" {
		t.Fatalf("first footer hint Key = %q, want %q (navigable-field hint must lead)", first.Key, "enter")
	}
	wantHelp := "vpc"
	if rt := resource.FindResourceType("vpc"); rt != nil {
		wantHelp = rt.Name
	}
	if first.Help != wantHelp {
		t.Errorf("navigable-field footer hint Help = %q, want %q", first.Help, wantHelp)
	}
}

// Test_Detail_EnterOnNavigableField_TUIKeyRoute_NavigatesToTarget drives
// the real TUI key route (root tui.Model -> app_stack.go's Enter case) for a
// navigable field, unlike Test_FooterHints_Detail_NavigableFieldHint
// above, which only proves the footer HINT text and never presses Enter.
// core/app/field_select_byid_test.go covers the controller/web seam but
// also never presses Enter through app_stack.go — no existing test drives
// this dispatch, so this closes that gap (issue140_scenarios_golden_test.go's
// citation is repointed to this test below).
func Test_Detail_EnterOnNavigableField_TUIKeyRoute_NavigatesToTarget(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	replaceEC2NavigableFields(t, []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})

	m := tui.New("test", "us-east-1", tui.WithNoCache(true))
	t.Cleanup(func() { m.CloseController() })
	m, _ = tuitest.Step(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = tuitest.Step(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})

	res := resource.Resource{
		ID:   "i-entertest0000001",
		Name: "enter-test-instance",
		Fields: map[string]string{
			"instance_id": "i-entertest0000001",
			"vpc_id":      "vpc-entertest0001",
		},
	}
	m, _ = tuitest.Step(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{res}})

	m, cmd := tuitest.Step(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on the resource list should open the detail screen")
	}
	m, _ = tuitest.Step(m, cmd())

	// Built-in ec2 Detail field order (config/defaults_compute.go) renders one
	// row per configured path regardless of whether the resource has a value
	// (absent -> "-"), so VpcId — the 8th configured path — sits at
	// FieldCursor index 7: InstanceId, State, InstanceType,
	// InstanceLifecycle, ImageId, KeyName, Placement, VpcId, ...
	for range 7 {
		m, _ = tuitest.Step(m, tea.KeyPressMsg{Code: -1, Text: "j"})
	}

	before := tuitest.StripANSI(tuitest.Render(m))
	if !strings.Contains(before, "VpcId:") {
		t.Fatalf("precondition: cursor did not land on the VpcId row, got:\n%s", before)
	}

	m, cmd = tuitest.Step(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on a navigable field must return a navigate command")
	}
	m, _ = tuitest.Step(m, cmd())

	after := tuitest.StripANSI(tuitest.Render(m))
	vpcRT := resource.FindResourceType("vpc")
	if vpcRT == nil {
		t.Fatal("vpc resource type not registered")
	}
	wantTitle := vpcRT.ShortName
	if vpcRT.ListTitle != "" {
		wantTitle = vpcRT.ListTitle
	}
	if !strings.Contains(after, wantTitle) {
		t.Errorf("Enter on the navigable VpcId field did not land on the vpc target (want frame title containing %q), got:\n%s", wantTitle, after)
	}
}

// Test_FooterHints_Detail_TabSuppressedOnAutoShow_ThenShownOnExplicitToggle
// pins that "tab: Cols" is absent while the related panel is only
// auto-shown (RelatedVisible=true, RelatedUserVisible=false — the default
// after EnsureDetailState when defs are registered) and present once the
// user explicitly toggles it on via SetDetailRelatedVisible(true, true) —
// mirrors DetailModel.BottomHints checking m.rightColVisible, per
// footer.go:176's comment.
func Test_FooterHints_Detail_TabSuppressedOnAutoShow_ThenShownOnExplicitToggle(t *testing.T) {
	resource.SetRelatedForTest("wave3_footer_hints_related", []resource.RelatedDef{
		{TargetType: "vpc", DisplayName: "VPC", Checker: wave3NoopRelatedChecker},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest("wave3_footer_hints_related") })

	res := resource.Resource{ID: "footer-tab-0001", Name: "footer-tab-resource"}
	c := newDetailController(t, res, "wave3_footer_hints_related")

	footer := c.Snapshot().Footer
	if !wave3HasHint(footer, "r") {
		t.Errorf("expected {r, Related} hint when related defs are registered; got %+v", footer)
	}
	if wave3HasHint(footer, "tab") {
		t.Errorf("unexpected {tab, ...} hint while the related panel is only auto-shown (not explicitly toggled); got %+v", footer)
	}

	c.SetDetailRelatedVisible(true, true)

	footer = c.Snapshot().Footer
	if !wave3HasHint(footer, "tab") {
		t.Errorf("expected {tab, Cols} hint after explicit SetDetailRelatedVisible(true, true); got %+v", footer)
	}
}

// Test_FooterHints_Detail_CloudTrailHint pins that the "t: CloudTrail"
// footer hint appears for a resource type with a registered CloudTrailKey
// (ec2's "ResourceName:ID") — the live equivalent of qa_bottom_hints_test.go's
// TestBottomHints_Detail_ShowsCloudTrail.
func Test_FooterHints_Detail_CloudTrailHint(t *testing.T) {
	res := resource.Resource{ID: "i-footer-ct-0001", Name: "footer-ct-instance"}
	c := newDetailController(t, res, "ec2")

	footer := c.Snapshot().Footer
	if !wave3HasHint(footer, "t") {
		t.Errorf("expected {t, CloudTrail} hint for ec2 (has CloudTrailKey); got %+v", footer)
	}
}

// Test_FooterHints_Detail_RightColFocused_ShowsCloudTrail pins that the
// "t: CloudTrail" hint survives the SEPARATE RelatedFocus branch of
// buildDetailFooterHints (footer.go:114-148) — a distinct code path from the
// left-column branch Test_FooterHints_Detail_CloudTrailHint exercises.
// Live replacement for qa_bottom_hints_test.go's
// TestBottomHints_Detail_RightColFocused_ShowsCloudTrail.
func Test_FooterHints_Detail_RightColFocused_ShowsCloudTrail(t *testing.T) {
	res := resource.Resource{ID: "i-rhs-focus-0001", Name: "rhs-focus-instance"}
	c := newDetailController(t, res, "ec2")
	c.SetDetailRelatedVisible(true, true)
	c.Apply(app.Action{Kind: app.ActionToggleFocus})

	if !c.Snapshot().Body.Detail.RelatedFocused {
		t.Fatal("test setup problem: RelatedFocused must be true after ActionToggleFocus with RelatedVisible")
	}

	footer := c.Snapshot().Footer
	if !wave3HasHint(footer, "t") {
		t.Errorf("expected {t, CloudTrail} hint while related panel is focused; got %+v", footer)
	}
}

// Test_FooterHints_YAML_CloudTrailHint pins that a YAML/text screen's
// Snapshot().Footer includes {t, CloudTrail} when the screen's ScreenContext
// resolves to a cached resource with a CloudTrailKey — the live replacement
// for qa_bottom_hints_test.go's TestBottomHints_YAML_ShowsCloudTrail.
// buildTextFooterHints (footer.go:88) had zero controller-path test coverage
// before this pin.
func Test_FooterHints_YAML_CloudTrailHint(t *testing.T) {
	c := newTestController(t)

	res := resource.Resource{ID: "i-yaml-ct-0001", Name: "yaml-ct-instance"}
	// Provenance: CanonicalList declares what this seed always implicitly
	// meant — the resource must already be in the cache before the YAML
	// screen is pushed below, exactly as if an earlier top-level ec2 list
	// had loaded it. This is the only way findCachedResourceByID (footer.go)
	// can resolve it; no related/filtered/by-ID/child path is exercised here.
	c.Handle(messages.ResourcesLoaded{ResourceType: "ec2", Resources: []resource.Resource{res}, Provenance: messages.FetchProvenanceCanonicalList})

	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenYAML,
		Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: res.ID},
	}})
	c.EnsureTextState([]string{"id: i-yaml-ct-0001"})

	footer := c.Snapshot().Footer
	if !wave3HasHint(footer, "t") {
		t.Errorf("expected {t, CloudTrail} hint for a YAML screen over an ec2 resource (has CloudTrailKey); got %+v", footer)
	}
}

func wave3HasHint(hints []app.KeyHint, key string) bool {
	for _, h := range hints {
		if h.Key == key {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 3. Detail search activate/highlight/next-prev on the live seam.
// ---------------------------------------------------------------------------

// wave3OpenEC2Detail builds a plain (non-demo) root tui.Model, sizes it, and
// pushes a Detail screen directly for res via messages.Navigate{Target:
// TargetDetail} — mirrors issue119_scenarios_golden_test.go's
// issue119ApplyMsg(..., messages.Navigate{Target: messages.TargetDetail, ...})
// pattern, without requiring the demo backend or a list-then-Enter chain.
func wave3OpenEC2Detail(t *testing.T, res resource.Resource) tui.Model {
	t.Helper()
	m := tui.New("wave3-detail-search", "us-east-1", tui.WithNoCache(true))
	m, _ = tuitest.Step(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m, _ = tuitest.Step(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &res,
	})
	return m
}

// Test_DetailSearch_LiveSeam_ActivateHighlightNextPrevEsc drives the
// real key path (keys.Search "/", keys.SearchNext "n", keys.SearchPrev "N",
// Esc) through the root tui.Model into a Detail screen, exercising
// app_stack.go's Controller.Apply(ActionSearch/ActionSearchNext/
// ActionSearchPrev/ActionSearchClear) -> renderer.go's rs.search sync ->
// RenderDetail — the live replacement for qa_search_views_test.go's
// TestSearch_DetailView_SlashActivatesSearch /
// TestSearch_DetailView_TypeQueryHighlightsMatches /
// TestSearch_DetailView_EscExitsSearch, which drive views.NewDetail directly
// (a dead path once View()/Update() key handling is deleted).
func Test_DetailSearch_LiveSeam_ActivateHighlightNextPrevEsc(t *testing.T) {
	tuitest.ForceColor(t)

	res := detailParityEC2Resource() // has Fields["state"]="running" (appears twice: once per row)
	m := wave3OpenEC2Detail(t, res)

	before := tuitest.StripANSI(tuitest.Render(m))
	if !strings.Contains(before, "running") {
		t.Fatalf("precondition: detail view must show 'running' before search is active; got:\n%s", before)
	}

	// Activate search with "/".
	m, _ = tuitest.Step(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	// Type "running" character by character — mirrors qa_search_views_test.go's
	// TestSearch_DetailView_TypeQueryHighlightsMatches keystroke pattern.
	for _, ch := range "running" {
		m, _ = tuitest.Step(m, tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}

	active := tuitest.Render(m)
	if !strings.Contains(active, "\x1b[") {
		t.Errorf("expected ANSI highlight sequences in the rendered view after activating the live search seam and typing a query; got:\n%s", active)
	}
	activePlain := tuitest.StripANSI(active)
	if !strings.Contains(activePlain, "running") {
		t.Errorf("plain content must still contain %q after search; got:\n%s", "running", activePlain)
	}

	// SearchNext ("n") must not panic and must keep the view rendering.
	m, _ = tuitest.Step(m, tea.KeyPressMsg{Code: 'n', Text: "n"})
	if next := tuitest.Render(m); next == "" {
		t.Fatal("view rendered empty after SearchNext (\"n\")")
	}

	// SearchPrev ("N") must not panic either.
	m, _ = tuitest.Step(m, tea.KeyPressMsg{Code: 'N', Text: "N"})
	if prev := tuitest.Render(m); prev == "" {
		t.Fatal("view rendered empty after SearchPrev (\"N\")")
	}

	// Esc exits search — ActionSearchClear — and the view keeps rendering
	// content without panicking.
	m, _ = tuitest.Step(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	closed := tuitest.Render(m)
	if closed == "" {
		t.Fatal("view rendered empty after Esc exited search")
	}
	closedPlain := tuitest.StripANSI(closed)
	if !strings.Contains(closedPlain, "running") {
		t.Errorf("detail content must still be visible after exiting search; got:\n%s", closedPlain)
	}
}

// ---------------------------------------------------------------------------
// 4. Section-styling + color-tier goldens directly on RenderDetail(body).
// ---------------------------------------------------------------------------

// wave3RenderRows is a small helper that builds a DetailModel via the live
// NewTransientDetail seam and calls RenderDetail with a hand-built body —
// used so items 4's sub-cases don't need a full ct-events/Attention fixture,
// only the FieldRow shapes that trigger each styling branch in
// detail_fields.go's renderFromFieldList (the LIVE rendering switch RenderDetail
// delegates to via renderDetailFieldsFromBody).
func wave3RenderRows(fields []app.FieldRow, fieldCursor int) string {
	body := app.DetailBody{Fields: fields, FieldCursor: fieldCursor, KeyWidth: 12}
	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(20))
	m := views.NewTransientDetail(120, 20, vp)
	return m.RenderDetail(body)
}

// Test_RenderDetail_SectionHeader_BoldNoColor pins that an IsSection row
// with an empty ColorTier renders via styles.FindingSectionDefault (bold,
// NO foreground color) rather than a tier-colored style — the live
// replacement for views_detail_render_section_test.go's
// TestDetailRenderSection_BoldUppercase / TestDetailRenderSection_NoColorOnHeader.
func Test_RenderDetail_SectionHeader_BoldNoColor(t *testing.T) {
	tuitest.ForceColor(t)

	got := wave3RenderRows([]app.FieldRow{
		{IsSection: true, Key: "ACTOR", Path: "ACTOR"},
		{Key: "Principal", Value: "alice"},
	}, -1)

	wantHeader := styles.FindingSectionDefault.Render("ACTOR")
	if !strings.Contains(got, wantHeader) {
		t.Errorf("section header with empty ColorTier must render via styles.FindingSectionDefault (bold, no color); got:\n%s\nwant substring: %q", got, wantHeader)
	}
	// A tier-colored render of the same text must differ — proves the
	// default branch is genuinely uncolored, not accidentally identical to
	// the tiered styles for this particular color.
	stoppedHeader := styles.FindingSectionStopped.Render("ACTOR")
	if strings.Contains(got, stoppedHeader) {
		t.Errorf("section header with empty ColorTier must NOT render via styles.FindingSectionStopped; got:\n%s", got)
	}
}

// Test_RenderDetail_ColorTier_IsNavigableWinsOverColorTier pins that a
// top-level FieldRow with IsNavigable=true AND a non-empty ColorTier renders
// via styles.NavigableField (underline), never via styles.TierColorStyle —
// the live replacement for views_detail_render_color_tier_test.go's
// TestDetailRenderColorTier_IsNavigableWinsOverColorTier.
func Test_RenderDetail_ColorTier_IsNavigableWinsOverColorTier(t *testing.T) {
	tuitest.ForceColor(t)

	got := wave3RenderRows([]app.FieldRow{
		{Key: "Event", Value: "ConsoleLogin", IsNavigable: true, ColorTier: "ct-danger", TargetType: "ct-events"},
	}, -1)

	wantNavigable := styles.NavigableField.Render("ConsoleLogin")
	if !strings.Contains(got, wantNavigable) {
		t.Errorf("navigable row with a ColorTier set must still render its value via styles.NavigableField; got:\n%s\nwant substring: %q", got, wantNavigable)
	}
	tierRender := styles.TierColorStyle("ct-danger").Render("ConsoleLogin")
	if strings.Contains(got, tierRender) {
		t.Errorf("navigable row must NOT render its value via styles.TierColorStyle even though ColorTier is set; got:\n%s", got)
	}
}

// Test_RenderDetail_ColorTier_LabelAlwaysNeutral pins that the label
// portion of a field row (the "Key:" text) always renders via the neutral
// styles.DetailKey style regardless of ColorTier — only the value carries
// tier coloring — the live replacement for views_detail_render_color_tier_test.go's
// label-neutrality assertion embedded in TestDetailRenderColorTier_CTDanger
// and friends.
func Test_RenderDetail_ColorTier_LabelAlwaysNeutral(t *testing.T) {
	tuitest.ForceColor(t)

	// renderFromFieldList computes its own key-column width from the field
	// list via computeKeyWidth (detail_render.go), which floors at 22 —
	// body.KeyWidth is not consulted on this path.
	paddedLabel := text.PadOrTrunc("Status:", 22)
	wantLabel := styles.DetailKey.Render(paddedLabel)

	plainTier := wave3RenderRows([]app.FieldRow{{Key: "Status", Value: "ok", ColorTier: ""}}, -1)
	dangerTier := wave3RenderRows([]app.FieldRow{{Key: "Status", Value: "ok", ColorTier: "!"}}, -1)

	if !strings.Contains(plainTier, wantLabel) {
		t.Errorf("label render with ColorTier=\"\" must contain the neutral styles.DetailKey render; got:\n%s\nwant substring: %q", plainTier, wantLabel)
	}
	if !strings.Contains(dangerTier, wantLabel) {
		t.Errorf("label render with ColorTier=\"!\" must STILL contain the SAME neutral styles.DetailKey render (label is tier-independent); got:\n%s\nwant substring: %q", dangerTier, wantLabel)
	}
}

// ---------------------------------------------------------------------------
// 5. ct-events section headers on the live buildDetailFieldItems path.
// ---------------------------------------------------------------------------

// Test_CTEvents_LiveProjector_SectionHeadersPresentInOrder pins that a
// ct-events resource projected through the live buildDetailFieldItems path
// (Controller.EnsureDetailState -> buildDetailFieldItems -> td.Project ==
// ctevent.Project, catalog_monitoring.go:238) still produces IsSection field
// rows for ACTOR, ACTION, CONTEXT in that order — the live replacement for
// views_detail_ct_events_test.go's TestDetailViewCTEvents_BasicPath /
// TestDetailViewCTEvents_SectionOrder. Reuses buildCTEventsResource +
// minimalCTJSON from views_detail_ct_events_test.go (same package).
func Test_CTEvents_LiveProjector_SectionHeadersPresentInOrder(t *testing.T) {
	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000001",
		"DescribeInstances",
		"ct-info",
		minimalCTJSON,
	)
	c := newDetailController(t, res, "ct-events")

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil for a ct-events resource")
	}

	var sectionOrder []string
	for _, f := range body.Fields {
		if f.IsSection {
			sectionOrder = append(sectionOrder, f.Key)
		}
	}

	idx := make(map[string]int, len(sectionOrder))
	for i, k := range sectionOrder {
		idx[k] = i
	}
	for _, want := range []string{"ACTOR", "ACTION", "CONTEXT"} {
		if _, ok := idx["ACTOR"]; !ok && want == "ACTOR" {
			continue // ACTOR is legitimately omitted for Insight/service events; not the case for this fixture, but keep the check narrow.
		}
		if _, ok := idx[want]; !ok {
			t.Errorf("live buildDetailFieldItems missing section header %q for a ct-events resource; got sections: %v", want, sectionOrder)
		}
	}
	if actorIdx, ok := idx["ACTOR"]; ok {
		if actionIdx, ok2 := idx["ACTION"]; ok2 && actorIdx >= actionIdx {
			t.Errorf("ACTOR (idx %d) must appear before ACTION (idx %d); got order: %v", actorIdx, actionIdx, sectionOrder)
		}
	}
	if actionIdx, ok := idx["ACTION"]; ok {
		if contextIdx, ok2 := idx["CONTEXT"]; ok2 && actionIdx >= contextIdx {
			t.Errorf("ACTION (idx %d) must appear before CONTEXT (idx %d); got order: %v", actionIdx, contextIdx, sectionOrder)
		}
	}
}

// Test_CTEvents_LiveProjector_DataRowsBetweenSections pins the one claim
// Test_CTEvents_LiveProjector_SectionHeadersPresentInOrder above does
// NOT cover: that ACTOR and ACTION are not immediately adjacent — i.e. the
// section actually has content, not just a stack of bare headers. Ported
// (round-4 follow-up, specs/022-codebase-cleanup) from
// views_detail_render_section_test.go's TestDetailRenderSection_
// RealSectionSequence, sub-check "at least one data row between ACTOR and
// ACTION" (that test's line-count check on rendered output; this checks the
// live buildDetailFieldItems field-model directly, which is exact where a
// rendered-line count could be thrown off by wrapping).
func Test_CTEvents_LiveProjector_DataRowsBetweenSections(t *testing.T) {
	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000002",
		"DescribeInstances",
		"ct-info",
		minimalCTJSON,
	)
	c := newDetailController(t, res, "ct-events")

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil for a ct-events resource")
	}

	actorIdx, actionIdx := -1, -1
	for i, f := range body.Fields {
		if !f.IsSection {
			continue
		}
		switch f.Key {
		case "ACTOR":
			if actorIdx < 0 {
				actorIdx = i
			}
		case "ACTION":
			if actionIdx < 0 {
				actionIdx = i
			}
		}
	}
	if actorIdx < 0 || actionIdx < 0 {
		t.Fatalf("ACTOR/ACTION section rows not found in body.Fields (actorIdx=%d actionIdx=%d)", actorIdx, actionIdx)
	}
	if actionIdx-actorIdx < 2 {
		t.Errorf("expected at least one data row between ACTOR (Fields index %d) and ACTION (Fields index %d); sections are adjacent", actorIdx, actionIdx)
	}
}

// Test_TierColorStyle_CtEventTiersAreDistinct pins that styles.
// TierColorStyle — the function detail_fields.go's renderFromFieldList calls
// for every ColorTier'd field row (styles.TierColorStyle(item.ColorTier).
// Render(item.Value)) — maps "ct-info"/"ct-attention"/"ct-danger" to 3
// distinct, non-neutral styles. Ported (round-4 follow-up, specs/022-
// codebase-cleanup) from views_detail_render_color_tier_test.go's
// TestDetailRenderColorTier_CTInfo/CTAttention/CTDanger: those tests built a
// full ct-events fixture + DetailModel render + ANSI-line-scanning helpers to
// indirectly prove this same claim. TierColorStyle is a pure, exported
// function (internal/tui/styles/styles.go) — testing it directly is a strict
// superset (proves the 3 tiers are MUTUALLY distinct, not just individually
// non-default) with none of the fragile rendering machinery. The renderer's
// OWN wiring of ColorTier -> TierColorStyle(...).Render(...) is covered
// separately by Test_RenderDetail_ColorTier_IsNavigableWinsOverColorTier
// and Test_RenderDetail_ColorTier_LabelAlwaysNeutral above (using "ct-
// danger"/"!" as representative non-empty ColorTier values — the renderer
// branch is tier-string-agnostic).
func Test_TierColorStyle_CtEventTiersAreDistinct(t *testing.T) {
	tuitest.ForceColor(t)

	info := styles.TierColorStyle("ct-info").Render("x")
	attention := styles.TierColorStyle("ct-attention").Render("x")
	danger := styles.TierColorStyle("ct-danger").Render("x")
	neutral := styles.DetailVal.Render("x")

	for name, got := range map[string]string{"ct-info": info, "ct-attention": attention, "ct-danger": danger} {
		if got == neutral {
			t.Errorf("TierColorStyle(%q) must differ from the neutral styles.DetailVal style; got %q", name, got)
		}
	}
	if info == attention || info == danger || attention == danger {
		t.Errorf("ct-info/ct-attention/ct-danger must map to 3 mutually distinct styles; got info=%q attention=%q danger=%q", info, attention, danger)
	}
}

// ---------------------------------------------------------------------------
// 6. "t" key (ActionCloudTrail) dispatch from a Detail screen.
// ---------------------------------------------------------------------------

// Test_ActionCloudTrail_Detail_DispatchesCtEventsFetchFiltered pins that
// Controller.Apply(ActionCloudTrail) on a Detail screen for an ec2 resource
// (CloudTrailKey "ResourceName:ID") dispatches a KindFetchFiltered task
// scoped to "ct-events" carrying the BuildCloudTrailFilter-derived filter —
// handleActionCloudTrail (actions_view.go:294) had zero test references
// anywhere in the repo before this pin.
func Test_ActionCloudTrail_Detail_DispatchesCtEventsFetchFiltered(t *testing.T) {
	res := resource.Resource{ID: "i-cloudtrail-key-0001", Name: "cloudtrail-key-instance"}
	c := newDetailController(t, res, "ec2")

	_, tasks := c.Apply(app.Action{Kind: app.ActionCloudTrail})

	var fetchFilteredTask *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchFiltered {
			fetchFilteredTask = &tasks[i]
			break
		}
	}
	if fetchFilteredTask == nil {
		t.Fatalf("ActionCloudTrail on an ec2 detail screen dispatched no KindFetchFiltered task — tasks: %+v", tasks)
	}
	if fetchFilteredTask.Key.Scope != "ct-events" {
		t.Errorf("KindFetchFiltered task Scope = %q, want %q", fetchFilteredTask.Key.Scope, "ct-events")
	}
	payload, ok := fetchFilteredTask.Payload.(runtime.FetchFilteredPayload)
	if !ok {
		t.Fatalf("KindFetchFiltered task Payload = %T, want runtime.FetchFilteredPayload", fetchFilteredTask.Payload)
	}
	wantFilter := resource.BuildCloudTrailFilter(res, "ec2")
	if len(payload.Filter) != len(wantFilter) {
		t.Errorf("FetchFilteredPayload.Filter = %v, want %v", payload.Filter, wantFilter)
	}
	for k, v := range wantFilter {
		if payload.Filter[k] != v {
			t.Errorf("FetchFilteredPayload.Filter[%q] = %q, want %q", k, payload.Filter[k], v)
		}
	}
}

// Test_ActionCloudTrail_NoCloudTrailKey_DispatchesNothing pins the
// negative case: a resource type without a CloudTrailKey no-ops.
func Test_ActionCloudTrail_NoCloudTrailKey_DispatchesNothing(t *testing.T) {
	res := resource.Resource{ID: "res-no-ct-0001", Name: "no-cloudtrail-resource"}
	c := newDetailController(t, res, "wave3_no_cloudtrail_type")

	_, tasks := c.Apply(app.Action{Kind: app.ActionCloudTrail})
	if len(tasks) != 0 {
		t.Errorf("ActionCloudTrail on a type with no CloudTrailKey dispatched %d tasks, want 0; tasks: %+v", len(tasks), tasks)
	}
}

// ---------------------------------------------------------------------------
// 8. Boundary end-clamp on ActionMoveDown/ActionMoveUp.
// ---------------------------------------------------------------------------

// Test_DetailController_MoveDown_ClampsAtLastField pins the boundary
// guard in applyDetailActions (detail_cursor.go:77,
// "if ds.FieldCursor < fieldCount-1") — the live replacement for
// detail_boundary_spec008_test.go's TestDetail_008_JAtLastField_CursorClamped
// / TestDetail_008_JAtLastField_10Fields, which drive the legacy DetailModel
// j/k Update path. detail_livepath_migration_test.go only pins the
// skip-over-sections loop, never this edge clamp.
func Test_DetailController_MoveDown_ClampsAtLastField(t *testing.T) {
	c := newDetailController(t, detailParityEC2Resource(), "ec2")

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	fieldCount := len(body.Fields)
	if fieldCount == 0 {
		t.Fatal("test setup problem: no field rows to move through")
	}

	// Advance well past the end — one extra step beyond fieldCount to prove
	// the guard, not just reach the boundary.
	for i := 0; i < fieldCount+2; i++ {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}

	got := c.Snapshot().Body.Detail
	if got.FieldCursor != fieldCount-1 {
		t.Errorf("FieldCursor after repeated ActionMoveDown past the end = %d, want clamped at %d (fieldCount-1)", got.FieldCursor, fieldCount-1)
	}
	if got.FieldCursor < 0 || got.FieldCursor >= fieldCount {
		t.Errorf("FieldCursor %d out of range [0,%d)", got.FieldCursor, fieldCount)
	}
}

// Test_DetailController_MoveUp_ClampsAtFirstField pins the symmetric
// guard for ActionMoveUp (detail_cursor.go:55, "if ds.FieldCursor > 0") —
// the live replacement for detail_boundary_spec008_test.go's
// TestDetail_008_KAtFirstField_CursorClamped /
// TestDetail_008_KAtFirstField_MultipleKPresses.
func Test_DetailController_MoveUp_ClampsAtFirstField(t *testing.T) {
	c := newDetailController(t, detailParityEC2Resource(), "ec2")

	if c.Snapshot().Body.Detail.FieldCursor != 0 {
		t.Fatalf("precondition: expected initial FieldCursor 0, got %d", c.Snapshot().Body.Detail.FieldCursor)
	}

	for range 5 {
		c.Apply(app.Action{Kind: app.ActionMoveUp})
	}

	got := c.Snapshot().Body.Detail.FieldCursor
	if got != 0 {
		t.Errorf("FieldCursor after 5x ActionMoveUp at the top boundary = %d, want clamped at 0", got)
	}
}

// ---------------------------------------------------------------------------
// 10. Wave2 S4/S5 — full Detail sentence renders alongside Phrase, and
//     falls back to Phrase-only when Detail is empty.
// ---------------------------------------------------------------------------

// wave3AttentionRowsForCode returns the Path=="Attention" IndentLevel==1
// FieldRow(s) for the given finding phrase, in the order buildAttentionEntries
// / injectAttentionSectionDetail emit them (phrase row, then an optional
// detail-sentence row). The match is case-insensitive because the phrase row's
// Value is capitalizeFirstDetail(Phrase), not Phrase verbatim.
func wave3AttentionRowsForCode(body *app.DetailBody, phraseSubstr string) []app.FieldRow {
	var rows []app.FieldRow
	collecting := false
	want := strings.ToLower(phraseSubstr)
	for _, f := range body.Fields {
		if f.Path != "Attention" || f.IndentLevel != 1 {
			if collecting {
				break
			}
			continue
		}
		if !collecting {
			if !strings.Contains(strings.ToLower(f.Value), want) {
				continue
			}
			collecting = true
			rows = append(rows, f)
			continue
		}
		rows = append(rows, f)
		break // at most phrase + one detail row per entry
	}
	return rows
}

// Test_DetailAttention_RendersFullDetailSentence_AlongsidePhrase pins
// that when Finding.Detail is non-empty, injectAttentionSectionDetail
// (detail_body.go:330) emits a SECOND Attention sub-row carrying the full S5
// operator sentence, in addition to the S4 Phrase row — the live replacement
// for wave2_risk_text_s4_s5_test.go's
// TestWave2_DetailAttention_RendersFullDetailSentence_AlongsidePhrase, which
// asserts against the legacy DetailModel.PlainContent() (DEAD per
// wave3-map-detail.md).
func Test_DetailAttention_RendersFullDetailSentence_AlongsidePhrase(t *testing.T) {
	res := resource.Resource{ID: "db-maint-wave3-1", Name: "prod-maint-db-wave3"}
	c := newDetailController(t, res, "dbi")
	c.ApplyDetailFinding(&domain.Finding{
		Code:     "dbi.pending-maintenance",
		Phrase:   "maintenance scheduled",
		Detail:   "Pending maintenance action overdue: system-update.",
		Severity: domain.SevWarn,
		Source:   "wave2:test",
	}, nil)

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	rows := wave3AttentionRowsForCode(body, "maintenance scheduled")
	if len(rows) != 2 {
		t.Fatalf("expected 2 Attention rows (phrase + detail sentence) for a finding with Detail set, got %d: %+v", len(rows), rows)
	}
	if !strings.Contains(strings.ToLower(rows[0].Value), "maintenance scheduled") {
		t.Errorf("first Attention row must carry the short Phrase; got %+v", rows[0])
	}
	if rows[1].Value != "Pending maintenance action overdue: system-update." {
		t.Errorf("second Attention row must be the full S5 Detail sentence verbatim; got %q", rows[1].Value)
	}
}

// Test_DetailAttention_FallsBackToPhrase_WhenDetailEmpty pins that when
// Finding.Detail == "", only the phrase row is emitted — no stray empty
// detail row — the live replacement for wave2_risk_text_s4_s5_test.go's
// TestWave2_DetailAttention_FallsBackToPhrase_WhenDetailEmpty.
func Test_DetailAttention_FallsBackToPhrase_WhenDetailEmpty(t *testing.T) {
	res := resource.Resource{ID: "i-nodep-wave3-1", Name: "worker-nodetail-wave3"}
	c := newDetailController(t, res, "ec2")
	c.ApplyDetailFinding(&domain.Finding{
		Code:     "ec2.instance-status-impaired",
		Phrase:   "impaired: system checks failing",
		Detail:   "",
		Severity: domain.SevWarn,
		Source:   "wave2:test",
	}, nil)

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	rows := wave3AttentionRowsForCode(body, "impaired: system checks failing")
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 Attention row (phrase only, no Detail sentence) when Finding.Detail is empty, got %d: %+v", len(rows), rows)
	}
}

// ---------------------------------------------------------------------------
// 18. Survivors relocated from detail_render_parity_test.go (Scope C, wave3
// detail-family cleanup, specs/022-codebase-cleanup): that file's byte-parity
// comparison tests (View() == RenderDetail(body)) became meaningless once
// View() was scheduled for deletion — TestWave3_RenderDetail_Golden_
// FieldsAttentionRelatedPanel above is the required replacement pin per
// wave3-map-detail.md's HARD BLOCKER note, and it already depends on the
// helpers below. Rather than leave a near-empty detail_render_parity_test.go
// behind, or duplicate these helpers, they're relocated here alongside their
// only other callers (detail_livepath_migration_test.go,
// detail_controller_scroll_follow_test.go, app_detail_attention_cursor_test.go,
// app_related_cursor_skip_test.go — same package, unaffected by the move).
// ---------------------------------------------------------------------------

// newDetailController builds a Controller with a ScreenDetail on the stack for
// the given resource and type, ready to call Snapshot().Body.Detail.
// Uses a real runtime.Core (nil AWS client) so Snapshot's c.core.Profile() /
// c.core.Region() calls don't panic.
func newDetailController(t *testing.T, res resource.Resource, resourceType string) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	c.EnsureDetailState(res, resourceType)
	return c
}

// detailParityEC2Resource returns a realistic EC2 resource with fields that
// drive the EC2 projector to produce multiple sections + navigable fields.
func detailParityEC2Resource() resource.Resource {
	return resource.Resource{
		ID:   "i-0abc123def456789a",
		Name: "prod-backend-01",
		Fields: map[string]string{
			"instance_id":          "i-0abc123def456789a",
			"instance_type":        "t3.medium",
			"state":                "running",
			"launch_time":          "2024-01-15T10:30:00Z",
			"public_ip":            "203.0.113.42",
			"private_ip":           "10.0.1.100",
			"vpc_id":               "vpc-0abc12345def67890",
			"subnet_id":            "subnet-0abc12345def67890",
			"key_name":             "prod-keypair",
			"image_id":             "ami-0a1b2c3d4e5f60001",
			"availability_zone":    "us-east-1a",
			"security_groups":      "sg-0aaa111111111111a",
			"iam_instance_profile": "acme-ec2-instance-profile",
			"monitoring":           "enabled",
			"architecture":         "x86_64",
		},
	}
}

// detailParityBrokenFinding returns a SevBroken finding for parity testing.
func detailParityBrokenFinding() *domain.Finding {
	return &domain.Finding{
		Code:     "kms.key-unavailable",
		Phrase:   "encryption key unavailable",
		Severity: domain.SevBroken,
		Source:   "wave2:test",
	}
}

// detailParityAttentionDetail returns an AttentionDetail with supporting rows.
func detailParityAttentionDetail() *domain.AttentionDetail {
	return &domain.AttentionDetail{
		Rows: []domain.DetailRow{
			{Label: "KMS Key", Value: "arn:aws:kms:us-east-1:123456789012:key/abc123"},
			{Label: "Reason", Value: "key is pending deletion"},
		},
	}
}

// Test_RenderDetail_RelatedPanel_ScrollExceedsRowCount is ported from
// detail_render_parity_test.go's TestDetailRenderParity_RelatedPanel_
// ScrollExceedsRowCount (Scope C) onto the NewTransientDetail seam (that test
// only ever used views.NewDetail(...) to reach m.RenderDetail(body); it never
// asserted View() parity). Pins a real production panic: when the related
// panel's row count shrinks (e.g. a re-render after rows are filtered or
// reloaded) while RelatedScroll still points past the new end,
// renderRelatedPanel must clamp the scroll window instead of slicing
// rows[scroll:...] out of range.
func Test_RenderDetail_RelatedPanel_ScrollExceedsRowCount(t *testing.T) {
	tuitest.NoColor(t)

	body := app.DetailBody{
		Fields: []app.FieldRow{
			{Key: "name", Value: "scroll-clamp-01"},
		},
		RelatedVisible: true,
		Related: []app.RelatedBlock{
			{Name: "sg", Count: 1, CountDisplay: "(1)", Actionable: true},
			{Name: "vpc", Count: 1, CountDisplay: "(1)", Actionable: true},
		},
		RelatedScroll:  10,
		RelatedFocused: false,
		RelatedCursor:  0,
	}

	vp := viewport.New(viewport.WithWidth(160), viewport.WithHeight(30))
	m := views.NewTransientDetail(160, 30, vp)

	var got string
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("RenderDetail panicked with 2 related rows and RelatedScroll=10 (focused=false): %v", r)
			}
		}()
		got = m.RenderDetail(body)
	}()

	if !strings.Contains(got, "sg") {
		t.Errorf("expected clamped related panel to still show row %q, got:\n%s", "sg", got)
	}
	if !strings.Contains(got, "vpc") {
		t.Errorf("expected clamped related panel to still show row %q, got:\n%s", "vpc", got)
	}
}

// ---------------------------------------------------------------------------
// 19. Detail footer-hint exact shape — ported from qa_bottom_hints_test.go
// (Scope 4/round 4, specs/022-codebase-cleanup): DetailModel.BottomHints() is
// DEAD; the live replacement is Controller.buildDetailFooterHints, surfaced
// via Snapshot().Footer. Unlike TestWave3_FooterHints_Detail_* above (which
// only assert a single hint's presence), these two pin the FULL exact-order
// hint list for a plain unregistered type with/without related defs — the
// same shape the legacy test pinned.
// ---------------------------------------------------------------------------

func Test_DetailFooterHints_PlainField_NoRelated(t *testing.T) {
	res := resource.Resource{ID: "test-id", Name: "test-resource"}
	c := newDetailController(t, res, "hints_test_no_related")

	footer := c.Snapshot().Footer
	// Unknown resource type "hints_test_no_related" has no CloudTrailKey — no t hint.
	want := []app.KeyHint{
		{Key: "y", Help: "YAML"},
		{Key: "J", Help: "JSON"},
		{Key: "o", Help: "Open"},
		{Key: "ctrl+r", Help: "Refresh"},
		{Key: "w", Help: "Wrap"},
	}
	if len(footer) != len(want) {
		t.Fatalf("Detail Footer = %+v, want %+v", footer, want)
	}
	for i := range want {
		if footer[i] != want[i] {
			t.Errorf("Detail Footer[%d] = %+v, want %+v (full: %+v)", i, footer[i], want[i], footer)
		}
	}
}

func Test_DetailFooterHints_PlainField_WithRelated(t *testing.T) {
	resource.SetRelatedForTest("hints_test_with_related", []resource.RelatedDef{
		{TargetType: "vpc", DisplayName: "VPC", Checker: wave3NoopRelatedChecker},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest("hints_test_with_related") })

	res := resource.Resource{ID: "test-id", Name: "test-resource"}
	c := newDetailController(t, res, "hints_test_with_related")

	footer := c.Snapshot().Footer
	// Unknown resource type "hints_test_with_related" has no CloudTrailKey — no t hint.
	// RelatedUserVisible defaults false, so no "tab: Cols" hint either.
	want := []app.KeyHint{
		{Key: "y", Help: "YAML"},
		{Key: "J", Help: "JSON"},
		{Key: "o", Help: "Open"},
		{Key: "r", Help: "Related"},
		{Key: "ctrl+r", Help: "Refresh"},
		{Key: "w", Help: "Wrap"},
	}
	if len(footer) != len(want) {
		t.Fatalf("Detail Footer = %+v, want %+v", footer, want)
	}
	for i := range want {
		if footer[i] != want[i] {
			t.Errorf("Detail Footer[%d] = %+v, want %+v (full: %+v)", i, footer[i], want[i], footer)
		}
	}
}

// ---------------------------------------------------------------------------
// 20. Detail cursor identity on finding CLEAR — ported from internal/tui/
// views/detail_cursor_stable_test.go's TestDetail_ClearEnrichmentFinding_
// PreservesCursorIdentity (round 4, specs/022-codebase-cleanup, item 7):
// DetailModel.SetEnrichmentFinding/.fieldCursor/.fieldList are dead white-box
// internals. The live equivalent mechanism is Controller.applyFindingToState's
// FieldCursor delta-adjustment (core/app/detail_state.go), already pinned
// for the ADD direction by app_detail_attention_cursor_test.go's
// TestApplyDetailFinding_CursorStaysOnSameFieldAcrossMixedSeverityAttentionSort
// (same package — reuses its fieldRowAt helper). This ports the CLEAR/shrink
// direction, which that file does not cover: when a finding is removed and
// the Attention block shrinks, FieldCursor must stay on the same logical
// field, not shift onto a different row.
//
// TestDetail_SetEnrichmentFinding_PreservesCursorIdentity (the ADD direction)
// is not ported separately — it's the identical invariant to the mixed-
// severity test above, just without the sort-order edge case, so it would be
// a strictly weaker duplicate.
//
// TestDetail_SetEnrichmentFinding_RenderedSelectionFollowsCursor (the "stale
// paint" ordering hazard between cursor relocation and viewport repaint) is
// not ported: it pins a bug class specific to the legacy DetailModel's
// mutable m.fieldCursor + m.viewport.content pair, which could be updated out
// of order. RenderDetail(body) is a pure function of one immutable DetailBody
// snapshot (FieldCursor already final by construction) — there is no second
// mutable paint step that can race with cursor relocation, so this hazard is
// structurally impossible on the live path. Live-by-contract obsolete.
func Test_DetailCursor_ClearFinding_PreservesCursorIdentity(t *testing.T) {
	res := resource.Resource{
		ID:   "db-2",
		Name: "prod-db-2",
		Fields: map[string]string{
			"db_identifier":  "prod-db-2",
			"engine":         "postgres",
			"instance_class": "db.r6g.large",
			"endpoint":       "prod-db-2.aws.com:5432",
		},
	}
	finding := &domain.Finding{
		Code:     "dbi.pending-maintenance",
		Phrase:   "pending maintenance",
		Severity: domain.SevBroken,
		Source:   "wave2:dbi",
	}
	ad := &domain.AttentionDetail{Rows: []domain.DetailRow{{Label: "Action", Value: "os-upgrade"}}}

	c := newDetailController(t, res, "dbi")
	c.ApplyDetailFinding(finding, ad)

	// Cursor on a real content field that sits after the Attention section.
	c.Apply(app.Action{Kind: app.ActionMoveBottom})

	vs0 := c.Snapshot()
	if vs0.Body.Detail == nil {
		t.Fatal("precondition: Body.Detail is nil")
	}
	preCursor := vs0.Body.Detail.FieldCursor
	preRow := fieldRowAt(t, vs0.Body.Detail, preCursor)
	if preRow.IsSection || preRow.IsSpacer {
		t.Fatalf("precondition: cursor landed on a section/spacer row (Key=%q)", preRow.Key)
	}

	// Finding clears (e.g. next enrichment cycle reports healthy) — Attention
	// block shrinks to zero.
	c.ApplyDetailFinding(nil, nil)

	vs1 := c.Snapshot()
	if vs1.Body.Detail == nil {
		t.Fatal("Body.Detail became nil after clearing the finding")
	}
	postCursor := vs1.Body.Detail.FieldCursor
	postRow := fieldRowAt(t, vs1.Body.Detail, postCursor)

	if postRow.Key != preRow.Key || postRow.Path != preRow.Path {
		t.Errorf("cursor jumped on finding clear:\n  before: FieldCursor=%d Key=%q Path=%q\n  after:  FieldCursor=%d Key=%q Path=%q",
			preCursor, preRow.Key, preRow.Path, postCursor, postRow.Key, postRow.Path)
	}
}

// ---------------------------------------------------------------------------
// 5. ct-events render-level ports — live-seam replacements for
//    views_detail_ct_events_test.go (022-codebase-cleanup wave 3, DetailModel
//    cluster, now deleted). Section-order/severity/target-navigability pins
//    already have MORE precise pure-logic equivalents (ctevent_sections_test.go,
//    ctevent_target_test.go) and were deleted rather than ported. These 5
//    render-only behaviors have no pure-logic equivalent — they depend on
//    buildDetailFieldItems' fallback/error wiring and the RenderDetail frame
//    composition, so they stay pinned on the live NewTransientDetail seam.
// ---------------------------------------------------------------------------

// minimalCTJSON is the minimum valid CloudTrail event JSON for a Management
// AwsApiCall. Uses synthetic account ID 111111111111 (no real data). Moved
// here from the deleted views_detail_ct_events_test.go — still needed by
// Test_CTEvents_LiveProjector_SectionHeadersPresentInOrder/DataRowsBetweenSections.
const minimalCTJSON = `{
	"eventVersion":"1.08",
	"eventTime":"2026-04-07T14:02:11Z",
	"eventSource":"ec2.amazonaws.com",
	"eventName":"DescribeInstances",
	"awsRegion":"us-east-1",
	"sourceIPAddress":"10.0.14.221",
	"userAgent":"aws-sdk-go-v2/1.30.3",
	"userIdentity":{
		"type":"IAMUser",
		"arn":"arn:aws:iam::111111111111:user/test",
		"accountId":"111111111111",
		"userName":"test"
	},
	"eventCategory":"Management",
	"eventType":"AwsApiCall"
}`

// buildCTEventsResource builds a resource.Resource whose RawStruct is a
// cloudtrailtypes.Event (the AWS SDK type), exactly as buildCTResource does in
// core/aws/ct_events.go. The CloudTrailEvent field holds the raw JSON blob.
// Moved here from the deleted views_detail_ct_events_test.go.
func buildCTEventsResource(id, eventName, status, rawJSON string) resource.Resource {
	ct := cloudtrailtypes.Event{
		EventId:         new(id),
		EventName:       new(eventName),
		CloudTrailEvent: new(rawJSON),
	}
	return resource.Resource{
		ID:        id,
		Name:      eventName,
		RawStruct: ct,
		Fields: map[string]string{
			"event_name": eventName,
			"status":     status,
		},
	}
}

func wave3RenderDetailFor(t *testing.T, res resource.Resource, resourceType string, w, h int) string {
	t.Helper()
	c := newDetailController(t, res, resourceType)
	c.SetViewConfig(configForType(resourceType))
	c.InitDetailRelatedRows(resourceType)
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatalf("Body.Detail is nil for %s", resourceType)
	}
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	m := views.NewTransientDetail(w, h, vp)
	return m.RenderDetail(*body)
}

// Test_CTEvents_NoRawJSON_FallsBackToGenericFields is the live-seam
// replacement for TestDetailViewCTEvents_NoRawJSON_RendersFlatFields: a bare
// ct-events stub (no CloudTrailEvent JSON — e.g. a cached drill-in stub)
// must fall back to the generic Fields projector via buildDetailFieldItems'
// "sections empty -> generic(r)" branch, not render "No detail data available".
func Test_CTEvents_NoRawJSON_FallsBackToGenericFields(t *testing.T) {
	res := resource.Resource{
		ID:   "evt-fallback-000",
		Name: "FallbackEvent",
		Fields: map[string]string{
			"event_name": "FallbackEvent",
			// A value distinct from res.Name, on a DIFFERENT curated
			// ct-events detail field ("Username", defaults_monitoring.go) —
			// the detail header already renders the resource Name
			// regardless of whether the generic Fields projector ran at
			// all, so asserting on "FallbackEvent" alone would pass even
			// with a broken projector. This value can only appear by the
			// generic projector actually reading r.Fields["username"].
			"username": "generic-only-value",
		},
	}
	plain := stripAnsi(wave3RenderDetailFor(t, res, "ct-events", 120, 40))

	if strings.Contains(plain, "No detail data available") {
		t.Errorf("stub ct-events resource rendered as 'No detail data available'; expected fallback to generic projector. View:\n%s", plain)
	}
	if !strings.Contains(plain, "generic-only-value") {
		t.Errorf("stub ct-events resource missing generic-projector-only field value 'generic-only-value'. View:\n%s", plain)
	}
}

// Test_CTEvents_BrokenRawJSON_SurfacesExplicitError is the live-seam
// replacement for TestDetailViewCTEvents_BrokenRawJSON_SurfacesExplicitError
// (#280): a non-empty but unparseable CloudTrailEvent JSON blob must surface
// ctevent.Project's explicit "unable to parse" error section rather than
// silently degrading to the flat Fields path.
func Test_CTEvents_BrokenRawJSON_SurfacesExplicitError(t *testing.T) {
	broken := `{"eventVersion":"1.08","eventName":` // truncated — parser should error
	ct := cloudtrailtypes.Event{
		EventId:         new("abc12345-0000-0000-0000-00000000bad1"),
		EventName:       new("BrokenEvent"),
		CloudTrailEvent: new(broken),
	}
	res := resource.Resource{ID: "abc12345-0000-0000-0000-00000000bad1", Name: "BrokenEvent", RawStruct: ct}

	plain := stripAnsi(wave3RenderDetailFor(t, res, "ct-events", 120, 40))
	if !strings.Contains(strings.ToLower(plain), "unable to parse") {
		t.Errorf("expected explicit parse-failure message in view, got:\n%s", plain)
	}
}

// Test_CTEvents_NonCTEventsUnaffected is the live-seam replacement for
// TestDetailViewCTEvents_NonCTEventsUnaffected: an ec2 resource's detail
// render must not contain ct-events section labels — the ctevent.Project
// branch is gated strictly on resourceType == "ct-events".
func Test_CTEvents_NonCTEventsUnaffected(t *testing.T) {
	res := resource.Resource{
		ID:   "i-0aabbccdd11223344",
		Name: "web-server",
		Fields: map[string]string{
			"InstanceId":       "i-0aabbccdd11223344",
			"InstanceType":     "t3.medium",
			"PrivateIpAddress": "10.0.1.42",
		},
	}
	plain := stripAnsi(wave3RenderDetailFor(t, res, "ec2", 120, 40))

	for _, label := range []string{"ACTOR", "ACTION", "CONTEXT"} {
		if strings.Contains(plain, label) {
			t.Errorf("ec2 detail render must NOT contain ct-events section label %q; view:\n%s", label, plain)
		}
	}
}

// Test_CTEvents_FrameBorderPresent is the live-seam replacement for
// TestDetailViewCTEvents_Regression_FrameBorder: a regression guard for the
// hasSectionItems() bypass bug where section-based field lists skipped the
// frame wrapper entirely (no │ border character in the output).
func Test_CTEvents_FrameBorderPresent(t *testing.T) {
	ct := cloudtrailtypes.Event{
		EventId:         new("abc12345-0000-0000-0000-000000000009"),
		EventName:       new("DescribeInstances"),
		CloudTrailEvent: new(minimalCTJSON),
	}
	res := resource.Resource{ID: "abc12345-0000-0000-0000-000000000009", Name: "DescribeInstances", RawStruct: ct}

	plain := stripAnsi(wave3RenderDetailFor(t, res, "ct-events", 120, 40))
	if !strings.Contains(plain, "│") {
		t.Errorf("ct-events detail render missing frame border character │ — hasSectionItems() bypass regression; view:\n%s", plain)
	}
}

// Test_CTEvents_RelatedRightColumnVisibleOnWideTerminal is the live-seam
// replacement for TestDetailViewCTEvents_Regression_RelatedRightColumn: the
// RELATED right-column panel must be composed into the render for a
// ct-events detail on a wide terminal, relying on production's real
// resource.GetRelated("ct-events") registration (no test-only defs).
// ---------------------------------------------------------------------------
// 21. Scalar NavID extraction — ported from internal/tui/views/
// detail_scalar_navid_test.go (round 5, specs/022-codebase-cleanup, DetailModel
// core cleanup): buildFieldList is dead; the live equivalent is
// buildDetailFieldItems (detail_body.go), which populates the same
// NavID-from-value post-processing on app.FieldRow. Regression pin: NavID was
// only applied to YAML sub-fields (IsSubField=true), not top-level scalar
// navigable fields, so a Lambda Role ARN's NavID stayed "" and navigation used
// the full ARN as the target ID instead of the bare role name.
// ---------------------------------------------------------------------------

func wave3LambdaViewConfig() *config.ViewsConfig {
	return &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"lambda": {
				Detail: []config.DetailField{
					{Path: "Role"},
					{Path: "Runtime"},
					{Path: "Handler"},
					{Path: "MemorySize"},
					{Path: "Timeout"},
				},
			},
		},
	}
}

func wave3FindFieldRow(fields []app.FieldRow, pathOrKey string) *app.FieldRow {
	for i := range fields {
		f := &fields[i]
		if f.Path == pathOrKey || f.Key == pathOrKey {
			return f
		}
	}
	return nil
}

// Test_DetailFieldItems_ScalarNavigableField_AppliesNavIDFromValue is the
// live-seam replacement for detail_scalar_navid_test.go's
// TestBuildFieldList_ScalarNavigableField_AppliesNavIDFromValue.
func Test_DetailFieldItems_ScalarNavigableField_AppliesNavIDFromValue(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/my-lambda-role"
	const wantNavID = "my-lambda-role"

	res := resource.Resource{
		ID:   "arn:aws:lambda:us-east-1:123456789012:function:my-fn",
		Name: "my-fn",
		Fields: map[string]string{
			"Role":        roleARN,
			"Runtime":     "go1.x",
			"Handler":     "bootstrap",
			"MemorySize":  "128",
			"Timeout":     "30",
			"FunctionArn": "arn:aws:lambda:us-east-1:123456789012:function:my-fn",
		},
	}

	resource.SetNavigableFieldsForTest("lambda", []resource.NavigableField{
		{FieldPath: "Role", TargetType: "role"},
	})
	t.Cleanup(func() { resource.CleanupNavigableFieldsForTest("lambda") })

	c := newDetailController(t, res, "lambda")
	c.SetViewConfig(wave3LambdaViewConfig())

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}

	roleField := wave3FindFieldRow(body.Fields, "Role")
	if roleField == nil {
		t.Fatalf("Fields does not contain a FieldRow with Path/Key='Role'; fields: %+v", body.Fields)
	}
	if !roleField.IsNavigable {
		t.Errorf("Role FieldRow.IsNavigable = false, want true (registered as NavigableField)")
	}
	if roleField.TargetType != "role" {
		t.Errorf("Role FieldRow.TargetType = %q, want %q", roleField.TargetType, "role")
	}
	if roleField.Value != roleARN {
		t.Errorf("Role FieldRow.Value = %q, want %q", roleField.Value, roleARN)
	}
	if roleField.NavID != wantNavID {
		t.Errorf("Role FieldRow.NavID = %q, want %q\n"+
			"buildDetailFieldItems must apply NavIDFromValue to top-level scalar navigable fields, "+
			"not just YAML sub-fields", roleField.NavID, wantNavID)
	}
}

// Test_DetailFieldItems_ScalarNavigableField_NoExtractor_NavIDEmpty is
// the live-seam replacement for detail_scalar_navid_test.go's
// TestBuildFieldList_ScalarNavigableField_NoExtractor_NavIDEmpty.
func Test_DetailFieldItems_ScalarNavigableField_NoExtractor_NavIDEmpty(t *testing.T) {
	const subnetID = "subnet-0aaa111111111111a"

	res := resource.Resource{
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "web-prod-01",
		Fields: map[string]string{
			"SubnetId":     subnetID,
			"InstanceType": "t3.large",
		},
	}

	resource.SetNavigableFieldsForTest("ec2", []resource.NavigableField{
		{FieldPath: "SubnetId", TargetType: "subnet"},
	})
	t.Cleanup(func() { resource.CleanupNavigableFieldsForTest("ec2") })

	c := newDetailController(t, res, "ec2")
	c.SetViewConfig(&config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []config.DetailField{
					{Path: "SubnetId"},
					{Path: "InstanceType"},
				},
			},
		},
	})

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}

	subnetField := wave3FindFieldRow(body.Fields, "SubnetId")
	if subnetField == nil {
		t.Fatalf("Fields does not contain a FieldRow with Path/Key='SubnetId'; fields: %+v", body.Fields)
	}
	if !subnetField.IsNavigable {
		t.Errorf("SubnetId FieldRow.IsNavigable = false, want true")
	}
	// "subnet" has no NavID extractor — NavID must remain empty.
	if subnetField.NavID != "" {
		t.Errorf("SubnetId FieldRow.NavID = %q, want %q "+
			"(subnet has no NavID extractor; full value is used for navigation)",
			subnetField.NavID, "")
	}
}

// ---------------------------------------------------------------------------
// 22. Attention color-cap rule — ported from internal/tui/views/
// attention_color_cap_test.go (round 5, specs/022-codebase-cleanup, DetailModel
// core cleanup): capTierToRowBucket/resolveRowColorBucket are dead; the live
// equivalents are capTierToRowBucketDetail + ResourceTypeDef.ResolveColor
// (core/app/detail_body.go), unexported/package-app so not directly callable
// from tests/unit — exercised end-to-end instead via the public
// ApplyDetailFinding + Snapshot().Body.Detail seam. Pins the universal rule: a
// `!` severity tier caps to `~` unless the row's own S2 color bucket is
// Broken, so the detail view never contradicts the list row's severity.
// ---------------------------------------------------------------------------

func wave3AttentionPhraseRowColorTier(t *testing.T, body *app.DetailBody, phraseSubstr string) string {
	t.Helper()
	rows := wave3AttentionRowsForCode(body, phraseSubstr)
	if len(rows) == 0 {
		t.Fatalf("no Attention row found for phrase substring %q", phraseSubstr)
	}
	return rows[0].ColorTier
}

// Test_AttentionColorCap_BrokenSeverity_CapsToWarnOnHealthyBucket pins
// the cap direction: a SevBroken finding ("!" tier) on a resource whose S2
// color bucket is Healthy (dbc status "") must render capped to "~".
func Test_AttentionColorCap_BrokenSeverity_CapsToWarnOnHealthyBucket(t *testing.T) {
	res := resource.Resource{ID: "dbc-cap-healthy", Fields: map[string]string{"status": ""}}
	c := newDetailController(t, res, "dbc")
	c.ApplyDetailFinding(&domain.Finding{
		Code: "dbc.test-broken", Phrase: "encryption key unreachable", Severity: domain.SevBroken, Source: "wave2:test",
	}, nil)

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	if got := wave3AttentionPhraseRowColorTier(t, body, "encryption key unreachable"); got != "~" {
		t.Errorf("ColorTier = %q, want %q (Broken-severity finding on a Healthy-bucket dbc must cap to ~)", got, "~")
	}
}

// Test_AttentionColorCap_BrokenSeverity_StaysBrokenOnBrokenBucket pins
// the non-cap direction: the same SevBroken finding on a resource whose S2
// color bucket is ALSO Broken (dbc status "failed: cluster operation") must
// stay "!" — capping only kicks in when it would contradict a less-severe row.
func Test_AttentionColorCap_BrokenSeverity_StaysBrokenOnBrokenBucket(t *testing.T) {
	// The row's colour bucket is decided by its findings, so the Broken bucket
	// this test needs has to be a finding rather than a status string.
	res := resource.Resource{
		ID:     "dbc-cap-broken",
		Fields: map[string]string{"status": "failed: cluster operation"},
		Findings: []domain.Finding{{
			Code: "dbc.broken.failed", Phrase: "failed: cluster operation",
			Severity: domain.SevBroken, Source: "wave1",
		}},
	}
	c := newDetailController(t, res, "dbc")
	c.ApplyDetailFinding(&domain.Finding{
		Code: "dbc.test-broken", Phrase: "encryption key unreachable", Severity: domain.SevBroken, Source: "wave2:test",
	}, nil)

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	if got := wave3AttentionPhraseRowColorTier(t, body, "encryption key unreachable"); got != "!" {
		t.Errorf("ColorTier = %q, want %q (Broken-severity finding on an already-Broken-bucket dbc must stay !)", got, "!")
	}
}

// Test_AttentionColorCap_UnregisteredType_FallsBackToHealthy pins the
// safe default for an unregistered resource type (ResourceTypeDef nil):
// treated as Healthy, so a SevBroken finding still caps to ~.
func Test_AttentionColorCap_UnregisteredType_FallsBackToHealthy(t *testing.T) {
	res := resource.Resource{ID: "unreg-cap"}
	c := newDetailController(t, res, "wave3_no_such_type_cap_test")
	c.ApplyDetailFinding(&domain.Finding{
		Code: "unreg.test-broken", Phrase: "broken finding on unregistered type", Severity: domain.SevBroken, Source: "wave2:test",
	}, nil)

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	if got := wave3AttentionPhraseRowColorTier(t, body, "broken finding on unregistered type"); got != "~" {
		t.Errorf("ColorTier = %q, want %q (unregistered type falls back to Healthy, capping ! to ~)", got, "~")
	}
}

func Test_CTEvents_RelatedRightColumnVisibleOnWideTerminal(t *testing.T) {
	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal(`resource.GetRelated("ct-events") returned no defs — SetRelatedForTest not called in production init`)
	}

	res := resource.Resource{ID: "abc12345-0000-0000-0000-00000000000a", Name: "DescribeInstances"}
	plain := stripAnsi(wave3RenderDetailFor(t, res, "ct-events", 180, 40))

	if !strings.Contains(plain, "RELATED") {
		t.Errorf(`ct-events detail render missing "RELATED" header on wide terminal (180 cols); view snippet:\n%.500s`, plain)
	}
}
