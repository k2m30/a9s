package unit

// aws_iam_group_enricher_test.go — Behavioral tests for EnrichIAMGroup.
//
// Contract assertions:
//   - GetGroup.Users=[user-1] + ListAttachedGroupPolicies=[policy-1] → 0 findings.
//   - GetGroup.Users=[] → 1 finding sev "~" (empty group).
//   - GetGroup.Users=[user-1] + ListAttachedGroupPolicies=[] + no inline → 1 finding sev "~" (no policies).
//   - clients.IAM == nil → 0 findings, no error.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// iamGroupFake implements IAMAPI for IAM group enrichment testing.
// It embeds IAMAPI and overrides GetGroup, ListAttachedGroupPolicies, ListGroupPolicies.
// All maps are keyed by GroupName.
type iamGroupFake struct {
	awsclient.IAMAPI

	// usersByGroup maps GroupName → slice of User returned by GetGroup.
	usersByGroup map[string][]iamtypes.User

	// attachedPoliciesByGroup maps GroupName → slice of AttachedPolicy.
	attachedPoliciesByGroup map[string][]iamtypes.AttachedPolicy

	// inlinePoliciesByGroup maps GroupName → slice of inline policy names.
	inlinePoliciesByGroup map[string][]string
}

func (f *iamGroupFake) GetGroup(
	_ context.Context,
	in *iam.GetGroupInput,
	_ ...func(*iam.Options),
) (*iam.GetGroupOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	users := f.usersByGroup[name]
	return &iam.GetGroupOutput{
		Group: &iamtypes.Group{GroupName: aws.String(name)},
		Users: users,
	}, nil
}

func (f *iamGroupFake) ListAttachedGroupPolicies(
	_ context.Context,
	in *iam.ListAttachedGroupPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListAttachedGroupPoliciesOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	policies := f.attachedPoliciesByGroup[name]
	return &iam.ListAttachedGroupPoliciesOutput{AttachedPolicies: policies}, nil
}

func (f *iamGroupFake) ListGroupPolicies(
	_ context.Context,
	in *iam.ListGroupPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListGroupPoliciesOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	names := f.inlinePoliciesByGroup[name]
	return &iam.ListGroupPoliciesOutput{PolicyNames: names}, nil
}

// Compile-time check: iamGroupFake satisfies IAMAPI.
var _ awsclient.IAMAPI = (*iamGroupFake)(nil)

// iamGroupResources returns a slice of iam-group Resource stubs.
func iamGroupResources(names ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(names))
	for _, name := range names {
		res = append(res, resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"group_name": name,
				"group_id":   "AGPA" + name,
				"path":       "/",
				"arn":        "arn:aws:iam::123456789012:group/" + name,
			},
		})
	}
	return res
}

// iamGroupUser builds a minimal User for GetGroup response.
func iamGroupUser(name string) iamtypes.User {
	return iamtypes.User{UserName: aws.String(name)}
}

// iamAttachedPolicy builds a minimal AttachedPolicy.
func iamAttachedPolicy(arn, name string) iamtypes.AttachedPolicy {
	return iamtypes.AttachedPolicy{
		PolicyArn:  aws.String(arn),
		PolicyName: aws.String(name),
	}
}

// TestEnrichIAMGroup_PopulatedGroupProducesNoFindings verifies that a group with
// at least one member and at least one attached policy produces no findings.
func TestEnrichIAMGroup_PopulatedGroupProducesNoFindings(t *testing.T) {
	fake := &iamGroupFake{
		usersByGroup: map[string][]iamtypes.User{
			"dev-team": {iamGroupUser("alice")},
			"ops-team": {iamGroupUser("bob"), iamGroupUser("carol")},
		},
		attachedPoliciesByGroup: map[string][]iamtypes.AttachedPolicy{
			"dev-team": {iamAttachedPolicy("arn:aws:iam::aws:policy/ReadOnlyAccess", "ReadOnlyAccess")},
			"ops-team": {iamAttachedPolicy("arn:aws:iam::aws:policy/AdministratorAccess", "AdministratorAccess")},
		},
		inlinePoliciesByGroup: map[string][]string{},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources("dev-team", "ops-team")

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
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

// TestEnrichIAMGroup_NoMembersProducesFindingSevTilde verifies that a group with
// no members produces a finding with severity "~". The populated group produces
// no finding.
func TestEnrichIAMGroup_NoMembersProducesFindingSevTilde(t *testing.T) {
	fake := &iamGroupFake{
		usersByGroup: map[string][]iamtypes.User{
			"dev-team": {},                    // empty group
			"ops-team": {iamGroupUser("bob")}, // non-empty
		},
		attachedPoliciesByGroup: map[string][]iamtypes.AttachedPolicy{
			"dev-team": {iamAttachedPolicy("arn:aws:iam::aws:policy/ReadOnlyAccess", "ReadOnlyAccess")},
			"ops-team": {iamAttachedPolicy("arn:aws:iam::aws:policy/ReadOnlyAccess", "ReadOnlyAccess")},
		},
		inlinePoliciesByGroup: map[string][]string{},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources("dev-team", "ops-team")

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["dev-team"]
	if !ok {
		t.Fatalf("expected finding keyed by %q (empty group)", "dev-team")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings["ops-team"]; ok {
		t.Error("ops-team must NOT appear in Findings — it has members")
	}
	// "~" findings do not contribute to IssueCount.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichIAMGroup_NoPoliciesProducesFindingSevTilde verifies that a group with
// members but no attached or inline policies produces a finding with severity "~".
func TestEnrichIAMGroup_NoPoliciesProducesFindingSevTilde(t *testing.T) {
	fake := &iamGroupFake{
		usersByGroup: map[string][]iamtypes.User{
			"dev-team": {iamGroupUser("alice")},
			"ops-team": {iamGroupUser("bob")},
		},
		attachedPoliciesByGroup: map[string][]iamtypes.AttachedPolicy{
			"dev-team": {}, // no attached policies
			"ops-team": {iamAttachedPolicy("arn:aws:iam::aws:policy/ReadOnlyAccess", "ReadOnlyAccess")},
		},
		inlinePoliciesByGroup: map[string][]string{
			"dev-team": {}, // no inline policies
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources("dev-team", "ops-team")

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["dev-team"]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no policies)", "dev-team")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings["ops-team"]; ok {
		t.Error("ops-team must NOT appear in Findings — it has a policy")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichIAMGroup_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.IAM is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichIAMGroup_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{IAM: nil}

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, iamGroupResources("dev-team", "ops-team"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when IAM client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}
