package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func secretsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("secrets") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("secrets related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("secrets related checker for %s not found", target)
	return nil
}

// secretsSource returns a canonical source resource for Secrets Manager tests.
func secretsSource() resource.Resource {
	return resource.Resource{
		ID: "prod/docdb/acme-docdb-prod",
		RawStruct: smtypes.SecretListEntry{
			Name:              aws.String("prod/docdb/acme-docdb-prod"),
			KmsKeyId:          aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			RotationLambdaARN: aws.String("arn:aws:lambda:us-east-1:123456789012:function:rotate-docdb-credentials"),
		},
	}
}

// --- Navigable Fields ---

func TestNavigableFields_Secrets_KmsKey(t *testing.T) {
	nav := resource.IsFieldNavigable("secrets", "KmsKeyId")
	if nav == nil {
		t.Fatal("expected KmsKeyId to be navigable for secrets")
	}
	if nav.TargetType != "kms" {
		t.Errorf("expected TargetType=kms, got %q", nav.TargetType)
	}
}

func TestNavigableFields_Secrets_RotationLambda(t *testing.T) {
	nav := resource.IsFieldNavigable("secrets", "RotationLambdaARN")
	if nav == nil {
		t.Fatal("expected RotationLambdaARN to be navigable for secrets")
	}
	if nav.TargetType != "lambda" {
		t.Errorf("expected TargetType=lambda, got %q", nav.TargetType)
	}
}

// --- KMS checker (forward: KmsKeyId ARN → kms cache by UUID) ---

func TestRelated_Secrets_KMS_Found(t *testing.T) {
	kmsRes := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "alias/acme-prod-key",
	}
	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{Resources: []resource.Resource{kmsRes}},
	}

	checker := secretsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, secretsSource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "a1b2c3d4-5678-90ab-cdef-111111111111" {
		t.Errorf("ResourceIDs = %v, want [a1b2c3d4-5678-90ab-cdef-111111111111]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Secrets_KMS_NotFound(t *testing.T) {
	kmsRes := resource.Resource{
		ID:   "ffffffff-ffff-ffff-ffff-ffffffffffff",
		Name: "alias/other-key",
	}
	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{Resources: []resource.Resource{kmsRes}},
	}

	checker := secretsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, secretsSource(), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Secrets_KMS_CacheMissNoClients(t *testing.T) {
	checker := secretsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, secretsSource(), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_Secrets_KMS_NoKmsKey(t *testing.T) {
	kmsRes := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "alias/acme-prod-key",
	}
	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{Resources: []resource.Resource{kmsRes}},
	}
	source := resource.Resource{
		ID: "prod/api/stripe-key",
		RawStruct: smtypes.SecretListEntry{
			Name:     aws.String("prod/api/stripe-key"),
			KmsKeyId: nil,
		},
	}

	checker := secretsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for nil KmsKeyId", result.Count)
	}
}

// --- Lambda checker (forward: RotationLambdaARN → lambda cache by function name) ---

func TestRelated_Secrets_Lambda_Found(t *testing.T) {
	// Lambda cache ID is the function name (last segment of ARN).
	lambdaRes := resource.Resource{
		ID:   "rotate-docdb-credentials",
		Name: "rotate-docdb-credentials",
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}

	checker := secretsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, secretsSource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "rotate-docdb-credentials" {
		t.Errorf("ResourceIDs = %v, want [rotate-docdb-credentials]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Secrets_Lambda_NotFound(t *testing.T) {
	// rotate-db-credentials is a different function — should not match rotate-docdb-credentials.
	lambdaRes := resource.Resource{
		ID:   "rotate-db-credentials",
		Name: "rotate-db-credentials",
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}

	checker := secretsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, secretsSource(), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Secrets_Lambda_CacheMissNoClients(t *testing.T) {
	checker := secretsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, secretsSource(), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_Secrets_Lambda_NoRotation(t *testing.T) {
	lambdaRes := resource.Resource{
		ID:   "rotate-docdb-credentials",
		Name: "rotate-docdb-credentials",
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID: "prod/api/stripe-key",
		RawStruct: smtypes.SecretListEntry{
			Name:              aws.String("prod/api/stripe-key"),
			RotationLambdaARN: nil,
		},
	}

	checker := secretsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for nil RotationLambdaARN", result.Count)
	}
}

// --- DBI checker: undeterminable from cache, returns Count: 0 ---

func TestRelated_Secrets_DBI_ReturnsZero(t *testing.T) {
	source := secretsSource()
	checker := secretsCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "dbi" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "dbi")
	}
}

// --- CFN checker (tag-based: aws:cloudformation:stack-name → cfn cache) ---

func TestRelated_Secrets_CFN_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/myapp/db-password",
		Name: "prod/myapp/db-password",
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("prod/myapp/db-password"),
			Tags: []smtypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("prod-stack")},
			},
		},
	}
	cfnRes := resource.Resource{
		ID:     "prod-stack",
		Name:   "prod-stack",
		Fields: map[string]string{"stack_name": "prod-stack"},
	}
	otherCfn := resource.Resource{
		ID:     "dev-stack",
		Name:   "dev-stack",
		Fields: map[string]string{"stack_name": "dev-stack"},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes, otherCfn}},
	}

	checker := secretsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "prod-stack" {
		t.Errorf("ResourceIDs = %v, want [prod-stack]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Secrets_CFN_NotFound(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/myapp/db-password",
		Name: "prod/myapp/db-password",
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("prod/myapp/db-password"),
			Tags: []smtypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("prod-stack")},
			},
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "other-stack", Name: "other-stack", Fields: map[string]string{"stack_name": "other-stack"}},
		}},
	}

	checker := secretsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Secrets_CFN_NoTag(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/myapp/db-password",
		Name: "prod/myapp/db-password",
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("prod/myapp/db-password"),
			Tags: []smtypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("some-tag")},
			},
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "any-stack", Name: "any-stack"},
		}},
	}

	checker := secretsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no cfn tag)", result.Count)
	}
}

func TestRelated_Secrets_CFN_CacheMiss(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/myapp/db-password",
		Name: "prod/myapp/db-password",
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("prod/myapp/db-password"),
			Tags: []smtypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("prod-stack")},
			},
		},
	}

	checker := secretsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}
