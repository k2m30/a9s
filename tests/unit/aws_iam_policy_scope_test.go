package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

type iamUserPolicyOverrideFake struct {
	*fakes.IAMFake
	attachedPolicies []iamtypes.AttachedPolicy
}

func (f *iamUserPolicyOverrideFake) ListAttachedUserPolicies(_ context.Context, input *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	if input == nil || input.UserName == nil {
		return &iam.ListAttachedUserPoliciesOutput{}, nil
	}
	return &iam.ListAttachedUserPoliciesOutput{AttachedPolicies: f.attachedPolicies}, nil
}

func TestRelated_IAMGroup_Policy_FiltersAWSManagedPolicies(t *testing.T) {
	checker := iamGroupCheckerByTarget(t, "policy")
	clients := &awsclient.ServiceClients{IAM: fakes.NewIAM()}
	source := resource.Resource{ID: "admins", Name: "admins"}

	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Fatalf("Count = %d, want 0 for AWS-managed-only attached policies", result.Count)
	}
	if len(result.ResourceIDs) != 0 {
		t.Fatalf("ResourceIDs = %v, want none", result.ResourceIDs)
	}
}

func TestRelated_IAMRole_Policy_FiltersAWSManagedPolicies(t *testing.T) {
	checker := roleCheckerByTarget(t, "policy")
	clients := &awsclient.ServiceClients{IAM: fakes.NewIAM()}
	source := resource.Resource{ID: "acme-eks-node-role", Name: "acme-eks-node-role"}

	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Fatalf("Count = %d, want 0 for AWS-managed-only attached policies", result.Count)
	}
	if len(result.ResourceIDs) != 0 {
		t.Fatalf("ResourceIDs = %v, want none", result.ResourceIDs)
	}
}

func TestRelated_IAMUser_Policy_FiltersAWSManagedPolicies(t *testing.T) {
	checker := iamUserCheckerByTarget(t, "policy")
	clients := &awsclient.ServiceClients{
		IAM: &iamUserPolicyOverrideFake{
			IAMFake: fakes.NewIAM(),
			attachedPolicies: []iamtypes.AttachedPolicy{
				{
					PolicyName: aws.String("AdministratorAccess"),
					PolicyArn:  aws.String("arn:aws:iam::aws:policy/AdministratorAccess"),
				},
			},
		},
	}
	source := resource.Resource{ID: "alice.johnson", Name: "alice.johnson"}

	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Fatalf("Count = %d, want 0 for AWS-managed-only attached policies", result.Count)
	}
	if len(result.ResourceIDs) != 0 {
		t.Fatalf("ResourceIDs = %v, want none", result.ResourceIDs)
	}
}

func TestDemoIAMFake_ListPolicies_HonorsLocalScope(t *testing.T) {
	fake := fakes.NewIAM()

	out, err := fake.ListPolicies(context.Background(), &iam.ListPoliciesInput{
		Scope: iamtypes.PolicyScopeTypeLocal,
	})
	if err != nil {
		t.Fatalf("ListPolicies returned error: %v", err)
	}

	for _, policy := range out.Policies {
		if policy.Arn == nil {
			continue
		}
		if !fixtures.IsCustomerManagedPolicyARN(*policy.Arn) {
			t.Fatalf("ListPolicies(Local) returned AWS-managed policy ARN %q", *policy.Arn)
		}
	}
}

func TestDemoExpectedTopLevelCounts_Policy_ExcludesAWSManaged(t *testing.T) {
	counts := fixtures.ExpectedTopLevelCounts()
	if got, want := counts["policy"], 22; got != want {
		t.Fatalf("policy count = %d, want %d", got, want)
	}
}
