package unit_test

import (
	"context"
	"testing"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// trailCheckerByTarget retrieves the RelatedChecker for the given targetType
// and fails the test if the checker is nil or not found.
func trailCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("trail") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("trail related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("trail related checker for %s not found", target)
	return nil
}

// trailSrcResource returns a canonical test resource for a CloudTrail trail.
func trailSrcResource() resource.Resource {
	return resource.Resource{
		ID:   "my-trail",
		Name: "my-trail",
		Fields: map[string]string{
			"trail_name": "my-trail",
			"s3_bucket":  "my-audit-bucket",
		},
		RawStruct: cloudtrailtypes.Trail{
			Name:                      strPtr("my-trail"),
			S3BucketName:              strPtr("my-audit-bucket"),
			CloudWatchLogsLogGroupArn: strPtr("arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail/my-trail:*"),
			SnsTopicARN:               strPtr("arn:aws:sns:us-east-1:123456789012:cloudtrail-notifications"),
			KmsKeyId:                  strPtr("arn:aws:kms:us-east-1:123456789012:key/abc-123"),
		},
	}
}

// --- S3 Bucket checker tests ---

// TestRelated_Trail_S3_Match verifies that a trail whose S3BucketName matches
// a bucket in the s3 cache produces Count=1.
func TestRelated_Trail_S3_Match(t *testing.T) {
	s3Res := resource.Resource{
		ID:   "my-audit-bucket",
		Name: "my-audit-bucket",
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{s3Res}},
	}

	checker := trailCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, trailSrcResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// TestRelated_Trail_S3_NoMatch verifies that a trail whose S3BucketName does
// not match any bucket in the s3 cache produces Count=0.
func TestRelated_Trail_S3_NoMatch(t *testing.T) {
	s3Res := resource.Resource{
		ID:   "different-bucket",
		Name: "different-bucket",
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{s3Res}},
	}

	checker := trailCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, trailSrcResource(), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- Log Groups checker tests ---

// TestRelated_Trail_Logs_Match verifies that a trail with a
// CloudWatchLogsLogGroupArn whose log group name matches a logs cache entry
// produces Count=1.
func TestRelated_Trail_Logs_Match(t *testing.T) {
	// The log group name extracted from the ARN is "/aws/cloudtrail/my-trail".
	logRes := resource.Resource{
		ID:   "/aws/cloudtrail/my-trail",
		Name: "/aws/cloudtrail/my-trail",
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	checker := trailCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, trailSrcResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// TestRelated_Trail_Logs_NilArn verifies that a trail without a
// CloudWatchLogsLogGroupArn produces Count=0.
func TestRelated_Trail_Logs_NilArn(t *testing.T) {
	res := resource.Resource{
		ID:   "no-logs-trail",
		Name: "no-logs-trail",
		Fields: map[string]string{
			"trail_name": "no-logs-trail",
		},
		RawStruct: cloudtrailtypes.Trail{
			Name:         strPtr("no-logs-trail"),
			S3BucketName: strPtr("some-bucket"),
			// CloudWatchLogsLogGroupArn intentionally nil
		},
	}
	logRes := resource.Resource{
		ID:   "/aws/cloudtrail/some-trail",
		Name: "/aws/cloudtrail/some-trail",
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	checker := trailCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil CloudWatchLogsLogGroupArn)", result.Count)
	}
}

// --- SNS Topic checker tests ---

// TestRelated_Trail_SNS_Match verifies that a trail with a SnsTopicARN matching
// an SNS topic in the cache produces Count=1.
func TestRelated_Trail_SNS_Match(t *testing.T) {
	topicARN := "arn:aws:sns:us-east-1:123456789012:cloudtrail-notifications"
	snsRes := resource.Resource{
		ID:   topicARN,
		Name: "cloudtrail-notifications",
		Fields: map[string]string{
			"topic_arn": topicARN,
		},
		RawStruct: snstypes.Topic{
			TopicArn: strPtr(topicARN),
		},
	}
	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{snsRes}},
	}

	checker := trailCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, trailSrcResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// TestRelated_Trail_SNS_NilArn verifies that a trail without a SnsTopicARN
// produces Count=0.
func TestRelated_Trail_SNS_NilArn(t *testing.T) {
	res := resource.Resource{
		ID:   "no-sns-trail",
		Name: "no-sns-trail",
		Fields: map[string]string{
			"trail_name": "no-sns-trail",
		},
		RawStruct: cloudtrailtypes.Trail{
			Name:         strPtr("no-sns-trail"),
			S3BucketName: strPtr("some-bucket"),
			// SnsTopicARN intentionally nil
		},
	}
	topicARN := "arn:aws:sns:us-east-1:123456789012:cloudtrail-notifications"
	snsRes := resource.Resource{
		ID:   topicARN,
		Name: "cloudtrail-notifications",
		Fields: map[string]string{
			"topic_arn": topicARN,
		},
	}
	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{snsRes}},
	}

	checker := trailCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil SnsTopicARN)", result.Count)
	}
}

// --- KMS Key checker tests ---

// TestRelated_Trail_KMS_Match verifies that a trail with a KmsKeyId matching a
// KMS key in the cache produces Count=1.
func TestRelated_Trail_KMS_Match(t *testing.T) {
	// KMS resources use the bare UUID as their ID.
	kmsRes := resource.Resource{
		ID:   "abc-123",
		Name: "abc-123",
	}
	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{Resources: []resource.Resource{kmsRes}},
	}

	checker := trailCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, trailSrcResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- Nil clients / empty cache test ---

// TestRelated_Trail_NilClients verifies that all checkers return Count=-1 when
// the cache has no relevant entry and clients is nil (cache miss).
func TestRelated_Trail_NilClients(t *testing.T) {
	emptyCache := resource.ResourceCache{}
	res := trailSrcResource()

	targets := []string{"s3", "logs", "sns", "kms"}
	for _, target := range targets {
		checker := trailCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, emptyCache)
		if result.Count != -1 {
			t.Errorf("target=%s: Count = %d, want -1 (empty cache, nil clients)", target, result.Count)
		}
	}
}

// --- NavigableFields test ---

// TestNavigableFields_Trail verifies that the S3BucketName→s3 navigable field
// is registered for the trail resource type.
func TestNavigableFields_Trail(t *testing.T) {
	fields := resource.GetNavigableFields("trail")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for trail")
	}

	found := false
	for _, f := range fields {
		if f.FieldPath == "S3BucketName" && f.TargetType == "s3" {
			found = true
			break
		}
	}
	if !found {
		t.Error("navigable field S3BucketName→s3 not found for trail")
	}
}

// --- Demo checker test ---

// TestRelatedDemo_Trail_Registered verifies the demo checker is registered and
// returns valid results with all expected target types present.
func TestRelatedDemo_Trail_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("trail")
	if checker == nil {
		t.Fatal("no demo checker registered for trail")
	}

	// Use the known fixture ID that has S3, KMS, and CloudWatch logs configured.
	src := resource.Resource{ID: "acme-management-trail"}
	results := checker(src)
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"s3": false, "logs": false, "sns": false, "kms": false}
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

	// At least one result should have Count > 0.
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
