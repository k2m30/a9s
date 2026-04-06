package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func s3CheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("s3") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("s3 related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("s3 related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_S3_None(t *testing.T) {
	navs := resource.GetNavigableFields("s3")
	if len(navs) > 0 {
		t.Errorf("expected no navigable fields for s3, got %d", len(navs))
	}
}

// --- checkS3Trail (reverse: search trail cache for S3BucketName match) ---

func TestRelated_S3_Trail_Found(t *testing.T) {
	trailRes := resource.Resource{
		ID:   "my-trail",
		Name: "my-trail",
		RawStruct: cloudtrailtypes.Trail{
			TrailARN:     aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/my-trail"),
			Name:         aws.String("my-trail"),
			S3BucketName: aws.String("cloudtrail-audit-logs"),
		},
	}
	cache := resource.ResourceCache{
		"trail": resource.ResourceCacheEntry{Resources: []resource.Resource{trailRes}},
	}
	source := resource.Resource{
		ID:   "cloudtrail-audit-logs",
		Name: "cloudtrail-audit-logs",
	}

	checker := s3CheckerByTarget(t, "trail")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-trail" {
		t.Errorf("ResourceIDs = %v, want [my-trail]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_S3_Trail_NotFound(t *testing.T) {
	trailRes := resource.Resource{
		ID:   "my-trail",
		Name: "my-trail",
		RawStruct: cloudtrailtypes.Trail{
			TrailARN:     aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/my-trail"),
			Name:         aws.String("my-trail"),
			S3BucketName: aws.String("cloudtrail-audit-logs"),
		},
	}
	cache := resource.ResourceCache{
		"trail": resource.ResourceCacheEntry{Resources: []resource.Resource{trailRes}},
	}
	source := resource.Resource{
		ID:   "other-bucket",
		Name: "other-bucket",
	}

	checker := s3CheckerByTarget(t, "trail")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_S3_Trail_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "cloudtrail-audit-logs",
		Name: "cloudtrail-audit-logs",
	}

	checker := s3CheckerByTarget(t, "trail")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_S3_Trail_EmptyID(t *testing.T) {
	trailRes := resource.Resource{
		ID:   "my-trail",
		Name: "my-trail",
		RawStruct: cloudtrailtypes.Trail{
			TrailARN:     aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/my-trail"),
			Name:         aws.String("my-trail"),
			S3BucketName: aws.String("cloudtrail-audit-logs"),
		},
	}
	cache := resource.ResourceCache{
		"trail": resource.ResourceCacheEntry{Resources: []resource.Resource{trailRes}},
	}
	source := resource.Resource{
		ID:   "",
		Name: "",
	}

	checker := s3CheckerByTarget(t, "trail")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty ID", result.Count)
	}
}

// --- checkS3CF (reverse: search cf cache for origin DomainName containing bucket name) ---

func TestRelated_S3_CF_Found(t *testing.T) {
	cfRes := resource.Resource{
		ID:   "E1A2B3C4D5E6F7",
		Name: "E1A2B3C4D5E6F7",
		RawStruct: cftypes.DistributionSummary{
			Id: aws.String("E1A2B3C4D5E6F7"),
			Origins: &cftypes.Origins{
				Quantity: aws.Int32(1),
				Items: []cftypes.Origin{
					{
						Id:         aws.String("s3-origin"),
						DomainName: aws.String("webapp-assets-prod.s3.amazonaws.com"),
					},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{cfRes}},
	}
	source := resource.Resource{
		ID:   "webapp-assets-prod",
		Name: "webapp-assets-prod",
	}

	checker := s3CheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "E1A2B3C4D5E6F7" {
		t.Errorf("ResourceIDs = %v, want [E1A2B3C4D5E6F7]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_S3_CF_NotFound(t *testing.T) {
	cfRes := resource.Resource{
		ID:   "E1A2B3C4D5E6F7",
		Name: "E1A2B3C4D5E6F7",
		RawStruct: cftypes.DistributionSummary{
			Id: aws.String("E1A2B3C4D5E6F7"),
			Origins: &cftypes.Origins{
				Quantity: aws.Int32(1),
				Items: []cftypes.Origin{
					{
						Id:         aws.String("s3-origin"),
						DomainName: aws.String("webapp-assets-prod.s3.amazonaws.com"),
					},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{cfRes}},
	}
	source := resource.Resource{
		ID:   "other-bucket",
		Name: "other-bucket",
	}

	checker := s3CheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_S3_CF_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "webapp-assets-prod",
		Name: "webapp-assets-prod",
	}

	checker := s3CheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- Lambda & CFN stub tests ---

func TestRelated_S3_Lambda_IsStub(t *testing.T) {
	defs := resource.GetRelated("s3")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for s3")
	}
	for _, def := range defs {
		if def.TargetType == "lambda" {
			if def.Checker != nil {
				t.Errorf("s3 lambda Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target lambda not found for s3")
}

func TestRelated_S3_CFN_IsStub(t *testing.T) {
	defs := resource.GetRelated("s3")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for s3")
	}
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Errorf("s3 cfn Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target cfn not found for s3")
}

// --- Demo Checker ---

func TestRelatedDemo_S3_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("s3")
	if checker == nil {
		t.Fatal("no demo checker registered for s3")
	}

	results := checker(resource.Resource{ID: "data-pipeline-logs"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"trail": false, "cf": false, "lambda": false, "cfn": false}
	for _, r := range results {
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}

	// At least one result must have Count > 0 (trail for data-pipeline-logs).
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
