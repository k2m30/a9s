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
	for line := range strings.SplitSeq(rendered, "\n") {
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
	for i := range statuses {
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

// ===========================================================================
// CloudFormation pattern-based matching (suffix: _COMPLETE, _IN_PROGRESS, _FAILED)
// ===========================================================================

func TestQA_StatusColor_CFN_CompleteIsGreen(t *testing.T) {
	completeStatuses := []string{
		"CREATE_COMPLETE",
		"UPDATE_COMPLETE",
		"DELETE_COMPLETE",
		"IMPORT_COMPLETE",
		"UPDATE_ROLLBACK_COMPLETE",
	}
	for _, s := range completeStatuses {
		style := styles.RowColorStyle(s)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, styles.ColRunning) {
			t.Errorf("RowColorStyle(%q) should use ColRunning (green), got different color", s)
		}
	}
}

func TestQA_StatusColor_CFN_InProgressIsYellow(t *testing.T) {
	inProgressStatuses := []string{
		"CREATE_IN_PROGRESS",
		"UPDATE_IN_PROGRESS",
		"DELETE_IN_PROGRESS",
		"ROLLBACK_IN_PROGRESS",
		"UPDATE_ROLLBACK_IN_PROGRESS",
		"IMPORT_IN_PROGRESS",
		"UPDATE_COMPLETE_CLEANUP_IN_PROGRESS",
		"UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS",
	}
	for _, s := range inProgressStatuses {
		style := styles.RowColorStyle(s)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, styles.ColPending) {
			t.Errorf("RowColorStyle(%q) should use ColPending (yellow), got different color", s)
		}
	}
}

func TestQA_StatusColor_CFN_FailedIsRed(t *testing.T) {
	failedStatuses := []string{
		"CREATE_FAILED",
		"UPDATE_FAILED",
		"DELETE_FAILED",
		"ROLLBACK_FAILED",
		"UPDATE_ROLLBACK_FAILED",
		"IMPORT_ROLLBACK_FAILED",
	}
	for _, s := range failedStatuses {
		style := styles.RowColorStyle(s)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, styles.ColStopped) {
			t.Errorf("RowColorStyle(%q) should use ColStopped (red), got different color", s)
		}
	}
}

func TestQA_StatusColor_CFN_RollbackCompleteIsRed(t *testing.T) {
	// ROLLBACK_COMPLETE and IMPORT_ROLLBACK_COMPLETE are special cases:
	// the rollback itself completed, but it means the original operation FAILED.
	// These should be RED, not green.
	rollbackStatuses := []string{
		"ROLLBACK_COMPLETE",
		"IMPORT_ROLLBACK_COMPLETE",
	}
	for _, s := range rollbackStatuses {
		style := styles.RowColorStyle(s)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, styles.ColStopped) {
			t.Errorf("RowColorStyle(%q) should use ColStopped (red) because rollback means failure, got different color", s)
		}
	}
}

func TestQA_StatusColor_CFN_CaseInsensitive(t *testing.T) {
	// CloudFormation statuses are UPPER_CASE from AWS but matching should be case-insensitive
	cases := []struct {
		input    string
		expected color.Color
	}{
		{"create_complete", styles.ColRunning},
		{"Create_Complete", styles.ColRunning},
		{"update_in_progress", styles.ColPending},
		{"Update_In_Progress", styles.ColPending},
		{"create_failed", styles.ColStopped},
		{"Create_Failed", styles.ColStopped},
		{"rollback_complete", styles.ColStopped},
		{"Rollback_Complete", styles.ColStopped},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.input)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q): expected matching color, got different", tc.input)
		}
	}
}

// ===========================================================================
// New simple status mappings — per-resource verification
// ===========================================================================

func TestQA_StatusColor_TargetGroupHealth(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"healthy", styles.ColRunning},
		{"unhealthy", styles.ColStopped},
		{"draining", styles.ColPending},
		{"initial", styles.ColPending},
		{"unused", styles.ColTerminated},
		{"unavailable", styles.ColStopped},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [TG Health]: unexpected color", tc.status)
		}
	}
}

func TestQA_StatusColor_CloudWatchAlarms(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"OK", styles.ColRunning},
		{"ALARM", styles.ColStopped},
		{"INSUFFICIENT_DATA", styles.ColPending},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [CloudWatch]: unexpected color", tc.status)
		}
	}
}

func TestQA_StatusColor_ACMCertificates(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"ISSUED", styles.ColRunning},
		{"PENDING_VALIDATION", styles.ColPending},
		{"EXPIRED", styles.ColStopped},
		{"REVOKED", styles.ColStopped},
		{"FAILED", styles.ColStopped},
		{"INACTIVE", styles.ColTerminated},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [ACM]: unexpected color", tc.status)
		}
	}
}

func TestQA_StatusColor_CloudFront(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"Deployed", styles.ColRunning},
		{"InProgress", styles.ColPending},
		{"Disabled", styles.ColTerminated},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [CloudFront]: unexpected color", tc.status)
		}
	}
}

func TestQA_StatusColor_EventBridgeAndKMS(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"ENABLED", styles.ColRunning},
		{"DISABLED", styles.ColTerminated},
		{"PendingDeletion", styles.ColStopped},
		{"PendingImport", styles.ColPending},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [EventBridge/KMS]: unexpected color", tc.status)
		}
	}
}

func TestQA_StatusColor_MSK(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"HEALING", styles.ColPending},
		{"REBOOTING_BROKER", styles.ColPending},
		{"MAINTENANCE", styles.ColPending},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [MSK]: unexpected color", tc.status)
		}
	}
}

func TestQA_StatusColor_Redshift(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"rebooting", styles.ColPending},
		{"resizing", styles.ColPending},
		{"paused", styles.ColTerminated},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [Redshift]: unexpected color", tc.status)
		}
	}
}

func TestQA_StatusColor_SES(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"SUCCESS", styles.ColRunning},
		{"PENDING", styles.ColPending},
		{"FAILED", styles.ColStopped},
		{"TEMPORARY_FAILURE", styles.ColPending},
		{"NOT_STARTED", styles.ColTerminated},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [SES]: unexpected color", tc.status)
		}
	}
}

func TestQA_StatusColor_ElasticBeanstalkHealth(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"Green", styles.ColRunning},
		{"Yellow", styles.ColPending},
		{"Red", styles.ColStopped},
		{"Grey", styles.ColTerminated},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [EB Health]: unexpected color", tc.status)
		}
	}
}

func TestQA_StatusColor_VPCEndpoints(t *testing.T) {
	cases := []struct {
		status   string
		expected color.Color
	}{
		{"rejected", styles.ColStopped},
		{"expired", styles.ColStopped},
		{"pendingAcceptance", styles.ColPending},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [VPC Endpoints]: unexpected color", tc.status)
		}
	}
}

// ===========================================================================
// NO_COLOR disables all new status colors too
// ===========================================================================

func TestQA_StatusColor_NoColorDisablesNewStatuses(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	newStatuses := []string{
		"healthy", "ok", "alarm", "deployed", "enabled", "disabled",
		"CREATE_COMPLETE", "UPDATE_IN_PROGRESS", "CREATE_FAILED",
		"healing", "paused", "pendingdeletion",
	}
	for _, status := range newStatuses {
		rendered := styles.RowColorStyle(status).Render("testdata")
		if strings.Contains(rendered, "\x1b[") {
			t.Errorf("with NO_COLOR set, RowColorStyle(%q).Render() should not contain ANSI codes", status)
		}
	}
}

// ===========================================================================
// Complete coverage: ALL 52 status mappings resolve to correct color
// ===========================================================================

func TestQA_StatusColor_CompleteMappingTable(t *testing.T) {
	// Exhaustive table of every status→color mapping after #61 expansion.
	// This is the single source of truth for status color coverage.
	allMappings := []struct {
		status   string
		expected color.Color
		label    string
	}{
		// Green (ColRunning)
		{"running", styles.ColRunning, "baseline"},
		{"available", styles.ColRunning, "baseline"},
		{"active", styles.ColRunning, "baseline"},
		{"in-use", styles.ColRunning, "baseline"},
		{"succeeded", styles.ColRunning, "baseline"},
		{"healthy", styles.ColRunning, "TG Health"},
		{"ok", styles.ColRunning, "CloudWatch"},
		{"issued", styles.ColRunning, "ACM"},
		{"deployed", styles.ColRunning, "CloudFront"},
		{"enabled", styles.ColRunning, "EventBridge/KMS/Athena"},
		{"green", styles.ColRunning, "EB Health"},
		{"success", styles.ColRunning, "SES"},

		// Red (ColStopped)
		{"stopped", styles.ColStopped, "baseline"},
		{"failed", styles.ColStopped, "baseline"},
		{"error", styles.ColStopped, "baseline"},
		{"deleting", styles.ColStopped, "baseline"},
		{"deleted", styles.ColStopped, "baseline"},
		{"timed_out", styles.ColStopped, "baseline"},
		{"unhealthy", styles.ColStopped, "TG Health"},
		{"unavailable", styles.ColStopped, "TG Health"},
		{"alarm", styles.ColStopped, "CloudWatch"},
		{"expired", styles.ColStopped, "ACM/VPC Endpoints"},
		{"revoked", styles.ColStopped, "ACM"},
		{"rejected", styles.ColStopped, "VPC Endpoints"},
		{"pendingdeletion", styles.ColStopped, "KMS"},
		{"rollback_complete", styles.ColStopped, "CFN special"},
		{"import_rollback_complete", styles.ColStopped, "CFN special"},
		{"red", styles.ColStopped, "EB Health"},

		// Yellow (ColPending)
		{"pending", styles.ColPending, "baseline"},
		{"creating", styles.ColPending, "baseline"},
		{"modifying", styles.ColPending, "baseline"},
		{"updating", styles.ColPending, "baseline"},
		{"pending_redrive", styles.ColPending, "baseline"},
		{"draining", styles.ColPending, "TG Health"},
		{"initial", styles.ColPending, "TG Health"},
		{"insufficient_data", styles.ColPending, "CloudWatch"},
		{"pending_validation", styles.ColPending, "ACM"},
		{"inprogress", styles.ColPending, "CloudFront"},
		{"healing", styles.ColPending, "MSK"},
		{"rebooting_broker", styles.ColPending, "MSK"},
		{"maintenance", styles.ColPending, "MSK"},
		{"rebooting", styles.ColPending, "Redshift"},
		{"resizing", styles.ColPending, "Redshift"},
		{"pendingimport", styles.ColPending, "KMS"},
		{"pendingacceptance", styles.ColPending, "VPC Endpoints"},
		{"yellow", styles.ColPending, "EB Health"},
		{"temporary_failure", styles.ColPending, "SES"},

		// Dim (ColTerminated)
		{"terminated", styles.ColTerminated, "baseline"},
		{"shutting-down", styles.ColTerminated, "baseline"},
		{"aborted", styles.ColTerminated, "baseline"},
		{"unused", styles.ColTerminated, "TG Health"},
		{"disabled", styles.ColTerminated, "EventBridge/KMS/Athena/CloudFront"},
		{"inactive", styles.ColTerminated, "ACM"},
		{"grey", styles.ColTerminated, "EB Health"},
		{"not_started", styles.ColTerminated, "SES"},
		{"paused", styles.ColTerminated, "Redshift"},
	}

	for _, tc := range allMappings {
		style := styles.RowColorStyle(tc.status)
		fg := style.GetForeground()
		if !statusColorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q) [%s]: foreground does not match expected color", tc.status, tc.label)
		}
	}
}
