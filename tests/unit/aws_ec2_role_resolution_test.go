// EKS/ASG instance profiles often carry a name different from their role, so
// checkEC2Role resolves Roles[].RoleName through iam:GetInstanceProfile unless
// cache["role"] already holds the profile-derived name.
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

func TestEC2Role_FastPath_CacheHit_ZeroAPICalls(t *testing.T) {
	res := ec2InstanceWithProfileARN("arn:aws:iam::123456789012:instance-profile/my-shared-name")

	fake := &recordingRoleIAM{}
	fake.getInstanceProfileFn = func(_ *iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
		t.Fatalf("GetInstanceProfile must not be called when cache[\"role\"] already has a matching entry")
		return nil, nil
	}
	clients := &awsclient.ServiceClients{IAM: fake}

	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "my-shared-name", Name: "my-shared-name"},
			},
		},
	}

	checker := ec2RoleCheckerByTarget(t)
	result := checker(context.Background(), clients, res, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (Err=%v)", result.Count(), result.Err())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-shared-name" {
		t.Fatalf("ResourceIDs = %v, want [my-shared-name]", result.ResourceIDs())
	}
	if fake.calls != 0 {
		t.Fatalf("GetInstanceProfile calls = %d, want 0 (fast path via cache)", fake.calls)
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
