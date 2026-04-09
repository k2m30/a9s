package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func roleCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("role") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("role related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("role related checker for %s not found", target)
	return nil
}

// --- Lambda checker (reverse-lookup: search lambda cache, ARN last segment) ---

func TestRelated_Role_Lambda_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-lambda-execution",
		Name: "acme-lambda-execution",
		Fields: map[string]string{
			"role_name": "acme-lambda-execution",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("acme-lambda-execution"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		},
	}

	lambdaRes := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}

	checker := roleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Role_Lambda_NotFound(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-other-role",
		Name: "acme-other-role",
		Fields: map[string]string{
			"role_name": "acme-other-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("acme-other-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/acme-other-role"),
		},
	}

	lambdaRes := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}

	checker := roleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Role_Lambda_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-lambda-execution",
		Name: "acme-lambda-execution",
		Fields: map[string]string{
			"role_name": "acme-lambda-execution",
		},
	}

	checker := roleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_Role_Lambda_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
		Fields: map[string]string{
			"role_name": "",
		},
	}

	lambdaRes := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}

	checker := roleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty ID", result.Count)
	}
}

// --- Glue checker (reverse-lookup: search glue cache, direct name match) ---

func TestRelated_Role_Glue_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-glue-role",
		Name: "acme-glue-role",
		Fields: map[string]string{
			"role_name": "acme-glue-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("acme-glue-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
		},
	}

	glueRes := resource.Resource{
		ID:   "my-etl-job",
		Name: "my-etl-job",
		RawStruct: gluetypes.Job{
			Name: aws.String("my-etl-job"),
			Role: aws.String("acme-glue-role"),
		},
	}
	cache := resource.ResourceCache{
		"glue": resource.ResourceCacheEntry{Resources: []resource.Resource{glueRes}},
	}

	checker := roleCheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Role_Glue_NotFound(t *testing.T) {
	source := resource.Resource{
		ID:   "other-role",
		Name: "other-role",
		Fields: map[string]string{
			"role_name": "other-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("other-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/other-role"),
		},
	}

	glueRes := resource.Resource{
		ID:   "my-etl-job",
		Name: "my-etl-job",
		RawStruct: gluetypes.Job{
			Name: aws.String("my-etl-job"),
			Role: aws.String("acme-glue-role"),
		},
	}
	cache := resource.ResourceCache{
		"glue": resource.ResourceCacheEntry{Resources: []resource.Resource{glueRes}},
	}

	checker := roleCheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Role_Glue_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-glue-role",
		Name: "acme-glue-role",
		Fields: map[string]string{
			"role_name": "acme-glue-role",
		},
	}

	checker := roleCheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- Node Group checker (reverse-lookup: search ng cache, ARN last segment) ---

func TestRelated_Role_NG_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "eks-node-role",
		Name: "eks-node-role",
		Fields: map[string]string{
			"role_name": "eks-node-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("eks-node-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
		},
	}

	ngRes := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	checker := roleCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Role_NG_NotFound(t *testing.T) {
	source := resource.Resource{
		ID:   "other-role",
		Name: "other-role",
		Fields: map[string]string{
			"role_name": "other-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("other-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/other-role"),
		},
	}

	ngRes := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	checker := roleCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Role_NG_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "eks-node-role",
		Name: "eks-node-role",
		Fields: map[string]string{
			"role_name": "eks-node-role",
		},
	}

	checker := roleCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- Policy checker (IAM API: ListAttachedRolePolicies) ---

func TestRelated_Role_Policy_NonNil(t *testing.T) {
	checker := roleCheckerByTarget(t, "policy")
	// checkerByTarget fatals if checker is nil — reaching here means it's non-nil.
	_ = checker
}

func TestRelated_Role_Policy_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "my-role",
		Name: "my-role",
		Fields: map[string]string{
			"role_name": "my-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("my-role"),
			Arn:      aws.String("arn:aws:iam::111122223333:role/my-role"),
		},
	}

	checker := roleCheckerByTarget(t, "policy")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "policy" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "policy")
	}
}

func TestRelated_Role_Policy_EmptyRoleName(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}

	checker := roleCheckerByTarget(t, "policy")
	// With nil clients it must return -1, not panic.
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- EC2 checker (Pattern C: scan EC2 cache for matching IamInstanceProfile ARN) ---

// TestRelated_Role_EC2_Found verifies that an EC2 instance whose
// IamInstanceProfile ARN contains "/"+roleName is counted.
// Uses constructed test data — not demo fixture alignment — because the
// demo EC2 fixture profile ARN ("acme-ec2-instance-profile") does not match
// any IAM role name in the IAM fixtures.
func TestRelated_Role_EC2_Found(t *testing.T) {
	const roleName = "my-app-role"
	source := resource.Resource{
		ID:   roleName,
		Name: roleName,
		RawStruct: iamtypes.Role{
			RoleName: aws.String(roleName),
			Arn:      aws.String("arn:aws:iam::123456789012:role/" + roleName),
		},
	}

	ec2Res := resource.Resource{
		ID:   "i-0abc1234def56789a",
		Name: "i-0abc1234def56789a",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc1234def56789a"),
			IamInstanceProfile: &ec2types.IamInstanceProfile{
				Arn: aws.String("arn:aws:iam::123456789012:instance-profile/" + roleName),
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}

	checker := roleCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "ec2" {
		t.Errorf("TargetType = %q, want \"ec2\"", result.TargetType)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Role_EC2_NoMatch verifies that EC2 instances whose profile ARN
// does not contain the role name produce count=0.
func TestRelated_Role_EC2_NoMatch(t *testing.T) {
	const roleName = "my-app-role"
	source := resource.Resource{
		ID:   roleName,
		Name: roleName,
		RawStruct: iamtypes.Role{
			RoleName: aws.String(roleName),
			Arn:      aws.String("arn:aws:iam::123456789012:role/" + roleName),
		},
	}

	ec2Res := resource.Resource{
		ID: "i-0abc1234def56789a",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc1234def56789a"),
			IamInstanceProfile: &ec2types.IamInstanceProfile{
				Arn: aws.String("arn:aws:iam::123456789012:instance-profile/other-role"),
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}

	checker := roleCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching profile)", result.Count)
	}
}

// TestRelated_Role_EC2_EmptyRoleName verifies that an empty role name
// produces count=0 immediately without scanning the cache.
func TestRelated_Role_EC2_EmptyRoleName(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
		// RawStruct with nil RoleName
		RawStruct: iamtypes.Role{RoleName: nil},
	}

	ec2Res := resource.Resource{
		ID: "i-0abc1234def56789a",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc1234def56789a"),
			IamInstanceProfile: &ec2types.IamInstanceProfile{
				Arn: aws.String("arn:aws:iam::123456789012:instance-profile/some-role"),
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}

	checker := roleCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty role name", result.Count)
	}
}

// TestRelated_Role_EC2_CacheMiss verifies that a missing EC2 cache entry
// (nil list) produces count=-1 (unknown).
func TestRelated_Role_EC2_CacheMiss(t *testing.T) {
	source := resource.Resource{
		ID:   "my-app-role",
		Name: "my-app-role",
		RawStruct: iamtypes.Role{
			RoleName: aws.String("my-app-role"),
		},
	}

	checker := roleCheckerByTarget(t, "ec2")
	// Empty cache — no "ec2" entry at all.
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss)", result.Count)
	}
	if result.TargetType != "ec2" {
		t.Errorf("TargetType = %q, want \"ec2\"", result.TargetType)
	}
}

// TestRelated_Role_EC2_InstanceNoProfile verifies that EC2 instances with no
// IamInstanceProfile set are skipped without panicking.
func TestRelated_Role_EC2_InstanceNoProfile(t *testing.T) {
	const roleName = "my-app-role"
	source := resource.Resource{
		ID:   roleName,
		Name: roleName,
		RawStruct: iamtypes.Role{
			RoleName: aws.String(roleName),
		},
	}

	ec2Res := resource.Resource{
		ID: "i-noProfile",
		RawStruct: ec2types.Instance{
			InstanceId:         aws.String("i-noProfile"),
			IamInstanceProfile: nil, // no profile attached
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}

	checker := roleCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no profile on instance)", result.Count)
	}
}

// --- Demo checker ---
