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

// These tests originally pinned the old behavior: the checker filtered
// AWS-managed ARNs out of the emitted list so Count reflected only
// customer-managed policies. That hid AWS-managed attachments from the
// operator entirely.
//
// The lazy-add path now resolves AWS-managed policy names on demand via
// FetchIAMPoliciesByIDsFull, so checkers emit every attached policy name
// (managed or AWS-managed). The drill lands on real entries because the
// orchestrator populates the cache with lazy-added AWS-managed policies.
// The new assertion is: the checker returns the full attached-count and
// every PolicyName appears in ResourceIDs.

func TestRelated_IAMGroup_Policy_EmitsAllAttachedPolicies(t *testing.T) {
	checker := iamGroupCheckerByTarget(t, "policy")
	clients := &awsclient.ServiceClients{IAM: fakes.NewIAM()}
	source := resource.Resource{ID: "admins", Name: "admins"}

	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	// Fixture: admins group has exactly one attached policy (AdministratorAccess,
	// AWS-managed) and no inline group policies.
	want := []string{"AdministratorAccess"}
	if result.Count != len(want) {
		t.Fatalf("Count = %d, want %d (attached=%v)", result.Count, len(want), want)
	}
	if !stringSetEqual(result.ResourceIDs, want) {
		t.Errorf("ResourceIDs = %v, want %v", result.ResourceIDs, want)
	}
}

func TestRelated_IAMRole_Policy_EmitsAllAttachedPolicies(t *testing.T) {
	checker := roleCheckerByTarget(t, "policy")
	clients := &awsclient.ServiceClients{IAM: fakes.NewIAM()}
	source := resource.Resource{ID: "acme-eks-node-role", Name: "acme-eks-node-role"}

	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	// Fixture: acme-eks-node-role has two AWS-managed attached policies
	// (AmazonEKSWorkerNodePolicy, AmazonEC2ContainerRegistryReadOnly).
	// Inline role policies (ListRolePolicies) are not surfaced by checkRolePolicy.
	want := []string{"AmazonEKSWorkerNodePolicy", "AmazonEC2ContainerRegistryReadOnly"}
	if result.Count != len(want) {
		t.Fatalf("Count = %d, want %d (attached=%v)", result.Count, len(want), want)
	}
	if !stringSetEqual(result.ResourceIDs, want) {
		t.Errorf("ResourceIDs = %v, want %v (order-insensitive)", result.ResourceIDs, want)
	}
}

// stringSetEqual reports whether a and b contain the same strings regardless
// of order or duplication. Used for attached-policy list comparisons where
// the AWS API order is not stable.
func stringSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}

func TestRelated_IAMUser_Policy_EmitsAWSManagedAttachedPolicy(t *testing.T) {
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

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (AdministratorAccess attached, lazy-add resolves AWS-managed at drill time)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "AdministratorAccess" {
		t.Errorf("ResourceIDs = %v, want [AdministratorAccess]", result.ResourceIDs)
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
	// Oracle mirrors the policy fetcher: customer-managed policies plus every
	// inline group policy surfaced by ListGroupPolicies. AWS-managed policies
	// are excluded by the Scope=Local filter in the fetcher and by
	// IsCustomerManagedPolicyARN in countTopLevelIAMPolicies.
	counts := fixtures.ExpectedTopLevelCounts()
	if got, want := counts["policy"], 27; got != want {
		t.Fatalf("policy count = %d, want %d", got, want)
	}
}
