package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	kmssvc "github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	secretsmanagertypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func kmsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("kms") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("kms related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("kms related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_KMS_None(t *testing.T) {
	nav := resource.IsFieldNavigable("kms", "KeyId")
	if nav != nil {
		t.Errorf("expected no navigable fields for kms, but KeyId resolved to %v", nav)
	}
}

// --- EBS checker (Pattern C — cache, KmsKeyId ARN) ---

func TestRelated_KMS_EBS_Found(t *testing.T) {
	const keyID = "a1b2c3d4-5678-90ab-cdef-111111111111"
	arn := "arn:aws:kms:us-east-1:123456789012:key/" + keyID

	ebsRes := resource.Resource{
		ID:     "vol-0abc1234",
		Fields: map[string]string{},
		RawStruct: ec2types.Volume{
			VolumeId: aws.String("vol-0abc1234"),
			KmsKeyId: aws.String(arn),
		},
	}
	cache := resource.ResourceCache{
		"ebs": resource.ResourceCacheEntry{Resources: []resource.Resource{ebsRes}},
	}
	source := resource.Resource{
		ID:   keyID,
		Name: "alias/my-key",
		Fields: map[string]string{
			"key_id": keyID,
		},
	}

	checker := kmsCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vol-0abc1234" {
		t.Errorf("ResourceIDs = %v, want [vol-0abc1234]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_KMS_EBS_NotFound(t *testing.T) {
	const keyID = "a1b2c3d4-5678-90ab-cdef-111111111111"

	ebsRes := resource.Resource{
		ID:     "vol-0abc1234",
		Fields: map[string]string{},
		RawStruct: ec2types.Volume{
			VolumeId: aws.String("vol-0abc1234"),
			KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/different-key-id"),
		},
	}
	cache := resource.ResourceCache{
		"ebs": resource.ResourceCacheEntry{Resources: []resource.Resource{ebsRes}},
	}
	source := resource.Resource{
		ID:   keyID,
		Name: "alias/my-key",
		Fields: map[string]string{
			"key_id": keyID,
		},
	}

	checker := kmsCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_KMS_EBS_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "alias/my-key",
		Fields: map[string]string{
			"key_id": "a1b2c3d4-5678-90ab-cdef-111111111111",
		},
	}

	checker := kmsCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- RDS checker (Pattern C — cache, KmsKeyId ARN) ---

func TestRelated_KMS_RDS_Found(t *testing.T) {
	const keyID = "b2c3d4e5-6789-01bc-defg-222222222222"
	arn := "arn:aws:kms:us-east-1:123456789012:key/" + keyID

	rdsRes := resource.Resource{
		ID:     "mydb",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("mydb"),
			KmsKeyId:             aws.String(arn),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{rdsRes}},
	}
	source := resource.Resource{
		ID:   keyID,
		Name: "alias/rds-key",
		Fields: map[string]string{
			"key_id": keyID,
		},
	}

	checker := kmsCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "mydb" {
		t.Errorf("ResourceIDs = %v, want [mydb]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_KMS_RDS_NotFound(t *testing.T) {
	const keyID = "b2c3d4e5-6789-01bc-defg-222222222222"

	rdsRes := resource.Resource{
		ID:     "mydb",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("mydb"),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/other-key-id"),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{rdsRes}},
	}
	source := resource.Resource{
		ID:   keyID,
		Name: "alias/rds-key",
		Fields: map[string]string{
			"key_id": keyID,
		},
	}

	checker := kmsCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_KMS_RDS_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "b2c3d4e5-6789-01bc-defg-222222222222",
		Name: "alias/rds-key",
		Fields: map[string]string{
			"key_id": "b2c3d4e5-6789-01bc-defg-222222222222",
		},
	}

	checker := kmsCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- Secrets checker (Pattern C — cache, KmsKeyId ARN) ---

func TestRelated_KMS_Secrets_Found(t *testing.T) {
	const keyID = "c3d4e5f6-7890-12cd-efgh-333333333333"
	arn := "arn:aws:kms:us-east-1:123456789012:key/" + keyID

	secretRes := resource.Resource{
		ID:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf",
		Name:   "my-secret",
		Fields: map[string]string{},
		RawStruct: secretsmanagertypes.SecretListEntry{
			Name:     aws.String("my-secret"),
			KmsKeyId: aws.String(arn),
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}
	source := resource.Resource{
		ID:   keyID,
		Name: "alias/secrets-key",
		Fields: map[string]string{
			"key_id": keyID,
		},
	}

	checker := kmsCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf" {
		t.Errorf("ResourceIDs = %v, want [arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_KMS_Secrets_NotFound(t *testing.T) {
	const keyID = "c3d4e5f6-7890-12cd-efgh-333333333333"

	secretRes := resource.Resource{
		ID:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf",
		Name:   "my-secret",
		Fields: map[string]string{},
		RawStruct: secretsmanagertypes.SecretListEntry{
			Name:     aws.String("my-secret"),
			KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/different-key"),
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}
	source := resource.Resource{
		ID:   keyID,
		Name: "alias/secrets-key",
		Fields: map[string]string{
			"key_id": keyID,
		},
	}

	checker := kmsCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_KMS_Secrets_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "c3d4e5f6-7890-12cd-efgh-333333333333",
		Name: "alias/secrets-key",
		Fields: map[string]string{
			"key_id": "c3d4e5f6-7890-12cd-efgh-333333333333",
		},
	}

	checker := kmsCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkKMSRole — Pattern C: GetKeyPolicy + ListGrants
// ---------------------------------------------------------------------------

// TestRelated_KMS_Role_Match verifies that grants with role ARNs in
// GranteePrincipal are returned as resource IDs.
func TestRelated_KMS_Role_Match(t *testing.T) {
	const keyID = "a1b2c3d4-0001-0001-0001-000000000001"
	roleARN1 := "arn:aws:iam::123456789012:role/my-ec2-role"
	roleARN2 := "arn:aws:iam::123456789012:role/my-lambda-role"

	src := resource.Resource{
		ID:   keyID,
		Name: "alias/my-key",
	}
	clients := &awsclient.ServiceClients{
		KMS: newFakeKMSWithGrants([]kmstypes.GrantListEntry{
			{GranteePrincipal: &roleARN1},
			{GranteePrincipal: &roleARN2},
		}),
	}
	checker := kmsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	if !seen["my-ec2-role"] {
		t.Errorf("ResourceIDs missing my-ec2-role; got %v", result.ResourceIDs)
	}
	if !seen["my-lambda-role"] {
		t.Errorf("ResourceIDs missing my-lambda-role; got %v", result.ResourceIDs)
	}
}

// TestRelated_KMS_Role_Empty verifies that a key with no grants and no
// policy roles returns Count=0.
func TestRelated_KMS_Role_Empty(t *testing.T) {
	const keyID = "a1b2c3d4-0001-0001-0001-000000000002"

	src := resource.Resource{
		ID:   keyID,
		Name: "alias/no-roles-key",
	}
	clients := &awsclient.ServiceClients{
		KMS: newFakeKMSWithGrants([]kmstypes.GrantListEntry{}),
	}
	checker := kmsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no role grants)", result.Count)
	}
}

// TestRelated_KMS_Role_WrongRawStruct verifies that a key resource with an
// empty ID returns Count=0 (short-circuit before API call).
func TestRelated_KMS_Role_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "",
		RawStruct: "not-a-kms-key",
	}
	checker := kmsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty key ID short-circuits)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// Fix 1: checkKMSRole error propagation — errors must return Count=-1
// ---------------------------------------------------------------------------

// TestRelated_KMS_Role_AccessDenied_ReturnsMinusOne verifies that when
// GetKeyPolicy returns AccessDeniedException the checker returns Count=-1 and
// a non-nil Err, not silently swallowing the error.
func TestRelated_KMS_Role_AccessDenied_ReturnsMinusOne(t *testing.T) {
	const keyID = "a1b2c3d4-0001-0001-0001-000000000099"

	src := resource.Resource{
		ID:   keyID,
		Name: "alias/denied-key",
	}
	// GetKeyPolicy returns AccessDenied; ListGrants returns empty (never reached).
	clients := &awsclient.ServiceClients{
		KMS: &fakeKMSUS1{
			getKeyPolicyErr: newAccessDeniedError(),
		},
	}
	checker := kmsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (GetKeyPolicy AccessDenied must propagate as -1)", result.Count)
	}
	if result.Err == nil {
		t.Error("Err = nil, want non-nil (AccessDenied error must not be swallowed)")
	}
}

// TestRelated_KMS_Role_ListGrantsAccessDenied_ReturnsMinusOne verifies that
// when GetKeyPolicy succeeds with no role principals but ListGrants returns
// AccessDeniedException, the checker returns Count=-1 and a non-nil Err.
func TestRelated_KMS_Role_ListGrantsAccessDenied_ReturnsMinusOne(t *testing.T) {
	const keyID = "a1b2c3d4-0001-0001-0001-000000000100"

	src := resource.Resource{
		ID:   keyID,
		Name: "alias/list-grants-denied-key",
	}
	// GetKeyPolicy returns an empty (non-nil) output with no policy JSON
	// so the policy parse finds no role principals.
	// ListGrants then returns AccessDenied.
	clients := &awsclient.ServiceClients{
		KMS: &fakeKMSUS1{
			getKeyPolicyOut: &kmssvc.GetKeyPolicyOutput{},
			listGrantsErr:   newAccessDeniedError(),
		},
	}
	checker := kmsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (ListGrants AccessDenied must propagate as -1)", result.Count)
	}
	if result.Err == nil {
		t.Error("Err = nil, want non-nil (ListGrants AccessDenied error must not be swallowed)")
	}
}
