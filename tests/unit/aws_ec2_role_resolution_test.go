// ---------------------------------------------------------------------------
// checkEC2Role must resolve the ACTUAL IAM role name behind an auto-generated
// instance profile (EKS/ASG profiles commonly have profile name != role
// name), not just the profile-ARN's last path segment. Contract:
//   - cache["role"] fast path: if a role in cache matches the profile-derived
//     name, return it with zero API calls.
//   - otherwise one iam:GetInstanceProfile call resolving Roles[].RoleName.
//   - API error -> Count:-1, Err set, no panic.
//   - zero roles on the profile -> Count:0.
// ---------------------------------------------------------------------------
package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ec2RoleCheckerByTarget returns the ec2->role RelatedChecker via the
// registry, mirroring checkerByTarget (aws_iam_policies_related_test.go)
// which lives in this same unit_test package.
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

// recordingRoleIAM implements awsclient.IAMAPI's superset by embedding
// fakeIAMBatch2 (defined in fakes_us1_batch2_test.go, same package) and adds
// a call counter for GetInstanceProfile so tests can assert zero-call fast
// paths and exactly-one-call resolution paths.
type recordingRoleIAM struct {
	fakeIAMBatch2
	calls int
}

func (f *recordingRoleIAM) GetInstanceProfile(ctx context.Context, input *iam.GetInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	f.calls++
	return f.fakeIAMBatch2.GetInstanceProfile(ctx, input, optFns...)
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

// 1. Auto-generated EKS/ASG profile name != role name: resolves the real
// role name via exactly one iam:GetInstanceProfile call.
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

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (Err=%v)", result.Count, result.Err)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "eks-test-node-role" {
		t.Fatalf("ResourceIDs = %v, want [eks-test-node-role]", result.ResourceIDs)
	}
	if fake.calls != 1 {
		t.Fatalf("GetInstanceProfile calls = %d, want exactly 1", fake.calls)
	}
}

// 2. Fast path: cache["role"] already contains a role whose ID equals the
// profile-derived name -> zero GetInstanceProfile calls.
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

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (Err=%v)", result.Count, result.Err)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-shared-name" {
		t.Fatalf("ResourceIDs = %v, want [my-shared-name]", result.ResourceIDs)
	}
	if fake.calls != 0 {
		t.Fatalf("GetInstanceProfile calls = %d, want 0 (fast path via cache)", fake.calls)
	}
}

// 3. iam:GetInstanceProfile API error -> Count:-1 with Err set, no panic.
func TestEC2Role_GetInstanceProfileError_ReturnsNegativeOneWithErr(t *testing.T) {
	res := ec2InstanceWithProfileARN("arn:aws:iam::123456789012:instance-profile/eks-test-dev_1234567890123456789")

	fake := &recordingRoleIAM{}
	fake.getInstanceProfileFn = func(_ *iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
		return nil, errors.New("AccessDenied: not authorized to perform iam:GetInstanceProfile")
	}
	clients := &awsclient.ServiceClients{IAM: fake}

	checker := ec2RoleCheckerByTarget(t)
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.State != domain.RelatedError {
		t.Fatalf("Count = %d, want -1", result.Count)
	}
	if result.Err == nil {
		t.Fatal("Err = nil, want non-nil API error")
	}
}

// 4. Instance profile resolves but has zero roles attached -> Count:0.
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

	if result.Count != 0 {
		t.Fatalf("Count = %d, want 0 (Err=%v, IDs=%v)", result.Count, result.Err, result.ResourceIDs)
	}
}
