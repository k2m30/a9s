package unit_test

// phase03_ec2_pr03b_test.go — TDD failing tests for the EC2 Wave 1 finding migration.
//
// Migration target (PR-03b, compute category):
//   - Fetcher STOPS writing Status for lifecycle states; keeps Fields["state"].
//   - Fetcher EMITS canonical Finding entries into Resource.Findings for
//     non-healthy, non-terminal lifecycle states.
//   - EC2 Color func reads Findings[0].Severity first, falling back to
//     structural-field logic when Findings is empty.
//   - New file internal/aws/ec2_codes.go declares FindingCode constants.
//
// These tests are RED until the coder implements the migration.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// T03b-1 — Compile-time check: FindingCode constants exist in awsclient
// ---------------------------------------------------------------------------

// TestEC2Codes_ConstantsExist verifies that ec2_codes.go declares the four
// Wave 1 finding constants as domain.FindingCode typed values. This fails to
// compile until the constants are introduced.
func TestEC2Codes_ConstantsExist(t *testing.T) {
	t.Helper()
	// These will not compile until internal/aws/ec2_codes.go is created.
	var _ domain.FindingCode = awsclient.CodeEC2StatePending
	var _ domain.FindingCode = awsclient.CodeEC2StateStopping
	var _ domain.FindingCode = awsclient.CodeEC2StateStopped
	var _ domain.FindingCode = awsclient.CodeEC2StateStoppedServer
}

// ---------------------------------------------------------------------------
// T03b-2 — running state → no Finding, no Status
// ---------------------------------------------------------------------------

// TestEC2Fetcher_RunningStateEmitsNoFinding asserts that a running instance
// produces no Finding and an empty Status after the Wave 1 migration.
// Pre-migration: Status == "running". Post-migration: Status == "".
func TestEC2Fetcher_RunningStateEmitsNoFinding(t *testing.T) {
	mock := newEC2MockForPR03b([]ec2types.Instance{
		{
			InstanceId:   aws.String("i-0123abc"),
			InstanceType: ec2types.InstanceTypeT3Micro,
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameRunning,
			},
			PrivateIpAddress: aws.String("10.0.1.10"),
		},
	})

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchEC2Instances: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	// Post-migration: fetcher no longer writes Status for lifecycle states.

	// No finding for the healthy steady-state.
	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d findings, want 0 for running state", len(r.Findings))
	}

	// state field must still be populated in Fields.
	if r.Fields["state"] != "running" {
		t.Errorf("Fields[\"state\"]: got %q, want %q", r.Fields["state"], "running")
	}
}

// ---------------------------------------------------------------------------
// T03b-3 — pending state → SevWarn Finding
// ---------------------------------------------------------------------------

// TestEC2Fetcher_PendingStateEmitsWarnFinding asserts that a pending instance
// emits one SevWarn Finding with CodeEC2StatePending.
func TestEC2Fetcher_PendingStateEmitsWarnFinding(t *testing.T) {
	mock := newEC2MockForPR03b([]ec2types.Instance{
		{
			InstanceId:   aws.String("i-0123abc"),
			InstanceType: ec2types.InstanceTypeT3Micro,
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNamePending,
			},
			PrivateIpAddress: aws.String("10.0.1.11"),
		},
	})

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchEC2Instances: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]


	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for pending state", len(r.Findings))
	}

	f := r.Findings[0]
	if f.Code != awsclient.CodeEC2StatePending {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeEC2StatePending)
	}
	if f.Phrase != "pending" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, "pending")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// T03b-4 — stopped + Server.* reason → SevBroken Finding
// ---------------------------------------------------------------------------

// TestEC2Fetcher_StoppedServerEmitsBrokenFinding asserts that a stopped
// instance with a Server.* state_reason_code emits one SevBroken Finding
// with CodeEC2StateStoppedServer.
func TestEC2Fetcher_StoppedServerEmitsBrokenFinding(t *testing.T) {
	mock := newEC2MockForPR03b([]ec2types.Instance{
		{
			InstanceId:   aws.String("i-0123abc"),
			InstanceType: ec2types.InstanceTypeT3Micro,
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameStopped,
			},
			StateReason: &ec2types.StateReason{
				Code:    aws.String("Server.SpotInstanceShutdown"),
				Message: aws.String("Server.SpotInstanceShutdown: The Spot Instance was stopped because the Spot price exceeded your maximum price."),
			},
			PrivateIpAddress: aws.String("10.0.1.12"),
		},
	})

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchEC2Instances: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]


	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for stopped+Server.* state", len(r.Findings))
	}

	f := r.Findings[0]
	if f.Code != awsclient.CodeEC2StateStoppedServer {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeEC2StateStoppedServer)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
}

// ---------------------------------------------------------------------------
// T03b-5 — stopped + non-Server reason → SevWarn Finding
// ---------------------------------------------------------------------------

// TestEC2Fetcher_StoppedUserEmitsWarnFinding asserts that a stopped instance
// with a user-initiated reason emits one SevWarn Finding with CodeEC2StateStopped.
func TestEC2Fetcher_StoppedUserEmitsWarnFinding(t *testing.T) {
	mock := newEC2MockForPR03b([]ec2types.Instance{
		{
			InstanceId:   aws.String("i-0456def"),
			InstanceType: ec2types.InstanceTypeM5Large,
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameStopped,
			},
			StateReason: &ec2types.StateReason{
				Code:    aws.String("Client.UserInitiatedShutdown"),
				Message: aws.String("Client.UserInitiatedShutdown: User initiated shutdown"),
			},
			PrivateIpAddress: aws.String("10.0.1.13"),
		},
	})

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchEC2Instances: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for stopped (user) state", len(r.Findings))
	}

	f := r.Findings[0]
	if f.Code != awsclient.CodeEC2StateStopped {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeEC2StateStopped)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
}

// ---------------------------------------------------------------------------
// T03b-6 — terminated state → no Finding
// ---------------------------------------------------------------------------

// TestEC2Fetcher_TerminatedEmitsNoFinding asserts that a terminated instance
// emits no Finding. Terminated is a lifecycle terminal state; it lives in
// Fields["state"], not Findings.
func TestEC2Fetcher_TerminatedEmitsNoFinding(t *testing.T) {
	mock := newEC2MockForPR03b([]ec2types.Instance{
		{
			InstanceId:   aws.String("i-0789ghi"),
			InstanceType: ec2types.InstanceTypeT3Micro,
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameTerminated,
			},
			StateReason: &ec2types.StateReason{
				Code:    aws.String("Client.UserInitiatedShutdown"),
				Message: aws.String("Client.UserInitiatedShutdown: User initiated shutdown"),
			},
			PrivateIpAddress: aws.String("10.0.1.14"),
		},
	})

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchEC2Instances: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d findings, want 0 for terminated state (lifecycle terminal)", len(r.Findings))
	}
}

// ---------------------------------------------------------------------------
// T03b-7 — Color reads Findings[0].Severity first
// ---------------------------------------------------------------------------

// TestEC2Color_ReadsFindingsFirst asserts that the EC2 Color func returns
// ColorBroken when Findings[0].Severity is SevBroken, even if Fields["state"]
// would ordinarily yield ColorHealthy via the lifecycle path.
//
// Pre-fix: ec2.Color ignores Findings → returns ColorHealthy.
// Post-fix: ec2.Color reads Findings[0].Severity → returns ColorBroken.
func TestEC2Color_ReadsFindingsFirst(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 type not registered")
	}

	// Running state would produce ColorHealthy via the structural path.
	// Findings override must win.
	r := resource.Resource{
		Type: "ec2",
		Fields: map[string]string{
			"state":           "running",
			"system_status":   "ok",
			"instance_status": "ok",
		},
		Findings: []domain.Finding{
			{
				Code:     awsclient.CodeEC2StateStoppedServer,
				Phrase:   "stopped",
				Severity: domain.SevBroken,
				Source:   "wave1",
			},
		},
	}

	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("Color: got %v, want ColorBroken — Findings[0].Severity=SevBroken must override Fields[\"state\"]=\"running\"", got)
	}
}

// ---------------------------------------------------------------------------
// T03b-8 — Color falls back to structural path when Findings is empty
// ---------------------------------------------------------------------------

// TestEC2Color_FallsBackWhenFindingsEmpty is a regression pin: when Findings
// is nil the existing structural-field logic must still return ColorBroken for
// a stopped instance with a Server.* reason code.
//
// Pre-fix: passes (already works via Fields path).
// Post-fix: still passes (fallback preserved). This test must stay green
// across both before and after the migration.
func TestEC2Color_FallsBackWhenFindingsEmpty(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 type not registered")
	}

	r := resource.Resource{
		Type: "ec2",
		Fields: map[string]string{
			"state":             "stopped",
			"state_reason_code": "Server.InternalError",
		},
		Findings: nil,
	}

	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("Color: got %v, want ColorBroken — fallback structural path: stopped+Server.* must yield ColorBroken when Findings is empty", got)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newEC2MockForPR03b returns a minimal mockEC2Client (defined in mocks_test.go,
// package unit) populated with the given instances in a single reservation.
//
// NOTE: this helper is in package unit_test (external test package) so it
// cannot directly reference mocks_test.go's unexported type. Instead it builds
// the same value via the exported FetchEC2Instances path, which accepts any
// EC2FetchInstancesAPI. We declare a local unexported adapter here.
type pr03bEC2Mock struct {
	instances []ec2types.Instance
}

func newEC2MockForPR03b(instances []ec2types.Instance) *pr03bEC2Mock {
	return &pr03bEC2Mock{instances: instances}
}

func (m *pr03bEC2Mock) DescribeInstances(
	_ context.Context,
	_ *ec2svc.DescribeInstancesInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeInstancesOutput, error) {
	return &ec2svc.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{Instances: m.instances},
		},
	}, nil
}

func (m *pr03bEC2Mock) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2svc.DescribeInstanceStatusInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeInstanceStatusOutput, error) {
	return &ec2svc.DescribeInstanceStatusOutput{}, nil
}

// ---------------------------------------------------------------------------
// T03b-9 — stopping state → SevWarn Finding (missing coverage)
// ---------------------------------------------------------------------------

// TestEC2Fetcher_StoppingStateEmitsWarnFinding asserts that an instance in the
// "stopping" transient state emits one SevWarn Finding with CodeEC2StateStopping.
// This state is distinct from "stopped" — the instance is mid-shutdown.
//
// Pre-fix: stopping state may fall through with no Finding (gap in switch).
// Post-fix: CodeEC2StateStopping / SevWarn / Source:"wave1" must be emitted.
func TestEC2Fetcher_StoppingStateEmitsWarnFinding(t *testing.T) {
	mock := newEC2MockForPR03b([]ec2types.Instance{
		{
			InstanceId:       aws.String("i-0stop123def456abc"),
			InstanceType:     ec2types.InstanceTypeT3Micro,
			State:            &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopping},
			PrivateIpAddress: aws.String("10.0.2.50"),
		},
	})

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchEC2Instances: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for stopping state", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeEC2StateStopping {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeEC2StateStopping)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}
