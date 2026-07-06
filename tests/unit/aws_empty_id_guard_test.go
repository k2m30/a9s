// aws_empty_id_guard_test.go — RED pins for a single defect class seen in
// LIVE readonly-profile validation: a related-panel drill can produce a row
// whose identifying field is empty (a rule with no name, a stack with no
// name, a hosted zone with no ID), and today each of the three per-row AWS
// fetchers below fires its API call anyway with that empty identifier,
// rather than skipping the row. Each pin drives the fetcher across two rows
// — one valid, one with an empty identifier — via a recording fake client,
// and asserts exactly one call was made (for the valid row only).
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/route53"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// recordingEBTargetsClient records every rule name it was called with for
// ListTargetsByRule and returns an empty result.
type recordingEBTargetsClient struct {
	ruleNames []string
}

func (f *recordingEBTargetsClient) ListTargetsByRule(
	_ context.Context,
	params *eventbridge.ListTargetsByRuleInput,
	_ ...func(*eventbridge.Options),
) (*eventbridge.ListTargetsByRuleOutput, error) {
	name := ""
	if params.Rule != nil {
		name = *params.Rule
	}
	f.ruleNames = append(f.ruleNames, name)
	return &eventbridge.ListTargetsByRuleOutput{}, nil
}

// TestFetchEventBridgeRuleTargets_SkipsEmptyRuleName pins the eb-rule guard:
// given rows [valid "deploy-hook", empty ""], exactly one ListTargetsByRule
// call must be made (for "deploy-hook"), never with an empty rule name.
func TestFetchEventBridgeRuleTargets_SkipsEmptyRuleName(t *testing.T) {
	fake := &recordingEBTargetsClient{}

	rows := []map[string]string{
		{"rule_name": "deploy-hook", "event_bus": "default"},
		{"rule_name": "", "event_bus": "default"},
	}

	for _, parentCtx := range rows {
		if _, err := awsclient.FetchEventBridgeRuleTargets(context.Background(), fake, parentCtx, ""); err != nil {
			t.Fatalf("FetchEventBridgeRuleTargets(%v) unexpected error: %v", parentCtx, err)
		}
	}

	if len(fake.ruleNames) != 1 {
		t.Fatalf("ListTargetsByRule called %d times, want exactly 1 (calls: %v)", len(fake.ruleNames), fake.ruleNames)
	}
	if fake.ruleNames[0] != "deploy-hook" {
		t.Errorf("ListTargetsByRule called with rule name %q, want %q", fake.ruleNames[0], "deploy-hook")
	}
	for _, name := range fake.ruleNames {
		if name == "" {
			t.Error("ListTargetsByRule was called with an empty rule name")
		}
	}
}

// recordingCFNStackEventsClient records every stack name it was called with
// for DescribeStackEvents and returns an empty result.
type recordingCFNStackEventsClient struct {
	stackNames []string
}

func (f *recordingCFNStackEventsClient) DescribeStackEvents(
	_ context.Context,
	params *cloudformation.DescribeStackEventsInput,
	_ ...func(*cloudformation.Options),
) (*cloudformation.DescribeStackEventsOutput, error) {
	name := ""
	if params.StackName != nil {
		name = *params.StackName
	}
	f.stackNames = append(f.stackNames, name)
	return &cloudformation.DescribeStackEventsOutput{}, nil
}

// TestFetchCfnEvents_SkipsEmptyStackName pins the cfn-events guard: given
// rows [valid "prod-network", empty ""], exactly one DescribeStackEvents
// call must be made (for "prod-network"), never with an empty stack name.
func TestFetchCfnEvents_SkipsEmptyStackName(t *testing.T) {
	fake := &recordingCFNStackEventsClient{}

	stackNames := []string{"prod-network", ""}

	for _, stackName := range stackNames {
		if _, err := awsclient.FetchCfnEvents(context.Background(), fake, stackName, ""); err != nil {
			t.Fatalf("FetchCfnEvents(%q) unexpected error: %v", stackName, err)
		}
	}

	if len(fake.stackNames) != 1 {
		t.Fatalf("DescribeStackEvents called %d times, want exactly 1 (calls: %v)", len(fake.stackNames), fake.stackNames)
	}
	if fake.stackNames[0] != "prod-network" {
		t.Errorf("DescribeStackEvents called with stack name %q, want %q", fake.stackNames[0], "prod-network")
	}
	for _, name := range fake.stackNames {
		if name == "" {
			t.Error("DescribeStackEvents was called with an empty stack name")
		}
	}
}

// recordingR53RecordsClient records every hosted zone ID it was called with
// for ListResourceRecordSets and returns an empty result.
type recordingR53RecordsClient struct {
	hostedZoneIDs []string
}

func (f *recordingR53RecordsClient) ListResourceRecordSets(
	_ context.Context,
	params *route53.ListResourceRecordSetsInput,
	_ ...func(*route53.Options),
) (*route53.ListResourceRecordSetsOutput, error) {
	id := ""
	if params.HostedZoneId != nil {
		id = *params.HostedZoneId
	}
	f.hostedZoneIDs = append(f.hostedZoneIDs, id)
	return &route53.ListResourceRecordSetsOutput{}, nil
}

// TestFetchR53Records_SkipsEmptyHostedZoneId pins the r53-records guard:
// given rows [valid "/hostedzone/Z123", empty ""], exactly one
// ListResourceRecordSets call must be made (for "/hostedzone/Z123"), never
// with an empty hosted zone ID.
func TestFetchR53Records_SkipsEmptyHostedZoneId(t *testing.T) {
	fake := &recordingR53RecordsClient{}

	hostedZoneIDs := []string{"/hostedzone/Z123", ""}

	for _, hostedZoneID := range hostedZoneIDs {
		if _, err := awsclient.FetchR53Records(context.Background(), fake, hostedZoneID, ""); err != nil {
			t.Fatalf("FetchR53Records(%q) unexpected error: %v", hostedZoneID, err)
		}
	}

	if len(fake.hostedZoneIDs) != 1 {
		t.Fatalf("ListResourceRecordSets called %d times, want exactly 1 (calls: %v)", len(fake.hostedZoneIDs), fake.hostedZoneIDs)
	}
	if fake.hostedZoneIDs[0] != "/hostedzone/Z123" {
		t.Errorf("ListResourceRecordSets called with hosted zone ID %q, want %q", fake.hostedZoneIDs[0], "/hostedzone/Z123")
	}
	for _, id := range fake.hostedZoneIDs {
		if id == "" {
			t.Error("ListResourceRecordSets was called with an empty hosted zone ID")
		}
	}
}
