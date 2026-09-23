// EKS/ASG instance profiles often carry a name different from their role, so
// checkEC2Role resolves Roles[].RoleName through iam:GetInstanceProfile.
package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func ec2RoleCheckerByTarget(t *testing.T) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType == "role" {
			if def.Checker == nil {
				t.Fatal("ec2->role checker is nil")
			}
			return def.Checker
		}
	}
	t.Fatal("ec2->role related def not found")
	return nil
}

type recordingRoleIAM struct {
	fakeIAMForASG
	calls int
}

func (f *recordingRoleIAM) GetInstanceProfile(ctx context.Context, input *iam.GetInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	f.calls++
	return f.fakeIAMForASG.GetInstanceProfile(ctx, input, optFns...)
}

func ec2InstanceWithProfileARN(arn string) resource.Resource {
	inst := ec2types.Instance{
		InstanceId: aws.String("i-0123456789abcdef0"),
		IamInstanceProfile: &ec2types.IamInstanceProfile{
			Arn: aws.String(arn),
		},
	}
	return resource.Resource{ID: "i-0123456789abcdef0", Name: "i-0123456789abcdef0", RawStruct: inst}
}

func TestEC2Role_ResolvesRealRoleName_ViaGetInstanceProfile(t *testing.T) {
	res := ec2InstanceWithProfileARN("arn:aws:iam::123456789012:instance-profile/eks-test-dev_1234567890123456789")

	fake := &recordingRoleIAM{}
	fake.getInstanceProfileFn = func(_ *iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
		return &iam.GetInstanceProfileOutput{
			InstanceProfile: &iamtypes.InstanceProfile{
				Roles: []iamtypes.Role{
					{RoleName: aws.String("eks-test-node-role")},
				},
			},
		}, nil
	}
	clients := &awsclient.ServiceClients{IAM: fake}

	checker := ec2RoleCheckerByTarget(t)
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (Err=%v)", result.Count(), result.Err())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "eks-test-node-role" {
		t.Fatalf("ResourceIDs = %v, want [eks-test-node-role]", result.ResourceIDs())
	}
	if fake.calls != 1 {
		t.Fatalf("GetInstanceProfile calls = %d, want exactly 1", fake.calls)
	}
}

// An instance profile's name says nothing about the role it holds: a profile
// "my-shared-name" may carry "my-shared-name-v2" while a role named like the
// profile also exists. Only the profile's own role list answers.
func TestEC2Role_ProfileNamedLikeAnotherRoleResolvesToTheRoleItHolds(t *testing.T) {
	res := ec2InstanceWithProfileARN("arn:aws:iam::123456789012:instance-profile/my-shared-name")

	fake := &recordingRoleIAM{}
	fake.getInstanceProfileFn = func(in *iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
		if aws.ToString(in.InstanceProfileName) != "my-shared-name" {
			return nil, errors.New("NoSuchEntity: Instance Profile " + aws.ToString(in.InstanceProfileName) + " cannot be found.")
		}
		return &iam.GetInstanceProfileOutput{
			InstanceProfile: &iamtypes.InstanceProfile{
				InstanceProfileName: aws.String("my-shared-name"),
				Roles:               []iamtypes.Role{{RoleName: aws.String("my-shared-name-v2")}},
			},
		}, nil
	}
	clients := &awsclient.ServiceClients{IAM: fake}

	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "my-shared-name", Name: "my-shared-name"},
				{ID: "my-shared-name-v2", Name: "my-shared-name-v2"},
			},
		},
	}

	result := ec2RoleCheckerByTarget(t)(context.Background(), clients, res, cache)

	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != "my-shared-name-v2" {
		t.Fatalf("ResourceIDs = %v (Err=%v), want [my-shared-name-v2], the role the profile holds", ids, result.Err())
	}
	if result.Truncated() {
		t.Error("the profile's role list was read whole; the count is exact, not a lower bound")
	}
}

func TestEC2Role_GetInstanceProfileError_ReturnsNegativeOneWithErr(t *testing.T) {
	res := ec2InstanceWithProfileARN("arn:aws:iam::123456789012:instance-profile/eks-test-dev_1234567890123456789")

	fake := &recordingRoleIAM{}
	fake.getInstanceProfileFn = func(_ *iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
		return nil, errors.New("AccessDenied: not authorized to perform iam:GetInstanceProfile")
	}
	clients := &awsclient.ServiceClients{IAM: fake}

	checker := ec2RoleCheckerByTarget(t)
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Fatalf("State = %v, want RelatedError", result.State())
	}
	if result.Err() == nil {
		t.Fatal("Err = nil, want non-nil API error")
	}
}

func TestEC2Role_ZeroRolesOnProfile_ReturnsZero(t *testing.T) {
	res := ec2InstanceWithProfileARN("arn:aws:iam::123456789012:instance-profile/eks-test-dev_1234567890123456789")

	fake := &recordingRoleIAM{}
	fake.getInstanceProfileFn = func(_ *iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
		return &iam.GetInstanceProfileOutput{
			InstanceProfile: &iamtypes.InstanceProfile{
				Roles: nil,
			},
		}, nil
	}
	clients := &awsclient.ServiceClients{IAM: fake}

	checker := ec2RoleCheckerByTarget(t)
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Fatalf("Count = %d, want 0 (Err=%v, IDs=%v)", result.Count(), result.Err(), result.ResourceIDs())
	}
}
