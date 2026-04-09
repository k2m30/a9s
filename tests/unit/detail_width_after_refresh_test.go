package unit

// detail_width_after_refresh_test.go — Tests revealing that Ctrl+R and
// ResetRightColumn use the constant m.rightColWidth (32) instead of calling
// m.currentRightColWidth() when sizing the right column.
//
// Bug location:
//   - internal/tui/views/detail.go:189 (Ctrl+R handler):
//       m.rightCol.SetSize(m.rightColWidth, m.height)  ← should use currentRightColWidth()
//   - internal/tui/views/detail.go:639 (ResetRightColumn):
//       m.rightCol.SetSize(m.rightColWidth, m.height)  ← should use currentRightColWidth()
//
// On an 80-col terminal currentRightColWidth() = max(24, 80/3)=26, capped by
// max(16, 80-40)=40 → 26. When SetSize uses m.rightColWidth=32 instead, the
// right column's internal viewport is 32 wide. After results are re-fed, its
// View() produces 32-wide lines. The detail View() computes leftW as
// 80 - currentRightColWidth() - 1 = 53. Total rendered width = 53 + 1 + 32 = 86,
// exceeding the 80-col terminal.
//
// Test sequence to reveal the bug:
//  1. Set up at width=80 with loaded results (right col visible).
//  2. Press Ctrl+R (or call ResetRightColumn) — right col is rebuilt with wrong width.
//  3. Re-feed RelatedCheckResultMsg — right col renders at the wrong (32) width.
//  4. Measure rendered output — expect ≤ 80 cols, but get > 80 with the bug.
//
// Both tests FAIL with current code and PASS after the fix.

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

const narrowTerminalWidth = 80

// setupEC2DetailWithResultsNarrow is like setupEC2DetailWithResults but uses an
// 80-col terminal so that currentRightColWidth() returns 26, not 32.
func setupEC2DetailWithResultsNarrow(t *testing.T) tui.Model {
	t.Helper()

	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: narrowTerminalWidth, Height: 36})

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	clients := demo.NewServiceClients()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), clients.EC2)
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing: err=%v len=%d", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2Res,
	})

	// Enter → navigate into detail; drain cmd chain until RelatedCheckStartedMsg.
	m, firstCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	m, _ = drainCmds(t, m, firstCmd, 5)

	// Feed results for every EC2 related type so right column is visible.
	for _, def := range resource.GetRelated("ec2") {
		m, _ = rootApplyMsg(m, messages.RelatedCheckResultMsg{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  def.TargetType,
				Count:       2,
				ResourceIDs: []string{"related-id-1", "related-id-2"},
			},
		})
	}

	return m
}

// feedEC2Results feeds RelatedCheckResultMsg for all EC2 related types with Count=2.
func feedEC2Results(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	for _, def := range resource.GetRelated("ec2") {
		m, _ = rootApplyMsg(m, messages.RelatedCheckResultMsg{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  def.TargetType,
				Count:       2,
				ResourceIDs: []string{"related-id-1", "related-id-2"},
			},
		})
	}
	return m
}

// maxLineWidth returns the maximum lipgloss.Width across all lines in the
// rendered view string. ANSI sequences are counted correctly by lipgloss.Width.
func maxLineWidth(view string) int {
	maxW := 0
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > maxW {
			maxW = w
		}
	}
	return maxW
}

// TestDetail_CtrlR_UsesCurrentRightColWidth_NarrowTerminal verifies that after
// pressing Ctrl+R on an 80-col terminal and re-feeding results, the rendered
// view does not exceed the terminal width.
//
// The Ctrl+R handler (detail.go:189) calls:
//   m.rightCol.SetSize(m.rightColWidth, m.height)   ← BUG: uses 32
//
// With the fix it calls:
//   m.rightCol.SetSize(m.currentRightColWidth(), m.height)  ← correct: 26 at 80 cols
//
// When the right column is set to 32 wide and results are re-fed, its View()
// produces 32-wide content. The detail View() pads the left pane to
// width - currentRightColWidth() - 1 = 53. Total = 53 + 1 + 32 = 86 > 80.
//
// This test FAILS with current code.
func TestDetail_CtrlR_UsesCurrentRightColWidth_NarrowTerminal(t *testing.T) {
	m := setupEC2DetailWithResultsNarrow(t)

	// Confirm right column is visible — related counts must appear.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(2)") {
		t.Fatalf("precondition failed: expected '(2)' in view before Ctrl+R to confirm "+
			"related counts are visible at width=%d.\nView:\n%s", narrowTerminalWidth, viewBefore)
	}

	// Press Ctrl+R — rebuilds right column with the buggy SetSize(m.rightColWidth=32).
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain the RelatedCheckStartedMsg cmd into the model so checkers are dispatched.
	if refreshCmd != nil {
		if msg := refreshCmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	// Re-feed results — now the right column is filled again.
	// With the bug the right col's internal viewport is 32 wide → overflow.
	// With the fix it is 26 wide → no overflow.
	m = feedEC2Results(t, m)

	viewAfter := rootViewContent(m) // keep ANSI so lipgloss.Width is accurate
	maxW := maxLineWidth(viewAfter)

	// BUG: m.rightColWidth (32) was used instead of currentRightColWidth() (26).
	// After re-feeding results the right col renders at 32, total line = 86 > 80.
	if maxW > narrowTerminalWidth {
		t.Fatalf("BUG: after Ctrl+R + re-fed results on a %d-col terminal the rendered "+
			"view is %d cols wide — the Ctrl+R handler (detail.go:189) must call "+
			"currentRightColWidth() instead of m.rightColWidth so the right column "+
			"fits the terminal after results are displayed.\n"+
			"Max line width: %d  Terminal width: %d",
			narrowTerminalWidth, maxW, maxW, narrowTerminalWidth)
	}
}

// TestDetail_ResetRightColumn_UsesCurrentRightColWidth_NarrowTerminal verifies
// that ResetRightColumn (detail.go:639) uses currentRightColWidth() when sizing
// the rebuilt right column.
//
// ResetRightColumn is called from app_handlers.go:handleRefresh when Ctrl+R is
// pressed at the root level. After reset, results arrive via RelatedCheckResultMsg.
// With the bug the right col is sized to 32; after results are fed the rendered
// view exceeds 80 cols.
//
// The test drives this via the root model handleRefresh path: Ctrl+R on the root
// model calls d.ResetRightColumn() then emits RelatedCheckStartedMsg.
//
// This test FAILS with current code.
func TestDetail_ResetRightColumn_UsesCurrentRightColWidth_NarrowTerminal(t *testing.T) {
	m := setupEC2DetailWithResultsNarrow(t)

	// Confirm right column is visible.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(2)") {
		t.Fatalf("precondition failed: expected '(2)' in view before reset; "+
			"view:\n%s", viewBefore)
	}

	// Ctrl+R at root level → handleRefresh → d.ResetRightColumn() → RelatedCheckStartedMsg.
	// app_handlers.go:512: d.ResetRightColumn() calls detail.go:639 SetSize(m.rightColWidth).
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain RelatedCheckStartedMsg → handleRelatedCheckStarted dispatches checkers.
	if refreshCmd != nil {
		if msg := refreshCmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	// Re-feed results to fill the reset right column.
	// With the bug: ResetRightColumn used m.rightColWidth=32, right col renders at 32.
	// With the fix: ResetRightColumn uses currentRightColWidth()=26, right col renders at 26.
	m = feedEC2Results(t, m)

	viewAfter := rootViewContent(m) // keep ANSI for accurate lipgloss.Width
	maxW := maxLineWidth(viewAfter)

	// BUG: ResetRightColumn (detail.go:639) uses m.rightColWidth (32) directly.
	// After re-feeding results the rendered line width is 53 + 1 + 32 = 86 > 80.
	if maxW > narrowTerminalWidth {
		t.Fatalf("BUG: after ResetRightColumn + re-fed results on a %d-col terminal "+
			"the rendered view is %d cols wide — ResetRightColumn (detail.go:639) must "+
			"call currentRightColWidth() instead of m.rightColWidth so the right column "+
			"fits the terminal after results are displayed.\n"+
			"Max line width: %d  Terminal width: %d",
			narrowTerminalWidth, maxW, maxW, narrowTerminalWidth)
	}
}
