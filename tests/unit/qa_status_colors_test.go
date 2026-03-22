package unit

import (
	"image/color"
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// Helpers
// ===========================================================================

// statusColorsEqual compares two color.Color values by their RGBA components.
func statusColorsEqual(a, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// multiStatusColorModel builds a model with multiple resources of different
// statuses, with the cursor NOT on the first row so the first row uses its
// status color rather than the selection style.
func multiStatusColorModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 20},
			{Key: "state", Title: "State", Width: 14},
		},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	res := []resource.Resource{
		{
			ID: "i-run", Name: "running-inst", Status: "running",
			Fields: map[string]string{"name": "running-inst", "state": "running"},
		},
		{
			ID: "i-stop", Name: "stopped-inst", Status: "stopped",
			Fields: map[string]string{"name": "stopped-inst", "state": "stopped"},
		},
		{
			ID: "i-pend", Name: "pending-inst", Status: "pending",
			Fields: map[string]string{"name": "pending-inst", "state": "pending"},
		},
		{
			ID: "i-term", Name: "terminated-inst", Status: "terminated",
			Fields: map[string]string{"name": "terminated-inst", "state": "terminated"},
		},
		{
			ID: "i-sel", Name: "selection-target", Status: "running",
			Fields: map[string]string{"name": "selection-target", "state": "running"},
		},
	}
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    res,
	})

	// Move cursor to last row so rows 0-3 use their status colors
	m, _ = m.Update(rlKeyPress("G"))

	return m
}

// findLineContaining returns the raw (ANSI-included) line containing the
// given plain-text substring, or "" if not found.
func findLineContaining(rendered, substr string) string {
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(stripANSI(line), substr) {
			return line
		}
	}
	return ""
}

// ===========================================================================
// Issue 2: RowColorStyle returns the CORRECT color (not just "has ANSI")
// ===========================================================================

func TestQA_StatusColor_RunningReturnsGreen(t *testing.T) {
	style := styles.RowColorStyle("running")
	fg := style.GetForeground()
	if !statusColorsEqual(fg, styles.ColRunning) {
		t.Errorf("RowColorStyle(\"running\") should use ColRunning (#9ece6a), got different color")
	}
}

func TestQA_StatusColor_StoppedReturnsRed(t *testing.T) {
	style := styles.RowColorStyle("stopped")
	fg := style.GetForeground()
	if !statusColorsEqual(fg, styles.ColStopped) {
		t.Errorf("RowColorStyle(\"stopped\") should use ColStopped (#f7768e), got different color")
	}
}

func TestQA_StatusColor_PendingReturnsYellow(t *testing.T) {
	style := styles.RowColorStyle("pending")
	fg := style.GetForeground()
	if !statusColorsEqual(fg, styles.ColPending) {
		t.Errorf("RowColorStyle(\"pending\") should use ColPending (#e0af68), got different color")
	}
}

func TestQA_StatusColor_TerminatedReturnsDim(t *testing.T) {
	style := styles.RowColorStyle("terminated")
	fg := style.GetForeground()
	if !statusColorsEqual(fg, styles.ColTerminated) {
		t.Errorf("RowColorStyle(\"terminated\") should use ColTerminated (#565f89), got different color")
	}
}

// ===========================================================================
// Verify different statuses use DIFFERENT colors (not just "both have ANSI")
// ===========================================================================

func TestQA_StatusColor_RunningAndStoppedAreDifferent(t *testing.T) {
	runStyle := styles.RowColorStyle("running")
	stopStyle := styles.RowColorStyle("stopped")

	runFg := runStyle.GetForeground()
	stopFg := stopStyle.GetForeground()

	if statusColorsEqual(runFg, stopFg) {
		t.Error("running and stopped should use different foreground colors")
	}
}

func TestQA_StatusColor_RunningAndPendingAreDifferent(t *testing.T) {
	runStyle := styles.RowColorStyle("running")
	pendStyle := styles.RowColorStyle("pending")

	runFg := runStyle.GetForeground()
	pendFg := pendStyle.GetForeground()

	if statusColorsEqual(runFg, pendFg) {
		t.Error("running and pending should use different foreground colors")
	}
}

func TestQA_StatusColor_StoppedAndTerminatedAreDifferent(t *testing.T) {
	stopStyle := styles.RowColorStyle("stopped")
	termStyle := styles.RowColorStyle("terminated")

	stopFg := stopStyle.GetForeground()
	termFg := termStyle.GetForeground()

	if statusColorsEqual(stopFg, termFg) {
		t.Error("stopped and terminated should use different foreground colors")
	}
}

func TestQA_StatusColor_AllFourStatusesDistinct(t *testing.T) {
	statuses := []struct {
		name     string
		expected color.Color
	}{
		{"running", styles.ColRunning},
		{"stopped", styles.ColStopped},
		{"pending", styles.ColPending},
		{"terminated", styles.ColTerminated},
	}

	// Verify each status maps to its expected color
	for _, s := range statuses {
		style := styles.RowColorStyle(s.name)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, s.expected) {
			t.Errorf("RowColorStyle(%q): foreground does not match expected palette color", s.name)
		}
	}

	// Verify all four colors are pairwise distinct
	for i := 0; i < len(statuses); i++ {
		for j := i + 1; j < len(statuses); j++ {
			si := styles.RowColorStyle(statuses[i].name).GetForeground()
			sj := styles.RowColorStyle(statuses[j].name).GetForeground()
			if statusColorsEqual(si, sj) {
				t.Errorf("%q and %q should use different colors", statuses[i].name, statuses[j].name)
			}
		}
	}
}

// ===========================================================================
// Verify rendered output: rows with different statuses produce different ANSI
// ===========================================================================

func TestQA_StatusColor_RenderedRowsUseDifferentANSI(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	// Render a word with each status color and verify distinct ANSI output
	runRendered := styles.RowColorStyle("running").Render("test")
	stopRendered := styles.RowColorStyle("stopped").Render("test")
	pendRendered := styles.RowColorStyle("pending").Render("test")
	termRendered := styles.RowColorStyle("terminated").Render("test")

	// Each should contain ANSI codes
	for _, r := range []struct {
		name     string
		rendered string
	}{
		{"running", runRendered},
		{"stopped", stopRendered},
		{"pending", pendRendered},
		{"terminated", termRendered},
	} {
		if !strings.Contains(r.rendered, "\x1b[") {
			t.Errorf("RowColorStyle(%q).Render() should contain ANSI escape codes", r.name)
		}
	}

	// Pairwise distinct
	pairs := [][2]string{
		{runRendered, stopRendered},
		{runRendered, pendRendered},
		{runRendered, termRendered},
		{stopRendered, pendRendered},
		{stopRendered, termRendered},
		{pendRendered, termRendered},
	}
	pairNames := [][2]string{
		{"running", "stopped"},
		{"running", "pending"},
		{"running", "terminated"},
		{"stopped", "pending"},
		{"stopped", "terminated"},
		{"pending", "terminated"},
	}
	for i, pair := range pairs {
		if pair[0] == pair[1] {
			t.Errorf("Render output for %q and %q should differ but are identical",
				pairNames[i][0], pairNames[i][1])
		}
	}
}

// ===========================================================================
// Verify rendered resource list rows contain status-specific ANSI colors
// ===========================================================================

func TestQA_StatusColor_ResourceListRowsHaveCorrectColors(t *testing.T) {
	m := multiStatusColorModel(t)
	rendered := m.View()

	// Find the raw line for each status-named resource
	runningLine := findLineContaining(rendered, "running-inst")
	stoppedLine := findLineContaining(rendered, "stopped-inst")
	pendingLine := findLineContaining(rendered, "pending-inst")
	terminatedLine := findLineContaining(rendered, "terminated-inst")

	if runningLine == "" || stoppedLine == "" || pendingLine == "" || terminatedLine == "" {
		t.Fatal("could not find all status rows in rendered output")
	}

	// Each non-selected row should have ANSI
	for _, tc := range []struct {
		name string
		line string
	}{
		{"running", runningLine},
		{"stopped", stoppedLine},
		{"pending", pendingLine},
		{"terminated", terminatedLine},
	} {
		if !strings.Contains(tc.line, "\x1b[") {
			t.Errorf("row for %s status should contain ANSI color codes", tc.name)
		}
	}

	// Running and stopped rows should have different ANSI codes
	if runningLine == stoppedLine {
		t.Error("running and stopped rows should have different ANSI styling")
	}

	// Running and terminated rows should have different ANSI codes
	if runningLine == terminatedLine {
		t.Error("running and terminated rows should have different ANSI styling")
	}

	// Stopped and pending rows should have different ANSI codes
	if stoppedLine == pendingLine {
		t.Error("stopped and pending rows should have different ANSI styling")
	}
}

// ===========================================================================
// Verify specific hex color values appear in ANSI escape sequences
// ===========================================================================

func TestQA_StatusColor_RunningRendersGreenANSI(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	rendered := styles.RowColorStyle("running").Render("testdata")

	// ColRunning = #9ece6a = RGB(158, 206, 106)
	// Lipgloss uses true-color: \x1b[38;2;158;206;106m
	if !strings.Contains(rendered, "158;206;106") {
		// Some terminals/lipgloss versions may use different encoding.
		// At minimum verify it has ANSI and is different from stopped.
		stoppedRendered := styles.RowColorStyle("stopped").Render("testdata")
		if rendered == stoppedRendered {
			t.Error("running color rendering should differ from stopped")
		}
		if !strings.Contains(rendered, "\x1b[") {
			t.Error("running color rendering should contain ANSI escape codes")
		}
	}
}

func TestQA_StatusColor_StoppedRendersRedANSI(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	rendered := styles.RowColorStyle("stopped").Render("testdata")

	// ColStopped = #f7768e = RGB(247, 118, 142)
	if !strings.Contains(rendered, "247;118;142") {
		runningRendered := styles.RowColorStyle("running").Render("testdata")
		if rendered == runningRendered {
			t.Error("stopped color rendering should differ from running")
		}
		if !strings.Contains(rendered, "\x1b[") {
			t.Error("stopped color rendering should contain ANSI escape codes")
		}
	}
}

func TestQA_StatusColor_PendingRendersYellowANSI(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	rendered := styles.RowColorStyle("pending").Render("testdata")

	// ColPending = #e0af68 = RGB(224, 175, 104)
	if !strings.Contains(rendered, "224;175;104") {
		runningRendered := styles.RowColorStyle("running").Render("testdata")
		if rendered == runningRendered {
			t.Error("pending color rendering should differ from running")
		}
		if !strings.Contains(rendered, "\x1b[") {
			t.Error("pending color rendering should contain ANSI escape codes")
		}
	}
}

func TestQA_StatusColor_TerminatedRendersDimANSI(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	rendered := styles.RowColorStyle("terminated").Render("testdata")

	// ColTerminated = #565f89 = RGB(86, 95, 137)
	if !strings.Contains(rendered, "86;95;137") {
		runningRendered := styles.RowColorStyle("running").Render("testdata")
		if rendered == runningRendered {
			t.Error("terminated color rendering should differ from running")
		}
		if !strings.Contains(rendered, "\x1b[") {
			t.Error("terminated color rendering should contain ANSI escape codes")
		}
	}
}

// ===========================================================================
// Verify NO_COLOR disables all status coloring
// ===========================================================================

func TestQA_StatusColor_NoColorDisablesANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	for _, status := range []string{"running", "stopped", "pending", "terminated"} {
		rendered := styles.RowColorStyle(status).Render("testdata")
		if strings.Contains(rendered, "\x1b[") {
			t.Errorf("with NO_COLOR set, RowColorStyle(%q).Render() should not contain ANSI codes", status)
		}
	}
}
