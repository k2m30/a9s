package unit

// Regression pins for the AWS-managed IAM policy lazy-add path
// (core/aws/iam_policies.go getAWSManagedPolicyByName / managedPolicyToResource):
//   - service-linked-role policies live under arn:aws:iam::aws:policy/aws-service-role/
//     and must be probed (Codex review #5).
//   - AWS-managed policies resolved on demand must carry policy_type "aws-managed",
//     not "managed", so callers can distinguish them from customer-managed (review #6).

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/session"
)

// awsManagedPolicyFake resolves AWS-managed policies by EXACT ARN via GetPolicy and
// returns no customer-managed policies or groups, so FetchIAMPoliciesByIDsFull falls
// through to getAWSManagedPolicyByName. GetPolicy errors for any ARN not in the map,
// modelling exactly which path prefixes resolve.
type awsManagedPolicyFake struct {
	awsclient.IAMAPI
	policiesByARN map[string]iamtypes.Policy
}

var _ awsclient.IAMAPI = (*awsManagedPolicyFake)(nil)

func (f *awsManagedPolicyFake) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{Policies: nil, IsTruncated: false}, nil
}

func (f *awsManagedPolicyFake) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{}, nil
}

func (f *awsManagedPolicyFake) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	arn := ""
	if in != nil && in.PolicyArn != nil {
		arn = *in.PolicyArn
	}
	if p, ok := f.policiesByARN[arn]; ok {
		return &iam.GetPolicyOutput{Policy: &p}, nil
	}
	return nil, fmt.Errorf("NoSuchEntity: no policy at %s", arn)
}

func TestFetchIAMPoliciesByIDsFull_ResolvesServiceLinkedRolePolicyPrefix(t *testing.T) {
	const name = "AWSServiceRoleForAutoScaling"
	arn := "arn:aws:iam::aws:policy/aws-service-role/" + name
	fake := &awsManagedPolicyFake{
		// ONLY the aws-service-role path resolves; every other prefix errors.
		policiesByARN: map[string]iamtypes.Policy{
			arn: {Arn: aws.String(arn), PolicyName: aws.String(name), IsAttachable: true},
		},
	}
	store := session.NewPolicyStore()

	results, _ := awsclient.FetchIAMPoliciesByIDsFull(context.Background(), fake, []string{name}, store)

	found := false
	for _, r := range results {
		if r.ID == name || r.Name == name {
			found = true
		}
	}
	if !found {
		t.Errorf("service-linked-role AWS-managed policy %q not resolved — "+
			"the arn:aws:iam::aws:policy/aws-service-role/ prefix must be probed", name)
	}
}

func TestFetchIAMPoliciesByIDsFull_AWSManagedPolicyTypeIsAWSManaged(t *testing.T) {
	const name = "AdministratorAccess"
	arn := "arn:aws:iam::aws:policy/" + name
	fake := &awsManagedPolicyFake{
		policiesByARN: map[string]iamtypes.Policy{
			arn: {Arn: aws.String(arn), PolicyName: aws.String(name), IsAttachable: true, Path: aws.String("/")},
		},
	}
	store := session.NewPolicyStore()

	results, _ := awsclient.FetchIAMPoliciesByIDsFull(context.Background(), fake, []string{name}, store)

	var policyType string
	found := false
	for _, r := range results {
		if r.ID == name || r.Name == name {
			found = true
			policyType = r.Fields["policy_type"]
		}
	}
	if !found {
		t.Fatalf("AWS-managed policy %q not resolved", name)
	}
	if policyType != "aws-managed" {
		t.Errorf("policy_type = %q, want \"aws-managed\" (lazy-added AWS-managed policies must be "+
			"distinguishable from customer-managed)", policyType)
	}
}
