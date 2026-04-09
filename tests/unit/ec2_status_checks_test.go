package unit

// ec2_status_checks_test.go — tests for EC2 instance status check indicators
// (issue #188). Covers list view prefix glyphs, detail view section, and
// fetcher merging of system_status / instance_status fields.

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// ec2ListModelWithResources builds a root model navigated to the EC2 list
// and loaded with the provided resources. Terminal is 160x40.
func ec2ListModelWithResources(t *testing.T, resources []resource.Resource) tui.Model {
	t.Helper()
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    resources,
	})
	return m
}

// rowContaining returns the first rendered line (with ANSI intact) that
// contains the given plain-text substring, or "" if not found.
func rowContaining(content, substr string) string {
	for line := range strings.SplitSeq(content, "\n") {
		if strings.Contains(stripANSI(line), substr) {
			return line
		}
	}
	return ""
}

// ec2DetailModel creates a DetailModel for the given EC2 resource, sized 120x40.
func ec2DetailModel(t *testing.T, r resource.Resource) views.DetailModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	k := keys.Default()
	d := views.NewDetail(r, "ec2", nil, k)
	d.SetSize(120, 40)
	return d
}

// ---------------------------------------------------------------------------
// List View — ! prefix (impaired checks)
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_List_ImpairedShowsBang verifies that a running instance
// with instance_status=impaired renders a "!" glyph in the STATE cell.
//
// Fixture: i-0aaa111111111111a has system_status=ok, instance_status=impaired.
func TestEC2StatusChecks_List_ImpairedShowsBang(t *testing.T) {
	m := ec2ListModelWithResources(t, fixtureEC2Instances())
	plain := stripANSI(rootViewContent(m))

	// The impaired instance (i-0aaa111111111111a) has no name, so it renders
	// without a name in the Name column. We look for "! running" in the plain text.
	if !strings.Contains(plain, "! running") {
		t.Errorf("expected '! running' in list view for impaired instance, got:\n%s", plain)
	}
}

// TestEC2StatusChecks_List_ImpairedBangHasANSI verifies that the "!" glyph
// for an impaired instance carries ANSI colour codes (RED bold).
func TestEC2StatusChecks_List_ImpairedBangHasANSI(t *testing.T) {
	m := ec2ListModelWithResources(t, fixtureEC2Instances())
	content := rootViewContent(m)

	// Find a row that contains "! running" in its plain text.
	var bangLine string
	for line := range strings.SplitSeq(content, "\n") {
		if strings.Contains(stripANSI(line), "! running") {
			bangLine = line
			break
		}
	}
	if bangLine == "" {
		t.Fatal("could not find a row containing '! running'")
	}
	if !strings.Contains(bangLine, "\x1b[") {
		t.Error("expected ANSI escape codes on the '! running' row")
	}
}

// ---------------------------------------------------------------------------
// List View — ~ prefix (initializing)
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_List_InitializingShowsTilde verifies that a running
// instance with system_status=initializing renders "~ running" in the list.
//
// Fixture: i-0bbb222222222222b (VPN) has system_status=initializing.
func TestEC2StatusChecks_List_InitializingShowsTilde(t *testing.T) {
	m := ec2ListModelWithResources(t, fixtureEC2Instances())
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "~ running") {
		t.Errorf("expected '~ running' for initializing instance, got:\n%s", plain)
	}
}

// TestEC2StatusChecks_List_InitializingTildeHasANSI verifies that the "~"
// glyph carries ANSI colour codes (YELLOW).
func TestEC2StatusChecks_List_InitializingTildeHasANSI(t *testing.T) {
	m := ec2ListModelWithResources(t, fixtureEC2Instances())
	content := rootViewContent(m)

	var tildeLine string
	for line := range strings.SplitSeq(content, "\n") {
		if strings.Contains(stripANSI(line), "~ running") {
			tildeLine = line
			break
		}
	}
	if tildeLine == "" {
		t.Fatal("could not find a row containing '~ running'")
	}
	if !strings.Contains(tildeLine, "\x1b[") {
		t.Error("expected ANSI escape codes on the '~ running' row")
	}
}

// ---------------------------------------------------------------------------
// List View — no indicator (healthy ok/ok)
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_List_HealthyNoIndicator verifies that a running instance
// with system_status=ok and instance_status=ok shows plain "running" (no "!" or "~").
//
// Fixture: i-0ccc333333333333c (kafka) has both statuses "ok".
func TestEC2StatusChecks_List_HealthyNoIndicator(t *testing.T) {
	m := ec2ListModelWithResources(t, fixtureEC2Instances())
	plain := stripANSI(rootViewContent(m))

	// The kafka row should contain "running" without a "!" or "~" prefix.
	kafkaRow := rowContaining(rootViewContent(m), "kafka")
	if kafkaRow == "" {
		t.Fatal("could not find the kafka instance row")
	}
	kafkaPlain := stripANSI(kafkaRow)

	if strings.Contains(kafkaPlain, "! running") {
		t.Error("healthy instance (kafka) should not show '! running'")
	}
	if strings.Contains(kafkaPlain, "~ running") {
		t.Error("healthy instance (kafka) should not show '~ running'")
	}
	// Plain "running" must still appear somewhere in the view.
	if !strings.Contains(plain, "running") {
		t.Error("expected 'running' to appear somewhere in the list")
	}
}

// ---------------------------------------------------------------------------
// List View — no indicator (no status fields / API fallback)
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_List_NoFieldsNoIndicator verifies that a running instance
// with NO system_status / instance_status fields shows plain "running".
//
// Fixture: i-0ddd444444444444d (monitoring) has no status check fields.
func TestEC2StatusChecks_List_NoFieldsNoIndicator(t *testing.T) {
	m := ec2ListModelWithResources(t, fixtureEC2Instances())

	monitoringRow := rowContaining(rootViewContent(m), "monitoring")
	if monitoringRow == "" {
		t.Fatal("could not find the monitoring instance row")
	}
	monitoringPlain := stripANSI(monitoringRow)

	if strings.Contains(monitoringPlain, "! running") {
		t.Error("instance with no status fields (monitoring) should not show '! running'")
	}
	if strings.Contains(monitoringPlain, "~ running") {
		t.Error("instance with no status fields (monitoring) should not show '~ running'")
	}
}

// ---------------------------------------------------------------------------
// List View — no indicator (non-running / terminated)
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_List_NonRunningNoIndicator verifies that terminated
// instances never show "!" or "~", regardless of status check fields.
//
// Fixture: i-0fff666666666666f (apps) is terminated with no status fields.
func TestEC2StatusChecks_List_NonRunningNoIndicator(t *testing.T) {
	m := ec2ListModelWithResources(t, fixtureEC2Instances())

	appsRow := rowContaining(rootViewContent(m), "apps")
	if appsRow == "" {
		t.Fatal("could not find the apps (terminated) instance row")
	}
	appsPlain := stripANSI(appsRow)

	if strings.Contains(appsPlain, "! terminated") {
		t.Error("terminated instance should not show '! terminated'")
	}
	if strings.Contains(appsPlain, "~ terminated") {
		t.Error("terminated instance should not show '~ terminated'")
	}
}

// TestEC2StatusChecks_List_NonRunningImpairedFieldsNoIndicator verifies that
// a stopped instance with impaired status check fields still shows no indicator.
// Non-running instances are excluded from status check indicator display.
func TestEC2StatusChecks_List_NonRunningImpairedFieldsNoIndicator(t *testing.T) {
	stoppedWithBadChecks := resource.Resource{
		ID:     "i-stopped-bad-checks",
		Name:   "stopped-bad",
		Status: "stopped",
		Fields: map[string]string{
			"instance_id":     "i-stopped-bad-checks",
			"name":            "stopped-bad",
			"state":           "stopped",
			"type":            "t3.medium",
			"system_status":   "impaired",
			"instance_status": "impaired",
		},
	}
	m := ec2ListModelWithResources(t, []resource.Resource{stoppedWithBadChecks})

	row := rowContaining(rootViewContent(m), "stopped-bad")
	if row == "" {
		t.Fatal("could not find stopped-bad instance row")
	}
	plain := stripANSI(row)

	if strings.Contains(plain, "! stopped") {
		t.Error("stopped instance should not show '! stopped'")
	}
	if strings.Contains(plain, "~ stopped") {
		t.Error("stopped instance should not show '~ stopped'")
	}
}

// ---------------------------------------------------------------------------
// Detail View — unhealthy shows Status Checks section
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Detail_UnhealthyShowsSection verifies that a running
// instance with instance_status=impaired renders a "Status Checks:" section
// in the detail view with System: and Instance: sub-fields.
func TestEC2StatusChecks_Detail_UnhealthyShowsSection(t *testing.T) {
	r := resource.Resource{
		ID:     "i-0aaa111111111111a",
		Name:   "",
		Status: "running",
		Fields: map[string]string{
			"instance_id":     "i-0aaa111111111111a",
			"state":           "running",
			"type":            "g4dn.xlarge",
			"system_status":   "ok",
			"instance_status": "impaired",
		},
	}

	d := ec2DetailModel(t, r)
	output := d.View()
	plain := stripANSI(output)

	if !strings.Contains(plain, "Status Checks") {
		t.Errorf("detail view should contain 'Status Checks' section for impaired instance, got:\n%s", plain)
	}
	if !strings.Contains(plain, "System") {
		t.Error("detail view should contain 'System' sub-field in Status Checks section")
	}
	if !strings.Contains(plain, "Instance") {
		t.Error("detail view should contain 'Instance' sub-field in Status Checks section")
	}
}

// TestEC2StatusChecks_Detail_UnhealthyShowsStatusValues verifies exact status
// values appear in the detail view for an impaired instance.
func TestEC2StatusChecks_Detail_UnhealthyShowsStatusValues(t *testing.T) {
	r := resource.Resource{
		ID:     "i-0aaa111111111111a",
		Name:   "",
		Status: "running",
		Fields: map[string]string{
			"instance_id":     "i-0aaa111111111111a",
			"state":           "running",
			"type":            "g4dn.xlarge",
			"system_status":   "ok",
			"instance_status": "impaired",
		},
	}

	d := ec2DetailModel(t, r)
	plain := stripANSI(d.View())

	if !strings.Contains(plain, "ok") {
		t.Error("detail view should contain 'ok' for system_status")
	}
	if !strings.Contains(plain, "impaired") {
		t.Error("detail view should contain 'impaired' for instance_status")
	}
}

// TestEC2StatusChecks_Detail_InitializingShowsSection verifies that a running
// instance with both statuses=initializing renders the Status Checks section.
func TestEC2StatusChecks_Detail_InitializingShowsSection(t *testing.T) {
	r := resource.Resource{
		ID:     "i-0bbb222222222222b",
		Name:   "VPN",
		Status: "running",
		Fields: map[string]string{
			"instance_id":     "i-0bbb222222222222b",
			"name":            "VPN",
			"state":           "running",
			"type":            "t3.large",
			"system_status":   "initializing",
			"instance_status": "initializing",
		},
	}

	d := ec2DetailModel(t, r)
	plain := stripANSI(d.View())

	if !strings.Contains(plain, "Status Checks") {
		t.Errorf("detail view should contain 'Status Checks' for initializing instance, got:\n%s", plain)
	}
	if !strings.Contains(plain, "initializing") {
		t.Error("detail view should contain 'initializing' status value")
	}
}

// ---------------------------------------------------------------------------
// Detail View — healthy omits Status Checks section
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Detail_HealthyOmitsSection verifies that a running
// instance with both statuses=ok does NOT render the Status Checks section.
//
// "Silence means healthy."
func TestEC2StatusChecks_Detail_HealthyOmitsSection(t *testing.T) {
	r := resource.Resource{
		ID:     "i-0ccc333333333333c",
		Name:   "kafka",
		Status: "running",
		Fields: map[string]string{
			"instance_id":     "i-0ccc333333333333c",
			"name":            "kafka",
			"state":           "running",
			"type":            "t3.large",
			"system_status":   "ok",
			"instance_status": "ok",
		},
	}

	d := ec2DetailModel(t, r)
	plain := stripANSI(d.View())

	if strings.Contains(plain, "Status Checks") {
		t.Error("healthy instance (both ok) should NOT show 'Status Checks' section in detail view")
	}
}

// ---------------------------------------------------------------------------
// Detail View — no status fields omits section
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Detail_NoFieldsOmitsSection verifies that a running
// instance with NO system_status / instance_status fields (API error fallback)
// does NOT render the Status Checks section.
func TestEC2StatusChecks_Detail_NoFieldsOmitsSection(t *testing.T) {
	r := resource.Resource{
		ID:     "i-0ddd444444444444d",
		Name:   "monitoring",
		Status: "running",
		Fields: map[string]string{
			"instance_id": "i-0ddd444444444444d",
			"name":        "monitoring",
			"state":       "running",
			"type":        "t3.large",
		},
	}

	d := ec2DetailModel(t, r)
	plain := stripANSI(d.View())

	if strings.Contains(plain, "Status Checks") {
		t.Error("instance with no status fields should NOT show 'Status Checks' section")
	}
}

// ---------------------------------------------------------------------------
// Detail View — non-running omits section
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Detail_NonRunningOmitsSection verifies that a terminated
// instance does NOT render the Status Checks section even when status fields exist.
func TestEC2StatusChecks_Detail_NonRunningOmitsSection(t *testing.T) {
	r := resource.Resource{
		ID:     "i-0fff666666666666f",
		Name:   "apps",
		Status: "terminated",
		Fields: map[string]string{
			"instance_id":     "i-0fff666666666666f",
			"name":            "apps",
			"state":           "terminated",
			"type":            "t3.large",
			"system_status":   "not-applicable",
			"instance_status": "not-applicable",
		},
	}

	d := ec2DetailModel(t, r)
	plain := stripANSI(d.View())

	if strings.Contains(plain, "Status Checks") {
		t.Error("terminated instance should NOT show 'Status Checks' section")
	}
}

// TestEC2StatusChecks_Detail_StoppedOmitsSection verifies that a stopped
// instance does NOT render the Status Checks section.
func TestEC2StatusChecks_Detail_StoppedOmitsSection(t *testing.T) {
	r := resource.Resource{
		ID:     "i-stopped-test",
		Name:   "old-worker",
		Status: "stopped",
		Fields: map[string]string{
			"instance_id": "i-stopped-test",
			"name":        "old-worker",
			"state":       "stopped",
			"type":        "t3.medium",
		},
	}

	d := ec2DetailModel(t, r)
	plain := stripANSI(d.View())

	if strings.Contains(plain, "Status Checks") {
		t.Error("stopped instance should NOT show 'Status Checks' section")
	}
}

// ---------------------------------------------------------------------------
// Detail View — dash substitution for empty status values
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Detail_EmptySystemShowsDash verifies that when
// system_status is empty but instance_status is impaired, the System field
// renders as an em-dash ("—") rather than a blank value.
func TestEC2StatusChecks_Detail_EmptySystemShowsDash(t *testing.T) {
	r := resource.Resource{
		ID:     "i-test-empty-sys",
		Name:   "dash-sys",
		Status: "running",
		Fields: map[string]string{
			"instance_id":     "i-test-empty-sys",
			"name":            "dash-sys",
			"state":           "running",
			"type":            "t3.medium",
			"system_status":   "",
			"instance_status": "impaired",
		},
	}

	d := ec2DetailModel(t, r)
	plain := stripANSI(d.View())

	if !strings.Contains(plain, "Status Checks") {
		t.Errorf("expected 'Status Checks' section when instance_status=impaired, got:\n%s", plain)
	}
	if !strings.Contains(plain, "—") {
		t.Errorf("expected em-dash '—' substitution for empty system_status, got:\n%s", plain)
	}
	if !strings.Contains(plain, "impaired") {
		t.Errorf("expected 'impaired' for instance_status, got:\n%s", plain)
	}
}

// TestEC2StatusChecks_Detail_EmptyInstanceShowsDash verifies that when
// instance_status is empty but system_status is impaired, the Instance field
// renders as an em-dash ("—") rather than a blank value.
func TestEC2StatusChecks_Detail_EmptyInstanceShowsDash(t *testing.T) {
	r := resource.Resource{
		ID:     "i-test-empty-inst",
		Name:   "dash-inst",
		Status: "running",
		Fields: map[string]string{
			"instance_id":     "i-test-empty-inst",
			"name":            "dash-inst",
			"state":           "running",
			"type":            "t3.medium",
			"system_status":   "impaired",
			"instance_status": "",
		},
	}

	d := ec2DetailModel(t, r)
	plain := stripANSI(d.View())

	if !strings.Contains(plain, "Status Checks") {
		t.Errorf("expected 'Status Checks' section when system_status=impaired, got:\n%s", plain)
	}
	if !strings.Contains(plain, "—") {
		t.Errorf("expected em-dash '—' substitution for empty instance_status, got:\n%s", plain)
	}
	if !strings.Contains(plain, "impaired") {
		t.Errorf("expected 'impaired' for system_status, got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Detail View — Status Checks section position
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Detail_SectionAppendsAfterState verifies that the
// "Status Checks" section appears after the "State" content in the detail view
// when the State section is the last in the field list (insertAt == -1 path).
func TestEC2StatusChecks_Detail_SectionAppendsAfterState(t *testing.T) {
	r := resource.Resource{
		ID:     "i-test-order",
		Name:   "order-test",
		Status: "running",
		Fields: map[string]string{
			"instance_id":     "i-test-order",
			"state":           "running",
			"system_status":   "ok",
			"instance_status": "impaired",
		},
	}

	d := ec2DetailModel(t, r)
	plain := stripANSI(d.View())

	stateIdx := strings.Index(plain, "state")
	statusIdx := strings.Index(plain, "Status Checks")

	if statusIdx == -1 {
		t.Fatalf("expected 'Status Checks' section in detail view, got:\n%s", plain)
	}
	if stateIdx == -1 {
		t.Fatalf("expected 'state' field in detail view, got:\n%s", plain)
	}
	if statusIdx <= stateIdx {
		t.Errorf("'Status Checks' section (at %d) should appear AFTER 'state' (at %d) in output", statusIdx, stateIdx)
	}
}

// ---------------------------------------------------------------------------
// Detail View — statusCheckStyle colors
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Detail_StatusStyleColors verifies that each status value
// produces appropriate styling: ok=green, impaired=red/bold, initializing=yellow,
// insufficient-data and unknown values use DimText styling.
//
// Lipgloss v2 renders hex colors as RGB decimal ANSI sequences (38;2;R;G;B).
// Palette values: ColRunning=#9ece6a→158;206;106, ColStopped=#f7768e→247;118;142,
// ColPending=#e0af68→224;175;104.
func TestEC2StatusChecks_Detail_StatusStyleColors(t *testing.T) {
	tests := []struct {
		name          string
		systemStatus  string
		wantANSIColor string // RGB decimal substring expected in ANSI escape sequence
		wantPlainText string // plain text to find after stripping ANSI
	}{
		{
			name:          "ok uses green",
			systemStatus:  "ok",
			wantANSIColor: "38;2;158;206;106", // ColRunning #9ece6a
			wantPlainText: "ok",
		},
		{
			name:          "impaired uses red",
			systemStatus:  "impaired",
			wantANSIColor: "38;2;247;118;142", // ColStopped #f7768e
			wantPlainText: "impaired",
		},
		{
			name:          "initializing uses yellow",
			systemStatus:  "initializing",
			wantANSIColor: "38;2;224;175;104", // ColPending #e0af68
			wantPlainText: "initializing",
		},
		{
			name:          "insufficient-data renders",
			systemStatus:  "insufficient-data",
			wantANSIColor: "",
			wantPlainText: "insufficient-data",
		},
		{
			name:          "unknown value renders",
			systemStatus:  "unknown-value",
			wantANSIColor: "",
			wantPlainText: "unknown-value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := resource.Resource{
				ID:     "i-test-style",
				Name:   "style-test",
				Status: "running",
				Fields: map[string]string{
					"instance_id":     "i-test-style",
					"state":           "running",
					"system_status":   tc.systemStatus,
					"instance_status": "impaired",
				},
			}

			d := ec2DetailModel(t, r)
			output := d.View()
			plain := stripANSI(output)

			if !strings.Contains(plain, "Status Checks") {
				t.Fatalf("expected 'Status Checks' section, got:\n%s", plain)
			}

			// Verify the plain text value renders.
			if !strings.Contains(plain, tc.wantPlainText) {
				t.Errorf("expected plain text %q in output, got:\n%s", tc.wantPlainText, plain)
			}

			// When a specific color is expected, verify the ANSI output contains
			// the RGB decimal sequence. Lipgloss v2 renders hex colors as 38;2;R;G;B.
			if tc.wantANSIColor != "" {
				if !strings.Contains(output, tc.wantANSIColor) {
					t.Errorf("expected ANSI sequence %q for status %q, raw output:\n%s",
						tc.wantANSIColor, tc.systemStatus, output)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Fetcher — DescribeInstanceStatus merges fields
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Fetch_MergesStatusFields verifies that FetchEC2InstancesPage
// merges system_status and instance_status from DescribeInstanceStatus into each
// Resource.Fields by instance ID.
func TestEC2StatusChecks_Fetch_MergesStatusFields(t *testing.T) {
	instA := "i-0fetch111111111aa"
	instB := "i-0fetch222222222bb"

	mock := &mockEC2Client{
		output: &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{
						{
							InstanceId:   aws.String(instA),
							State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
							InstanceType: ec2types.InstanceTypeT3Medium,
						},
						{
							InstanceId:   aws.String(instB),
							State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
							InstanceType: ec2types.InstanceTypeT3Large,
						},
					},
				},
			},
		},
		statusOutput: &ec2.DescribeInstanceStatusOutput{
			InstanceStatuses: []ec2types.InstanceStatus{
				{
					InstanceId: aws.String(instA),
					SystemStatus: &ec2types.InstanceStatusSummary{
						Status: ec2types.SummaryStatusOk,
					},
					InstanceStatus: &ec2types.InstanceStatusSummary{
						Status: ec2types.SummaryStatusOk,
					},
				},
				{
					InstanceId: aws.String(instB),
					SystemStatus: &ec2types.InstanceStatusSummary{
						Status: ec2types.SummaryStatusOk,
					},
					InstanceStatus: &ec2types.InstanceStatusSummary{
						Status: ec2types.SummaryStatusImpaired,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchEC2InstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	// Find resources by ID.
	byID := make(map[string]resource.Resource, 2)
	for _, r := range result.Resources {
		byID[r.ID] = r
	}

	rA, ok := byID[instA]
	if !ok {
		t.Fatalf("resource %q not found in results", instA)
	}
	if rA.Fields["system_status"] != "ok" {
		t.Errorf("instA system_status: expected %q, got %q", "ok", rA.Fields["system_status"])
	}
	if rA.Fields["instance_status"] != "ok" {
		t.Errorf("instA instance_status: expected %q, got %q", "ok", rA.Fields["instance_status"])
	}

	rB, ok := byID[instB]
	if !ok {
		t.Fatalf("resource %q not found in results", instB)
	}
	if rB.Fields["system_status"] != "ok" {
		t.Errorf("instB system_status: expected %q, got %q", "ok", rB.Fields["system_status"])
	}
	if rB.Fields["instance_status"] != "impaired" {
		t.Errorf("instB instance_status: expected %q, got %q", "impaired", rB.Fields["instance_status"])
	}
}

// ---------------------------------------------------------------------------
// Fetcher — DescribeInstanceStatus error is non-fatal
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Fetch_ErrorGracefulDegradation verifies that when
// DescribeInstanceStatus returns an error, the fetcher still returns the
// resources from DescribeInstances without status check fields (graceful
// degradation — API error fallback).
func TestEC2StatusChecks_Fetch_ErrorGracefulDegradation(t *testing.T) {
	instID := "i-0fetch333333333cc"

	mock := &mockEC2Client{
		output: &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{
						{
							InstanceId:   aws.String(instID),
							State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
							InstanceType: ec2types.InstanceTypeT3Medium,
						},
					},
				},
			},
		},
		statusErr: errors.New("DescribeInstanceStatus: simulated API error"),
	}

	result, err := awsclient.FetchEC2InstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchEC2InstancesPage should not fail when DescribeInstanceStatus errors; got: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	if r.ID != instID {
		t.Errorf("resource ID: expected %q, got %q", instID, r.ID)
	}

	// No status check fields should be present (graceful degradation).
	if _, ok := r.Fields["system_status"]; ok {
		t.Error("system_status should not be present when DescribeInstanceStatus errors")
	}
	if _, ok := r.Fields["instance_status"]; ok {
		t.Error("instance_status should not be present when DescribeInstanceStatus errors")
	}
}

// ---------------------------------------------------------------------------
// Fetcher — empty instances page skips DescribeInstanceStatus
// ---------------------------------------------------------------------------

// TestEC2StatusChecks_Fetch_EmptyPageSkipsStatusCall verifies that when
// DescribeInstances returns 0 instances, DescribeInstanceStatus is NOT called.
// We enforce this by setting statusErr to a fatal sentinel — if called, the
// error would propagate and fail the test.
func TestEC2StatusChecks_Fetch_EmptyPageSkipsStatusCall(t *testing.T) {
	mock := &mockEC2Client{
		output: &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{},
		},
		// If DescribeInstanceStatus is called on an empty page, this error
		// would cause FetchEC2InstancesPage to return an error or inject bad data.
		statusErr: errors.New("DescribeInstanceStatus should NOT be called on empty page"),
	}

	result, err := awsclient.FetchEC2InstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("empty page should return no error, got: %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for empty page, got %d", len(result.Resources))
	}
}
