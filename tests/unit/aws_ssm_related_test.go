package unit_test

import (
	"context"
	"testing"

	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func ssmCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ssm") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ssm related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ssm related checker for %s not found", target)
	return nil
}

// ssmSecureRes returns a canonical SecureString SSM parameter for tests.
func ssmSecureRes() resource.Resource {
	return resource.Resource{
		ID:   "/app/db-password",
		Name: "/app/db-password",
		Fields: map[string]string{
			"name": "/app/db-password",
			"type": "SecureString",
		},
		RawStruct: ssmtypes.ParameterMetadata{
			Name:  strPtr("/app/db-password"),
			Type:  ssmtypes.ParameterTypeSecureString,
			KeyId: strPtr("alias/my-key"),
		},
	}
}

// ssmKMSCache returns a KMS ResourceCache containing one key whose alias matches
// the alias referenced by ssmSecureRes. The "alias" field is set to "alias/my-key"
// so matchesKMSKeyRef can match it against the SSM KeyId "alias/my-key".
func ssmKMSCache() resource.ResourceCache {
	return resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "arn:aws:kms:us-east-1:123456789012:key/abc-123",
					Name: "my-key",
					Fields: map[string]string{
						"key_id": "abc-123",
						"arn":    "arn:aws:kms:us-east-1:123456789012:key/abc-123",
						"alias":  "alias/my-key",
					},
					RawStruct: kmstypes.KeyListEntry{
						KeyId:  strPtr("abc-123"),
						KeyArn: strPtr("arn:aws:kms:us-east-1:123456789012:key/abc-123"),
					},
				},
			},
		},
	}
}

// --- KMS checker tests ---

func TestRelated_SSM_KMS_Match(t *testing.T) {
	checker := ssmCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, ssmSecureRes(), ssmKMSCache())

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_SSM_KMS_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "arn:aws:kms:us-east-1:123456789012:key/ffffffff-0000-0000-0000-ffffffffffff",
					Name: "other-key",
					Fields: map[string]string{
						"key_id": "ffffffff-0000-0000-0000-ffffffffffff",
						"alias":  "alias/other-key",
					},
					RawStruct: kmstypes.KeyListEntry{
						KeyId:  strPtr("ffffffff-0000-0000-0000-ffffffffffff"),
						KeyArn: strPtr("arn:aws:kms:us-east-1:123456789012:key/ffffffff-0000-0000-0000-ffffffffffff"),
					},
				},
			},
		},
	}

	checker := ssmCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, ssmSecureRes(), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_SSM_KMS_NotSecureString(t *testing.T) {
	res := resource.Resource{
		ID:   "/app/config-flag",
		Name: "/app/config-flag",
		Fields: map[string]string{
			"name": "/app/config-flag",
			"type": "String",
		},
		RawStruct: ssmtypes.ParameterMetadata{
			Name:  strPtr("/app/config-flag"),
			Type:  ssmtypes.ParameterTypeString,
			KeyId: nil,
		},
	}

	checker := ssmCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, ssmKMSCache())

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (non-SecureString has no KMS key)", result.Count)
	}
}

func TestRelated_SSM_KMS_NilKeyId(t *testing.T) {
	res := resource.Resource{
		ID:   "/app/db-password",
		Name: "/app/db-password",
		Fields: map[string]string{
			"name": "/app/db-password",
			"type": "SecureString",
		},
		RawStruct: ssmtypes.ParameterMetadata{
			Name:  strPtr("/app/db-password"),
			Type:  ssmtypes.ParameterTypeSecureString,
			KeyId: nil,
		},
	}

	checker := ssmCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, ssmKMSCache())

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil KeyId)", result.Count)
	}
}

func TestRelated_SSM_NilClients(t *testing.T) {
	checker := ssmCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, ssmSecureRes(), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients, empty cache)", result.Count)
	}
}

func TestRelated_SSM_EmptyCache(t *testing.T) {
	checker := ssmCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, ssmSecureRes(), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

// --- Demo checker test ---
