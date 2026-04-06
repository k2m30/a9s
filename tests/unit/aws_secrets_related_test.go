package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
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

// --- DBI checker (stub — Checker==nil) ---

func TestRelated_Secrets_DBI_IsStub(t *testing.T) {
	defs := resource.GetRelated("secrets")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for secrets")
	}
	for _, def := range defs {
		if def.TargetType == "dbi" {
			if def.Checker != nil {
				t.Errorf("secrets dbi Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target dbi not found for secrets")
}

// --- CFN checker (stub — Checker==nil) ---

func TestRelated_Secrets_CFN_IsStub(t *testing.T) {
	defs := resource.GetRelated("secrets")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for secrets")
	}
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Errorf("secrets cfn Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target cfn not found for secrets")
}

// --- Demo Checker ---

func TestRelatedDemo_Secrets_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("secrets")
	if checker == nil {
		t.Fatal("no demo checker registered for secrets")
	}

	results := checker(resource.Resource{ID: "prod/docdb/acme-docdb-prod"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify exactly 4 target types are present.
	if len(results) != 4 {
		t.Errorf("demo checker returned %d results, want 4", len(results))
	}

	// At least one result must have Count > 0 (kms or lambda).
	hasPositive := false
	for _, r := range results {
		if r.Count > 0 {
			hasPositive = true
			break
		}
	}
	if !hasPositive {
		t.Error("demo checker returned no result with Count > 0")
	}
}
