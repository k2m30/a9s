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
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// ---------------------------------------------------------------------------
// RightColumnModel.Init (rightcolumn.go:73)
// ---------------------------------------------------------------------------

func TestTopUp_RightColumn_Init_ReturnsSameModelNilCmd(t *testing.T) {
	m := newRightColumn(nil, resource.Resource{}, "ec2")
	got, cmd := m.Init()
	if cmd != nil {
		t.Errorf("Init() cmd = %v, want nil", cmd)
	}
	// Compare identity-relevant fields directly (not reflect.DeepEqual: rows
	// carry an `error` field, which golangci-lint's deepequalerrors check
	// forbids comparing that way).
	if got.cursor != m.cursor || got.focused != m.focused || len(got.rows) != len(m.rows) {
		t.Errorf("Init() model = %+v, want unchanged %+v", got, m)
	}
}

// ---------------------------------------------------------------------------
// RightColumnModel.View (rightcolumn.go:214)
// ---------------------------------------------------------------------------

func TestTopUp_RightColumn_View_WidthZero_ReturnsEmpty(t *testing.T) {
	m := newRightColumn([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
	}, resource.Resource{}, "ec2")
	// SetSize never called: width defaults to zero.
	if got := m.View(); got != "" {
		t.Errorf("View() with width<=0 = %q, want empty string", got)
	}
}

func TestTopUp_RightColumn_View_NoRowsRegistered_ShowsHint(t *testing.T) {
	m := newRightColumn(nil, resource.Resource{}, "ec2")
	m.SetSize(30, 10)
	view := m.View()
	if !strings.Contains(view, "No related types registered") {
		t.Errorf("View() with no RelatedDefs = %q, want it to contain %q", view, "No related types registered")
	}
}

func TestTopUp_RightColumn_View_LoadingRows_ShowsDisplayNamesAndHeader(t *testing.T) {
	defs := []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
	}
	m := newRightColumn(defs, resource.Resource{}, "ec2")
	m.SetSize(40, 10)
	view := m.View()
	if !strings.Contains(view, "RELATED") {
		t.Errorf("View() = %q, want it to contain the \"RELATED\" header", view)
	}
	if !strings.Contains(view, "Target Groups") {
		t.Errorf("View() with a loading row = %q, want it to contain the row's display name %q", view, "Target Groups")
	}
}

func TestTopUp_RightColumn_View_FilterMatchesNothing_ShowsNoMatches(t *testing.T) {
	defs := []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
	}
	m := newRightColumn(defs, resource.Resource{}, "ec2")
	m.SetSize(40, 10)
	m.filterQuery = "zzz-no-such-related-type-zzz"

	view := m.View()
	if !strings.Contains(view, "No matches") {
		t.Errorf("View() with a filter query matching nothing = %q, want it to contain %q", view, "No matches")
	}
	if strings.Contains(view, "Target Groups") {
		t.Errorf("View() with a non-matching filter = %q, should not still show the filtered-out row", view)
	}
}

// ---------------------------------------------------------------------------
// RightColumnModel.Update / updateKeyMsg (rightcolumn.go:78, :133)
// ---------------------------------------------------------------------------

// topUpDefs3 returns three RelatedDefs and delivers RelatedCheckResult
// messages via m.Update so the resulting rows carry realistic, resolved
// states: "Target Groups" is actionable (Count=2), "ASGs" is a proven dead
// end (Count=0, not truncated), "Security Groups" is actionable via the
// blank-navigable RelatedDeferred state.
func topUpDefs3ResolvedModel(t *testing.T) RightColumnModel {
	t.Helper()
	defs := []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
		{TargetType: "asg", DisplayName: "ASGs", Checker: resource.NoopChecker},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: resource.NoopChecker},
	}
	m := newRightColumn(defs, resource.Resource{ID: "i-topup"}, "ec2")
	m.SetSize(40, 10)
	m.SetFocused(true)

	deliver := func(displayName, targetType string, result domain.RelatedCheckResult) {
		var cmd tea.Cmd
		m, cmd = m.Update(messages.RelatedCheckResult{DefDisplayName: displayName, Result: result})
		if cmd != nil {
			t.Fatalf("Update(RelatedCheckResult{%s}) returned a non-nil cmd, want nil", displayName)
		}
	}
	deliver("Target Groups", "tg", domain.RelatedCheckResult{TargetType: "tg", Count: 2, ResourceIDs: []string{"tg-1", "tg-2"}})
	deliver("ASGs", "asg", domain.RelatedCheckResult{TargetType: "asg", Count: 0})
	deliver("Security Groups", "sg", domain.RelatedCheckResult{TargetType: "sg", State: domain.RelatedDeferred})
	return m
}

func TestTopUp_RightColumn_Update_RelatedCheckResult_ResolvesRowsByDisplayName(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	view := m.View()
	if !strings.Contains(view, "(2)") {
		t.Errorf("View() after delivering Count=2 for tg = %q, want it to contain \"(2)\"", view)
	}
}

func TestTopUp_RightColumn_Update_Down_SkipsNonActionableRowToNextActionable(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	// ensureCursorValid already parked the cursor on the first actionable row (tg).
	if got := m.SelectedTypeName(); got != "Target Groups" {
		t.Fatalf("precondition: SelectedTypeName() = %q, want %q", got, "Target Groups")
	}

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Errorf("Update(Down) plain cursor move returned a non-nil cmd, want nil")
	}
	if got := m.SelectedTypeName(); got != "Security Groups" {
		t.Errorf("Update(Down) from %q = %q, want %q (skips the proven-zero ASGs row)", "Target Groups", got, "Security Groups")
	}
}

func TestTopUp_RightColumn_Update_Up_SkipsNonActionableRowToPrevActionable(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // tg -> sg
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp}) // sg -> tg (skips asg)
	if cmd != nil {
		t.Errorf("Update(Up) plain cursor move returned a non-nil cmd, want nil")
	}
	if got := m.SelectedTypeName(); got != "Target Groups" {
		t.Errorf("Update(Up) from %q = %q, want %q (skips the proven-zero ASGs row)", "Security Groups", got, "Target Groups")
	}
}

func TestTopUp_RightColumn_Update_Enter_OnActionableRow_ReturnsRelatedNavigateCmd(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Update(Enter) on an actionable row returned a nil cmd, want a messages.RelatedNavigate cmd")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("Update(Enter)()() = %T (%#v), want messages.RelatedNavigate", msg, msg)
	}
	if nav.TargetType != "tg" {
		t.Errorf("RelatedNavigate.TargetType = %q, want %q", nav.TargetType, "tg")
	}
	if len(nav.RelatedIDs) != 2 || nav.RelatedIDs[0] != "tg-1" {
		t.Errorf("RelatedNavigate.RelatedIDs = %v, want [tg-1 tg-2]", nav.RelatedIDs)
	}
	if nav.SourceResource.ID != "i-topup" {
		t.Errorf("RelatedNavigate.SourceResource.ID = %q, want %q", nav.SourceResource.ID, "i-topup")
	}
}

func TestTopUp_RightColumn_Update_Enter_OnNonActionableRow_ReturnsNilCmd(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m.cursor = 1 // ASGs: resolved, Count=0, not truncated -> not actionable
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("Update(Enter) on a proven-zero (non-actionable) row returned a non-nil cmd, want nil")
	}
}

func TestTopUp_RightColumn_Update_NotFocused_KeyMsgIsNoop(t *testing.T) {
	m := topUpDefs3ResolvedModel(t)
	m.SetFocused(false)
	before := m.SelectedTypeName()
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Error("Update(Down) while unfocused returned a non-nil cmd, want nil")
	}
	if got := m.SelectedTypeName(); got != before {
		t.Errorf("Update(Down) while unfocused changed selection: %q -> %q, want unchanged", before, got)
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
		t.Fatalf("FilterQuery() = %q, want %q", got, "Target")
	}
	view := m.View()
	if !strings.Contains(view, "Target Groups") {
		t.Errorf("View() filtered on %q = %q, want it to still contain %q", "Target", view, "Target Groups")
	}
	if strings.Contains(view, "Security Groups") {
		t.Errorf("View() filtered on %q = %q, should not contain the filtered-out row %q", "Target", view, "Security Groups")
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

func TestTopUp_RightColumn_Update_FilterMode_UpDown_MoveCursorWithinFilteredSet(t *testing.T) {
	defs := []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups A", Checker: resource.NoopChecker},
		{TargetType: "tg2", DisplayName: "Target Groups B", Checker: resource.NoopChecker},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: resource.NoopChecker},
	}
	m := newRightColumn(defs, resource.Resource{}, "ec2")
	m.SetSize(40, 10)
	m.SetFocused(true)
	m, _ = m.Update(messages.RelatedCheckResult{DefDisplayName: "Target Groups A", Result: domain.RelatedCheckResult{TargetType: "tg", Count: 1}})
	m, _ = m.Update(messages.RelatedCheckResult{DefDisplayName: "Target Groups B", Result: domain.RelatedCheckResult{TargetType: "tg2", Count: 1}})
	m, _ = m.Update(messages.RelatedCheckResult{DefDisplayName: "Security Groups", Result: domain.RelatedCheckResult{TargetType: "sg", Count: 1}})

	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "Target"})

	if got := m.SelectedTypeName(); got != "Target Groups A" {
		t.Fatalf("precondition: SelectedTypeName() after filtering = %q, want %q", got, "Target Groups A")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if got := m.SelectedTypeName(); got != "Target Groups B" {
		t.Errorf("Down inside filter mode = %q, want %q", got, "Target Groups B")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if got := m.SelectedTypeName(); got != "Target Groups A" {
		t.Errorf("Up inside filter mode = %q, want %q", got, "Target Groups A")
	}
}

func TestTopUp_RightColumn_Update_RelatedCheckResult_AmbiguousTargetTypeWithoutDisplayName_NoBind(t *testing.T) {
	// Two rows sharing TargetType "ct-events" (the self-pivot case), delivered
	// with an empty DefDisplayName: production always sets DefDisplayName, so
	// this is the deliberately-refused ambiguous fallback.
	defs := []resource.RelatedDef{
		{TargetType: "ct-events", DisplayName: "CT events by AccessKeyId", Checker: resource.NoopChecker},
		{TargetType: "ct-events", DisplayName: "CT events by Username", Checker: resource.NoopChecker},
	}
	m := newRightColumn(defs, resource.Resource{}, "ct-events")
	m, _ = m.Update(messages.RelatedCheckResult{Result: domain.RelatedCheckResult{TargetType: "ct-events", Count: 5}})

	for i, row := range m.rows {
		if !row.loading {
			t.Errorf("row[%d] (%s) loading = false, want still-loading (ambiguous match must be refused)", i, row.displayName)
		}
	}
}

func TestTopUp_RightColumn_Update_RelatedCheckResult_UnambiguousTargetTypeFallback_Binds(t *testing.T) {
	defs := []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
	}
	m := newRightColumn(defs, resource.Resource{}, "ec2")
	// No DefDisplayName, but exactly one row carries TargetType "tg" -> the
	// tight fallback should bind it.
	m, _ = m.Update(messages.RelatedCheckResult{Result: domain.RelatedCheckResult{TargetType: "tg", Count: 7}})

	if m.rows[0].loading {
		t.Error("row loading = true, want the unique-TargetType fallback to have resolved it")
	}
	if m.rows[0].count != 7 {
		t.Errorf("row.count = %d, want 7", m.rows[0].count)
	}
}

// ---------------------------------------------------------------------------
// RightColumnModel.HasActionableRows (rightcolumn.go:428)
// ---------------------------------------------------------------------------

// TestTopUp_RightColumn_Update_RelatedCheckResult_ReassignsCursorOffNonActionableSelection
// exercises the ensureCursorValid branch that jumps the cursor onto the first
// actionable row when the CURRENTLY SELECTED row is not actionable but some
// other visible row is: resolve the non-selected rows first (a proven-zero,
// then an actionable deferred row) while the cursor sits on the still-loading
// row at index 0 (not actionable), which must trigger the reassignment.
func TestTopUp_RightColumn_Update_RelatedCheckResult_ReassignsCursorOffNonActionableSelection(t *testing.T) {
	defs := []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
		{TargetType: "asg", DisplayName: "ASGs", Checker: resource.NoopChecker},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: resource.NoopChecker},
	}
	m := newRightColumn(defs, resource.Resource{}, "ec2")
	m.SetSize(40, 10)
	m.SetFocused(true)
	if got := m.SelectedTypeName(); got != "Target Groups" {
		t.Fatalf("precondition: cursor should default to index 0 (%q), got %q", "Target Groups", got)
	}

	// ASGs resolves to a proven zero: still no actionable row exists yet, so
	// the cursor (on the still-loading Target Groups) is left alone.
	m, _ = m.Update(messages.RelatedCheckResult{DefDisplayName: "ASGs", Result: domain.RelatedCheckResult{TargetType: "asg", Count: 0}})
	if got := m.SelectedTypeName(); got != "Target Groups" {
		t.Fatalf("after resolving a non-selected proven-zero row (no actionable rows yet): SelectedTypeName() = %q, want unchanged %q", got, "Target Groups")
	}

	// Security Groups resolves as actionable (RelatedDeferred): the selected
	// row (Target Groups, still loading -> not actionable) must now be
	// reassigned to the first actionable visible row.
	m, _ = m.Update(messages.RelatedCheckResult{DefDisplayName: "Security Groups", Result: domain.RelatedCheckResult{TargetType: "sg", State: domain.RelatedDeferred}})
	if got := m.SelectedTypeName(); got != "Security Groups" {
		t.Errorf("after a non-selected row resolves as the only actionable row: SelectedTypeName() = %q, want %q", got, "Security Groups")
	}
}

func TestTopUp_RightColumn_HasActionableRows_EmptyRows_False(t *testing.T) {
	m := newRightColumn(nil, resource.Resource{}, "ec2")
	if m.HasActionableRows() {
		t.Error("HasActionableRows() with no rows = true, want false")
	}
}

func TestTopUp_RightColumn_HasActionableRows_AllLoading_True(t *testing.T) {
	m := newRightColumn([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
	}, resource.Resource{}, "ec2")
	if !m.HasActionableRows() {
		t.Error("HasActionableRows() with a still-loading row = false, want true (loading rows are focusable)")
	}
}

func TestTopUp_RightColumn_HasActionableRows_AllResolvedZero_False(t *testing.T) {
	m := newRightColumn([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
	}, resource.Resource{}, "ec2")
	m, _ = m.Update(messages.RelatedCheckResult{DefDisplayName: "Target Groups", Result: domain.RelatedCheckResult{TargetType: "tg", Count: 0}})
	if m.HasActionableRows() {
		t.Error("HasActionableRows() with only a proven-zero row = true, want false")
	}
}

func TestTopUp_RightColumn_HasActionableRows_OneActionable_True(t *testing.T) {
	m := newRightColumn([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
	}, resource.Resource{}, "ec2")
	m, _ = m.Update(messages.RelatedCheckResult{DefDisplayName: "Target Groups", Result: domain.RelatedCheckResult{TargetType: "tg", Count: 3}})
	if !m.HasActionableRows() {
		t.Error("HasActionableRows() with a Count=3 resolved row = false, want true")
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
			{Key: "state", Label: "State"},                      // Key-form, present
			{Key: "missing_key", Label: "Missing"},               // Key-form, absent -> "-"
			{Path: "Name", Label: "Name"},                        // Path-form, exact Fields match
			{Path: "InstanceType", Label: "Type"},                // Path-form, snake_case Fields fallback
			{Path: "Nested.Value", Label: "Nested"},              // Path-form, RawStruct fallback
			{Path: "NoSuchField", Label: "Missing Path"},         // Path-form, resolves to "-"
			{Path: "Notes", Label: "Notes"},                      // Path-form, multiline value
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
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
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
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopChecker},
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
