// SPDX-License-Identifier: GPL-3.0-or-later

// coverage_topup_whitebox_test.go — white-box tests for live,
// production-reachable functions carrying low/zero coverage under the
// codecov gate (coverpkg=./internal/...): RightColumnModel's
// Init/View/Update/updateKeyMsg/HasActionableRows (reached in production via
// rs.rightCol in app_stack.go / NewRightColumn in runtime_adapter_related.go,
// but never previously exercised directly), the config-driven detail render
// path (renderFromConfig/computeKeyWidthFromFields/renderContent), and the
// DetailModel-specific SetSize/refreshViewportContent pair in
// detail_helpers.go. All are unexported (or exercise unexported branches), so
// they are tested directly from package views.
package views

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// ---------------------------------------------------------------------------
// RightColumnModel.Init (rightcolumn.go:53)
// ---------------------------------------------------------------------------

func TestTopUp_RightColumn_Init_ReturnsSameModelNilCmd(t *testing.T) {
	m := newRightColumn(nil, resource.Resource{}, "ec2")
	got, cmd := m.Init()
	if cmd != nil {
		t.Errorf("Init() cmd = %v, want nil", cmd)
	}
	if got.cursor != m.cursor || got.focused != m.focused {
		t.Errorf("Init() model = %+v, want unchanged %+v", got, m)
	}
}

// ---------------------------------------------------------------------------
// renderRelatedPanel (detail_helpers.go:248) — RightColumnModel's row-fact
// store (rows, View, SelectedTypeName, HasActionableRows) was removed; row
// facts and the RELATED panel's single render implementation now live in
// core/app (buildDetailRelatedBlocks) and renderRelatedPanel respectively.
// The "no defs registered" / "loading rows show display names + header"
// cases already have direct coverage via the live Controller+RenderDetail
// seam (tests/unit/rightcolumn_test.go's TestRightColumn_EmptyDefsShowsHint,
// TestRightColumn_ShowsLoadingState, TestRightColumn_ToggleShowsRelatedHeader);
// only the width<=0 guard and the filter-matches-nothing branch lacked
// coverage anywhere, so those two are pinned here directly.
// ---------------------------------------------------------------------------

func TestTopUp_RenderRelatedPanel_WidthZero_ReturnsEmpty(t *testing.T) {
	if got := renderRelatedPanel(nil, false, -1, 0, false, 0, 10); got != "" {
		t.Errorf("renderRelatedPanel(w<=0) = %q, want empty string", got)
	}
}

func TestTopUp_RenderRelatedPanel_FilterActiveNoRows_ShowsNoMatches(t *testing.T) {
	got := renderRelatedPanel(nil, true, -1, 0, false, 40, 10)
	if !strings.Contains(got, "No matches") {
		t.Errorf("renderRelatedPanel(filterActive=true, rows=nil) = %q, want it to contain %q", got, "No matches")
	}
}

// ---------------------------------------------------------------------------
// RightColumnModel.Update / updateKeyMsg (rightcolumn.go:61, :69) — the
// widget now holds only interaction state (focus/cursor/filter/scroll); row
// facts and cursor skip-to-actionable semantics moved to
// core/app/detail_cursor.go and are covered there:
//   - resolve-by-DisplayName + DefDisplayName ambiguity fallback:
//     tests/unit/detail_livepath_migration_test.go's
//     TestDetailController_ApplyDetailRelatedResultForResource_CtEventsSelfPivots_ResolveByDefDisplayName,
//     plus coverage_live_gaps_test.go's
//     TestLiveGap_ApplyDetailRelatedResultForResource_AmbiguousTargetTypeWithoutDisplayName_NoBind
//     and _UnambiguousTargetTypeFallback_Binds for the DefDisplayName-omitted
//     branches.
//   - Down/Up skip-to-next-actionable: tests/unit/app_related_cursor_skip_test.go
//     (TestRelatedCursor_MoveDown_SkipsDimmedRow and siblings).
//   - focus-entry landing on the first actionable/drillable row (the old
//     live incremental "reassign cursor as new results stream in" behavior
//     no longer exists — cursor placement only happens once, at
//     ActionToggleFocus time): tests/unit/app_related_focus_entry_test.go.
//   - filtered-set Up/Down movement: coverage_live_gaps_test.go's
//     TestLiveGap_RelatedCursor_FilterMode_UpDown_MoveWithinFilteredSet.
//   - Enter navigation sourcing SelectedRelatedRow from controller state:
//     coverage_live_gaps_test.go's
//     TestLiveGap_HandleDetailKeyMsg_RelatedPanelEnter_OnActionableRow_NavigatesUsingControllerRow.
// ---------------------------------------------------------------------------

// topUpDefs3ResolvedModel returns a focused, sized RightColumnModel. defs are
// accepted only for newRightColumn call-site parity — per rightcolumn.go's
// package doc, the widget stores no row facts, so there is nothing left to
// "resolve" here.
func topUpDefs3ResolvedModel(t *testing.T) RightColumnModel {
	t.Helper()
	defs := []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopCheckerForTest},
		{TargetType: "asg", DisplayName: "ASGs", Checker: resource.NoopCheckerForTest},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: resource.NoopCheckerForTest},
	}
	m := newRightColumn(defs, resource.Resource{ID: "i-topup"}, "ec2")
	m.SetSize(40, 10)
	m.SetFocused(true)
	return m
}

func TestTopUp_RightColumn_Update_NotFocused_KeyMsgIsNoop(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m.SetFocused(false)
	before := m.cursor
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Error("Update(Down) while unfocused returned a non-nil cmd, want nil")
	}
	if m.cursor != before {
		t.Errorf("Update(Down) while unfocused changed cursor: %d -> %d, want unchanged", before, m.cursor)
	}
}

func TestTopUp_RightColumn_Update_SearchKey_EntersFilterMode(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.IsFiltering() {
		t.Error("Update(Search key) did not enter filter mode (IsFiltering() = false)")
	}
	if m.FilterQuery() != "" {
		t.Errorf("Update(Search key) FilterQuery() = %q, want empty on entry", m.FilterQuery())
	}
}

func TestTopUp_RightColumn_Update_FilterMode_TypingNarrowsVisibleRows(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "Target"})

	if got := m.FilterQuery(); got != "Target" {
		t.Errorf("FilterQuery() after typing %q = %q, want %q", "Target", got, "Target")
	}
}

func TestTopUp_RightColumn_Update_FilterMode_BackspaceRemovesLastChar(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "Target"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	if got := m.FilterQuery(); got != "Targe" {
		t.Errorf("FilterQuery() after backspace = %q, want %q", got, "Targe")
	}
}

func TestTopUp_RightColumn_Update_FilterMode_BackspaceOnEmptyQuery_Noop(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	if got := m.FilterQuery(); got != "" {
		t.Errorf("FilterQuery() after backspace on empty query = %q, want empty", got)
	}
	if !m.IsFiltering() {
		t.Error("backspace on an empty filter query should not exit filter mode")
	}
}

func TestTopUp_RightColumn_Update_FilterMode_EscapeWithQuery_ClearsFilterOuterBranch(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "Target"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.IsFiltering() {
		t.Error("Escape with a non-empty filter query should exit filter mode")
	}
	if got := m.FilterQuery(); got != "" {
		t.Errorf("FilterQuery() after Escape = %q, want empty", got)
	}
}

func TestTopUp_RightColumn_Update_FilterMode_EscapeWithEmptyQuery_ClearsInnerBranch(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	// filterQuery is still "" here, so the outer "escape && non-empty query"
	// guard does not fire; this exercises the filterActive-block Escape case.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.IsFiltering() {
		t.Error("Escape while filtering with an empty query should still exit filter mode")
	}
}

func TestTopUp_RightColumn_Update_FilterMode_Enter_ExitsFilterModeKeepsQuery(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "Target"})
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Error("Update(Enter) inside filter mode returned a non-nil cmd, want nil")
	}
	if m.IsFiltering() {
		t.Error("Update(Enter) inside filter mode should exit filter mode")
	}
	if got := m.FilterQuery(); got != "Target" {
		t.Errorf("FilterQuery() after Enter = %q, want the query preserved (%q)", got, "Target")
	}
}

// ---------------------------------------------------------------------------
// computeKeyWidthFromFields (detail_render.go:96)
// ---------------------------------------------------------------------------

func TestTopUp_ComputeKeyWidthFromFields_ShortLabels_ReturnsMinimum22(t *testing.T) {
	fields := []config.DetailField{{Key: "a"}, {Label: "Short"}}
	if got := computeKeyWidthFromFields(fields); got != 22 {
		t.Errorf("computeKeyWidthFromFields(short labels) = %d, want minimum 22", got)
	}
}

func TestTopUp_ComputeKeyWidthFromFields_LongLabel_ExpandsWidth(t *testing.T) {
	fields := []config.DetailField{{Label: "ThisIsAVeryLongDetailFieldLabel"}} // 31 chars
	want := len("ThisIsAVeryLongDetailFieldLabel") + 1
	if got := computeKeyWidthFromFields(fields); got != want {
		t.Errorf("computeKeyWidthFromFields(long label) = %d, want %d", got, want)
	}
}

// ---------------------------------------------------------------------------
// renderFromConfig (detail_render.go:111) + renderContent (detail_render.go:39)
// ---------------------------------------------------------------------------

type topUpRawStruct struct {
	Nested struct {
		Value string
	}
}

func topUpConfigModel(res resource.Resource) DetailModel {
	const rt = "topup-renderfromconfig-type"
	vc := &config.ViewsConfig{Views: map[string]config.ViewDef{
		rt: {Detail: []config.DetailField{
			{Key: "state", Label: "State"},               // Key-form, present
			{Key: "missing_key", Label: "Missing"},       // Key-form, absent -> "-"
			{Path: "Name", Label: "Name"},                // Path-form, exact Fields match
			{Path: "InstanceType", Label: "Type"},        // Path-form, snake_case Fields fallback
			{Path: "Nested.Value", Label: "Nested"},      // Path-form, RawStruct fallback
			{Path: "NoSuchField", Label: "Missing Path"}, // Path-form, resolves to "-"
			{Path: "Notes", Label: "Notes"},              // Path-form, multiline value
		}},
	}}
	m := NewDetailWithCtrl(res, rt, vc, keys.Default(), nil)
	return m
}

func topUpConfigResource() resource.Resource {
	raw := topUpRawStruct{}
	raw.Nested.Value = "nested-value"
	return resource.Resource{
		Fields: map[string]string{
			"state":         "running",
			"Name":          "my-instance",
			"instance_type": "t3.micro",
			"Notes":         "line one\nline two",
		},
		RawStruct: raw,
	}
}

func TestTopUp_RenderFromConfig_KeyForm_PresentAndAbsent(t *testing.T) {
	m := topUpConfigModel(topUpConfigResource())
	kv := func(k, v string) string { return k + "=" + v }
	lines := m.renderFromConfig(kv)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "State=running") {
		t.Errorf("renderFromConfig() = %q, want it to contain %q (Key-form present)", joined, "State=running")
	}
	if !strings.Contains(joined, "Missing=-") {
		t.Errorf("renderFromConfig() = %q, want it to contain %q (Key-form absent falls back to \"-\")", joined, "Missing=-")
	}
}

func TestTopUp_RenderFromConfig_PathForm_ExactFieldsMatch(t *testing.T) {
	m := topUpConfigModel(topUpConfigResource())
	kv := func(k, v string) string { return k + "=" + v }
	lines := m.renderFromConfig(kv)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Name=my-instance") {
		t.Errorf("renderFromConfig() = %q, want it to contain %q (Path-form exact Fields match)", joined, "Name=my-instance")
	}
}

func TestTopUp_RenderFromConfig_PathForm_SnakeCaseFallback(t *testing.T) {
	m := topUpConfigModel(topUpConfigResource())
	kv := func(k, v string) string { return k + "=" + v }
	lines := m.renderFromConfig(kv)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Type=t3.micro") {
		t.Errorf("renderFromConfig() = %q, want it to contain %q (Path %q snake_case fallback to Fields[%q])", joined, "Type=t3.micro", "InstanceType", "instance_type")
	}
}

func TestTopUp_RenderFromConfig_PathForm_RawStructFallback(t *testing.T) {
	m := topUpConfigModel(topUpConfigResource())
	kv := func(k, v string) string { return k + "=" + v }
	lines := m.renderFromConfig(kv)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Nested=nested-value") {
		t.Errorf("renderFromConfig() = %q, want it to contain %q (Path-form RawStruct reflection fallback)", joined, "Nested=nested-value")
	}
}

func TestTopUp_RenderFromConfig_PathForm_UnresolvedFallsBackToDash(t *testing.T) {
	m := topUpConfigModel(topUpConfigResource())
	kv := func(k, v string) string { return k + "=" + v }
	lines := m.renderFromConfig(kv)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Missing Path=-") {
		t.Errorf("renderFromConfig() = %q, want it to contain %q (no Fields nor RawStruct match falls back to \"-\")", joined, "Missing Path=-")
	}
}

func TestTopUp_RenderFromConfig_MultilineValue_RendersAsSection(t *testing.T) {
	m := topUpConfigModel(topUpConfigResource())
	kv := func(k, v string) string { return k + "=" + v }
	lines := m.renderFromConfig(kv)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "line one") || !strings.Contains(joined, "line two") {
		t.Errorf("renderFromConfig() = %q, want both multiline sub-lines rendered", joined)
	}
	// A multiline value must NOT go through kv() (no "Notes=" prefix line).
	if strings.Contains(joined, "Notes=line one") {
		t.Errorf("renderFromConfig() = %q, multiline values should render as a section, not via kv()", joined)
	}
}

func TestTopUp_RenderFromConfig_NoDetailFields_ReturnsNil(t *testing.T) {
	const rt = "topup-renderfromconfig-empty"
	m := NewDetailWithCtrl(resource.Resource{}, rt, nil, keys.Default(), nil) // no ViewsConfig and no default entry for rt
	kv := func(k, v string) string { return k + "=" + v }
	if got := m.renderFromConfig(kv); got != nil {
		t.Errorf("renderFromConfig() with no registered Detail fields = %v, want nil", got)
	}
}

func TestTopUp_RenderContent_ConfigDriven_UsesRenderFromConfig(t *testing.T) {
	m := topUpConfigModel(topUpConfigResource())
	content := m.renderContent()
	if !strings.Contains(content, "running") {
		t.Errorf("renderContent() (config-driven) = %q, want it to contain the Key-form value %q", content, "running")
	}
}

func TestTopUp_RenderContent_FieldsMapFallback_NoConfig_SortedKeys(t *testing.T) {
	m := NewDetailWithCtrl(resource.Resource{Fields: map[string]string{
		"Zebra": "z-val",
		"Alpha": "a-val",
	}}, "topup-renderfromconfig-fieldsfallback", nil, keys.Default(), nil)
	content := m.renderContent()

	if !strings.Contains(content, "a-val") || !strings.Contains(content, "z-val") {
		t.Fatalf("renderContent() (Fields-map fallback, no config) = %q, want both field values", content)
	}
	if strings.Index(content, "Alpha") > strings.Index(content, "Zebra") {
		t.Errorf("renderContent() (Fields-map fallback) = %q, want keys in sorted order (Alpha before Zebra)", content)
	}
}

func TestTopUp_RenderContent_NoViewConfigNoFields_ShowsNoDetailData(t *testing.T) {
	m := NewDetailWithCtrl(resource.Resource{}, "topup-renderfromconfig-nodata", nil, keys.Default(), nil)
	content := m.renderContent()
	if !strings.Contains(content, "No detail data available") {
		t.Errorf("renderContent() with no config and no fields = %q, want %q", content, "No detail data available")
	}
}

// ---------------------------------------------------------------------------
// DetailModel.SetSize (detail_helpers.go:25)
// ---------------------------------------------------------------------------

func TestTopUp_DetailModel_SetSize_WideWithRelatedDefs_AutoShowsRightColumn(t *testing.T) {
	const rt = "topup-setsize-wide"
	resource.SetRelatedForTest(rt, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopCheckerForTest},
	})
	defer resource.CleanupRelatedForTest(rt)

	m := NewDetailWithCtrl(resource.Resource{Fields: map[string]string{"a": "b"}}, rt, nil, keys.Default(), nil)
	m.SetSize(100, 20)

	if !m.rightColShowing() {
		t.Error("SetSize(100, 20) with registered related defs: right column should auto-show")
	}
	if !m.NeedsRelatedCheck() {
		t.Error("SetSize(100, 20) with registered related defs: NeedsRelatedCheck() should be true")
	}
}

func TestTopUp_DetailModel_SetSize_NoRelatedDefs_NeverShowsRightColumn(t *testing.T) {
	m := NewDetailWithCtrl(resource.Resource{Fields: map[string]string{"a": "b"}}, "topup-setsize-none", nil, keys.Default(), nil)
	m.SetSize(100, 20)

	if m.rightColShowing() {
		t.Error("SetSize(100, 20) with no registered related defs: right column should stay hidden")
	}
	if m.NeedsRelatedCheck() {
		t.Error("SetSize(100, 20) with no registered related defs: NeedsRelatedCheck() should be false")
	}
}

func TestTopUp_DetailModel_SetSize_NarrowAfterWide_HidesAutoShownRightColumn(t *testing.T) {
	const rt = "topup-setsize-resize"
	resource.SetRelatedForTest(rt, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopCheckerForTest},
	})
	defer resource.CleanupRelatedForTest(rt)

	m := NewDetailWithCtrl(resource.Resource{Fields: map[string]string{"a": "b"}}, rt, nil, keys.Default(), nil)
	m.SetSize(100, 20)
	if !m.rightColShowing() {
		t.Fatal("precondition: right column should auto-show at width=100")
	}

	m.SetSize(40, 20)
	if m.rightColShowing() {
		t.Error("SetSize(40, 20) after an auto-shown right column: should hide below MinInnerContentWidth")
	}
}

// ---------------------------------------------------------------------------
// DetailModel.refreshViewportContent (detail_helpers.go:105)
// ---------------------------------------------------------------------------

func TestTopUp_DetailModel_RefreshViewportContent_SearchActiveScrollsToMatch(t *testing.T) {
	const marker = "zzz-detailhelpers-marker-zzz"
	res := resource.Resource{Fields: map[string]string{
		"AFirst": "first-field",
		"Marker": marker,
		"ZLast":  "last-field",
	}}
	m := NewDetailWithCtrl(res, "topup-refreshviewport-match", nil, keys.Default(), nil)
	m.SetSize(80, 1)

	wantLine := livegapFindLine(m.renderContent(), marker)
	if wantLine <= 0 {
		t.Fatalf("precondition: want the marker on a non-zero line, got %d in:\n%s", wantLine, m.renderContent())
	}

	m.search.Activate()
	m.search.SetQuery(marker)
	m.refreshViewportContent()

	vp := m.Viewport()
	if got := vp.YOffset(); got != wantLine {
		t.Errorf("refreshViewportContent() with active search: viewport YOffset = %d, want %d", got, wantLine)
	}
}

func TestTopUp_DetailModel_RefreshViewportContent_SearchNoMatch_NoScroll(t *testing.T) {
	res := resource.Resource{Fields: map[string]string{"Field": "value"}}
	m := NewDetailWithCtrl(res, "topup-refreshviewport-nomatch", nil, keys.Default(), nil)
	m.SetSize(80, 10)

	m.search.Activate()
	m.search.SetQuery("no-such-substring-anywhere")
	m.refreshViewportContent()

	vp := m.Viewport()
	if got := vp.YOffset(); got != 0 {
		t.Errorf("refreshViewportContent() with a non-matching search query: viewport YOffset = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// padLeft (costs.go:273)
// ---------------------------------------------------------------------------

func TestTopUp_PadLeft_WidthZeroOrNegative_ReturnsEmpty(t *testing.T) {
	if got := padLeft("abc", 0); got != "" {
		t.Errorf("padLeft(%q, 0) = %q, want empty", "abc", got)
	}
	if got := padLeft("abc", -5); got != "" {
		t.Errorf("padLeft(%q, -5) = %q, want empty", "abc", got)
	}
}

func TestTopUp_PadLeft_StringWiderThanWidth_TruncatesLikePadOrTrunc(t *testing.T) {
	got := padLeft("abcdef", 3)
	want := text.PadOrTrunc("abcdef", 3)
	if got != want {
		t.Errorf("padLeft(%q, 3) = %q, want %q (delegate to text.PadOrTrunc)", "abcdef", got, want)
	}
}

func TestTopUp_PadLeft_StringNarrowerThanWidth_PadsWithLeadingSpaces(t *testing.T) {
	got := padLeft("ab", 5)
	want := "   ab"
	if got != want {
		t.Errorf("padLeft(%q, 5) = %q, want %q", "ab", got, want)
	}
}

// ---------------------------------------------------------------------------
// clipCostsRows (costs.go:198)
// ---------------------------------------------------------------------------

func topUpCostsLines(n int) []string {
	lines := make([]string, 0, n+2)
	lines = append(lines, "HEADER")
	for i := range n {
		lines = append(lines, "r"+string(rune('0'+i)))
	}
	lines = append(lines, "TOTAL")
	return lines
}

func TestTopUp_ClipCostsRows_HeightNonPositive_ReturnsNil(t *testing.T) {
	lines := topUpCostsLines(10)
	got := clipCostsRows(lines, 3, 0)
	if got != nil {
		t.Errorf("clipCostsRows(height=0) = %v, want nil", got)
	}
}

func TestTopUp_ClipCostsRows_HeightOne_ReturnsTotalOnly(t *testing.T) {
	lines := topUpCostsLines(10)
	got := clipCostsRows(lines, 3, 1)
	want := []string{"TOTAL"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("clipCostsRows(height=1) = %v, want %v (total only)", got, want)
	}
}

func TestTopUp_ClipCostsRows_FitsWithinHeight_ReturnsUnchanged(t *testing.T) {
	lines := topUpCostsLines(3) // 5 total lines
	got := clipCostsRows(lines, 1, 10)
	if !reflect.DeepEqual(got, lines) {
		t.Errorf("clipCostsRows(len(lines)<=height) = %v, want unchanged input", got)
	}
}

func TestTopUp_ClipCostsRows_BudgetNonPositive_ReturnsHeaderAndTotalOnly(t *testing.T) {
	lines := topUpCostsLines(10) // 12 total lines
	got := clipCostsRows(lines, 5, 2)
	want := []string{"HEADER", "TOTAL"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("clipCostsRows(height=2 -> budget<=0) = %v, want %v (header+total only, no data rows)", got, want)
	}
}

func TestTopUp_ClipCostsRows_DataFitsBudget_ReturnsHeaderAllDataTotal(t *testing.T) {
	lines := topUpCostsLines(10) // budget = 13 at height=15
	got := clipCostsRows(lines, 5, 15)
	if !reflect.DeepEqual(got, lines) {
		t.Errorf("clipCostsRows(len(data)<=budget) = %v, want header+all-data+total unchanged", got)
	}
}

func TestTopUp_ClipCostsRows_WindowsAroundCursor(t *testing.T) {
	lines := topUpCostsLines(10) // header, r0..r9, total; height=5 -> budget=3
	got := clipCostsRows(lines, 5, 5)
	want := []string{"HEADER", "r4", "r5", "r6", "TOTAL"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("clipCostsRows(cursorRow=5, height=5) = %v, want %v", got, want)
	}
}

func TestTopUp_ClipCostsRows_CursorNearEnd_ClampsWindowStart(t *testing.T) {
	lines := topUpCostsLines(10)
	got := clipCostsRows(lines, 9, 5)
	want := []string{"HEADER", "r7", "r8", "r9", "TOTAL"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("clipCostsRows(cursorRow=9, height=5) = %v, want %v (clamped to the last full window)", got, want)
	}
}

// ---------------------------------------------------------------------------
// SearchModel.Update (search.go:153)
// ---------------------------------------------------------------------------

func TestTopUp_SearchModel_Update_PasteMsg_AppendsAndRecomputes(t *testing.T) {
	var s SearchModel
	s.SetContent("hello world")
	s.SetQuery("wor")
	s, _ = s.Update(tea.PasteMsg{Content: "ld"})
	if got := s.Query(); got != "world" {
		t.Errorf("Update(PasteMsg{Content:%q}) query = %q, want %q", "ld", got, "world")
	}
	if s.MatchCount() != 1 {
		t.Errorf("Update(PasteMsg) MatchCount() = %d, want 1 (recomputed against %q)", s.MatchCount(), "hello world")
	}
}

func TestTopUp_SearchModel_Update_SearchPasteMsg_AppendsAndRecomputes(t *testing.T) {
	var s SearchModel
	s.SetContent("hello world")
	s.SetQuery("wor")
	s, _ = s.Update(searchPasteMsg("ld"))
	if got := s.Query(); got != "world" {
		t.Errorf("Update(searchPasteMsg(%q)) query = %q, want %q", "ld", got, "world")
	}
	if s.MatchCount() != 1 {
		t.Errorf("Update(searchPasteMsg) MatchCount() = %d, want 1 (recomputed against %q)", s.MatchCount(), "hello world")
	}
}

func TestTopUp_SearchModel_Update_UnrelatedMsgType_Noop(t *testing.T) {
	var s SearchModel
	s.SetQuery("abc")
	before := s
	got, cmd := s.Update(tea.WindowSizeMsg{})
	if cmd != nil {
		t.Error("Update(unrelated msg) returned a non-nil cmd, want nil")
	}
	if got.Query() != before.Query() {
		t.Errorf("Update(unrelated msg) changed query %q -> %q, want unchanged", before.Query(), got.Query())
	}
}

func TestTopUp_SearchModel_Update_Backspace_RemovesLastRune(t *testing.T) {
	var s SearchModel
	s.SetQuery("abc")
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got := s.Query(); got != "ab" {
		t.Errorf("Update(Backspace) query = %q, want %q", got, "ab")
	}
}

func TestTopUp_SearchModel_Update_BackspaceOnEmptyQuery_Noop(t *testing.T) {
	var s SearchModel
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got := s.Query(); got != "" {
		t.Errorf("Update(Backspace) on empty query = %q, want empty", got)
	}
}

func TestTopUp_SearchModel_Update_EnterOnEmptyQuery_Deactivates(t *testing.T) {
	var s SearchModel
	s.Activate()
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if s.IsActive() {
		t.Error("Update(Enter) on empty query should Deactivate()")
	}
}

func TestTopUp_SearchModel_Update_EnterOnNonEmptyQuery_ExitsInputModeKeepsActive(t *testing.T) {
	var s SearchModel
	s.Activate()
	s.SetQuery("abc")
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !s.IsActive() {
		t.Error("Update(Enter) on a non-empty query should stay active")
	}
	if s.IsInputMode() {
		t.Error("Update(Enter) on a non-empty query should exit input mode")
	}
	if s.Query() != "abc" {
		t.Errorf("Update(Enter) query = %q, want preserved %q", s.Query(), "abc")
	}
}

func TestTopUp_SearchModel_Update_Escape_Deactivates(t *testing.T) {
	var s SearchModel
	s.Activate()
	s.SetQuery("abc")
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if s.IsActive() {
		t.Error("Update(Escape) should Deactivate()")
	}
	if s.Query() != "" {
		t.Errorf("Update(Escape) query = %q, want cleared", s.Query())
	}
}

func TestTopUp_SearchModel_Update_CtrlV_ReturnsClipboardReadCmd(t *testing.T) {
	var s SearchModel
	s.SetQuery("abc")
	got, cmd := s.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Update(ctrl+v) returned a nil cmd, want the clipboard-read cmd")
	}
	if got.Query() != "abc" {
		t.Errorf("Update(ctrl+v) query = %q, want unchanged %q (async read, not yet applied)", got.Query(), "abc")
	}
}

func TestTopUp_SearchModel_Update_PrintableChar_AppendsAndRecomputes(t *testing.T) {
	var s SearchModel
	s.SetContent("hello world")
	s, _ = s.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if got := s.Query(); got != "w" {
		t.Errorf("Update(printable 'w') query = %q, want %q", got, "w")
	}
	if s.MatchCount() != 1 {
		t.Errorf("Update(printable 'w') MatchCount() against %q = %d, want 1", "hello world", s.MatchCount())
	}
}

func TestTopUp_SearchModel_Update_UnmatchedSpecialKeyNoText_Noop(t *testing.T) {
	var s SearchModel
	s.SetQuery("abc")
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if got := s.Query(); got != "abc" {
		t.Errorf("Update(Tab, no text) query = %q, want unchanged %q", got, "abc")
	}
}

// ---------------------------------------------------------------------------
// ResourceListModel.Update's ScrollRight key-handler guard (resourcelist.go
// ~line 349-373): consults m.fitColumns before forwarding
// app.ActionScrollRight to the controller, so pressing "l" is a no-op once
// every column already fits (unlike the controller action itself, which
// increments ScrollX unconditionally — see core/app/list_test.go's
// TestListAction_ScrollRight_IncreasesScrollX). Only reachable through
// Update(), so it is tested here directly rather than via the dead View().
// ---------------------------------------------------------------------------

func TestTopUp_ResourceList_ScrollRightGuard_BlocksWhenAllColumnsFit(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName: "topup-scrollguard-fit",
		Columns: []resource.Column{
			{Key: "a", Title: "A", Width: 10},
			{Key: "b", Title: "B", Width: 10},
		},
	}
	m := NewResourceList(td, nil, keys.Default())
	m.SetSize(200, 20) // plenty of room — both columns already fit.
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "topup-scrollguard-fit",
		Resources:    []resource.Resource{{ID: "r-1", Fields: map[string]string{"a": "1", "b": "2"}}},
	})

	before := m.ctrl.Snapshot().Body.List.ScrollX
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
	after := m.ctrl.Snapshot().Body.List.ScrollX

	if after != before {
		t.Errorf("ScrollRight guard: ScrollX changed from %d to %d even though all columns already fit", before, after)
	}
}

func TestTopUp_ResourceList_ScrollRightGuard_AllowsScroll_ClampsAtLastColumn(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName: "topup-scrollguard-overflow",
		Columns: []resource.Column{
			{Key: "a", Title: "A", Width: 20},
			{Key: "b", Title: "B", Width: 20},
			{Key: "c", Title: "C", Width: 20},
			{Key: "d", Title: "D", Width: 20},
		},
	}
	m := NewResourceList(td, nil, keys.Default())
	m.SetSize(30, 20) // narrow — columns overflow, scrolling should be allowed.
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "topup-scrollguard-overflow",
		Resources:    []resource.Resource{{ID: "r-1", Fields: map[string]string{"a": "1", "b": "2", "c": "3", "d": "4"}}},
	})

	// Press "l" many more times than there are columns.
	for range 20 {
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
	}

	body := *m.ctrl.Snapshot().Body.List
	if body.ScrollX <= 0 {
		t.Fatal("ScrollRight guard: expected ScrollX to have advanced past 0 when columns overflow")
	}
	if body.ScrollX >= len(td.Columns) {
		t.Errorf("ScrollRight guard: ScrollX (%d) must clamp below the column count (%d), not run past it", body.ScrollX, len(td.Columns))
	}
	out := m.RenderList(body)
	if strings.Contains(out, "No resources found") {
		t.Errorf("ScrollRight guard: scrolling past the last column collapsed the render to empty:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// ScrollState.Clamp (scroll.go:39)
// ---------------------------------------------------------------------------

func TestTopUp_ScrollState_Clamp_TotalZero_SetsCursorZero(t *testing.T) {
	s := ScrollState{cursor: 5, total: 0}
	s.Clamp()
	if s.cursor != 0 {
		t.Errorf("Clamp() with total=0: cursor = %d, want 0", s.cursor)
	}
}

func TestTopUp_ScrollState_Clamp_NegativeCursor_ClampsToZero(t *testing.T) {
	s := ScrollState{cursor: -5, total: 10}
	s.Clamp()
	if s.cursor != 0 {
		t.Errorf("Clamp() with cursor=-5, total=10: cursor = %d, want 0", s.cursor)
	}
}

func TestTopUp_ScrollState_Clamp_CursorBeyondTotal_ClampsToLastIndex(t *testing.T) {
	s := ScrollState{cursor: 99, total: 10}
	s.Clamp()
	if s.cursor != 9 {
		t.Errorf("Clamp() with cursor=99, total=10: cursor = %d, want 9", s.cursor)
	}
}

func TestTopUp_ScrollState_Clamp_CursorWithinRange_Unchanged(t *testing.T) {
	s := ScrollState{cursor: 4, total: 10}
	s.Clamp()
	if s.cursor != 4 {
		t.Errorf("Clamp() with cursor=4 (in-range), total=10: cursor = %d, want unchanged 4", s.cursor)
	}
}
