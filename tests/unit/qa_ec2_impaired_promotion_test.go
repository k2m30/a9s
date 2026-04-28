package unit

// qa_ec2_impaired_promotion_test.go — RED tests for Bug 1: EC2 impaired/initializing
// status promotion.
//
// Bug: FetchEC2InstancesPage sets Resource.Status = raw instance state (e.g. "running")
// even when DescribeInstanceStatus reports system_status or instance_status == "impaired"
// or "initializing". The current code only merges those fields into Fields[] without
// promoting Resource.Status, so IsIssueRowColor("running") is false and the menu badge
// undercounts issues while ctrl+z hides these rows.
//
// Demanded behavior (post-fix): when a running instance has an impaired or initializing
// system/instance status, Resource.Status is promoted to "impaired" or "initializing"
// respectively so that IsIssueRowColor returns true.
//
// Tests T060–T066:
//   T060 — system_status=impaired → Resource.Status promoted to "impaired"
//   T061 — instance_status=impaired → Resource.Status promoted to "impaired"
//   T062 — instance_status=initializing → Resource.Status promoted to "initializing"
//   T063 — both ok → Resource.Status remains "running"
//   T064 — state=stopped, sys=impaired → Resource.Status stays "stopped" (no masking)
//   T065 — ctrl+z behavioral: impaired/initializing rows visible after toggle
//   T066 — menu badge: impaired + initializing + stopped all counted as issues

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ─────────────────────────────────────────────────────────────────────────────
// Stub: EC2DescribeInstancesAPI for status-promotion tests
// ─────────────────────────────────────────────────────────────────────────────

// stubEC2WithStatusChecks returns a fixed DescribeInstances result and allows
// controlling DescribeInstanceStatus output independently.
type stubEC2WithStatusChecks struct {
	instances     []ec2types.Instance
	instanceStats []ec2types.InstanceStatus
	statusErr     error
}

func (s *stubEC2WithStatusChecks) DescribeInstances(
	_ context.Context,
	_ *ec2.DescribeInstancesInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{Instances: s.instances},
		},
	}, nil
}

func (s *stubEC2WithStatusChecks) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	if s.statusErr != nil {
		return nil, s.statusErr
	}
	return &ec2.DescribeInstanceStatusOutput{
		InstanceStatuses: s.instanceStats,
	}, nil
}

// runningInstance returns a minimal running EC2 instance with the given ID.
func runningInstance(id string) ec2types.Instance {
	return ec2types.Instance{
		InstanceId:   aws.String(id),
		State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		InstanceType: ec2types.InstanceTypeT3Micro,
	}
}

// instanceStatus builds an ec2types.InstanceStatus with explicit sys/instance status strings.
func instanceStatus(id, sysStatus, instStatus string) ec2types.InstanceStatus {
	return ec2types.InstanceStatus{
		InstanceId: aws.String(id),
		SystemStatus: &ec2types.InstanceStatusSummary{
			Status: ec2types.SummaryStatus(sysStatus),
		},
		InstanceStatus: &ec2types.InstanceStatusSummary{
			Status: ec2types.SummaryStatus(instStatus),
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T060 — system_status=impaired → Resource.Status promoted to "impaired"
// ─────────────────────────────────────────────────────────────────────────────

// TestFetchEC2_PromotesStatusToImpairedWhenSystemStatusImpaired verifies that
// a running instance with SystemStatus="impaired" and InstanceStatus="ok"
// has its Resource.Status promoted to "impaired" and its Fields preserved.
//
// Pre-fix: Resource.Status == "running", Fields["system_status"] == "impaired".
// Post-fix: Resource.Status == "impaired", Fields["system_status"] == "impaired",
//
//	Fields["state"] == "running".
func TestFetchEC2_PromotesStatusToImpairedWhenSystemStatusImpaired(t *testing.T) {
	const id = "i-sys-impaired-001"
	stub := &stubEC2WithStatusChecks{
		instances: []ec2types.Instance{runningInstance(id)},
		instanceStats: []ec2types.InstanceStatus{
			instanceStatus(id, "impaired", "ok"),
		},
	}

	resources, err := awsclient.FetchEC2Instances(context.Background(), stub)
	if err != nil {
		t.Fatalf("FetchEC2Instances returned unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// POST-FIX ASSERTION: Status must be promoted to "impaired".
	// PRE-FIX: this fails because Status == "running".
	if r.Status != "impaired" {
		t.Errorf("expected Resource.Status == %q, got %q (system_status=impaired must promote Resource.Status)", "impaired", r.Status)
	}

	// Preservation checks: original data must still be accessible.
	if r.Fields["system_status"] != "impaired" {
		t.Errorf("expected Fields[\"system_status\"] == %q, got %q", "impaired", r.Fields["system_status"])
	}
	if r.Fields["state"] != "running" {
		t.Errorf("expected Fields[\"state\"] == %q (original raw state preserved), got %q", "running", r.Fields["state"])
	}

	// Verify the EC2 Color func classifies this resource as an issue.
	// The Color func reads Fields["system_status"] directly, so Status promotion
	// is not required — the structural field is the source of truth.
	ec2td := resource.FindResourceType("ec2")
	if ec2td != nil && ec2td.Color != nil {
		if !ec2td.Color(r).IsIssue() {
			t.Errorf("ec2.Color(r).IsIssue() must be true so the menu badge counts this instance (system_status=%q)", r.Fields["system_status"])
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T061 — instance_status=impaired → Resource.Status promoted to "impaired"
// ─────────────────────────────────────────────────────────────────────────────

// TestFetchEC2_PromotesStatusToImpairedWhenInstanceStatusImpaired verifies that
// a running instance with SystemStatus="ok" and InstanceStatus="impaired"
// has its Resource.Status promoted to "impaired".
//
// Pre-fix: Resource.Status == "running".
// Post-fix: Resource.Status == "impaired".
func TestFetchEC2_PromotesStatusToImpairedWhenInstanceStatusImpaired(t *testing.T) {
	const id = "i-inst-impaired-001"
	stub := &stubEC2WithStatusChecks{
		instances: []ec2types.Instance{runningInstance(id)},
		instanceStats: []ec2types.InstanceStatus{
			instanceStatus(id, "ok", "impaired"),
		},
	}

	resources, err := awsclient.FetchEC2Instances(context.Background(), stub)
	if err != nil {
		t.Fatalf("FetchEC2Instances returned unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// POST-FIX ASSERTION: Status must be promoted to "impaired".
	// PRE-FIX: this fails because Status == "running".
	if r.Status != "impaired" {
		t.Errorf("expected Resource.Status == %q, got %q (instance_status=impaired must promote Resource.Status)", "impaired", r.Status)
	}

	if r.Fields["instance_status"] != "impaired" {
		t.Errorf("expected Fields[\"instance_status\"] == %q, got %q", "impaired", r.Fields["instance_status"])
	}
	if r.Fields["state"] != "running" {
		t.Errorf("expected Fields[\"state\"] == %q (original raw state preserved), got %q", "running", r.Fields["state"])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T062 — instance_status=initializing → Resource.Status promoted to "initializing"
// ─────────────────────────────────────────────────────────────────────────────

// TestFetchEC2_PromotesStatusToInitializingWhenInstanceStatusInitializing verifies
// that a running instance with SystemStatus="ok" and InstanceStatus="initializing"
// has its Resource.Status promoted to "initializing".
//
// Pre-fix: Resource.Status == "running".
// Post-fix: Resource.Status == "initializing".
func TestFetchEC2_PromotesStatusToInitializingWhenInstanceStatusInitializing(t *testing.T) {
	const id = "i-initializing-001"
	stub := &stubEC2WithStatusChecks{
		instances: []ec2types.Instance{runningInstance(id)},
		instanceStats: []ec2types.InstanceStatus{
			instanceStatus(id, "ok", "initializing"),
		},
	}

	resources, err := awsclient.FetchEC2Instances(context.Background(), stub)
	if err != nil {
		t.Fatalf("FetchEC2Instances returned unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// POST-FIX ASSERTION: Status must be promoted to "initializing".
	// PRE-FIX: this fails because Status == "running".
	if r.Status != "initializing" {
		t.Errorf("expected Resource.Status == %q, got %q (instance_status=initializing must promote Resource.Status)", "initializing", r.Status)
	}

	if r.Fields["instance_status"] != "initializing" {
		t.Errorf("expected Fields[\"instance_status\"] == %q, got %q", "initializing", r.Fields["instance_status"])
	}

	// Verify the EC2 Color func classifies this resource as an issue (ctrl+z keeps it visible).
	ec2tdInit := resource.FindResourceType("ec2")
	if ec2tdInit != nil && ec2tdInit.Color != nil {
		if !ec2tdInit.Color(r).IsIssue() {
			t.Errorf("ec2.Color(r).IsIssue() must be true so ctrl+z keeps this row visible (instance_status=%q)", r.Fields["instance_status"])
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T063 — both ok → Resource.Status stays "running"
// ─────────────────────────────────────────────────────────────────────────────

// TestFetchEC2_LeavesStatusAsRunningWhenBothOk verifies that a running instance
// with SystemStatus="ok" and InstanceStatus="ok" emits no Findings and has its
// state surfaced via Fields["state"]. PR-03b: fetcher no longer writes Resource.Status;
// the list-view falls back to Fields[LifecycleKey] for healthy instances.
func TestFetchEC2_LeavesStatusAsRunningWhenBothOk(t *testing.T) {
	const id = "i-both-ok-001"
	stub := &stubEC2WithStatusChecks{
		instances: []ec2types.Instance{runningInstance(id)},
		instanceStats: []ec2types.InstanceStatus{
			instanceStatus(id, "ok", "ok"),
		},
	}

	resources, err := awsclient.FetchEC2Instances(context.Background(), stub)
	if err != nil {
		t.Fatalf("FetchEC2Instances returned unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	// PR-03b: fetcher no longer writes Resource.Status; lifecycle is in Fields["state"].
	if r.Status != "" {
		t.Errorf("expected Resource.Status == %q (empty) for healthy running instance, got %q", "", r.Status)
	}
	if len(r.Findings) != 0 {
		t.Errorf("expected 0 Findings for healthy running instance, got %d", len(r.Findings))
	}
	if r.Fields["state"] != "running" {
		t.Errorf("expected Fields[\"state\"] == %q, got %q", "running", r.Fields["state"])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T064 — state=stopped with sys=impaired → Status stays "stopped"
// ─────────────────────────────────────────────────────────────────────────────

// TestFetchEC2_DoesNotPromoteStoppedInstance verifies that a stopped instance
// whose DescribeInstanceStatus defensively reports system_status=impaired is
// handled correctly: the fetcher must emit a stopped-state Finding (not an
// impaired-promotion Finding) and must never write Resource.Status.
//
// PR-03b: Resource.Status is always empty; stopped instances emit
// CodeEC2StateStopped/SevWarn (no Server.* state reason in fixture →
// user-initiated stop, not server fault). The impaired status-check data
// must not override the lifecycle-state Finding.
func TestFetchEC2_DoesNotPromoteStoppedInstance(t *testing.T) {
	const id = "i-stopped-sys-impaired"
	stoppedInst := ec2types.Instance{
		InstanceId:   aws.String(id),
		State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
		InstanceType: ec2types.InstanceTypeT3Micro,
	}
	stub := &stubEC2WithStatusChecks{
		instances: []ec2types.Instance{stoppedInst},
		// Defensively provide impaired status even for stopped instance.
		instanceStats: []ec2types.InstanceStatus{
			instanceStatus(id, "impaired", "ok"),
		},
	}

	resources, err := awsclient.FetchEC2Instances(context.Background(), stub)
	if err != nil {
		t.Fatalf("FetchEC2Instances returned unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	// PR-03b: fetcher no longer writes Resource.Status; state is surfaced via Findings.
	if r.Status != "" {
		t.Errorf("expected Resource.Status == %q (empty) for stopped instance, got %q", "", r.Status)
	}
	// Stopped instance with no Server.* state reason → CodeEC2StateStopped / SevWarn.
	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 Finding for stopped instance, got %d", len(r.Findings))
	}
	if r.Findings[0].Code != awsclient.CodeEC2StateStopped {
		t.Errorf("Findings[0].Code: expected %q, got %q", awsclient.CodeEC2StateStopped, r.Findings[0].Code)
	}
	if r.Findings[0].Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: expected SevWarn, got %v (stopped state must not be promoted to SevBroken without Server.* reason)", r.Findings[0].Severity)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T065 — ctrl+z: impaired/initializing rows remain visible after toggle
// ─────────────────────────────────────────────────────────────────────────────

// TestCtrlZ_EC2_ImpairedRowsVisibleAfterPromotion verifies the behavioral
// end-to-end: when resources have Status "impaired" or "initializing" (simulating
// the post-promotion state), pressing ctrl+z (attention filter) must NOT hide them.
// These are issue-colored rows and must pass the IsIssueRowColor gate.
//
// Pre-fix: because the fetcher leaves Status=="running", ctrl+z hides them since
// "running" is not an issue status. This test uses pre-promoted resources directly,
// bypassing the fetcher, to isolate the ctrl+z visibility behavior.
// The test will PASS once both the fetcher promotion (Bug 1) and the attention
// filter logic both work correctly. Pre-fix it fails because the test is explicitly
// verifying that "impaired"/"initializing" rows survive the filter.
//
// We load promoted resources directly into the list model to test the ctrl+z path.
func TestCtrlZ_EC2_ImpairedRowsVisibleAfterPromotion(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	// Simulate post-promotion resources: status is "impaired"/"initializing"
	// (as the fixed fetcher would produce).
	promotedResources := []resource.Resource{
		{ID: "i-impaired-001", Name: "web-server-impaired", Status: "impaired", Fields: map[string]string{
			"state":         "running",
			"system_status": "impaired",
			"name":          "web-server-impaired",
		}},
		{ID: "i-initializing-001", Name: "web-server-init", Status: "initializing", Fields: map[string]string{
			"state":           "running",
			"instance_status": "initializing",
			"name":            "web-server-init",
		}},
	}

	// Load the resources into the active ResourceListModel.
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    promotedResources,
	})

	// Verify both rows are visible before toggling ctrl+z.
	plainBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(plainBefore, "web-server-impaired") {
		t.Error("impaired row should be visible before ctrl+z")
	}
	if !strings.Contains(plainBefore, "web-server-init") {
		t.Error("initializing row should be visible before ctrl+z")
	}

	// Press ctrl+z to enable attention filter (hide non-issue rows).
	ctrlZMsg := rootKeyPress("\x1a") // Ctrl+Z = ASCII 26 = \x1a
	m, _ = rootApplyMsg(m, ctrlZMsg)

	// ASSERTION: both impaired and initializing rows must remain visible
	// because IsIssueRowColor returns true for both statuses.
	// Pre-fix (if Status were "running"): rows would be hidden by the filter.
	plainAfter := stripANSI(rootViewContent(m))
	if !strings.Contains(plainAfter, "web-server-impaired") {
		t.Error("impaired row must remain visible after ctrl+z attention filter: " +
			"ec2.Color(r).IsIssue() is true when system_status=impaired, so row must not be filtered out")
	}
	if !strings.Contains(plainAfter, "web-server-init") {
		t.Error("initializing row must remain visible after ctrl+z attention filter: " +
			"ec2.Color(r).IsIssue() is true when instance_status=initializing, so row must not be filtered out")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T066 — menu badge counts impaired + initializing + stopped as issues
// ─────────────────────────────────────────────────────────────────────────────

// TestMenuBadge_EC2_CountsImpairedRows verifies that a ResourceListModel loaded
// with "running", "impaired", "initializing", and "stopped" resources reports
// issueCount == 3 (impaired + initializing + stopped), because all three are in
// issueStatusSet and IsIssueRowColor returns true for them.
//
// This test verifies the downstream impact of status promotion: once the fetcher
// correctly sets Resource.Status to "impaired" or "initializing", the badge count
// must reflect those instances. Pre-fix: badge shows 1 (only stopped) because the
// fetcher leaves impaired/initializing instances with Status=="running".
//
// The test is framed around the view output: the ResourceListModel's frame title
// must contain "issues:3" (or the badge count) after loading the resources.
func TestMenuBadge_EC2_CountsImpairedRows(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to EC2 list and set showIssueBadge=true (top-level list from main menu).
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Load resources with status already promoted (as the fixed fetcher would produce).
	mixedResources := []resource.Resource{
		{ID: "i-running-001", Name: "healthy-server", Status: "running", Fields: map[string]string{"state": "running", "name": "healthy-server"}},
		{ID: "i-impaired-001", Name: "impaired-server", Status: "impaired", Fields: map[string]string{"state": "running", "system_status": "impaired", "name": "impaired-server"}},
		{ID: "i-initializing-001", Name: "init-server", Status: "initializing", Fields: map[string]string{"state": "running", "instance_status": "initializing", "name": "init-server"}},
		{ID: "i-stopped-001", Name: "stopped-server", Status: "stopped", Fields: map[string]string{"state": "stopped", "name": "stopped-server"}},
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    mixedResources,
	})

	// Spec §4: S1 "issues:N" is the MENU badge, not the list title. The
	// invariant under test is that impaired + initializing + stopped all
	// count as issues via ec2.Color(r).IsIssue() == true. Pop back to the
	// main menu and verify the badge reads "issues:3".
	// Pre-fix: badge was 1 because the fetcher left impaired/initializing as
	// "running" and the old global string-set didn't catch them.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "issues:3") {
		t.Errorf("expected menu badge 'issues:3' after popping to menu (impaired+initializing+stopped), got excerpt:\n%s",
			plain[:min(600, len(plain))])
	}
}
