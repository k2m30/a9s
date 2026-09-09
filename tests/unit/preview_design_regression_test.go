package unit_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

func previewApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	return tuitest.Step(m, msg)
}

func previewView(m tui.Model) string {
	return stripAnsi(m.View().Content)
}

func newPreviewDemoModel(t *testing.T, w, h int) tui.Model {
	t.Helper()
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = previewApplyMsg(m, tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func previewEC2Resource() resource.Resource {
	return resource.Resource{
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "web-prod-01",
		Fields: map[string]string{
			"InstanceId": "i-0a1b2c3d4e5f60001",
			"VpcId":      "vpc-0abc123def456789a",
			"SubnetId":   "subnet-0aaa111111111111a",
			"ImageId":    "ami-0abc123def456789a",
			"status":     "running",
		},
	}
}

// TestPreview_RightColumnFilter_HidesNonMatchingRows (DetailModel.View()) and
// TestPreview_DetailCopyContent_CopiesCurrentFieldValue (DetailModel.
// CopyContent()) deleted (round 4, specs/022-codebase-cleanup, MIXED verdict:
// delete Detail .View()/.CopyContent halves). Live equivalents:
// TestBug_Root_RightColumnFilter_TypingFiltersRows (rightcolumn_root_filter_
// regression_test.go — same right-column filter narrows/hides contract, on
// the live tui.New() root path) and TestQA_Copy_Detail_CopiesFieldValue
// (qa_copy_test.go — same "c copies the active field's value" contract, on
// the live tui.New() root path).
//
// TestPreview_RightColumnTabFocus_SkipsDimRowsOnEnter below still drives
// DetailModel.Update() directly (not .View()/.CopyContent) — out of this
// item's literal scope; kept as-is and flagged as a residual gap.

// TestPreview_RightColumnTabFocus_SkipsDimRowsOnEnter is the live-seam
// replacement for the retired views.NewDetail(...).Update(RelatedCheckResult)
// .Update(KeyTab).Update(KeyEnter) chain (DetailModel.Update is dead; see
// specs/022-codebase-cleanup/wave3-map-detail.md). Drives the real
// tui.Model root path: Tab focuses the right column (auto-skipping to the
// first drillable row), Enter dispatches navigation for that row.
func TestPreview_RightColumnTabFocus_SkipsDimRowsOnEnter(t *testing.T) {
	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	t.Cleanup(func() { resource.SetRelatedForTest("ec2", oldDefs) })
	resource.SetRelatedForTest("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopCheckerForTest},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: resource.NoopCheckerForTest},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: resource.NoopCheckerForTest},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: resource.NoopCheckerForTest},
	})

	m := newPreviewDemoModel(t, 120, 30)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// tg=0 (dim), asg=2 (available), others dim. SourceResourceID and
	// OperationID must be set — a compliant adapter (session-backed
	// Controller) drops any RelatedCheckResult missing the source ID or
	// carrying a stale operation id, leaving every row unresolved regardless
	// of the test's intent. activeOp is the live DetailOperation the
	// Navigate above just began (read via the Core accessor rather than
	// hardcoding a value — no refresh/profile/region switch happens between
	// Navigate and this loop).
	activeOp := m.Core().ActiveDetailOp()
	for _, tc := range []struct {
		target string
		count  int
		ids    []string
	}{
		{"tg", 0, nil},
		// These synthetic asg IDs only seed the RIGHT COLUMN's pre-Enter
		// count/actionable state (making the "asg" row Resolved, non-dim, so
		// Tab lands on it). Post-Enter, the real ec2->asg RelatedDef.Checker
		// (NoopChecker, set above) re-runs against the freshly fetched asg
		// list and always returns zero matches — the navigated screen's own
		// resolved count is asserted separately below via the source-scoped
		// title breadcrumb, not this seed count.
		{"asg", 2, []string{"asg-1", "asg-2"}},
		{"alarm", 0, nil},
		{"cfn", 0, nil},
	} {
		m, _ = previewApplyMsg(m, messages.RelatedCheckResult{
			ResourceType:     "ec2",
			SourceResourceID: ec2Res.ID,
			OperationID:      activeOp,
			Result:           resource.KnownRelated(tc.target, tc.ids, false),
		})
	}

	// Focus right column and press Enter. Expected: first actionable row (asg) is selected.
	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m, cmd := previewApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on focused right column should dispatch a navigation command for the first non-dim row")
	}
	// The Enter cmd resolves to messages.RelatedNavigate, which itself
	// dispatches the demo fetch as a further tea.Cmd (a tea.BatchMsg wrapping
	// the fetch + a ClearFlash). tui.Model.Update's own tea.BatchMsg case
	// (internal/tui/app.go) recursively resolves every sub-command, so one
	// more previewApplyMsg round-trip is enough to land on the fully loaded
	// asg list screen.
	m, cmd = previewApplyMsg(m, cmd())
	if cmd != nil {
		m, _ = previewApplyMsg(m, cmd())
	}

	view := previewView(m)
	asgRT := resource.FindResourceType("asg")
	if asgRT == nil {
		t.Fatal("asg resource type not registered")
	}
	// A bare Contains(view, "Auto Scaling Groups") (asgRT's DisplayName) would
	// pass even if Enter never navigated anywhere — the EC2 detail's own
	// RELATED panel already renders that exact string as a row label before
	// Enter is ever pressed. Instead require BOTH:
	//   1. the source-scoped title breadcrumb runtime.RelatedTitleSuffix
	//      produces (" -- <ec2 ID> (<ec2 Name>)") — the single source both
	//      the TUI and the headless Controller append to a related child
	//      list's frame title (core/runtime/related.go), naming the exact
	//      resource this navigation drilled from. This cannot appear by
	//      accident from the pre-Enter RELATED panel.
	//   2. the EC2 detail's own "RELATED" panel header is gone — proving the
	//      screen actually changed, not just that a similar string overlaps.
	wantSuffix := runtime.RelatedTitleSuffix(ec2Res)
	if !strings.Contains(view, wantSuffix) {
		t.Errorf("focused right column should skip dim rows, land on asg, and show the source-scoped title suffix %q; got:\n%s", wantSuffix, view)
	}
	if strings.Contains(view, "RELATED") {
		t.Errorf("navigating to asg should leave the EC2 detail's RELATED panel behind; got:\n%s", view)
	}
	// The suffix + RELATED-gone checks above prove SOME navigation happened,
	// not that it landed on asg specifically. asg has no catalog ListTitle
	// override (core/aws/catalog_compute.go), so buildListFrameTitle
	// (core/app/list_body.go) falls back to the bare type key ShortName
	// ("asg") for the frame name — this navigates against the real demo
	// fetcher with a NoopChecker (see the loop comment above), so the
	// destination list is empty and its column headers never render;
	// the frame-title type marker is the only asg-specific signal available
	// on an empty destination screen.
	wantMarker := asgRT.ShortName
	if asgRT.ListTitle != "" {
		wantMarker = asgRT.ListTitle
	}
	wantMarker += "("
	if !strings.Contains(view, wantMarker) {
		t.Errorf("navigating to asg should show its own frame-title marker %q, proving the destination is the asg list; got:\n%s", wantMarker, view)
	}
}

// TestPreview_RightColumnFocus_HLAndTabToggleFocus is live-path coverage for
// two issue140 stories previously (mis)mapped only to
// TestPreview_RightColumnTabFocus_SkipsDimRowsOnEnter, which exercises Tab +
// Enter alone: "Focus indicator changes with the active detail column" and "H
// and L switch focus instead of horizontally scrolling the detail view".
//
// The design docs also describe a third story, "Tab flips focus between the
// two visible columns in both directions" (docs/design/qa-user-stories-
// related-views-ec2.md). keys.Default() (internal/tui/keys/keys.go) binds only
// "tab", and every production call site (app_input.go:224, app_stack.go:
// 351,475) matches via key.Matches(msg, m.keys.Tab), which only matches a
// literal "tab" key string — there is no "shift+tab" binding anywhere in
// production, and there does not need to be: focus here is a binary toggle
// between exactly two columns, so Tab alone already flips it in both
// directions (a second Tab press returns focus to where it started — the
// same round trip a dedicated Shift-Tab would provide for a two-element
// cycle). The two-Tab-press assertion below is therefore real, live-path
// coverage of that story, not a stand-in for a key binding that would be
// redundant if it existed.
//
// The focus indicator (footer hints) is genuinely textual, not just a style
// difference: app_stack.go's key-help footer swaps "r Related" for
// "enter Auto Scaling Groups──tab Fields" once the right column is focused —
// so plain stripAnsi(view) Contains checks on those hint strings are a real,
// non-cosmetic proof that focus moved, without depending on ANSI byte
// comparison.
func TestPreview_RightColumnFocus_HLAndTabToggleFocus(t *testing.T) {
	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	t.Cleanup(func() { resource.SetRelatedForTest("ec2", oldDefs) })
	resource.SetRelatedForTest("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopCheckerForTest},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: resource.NoopCheckerForTest},
	})

	m := newPreviewDemoModel(t, 120, 30)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})
	activeOp := m.Core().ActiveDetailOp()
	for _, tc := range []struct {
		target string
		count  int
		ids    []string
	}{
		{"tg", 0, nil},
		{"asg", 2, []string{"asg-1", "asg-2"}},
	} {
		m, _ = previewApplyMsg(m, messages.RelatedCheckResult{
			ResourceType:     "ec2",
			SourceResourceID: ec2Res.ID,
			OperationID:      activeOp,
			Result:           resource.KnownRelated(tc.target, tc.ids, false),
		})
	}

	unfocused := previewView(m)
	if !strings.Contains(unfocused, "r Related") {
		t.Fatalf("precondition: unfocused left-column footer should hint \"r Related\"; got:\n%s", unfocused)
	}
	if strings.Contains(unfocused, "tab Fields") {
		t.Fatalf("precondition: unfocused footer should not yet show the right-column-focused \"tab Fields\" hint; got:\n%s", unfocused)
	}

	// 'l' (m.keys.ScrollRight) focuses the right column from the unfocused
	// left column — app_stack.go's ScrollRight case, distinct from Tab.
	mAfterL, _ := previewApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "l"})
	focusedRight := previewView(mAfterL)
	if !strings.Contains(focusedRight, "tab Fields") || !strings.Contains(focusedRight, "enter Auto Scaling Groups") {
		t.Errorf("'l' should focus the right column (footer should hint \"enter Auto Scaling Groups\" / \"tab Fields\"); got:\n%s", focusedRight)
	}
	// Functional proof the right column really is focused: Enter now
	// dispatches the actionable "asg" row's navigation.
	_, cmd := previewApplyMsg(mAfterL, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter after 'l' should dispatch a navigation command for the focused right column's actionable row")
	}
	msg := cmd()
	if nav, ok := msg.(messages.RelatedNavigate); !ok || nav.TargetType != "asg" {
		t.Errorf("Enter after 'l' = %#v, want messages.RelatedNavigate{TargetType: \"asg\"}", msg)
	}

	// 'h' (m.keys.ScrollLeft) focuses back to the left column — the render
	// must return exactly to the original unfocused footer.
	mAfterH, _ := previewApplyMsg(mAfterL, tea.KeyPressMsg{Code: -1, Text: "h"})
	if got := previewView(mAfterH); got != unfocused {
		t.Errorf("'h' should return the render to the original unfocused state; got:\n%s\nwant:\n%s", got, unfocused)
	}

	// Tab round trip: Tab focuses right (same visual change as 'l' above);
	// pressing Tab again flips back to left, exactly like 'h'. This is real
	// coverage of the Tab binding's own toggle behavior only — see the
	// doc comment above for why it is NOT a stand-in for Shift-Tab (no such
	// binding or handler exists in production).
	mAfterTab1, _ := previewApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	if got := previewView(mAfterTab1); got != focusedRight {
		t.Errorf("Tab should focus the right column identically to 'l'; got:\n%s\nwant:\n%s", got, focusedRight)
	}
	mAfterTab2, _ := previewApplyMsg(mAfterTab1, tea.KeyPressMsg{Code: tea.KeyTab})
	if got := previewView(mAfterTab2); got != unfocused {
		t.Errorf("a second Tab press should flip focus back to the left column; got:\n%s\nwant:\n%s", got, unfocused)
	}
}

func TestPreview_RightColumnScroll_KeepsDeepCursorRowVisible(t *testing.T) {
	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	t.Cleanup(func() { resource.SetRelatedForTest("ec2", oldDefs) })

	defs := make([]resource.RelatedDef, 0, 20)
	for i := 1; i <= 20; i++ {
		target := fmt.Sprintf("t%02d", i)
		defs = append(defs, resource.RelatedDef{
			TargetType:  target,
			DisplayName: fmt.Sprintf("Type %02d", i),
			Checker:     resource.NoopCheckerForTest,
		})
	}
	resource.SetRelatedForTest("ec2", defs)

	m := newPreviewDemoModel(t, 120, 8)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// Focus right column and move cursor deep into the list.
	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	for range 14 {
		m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "j"})
	}

	view := previewView(m)
	if !strings.Contains(view, "Type 15") {
		t.Errorf("right column should scroll to keep focused deep row visible (expected Type 15); got:\n%s", view)
	}
}

func TestPreview_DetailHelp_IncludesRelatedFilterAndCopyValue(t *testing.T) {
	m := newPreviewDemoModel(t, 120, 30)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: '?', Text: "?"})
	view := previewView(m)

	if !strings.Contains(view, "filter list") {
		t.Errorf("detail help should include related-column '/ Filter list' binding; got:\n%s", view)
	}
	if !strings.Contains(view, "copy value") {
		t.Errorf("detail help should describe c as 'copy value'; got:\n%s", view)
	}
	if !strings.Contains(view, "RELATED") {
		t.Errorf("detail help should include RELATED section; got:\n%s", view)
	}
}

// compile-time guard: keep this file in external-test package where stripAnsi helper is available.
var _ = views.DetailModel{}
