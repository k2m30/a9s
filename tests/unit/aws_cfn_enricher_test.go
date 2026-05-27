package unit

// aws_cfn_enricher_test.go — Behavioral tests for EnrichCFNDrift.
//
// Contract assertions:
//   - DescribeStacks is called once per CFN resource (keyed by stack name).
//   - A stack with DriftInformation.StackDriftStatus=DRIFTED → 1 finding sev "~" for that stack.
//   - A stack with DriftInformation.StackDriftStatus=IN_SYNC → 0 findings.
//   - clients.CloudFormation == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for a resource → Truncated=true, no finding for that stack, no error returned.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestCFNRegisteredInIssueEnricherRegistry verifies that cfn has a non-nil
// Wave 2 enricher reachable via awsclient.Wave2EnricherFor — the catalog
// Wave2 field on the cfn ResourceTypeDef. Every documented Wave 2 row must
// have a registry entry per docs/attention-signals.md.
func TestCFNRegisteredInIssueEnricherRegistry(t *testing.T) {
	if _, ok := awsclient.Wave2EnricherFor("cfn"); !ok {
		t.Error("cfn must have a non-nil catalog Wave2 enricher per docs/attention-signals.md")
	}
}

// cfnDescribeStacksDriftFake implements CFNAPI for enrichment testing.
// It embeds the aggregate interface and overrides only DescribeStacks.
// The stacksByName map is keyed by stack name; errByStack overrides when set.
type cfnDescribeStacksDriftFake struct {
	awsclient.CFNAPI
	// stacksByName maps stack name → slice of Stack returned by DescribeStacks.
	stacksByName map[string][]cfntypes.Stack
	// errByStack maps stack name → error; overrides stacksByName when set.
	errByStack map[string]error
}

func (f *cfnDescribeStacksDriftFake) DescribeStacks(
	_ context.Context,
	in *cloudformation.DescribeStacksInput,
	_ ...func(*cloudformation.Options),
) (*cloudformation.DescribeStacksOutput, error) {
	name := ""
	if in != nil && in.StackName != nil {
		name = *in.StackName
	}
	if f.errByStack != nil {
		if err, ok := f.errByStack[name]; ok {
			return nil, err
		}
	}
	stacks, ok := f.stacksByName[name]
	if !ok {
		return &cloudformation.DescribeStacksOutput{}, nil
	}
	return &cloudformation.DescribeStacksOutput{Stacks: stacks}, nil
}

// Compile-time check: cfnDescribeStacksDriftFake satisfies CFNAPI.
var _ awsclient.CFNAPI = (*cfnDescribeStacksDriftFake)(nil)

// cfnDriftStackResource builds a CFN Resource stub matching what
// FetchCloudFormationStacksPage produces: ID = stack name.
func cfnDriftStackResource(name, status string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"stack_name":    name,
			"status":        status,
			"creation_time": "2025-03-01 12:00",
			"last_updated":  "2025-04-10 09:00",
			"description":   "test stack " + name,
		},
	}
}

// cfnDriftedStack builds a Stack with DRIFTED drift status.
func cfnDriftedStack(name string) cfntypes.Stack {
	return cfntypes.Stack{
		StackName:   aws.String(name),
		StackStatus: cfntypes.StackStatusUpdateComplete,
		DriftInformation: &cfntypes.StackDriftInformation{
			StackDriftStatus: cfntypes.StackDriftStatusDrifted,
		},
	}
}

// cfnInSyncStack builds a Stack with IN_SYNC drift status.
func cfnInSyncStack(name string) cfntypes.Stack {
	return cfntypes.Stack{
		StackName:   aws.String(name),
		StackStatus: cfntypes.StackStatusUpdateComplete,
		DriftInformation: &cfntypes.StackDriftInformation{
			StackDriftStatus: cfntypes.StackDriftStatusInSync,
		},
	}
}

const (
	cfnDriftStack1 = "my-service-stack"
	cfnDriftStack2 = "my-infra-stack"
)

// TestEnrichCFNDrift_DriftedStackProducesFindingSevTilde verifies that when stack-1 has
// DriftInformation.StackDriftStatus=DRIFTED, a finding with severity "~" is produced
// for stack-1. stack-2 (IN_SYNC) must have no finding.
// "~" findings do NOT contribute to IssueCount per the EnricherResult contract.
func TestEnrichCFNDrift_DriftedStackProducesFindingSevTilde(t *testing.T) {
	fake := &cfnDescribeStacksDriftFake{
		stacksByName: map[string][]cfntypes.Stack{
			cfnDriftStack1: {cfnDriftedStack(cfnDriftStack1)},
			cfnDriftStack2: {cfnInSyncStack(cfnDriftStack2)},
		},
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{
		cfnDriftStackResource(cfnDriftStack1, "UPDATE_COMPLETE"),
		cfnDriftStackResource(cfnDriftStack2, "UPDATE_COMPLETE"),
	}

	result, err := awsclient.EnrichCFNDrift(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	f, ok := result.Findings[cfnDriftStack1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (drifted stack)", cfnDriftStack1)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings[cfnDriftStack2]; ok {
		t.Error("stack-2 must NOT appear in Findings — it is IN_SYNC")
	}
	// "~" findings do NOT contribute to IssueCount per the EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichCFNDrift_InSyncStackProducesNoFinding verifies that when both stacks have
// DriftInformation.StackDriftStatus=IN_SYNC, no findings are produced and IssueCount=0.
func TestEnrichCFNDrift_InSyncStackProducesNoFinding(t *testing.T) {
	fake := &cfnDescribeStacksDriftFake{
		stacksByName: map[string][]cfntypes.Stack{
			cfnDriftStack1: {cfnInSyncStack(cfnDriftStack1)},
			cfnDriftStack2: {cfnInSyncStack(cfnDriftStack2)},
		},
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{
		cfnDriftStackResource(cfnDriftStack1, "CREATE_COMPLETE"),
		cfnDriftStackResource(cfnDriftStack2, "CREATE_COMPLETE"),
	}

	result, err := awsclient.EnrichCFNDrift(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichCFNDrift_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.CloudFormation is nil the enricher returns a non-nil empty Findings map
// and no error.
func TestEnrichCFNDrift_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{CloudFormation: nil}
	resources := []resource.Resource{
		cfnDriftStackResource(cfnDriftStack1, "UPDATE_COMPLETE"),
		cfnDriftStackResource(cfnDriftStack2, "UPDATE_COMPLETE"),
	}

	result, err := awsclient.EnrichCFNDrift(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when CloudFormation client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichCFNDrift_APIErrorSetsTruncatedAndSurfacesError verifies that when the
// API call for stack-1 returns a generic error, the enricher sets Truncated=true,
// produces 0 findings for that stack, and returns a composite error containing the
// failing stack ID.
func TestEnrichCFNDrift_APIErrorSetsTruncatedAndSurfacesError(t *testing.T) {
	apiErr := errors.New("cloudformation: DescribeStacks throttled")
	fake := &cfnDescribeStacksDriftFake{
		errByStack: map[string]error{
			cfnDriftStack1: apiErr,
		},
		stacksByName: map[string][]cfntypes.Stack{
			cfnDriftStack2: {cfnInSyncStack(cfnDriftStack2)},
		},
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{
		cfnDriftStackResource(cfnDriftStack1, "UPDATE_COMPLETE"),
		cfnDriftStackResource(cfnDriftStack2, "UPDATE_COMPLETE"),
	}

	result, err := awsclient.EnrichCFNDrift(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when at least one stack API call failed")
	}
	if errStr := err.Error(); !strings.Contains(errStr, "cfn-enrich:") {
		t.Errorf("composite error must contain \"cfn-enrich:\", got: %q", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, cfnDriftStack1) {
		t.Errorf("composite error must contain the failing stack ID %q, got: %q", cfnDriftStack1, errStr)
	}
	if _, ok := result.Findings[cfnDriftStack1]; ok {
		t.Error("stack-1 must NOT appear in Findings on generic API error")
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a generic API call fails")
	}
}
