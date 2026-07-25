package unit

// related_canonical_id_test.go — Tests for the canonical-target-identity
// contract (#279). ValidateRelatedResultAgainstCacheForTest cross-checks that every
// ResourceID a checker returns for a given TargetType exists as a
// Resource.ID in the target type's cache entry. This catches the class of
// checker bugs where an ARN, adjacent name, or wrong ID kind is returned
// instead of the target type's canonical Resource.ID.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

func TestValidateRelatedResultAgainstCacheForTest_HappyPath(t *testing.T) {
	r := resource.KnownRelated("ec2", []string{"i-0a1b2c", "i-0d4e5f"}, false)
	cache := resource.ResourceCache{
		"ec2": {
			Resources: []resource.Resource{
				{ID: "i-0a1b2c"},
				{ID: "i-0d4e5f"},
				{ID: "i-unused"},
			},
			IsTruncated: false,
		},
	}
	if err := resource.ValidateRelatedResultAgainstCacheForTest(r, cache); err != nil {
		t.Fatalf("expected nil error for canonical IDs, got %v", err)
	}
}

func TestValidateRelatedResultAgainstCacheForTest_WrongIDKind_Fails(t *testing.T) {
	// Checker bug: returns the EC2 instance ARN instead of the canonical
	// instance ID (i-...). The cache holds canonical instance IDs, so the ARN
	// has no match and the validator MUST catch it.
	arn := "arn:aws:ec2:us-east-1:111122223333:instance/i-0a1b2c"
	r := resource.KnownRelated("ec2", []string{arn}, false)
	cache := resource.ResourceCache{
		"ec2": {
			Resources:   []resource.Resource{{ID: "i-0a1b2c"}},
			IsTruncated: false,
		},
	}
	err := resource.ValidateRelatedResultAgainstCacheForTest(r, cache)
	if err == nil {
		t.Fatal("expected error when checker returned an ARN for a type whose canonical ID is the instance ID, got nil")
	}
	if !strings.Contains(err.Error(), arn) {
		t.Errorf("error should mention the offending ID %q, got: %v", arn, err)
	}
}

func TestValidateRelatedResultAgainstCacheForTest_TruncatedCache_Skips(t *testing.T) {
	// Cache is truncated — we cannot prove an ID is missing because the cache
	// might not have seen it. Validator must skip cross-checking in this case
	// to avoid false positives.
	r := resource.KnownRelated("ec2", []string{"i-not-in-cache"}, false)
	cache := resource.ResourceCache{
		"ec2": {
			Resources:   []resource.Resource{{ID: "i-different"}},
			IsTruncated: true, // partial cache
		},
	}
	if err := resource.ValidateRelatedResultAgainstCacheForTest(r, cache); err != nil {
		t.Fatalf("expected nil error on truncated cache (skip rule), got %v", err)
	}
}

func TestValidateRelatedResultAgainstCacheForTest_NoCacheEntry_Skips(t *testing.T) {
	// No cache entry for the target type at all. Validator cannot compare;
	// must skip rather than fail.
	r := resource.KnownRelated("ec2", []string{"i-0a1b2c"}, false)
	cache := resource.ResourceCache{}
	if err := resource.ValidateRelatedResultAgainstCacheForTest(r, cache); err != nil {
		t.Fatalf("expected nil error when no cache entry exists, got %v", err)
	}
}

func TestValidateRelatedResultAgainstCacheForTest_ShapeViolation_DelegatesToValidateRelatedResult(t *testing.T) {
	// Empty TargetType is a shape invariant caught by ValidateRelatedResult.
	// ValidateRelatedResultAgainstCacheForTest must propagate it.
	r := resource.KnownRelated("", []string{"x"}, false)
	cache := resource.ResourceCache{}
	err := resource.ValidateRelatedResultAgainstCacheForTest(r, cache)
	if err == nil {
		t.Fatal("expected error for empty TargetType (shape invariant), got nil")
	}
}

func TestValidateRelatedResultAgainstCacheForTest_ZeroIDs_NoCacheCheck(t *testing.T) {
	// Count=0 with no IDs is valid and must not require cache inspection.
	r := resource.KnownRelated("ec2", nil, false)
	cache := resource.ResourceCache{
		"ec2": {Resources: []resource.Resource{}, IsTruncated: false},
	}
	if err := resource.ValidateRelatedResultAgainstCacheForTest(r, cache); err != nil {
		t.Fatalf("expected nil error for Count=0 with no IDs, got %v", err)
	}
}
