package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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

// --- checkS3Lambda (forward: notification_lambda ARN → function name) ---

func TestRelated_S3_Lambda_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "data-pipeline-logs",
		Name: "data-pipeline-logs",
		Fields: map[string]string{
			"notification_lambda": "arn:aws:lambda:us-east-1:123456789012:function:process-pipeline-events",
		},
	}
	checker := s3CheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "process-pipeline-events" {
		t.Errorf("ResourceIDs = %v, want [process-pipeline-events]", result.ResourceIDs)
	}
	if result.TargetType != "lambda" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "lambda")
	}
}

func TestRelated_S3_Lambda_NoNotification(t *testing.T) {
	source := resource.Resource{
		ID:   "data-pipeline-logs",
		Name: "data-pipeline-logs",
		Fields: map[string]string{
			"notification_lambda": "",
		},
	}
	checker := s3CheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no lambda notification)", result.Count)
	}
}

// --- checkS3CFN: undeterminable without GetBucketTagging, returns Count: -1 ---

func TestRelated_S3_CFN_Unknown(t *testing.T) {
	source := resource.Resource{
		ID:   "data-pipeline-logs",
		Name: "data-pipeline-logs",
	}
	checker := s3CheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (tags need GetBucketTagging enrichment)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}

// TestRelated_S3_CFN_Found verifies that a bucket with the aws:cloudformation:stack-name
// tag matching a CFN stack in cache returns Count:1.
func TestRelated_S3_CFN_Found(t *testing.T) {
	const stackName = "webapp-stack"
	const bucketName = "webapp-assets-prod"

	fakeS3 := newFakeS3CRWithTagging("aws:cloudformation:stack-name", stackName)
	clients := &awsclient.ServiceClients{S3: fakeS3}

	cfnRes := resource.Resource{
		ID:   stackName,
		Name: stackName,
		RawStruct: cfntypes.Stack{
			StackName: aws.String(stackName),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID:   bucketName,
		Name: bucketName,
	}

	checker := s3CheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != stackName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, stackName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_S3_CFN_NoTag verifies that a bucket with no aws:cloudformation:stack-name
// tag returns Count:0.
func TestRelated_S3_CFN_NoTag(t *testing.T) {
	// GetBucketTagging returns tags, but none is the CFN stack-name tag.
	fakeS3 := newFakeS3CRWithTagging("Environment", "prod", "Team", "platform")
	clients := &awsclient.ServiceClients{S3: fakeS3}

	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "some-stack", Name: "some-stack"}},
		},
	}
	source := resource.Resource{
		ID:   "untagged-bucket",
		Name: "untagged-bucket",
	}

	checker := s3CheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no CFN stack-name tag)", result.Count)
	}
}

// TestRelated_S3_CFN_EmptyBucketID verifies that an empty bucket ID returns Count:0
// without making any API calls.
func TestRelated_S3_CFN_EmptyBucketID(t *testing.T) {
	// No real clients needed — empty ID short-circuits before API call.
	source := resource.Resource{ID: "", Name: ""}
	checker := s3CheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty bucket ID)", result.Count)
	}
}
