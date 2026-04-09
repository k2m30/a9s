package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	secretsmanagertypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

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
