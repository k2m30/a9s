package unit

// detail_width_after_refresh_test.go — after Ctrl+R the right column is
// sized with the current right-column width, so the rendered view never
// exceeds the terminal. At 80 columns the right column is max(24, 80/3)=26
// wide and the left pane 80-26-1=53; a right column sized to the 32-column
// default renders 53+1+32=86 columns.

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

const narrowTerminalWidth = 80

// setupEC2DetailWithResultsNarrow is like setupEC2DetailWithResults but uses an
// 80-col terminal so that currentRightColWidth() returns 26, not 32.
func setupEC2DetailWithResultsNarrow(t *testing.T) (tui.Model, resource.Resource) {
	t.Helper()

	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: narrowTerminalWidth, Height: 36})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	clients := demo.NewServiceClients()
	ec2Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), clients.EC2, token)
	})
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing: err=%v len=%d", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    ec2Res,
	})

	// Enter → navigate into detail; drain cmd chain until RelatedCheckStartedMsg.
	m, firstCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	m, _ = drainCmds(t, m, firstCmd, 5)

	// Feed results for every EC2 related type so right column is visible.
	// SourceResourceID must match the open detail's resource: foldRelatedCheckResultLocked
	// only merges a result into a stacked ScreenDetail when
	// ds.Resource.ID == SourceResourceID (core/app/handle.go).
	firstInstance := ec2Res[0]
	for _, def := range resource.GetRelated("ec2") {
		m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
			ResourceType:     "ec2",
			SourceResourceID: firstInstance.ID,
			Result:           resource.KnownRelated(def.TargetType, stubRelatedIDs, false),
		})
	}

	return m, firstInstance
}

// feedEC2Results feeds RelatedCheckResultMsg for all EC2 related types with
// Count=stubRelatedCount, stamped for srcID (see setupEC2DetailWithResultsNarrow).
func feedEC2Results(t *testing.T, m tui.Model, srcID string) tui.Model {
	t.Helper()
	for _, def := range resource.GetRelated("ec2") {
		m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
			ResourceType:     "ec2",
			SourceResourceID: srcID,
			Result:           resource.KnownRelated(def.TargetType, stubRelatedIDs, false),
		})
	}
	return m
}

// maxLineWidth returns the maximum lipgloss.Width across all lines in the
// rendered view string. ANSI sequences are counted correctly by lipgloss.Width.
func maxLineWidth(view string) int {
	maxW := 0
	for line := range strings.SplitSeq(view, "\n") {
		if w := lipgloss.Width(line); w > maxW {
			maxW = w
		}
	}
	return maxW
}

// TestDetail_CtrlR_UsesCurrentRightColWidth_NarrowTerminal: after Ctrl+R on
// an 80-col terminal and re-fed results, the rendered view does not exceed
// the terminal width.
func TestDetail_CtrlR_UsesCurrentRightColWidth_NarrowTerminal(t *testing.T) {
	m, firstInstance := setupEC2DetailWithResultsNarrow(t)

	// Confirm right column is visible — related counts must appear.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(7)") {
		t.Fatalf("precondition failed: expected '(7)' in view before Ctrl+R to confirm "+
			"related counts are visible at width=%d.\nView:\n%s", narrowTerminalWidth, viewBefore)
	}

	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain the RelatedCheckStartedMsg cmd into the model so checkers are dispatched.
	if refreshCmd != nil {
		if msg := refreshCmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	m = feedEC2Results(t, m, firstInstance.ID)

	viewAfter := rootViewContent(m) // keep ANSI so lipgloss.Width is accurate
	maxW := maxLineWidth(viewAfter)

	if maxW > narrowTerminalWidth {
		t.Fatalf("BUG: after Ctrl+R + re-fed results on a %d-col terminal the rendered "+
			"view is %d cols wide — the Ctrl+R handler (detail.go:189) must call "+
			"currentRightColWidth() instead of m.rightColWidth so the right column "+
			"fits the terminal after results are displayed.\n"+
			"Max line width: %d  Terminal width: %d",
			narrowTerminalWidth, maxW, maxW, narrowTerminalWidth)
	}
}

// TestDetail_ResetRightColumn_UsesCurrentRightColWidth_NarrowTerminal: Ctrl+R
// at the root level resets the right column; once results arrive via
// RelatedCheckResultMsg the rebuilt column must fit the 80-col terminal.
func TestDetail_ResetRightColumn_UsesCurrentRightColWidth_NarrowTerminal(t *testing.T) {
	m, firstInstance := setupEC2DetailWithResultsNarrow(t)

	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(7)") {
		t.Fatalf("precondition failed: expected '(7)' in view before reset; "+
			"view:\n%s", viewBefore)
	}

	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain RelatedCheckStartedMsg → handleRelatedCheckStarted dispatches checkers.
	if refreshCmd != nil {
		if msg := refreshCmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	m = feedEC2Results(t, m, firstInstance.ID)

	viewAfter := rootViewContent(m) // keep ANSI for accurate lipgloss.Width
	maxW := maxLineWidth(viewAfter)

	if maxW > narrowTerminalWidth {
		t.Fatalf("BUG: after ResetRightColumn + re-fed results on a %d-col terminal "+
			"the rendered view is %d cols wide — ResetRightColumn (detail.go:639) must "+
			"call currentRightColWidth() instead of m.rightColWidth so the right column "+
			"fits the terminal after results are displayed.\n"+
			"Max line width: %d  Terminal width: %d",
			narrowTerminalWidth, maxW, maxW, narrowTerminalWidth)
	}
}
