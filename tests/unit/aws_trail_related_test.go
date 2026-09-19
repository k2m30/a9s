package unit_test

import (
	"context"
	"testing"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

func trailSrcResource() resource.Resource {
	return resource.Resource{
		ID:   "my-trail",
		Name: "my-trail",
		Fields: map[string]string{
			"trail_name": "my-trail",
			"s3_bucket":  "my-audit-bucket",
		},
		RawStruct: cloudtrailtypes.Trail{
			Name:                      new("my-trail"),
			S3BucketName:              new("my-audit-bucket"),
			CloudWatchLogsLogGroupArn: new("arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail/my-trail:*"),
			SnsTopicARN:               new("arn:aws:sns:us-east-1:123456789012:cloudtrail-notifications"),
			KmsKeyId:                  new("arn:aws:kms:us-east-1:123456789012:key/abc-123"),
		},
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Trail_Logs_Match(t *testing.T) {
	logRes := resource.Resource{
		ID:   "/aws/cloudtrail/my-trail",
		Name: "/aws/cloudtrail/my-trail",
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	checker := trailCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, trailSrcResource(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

func TestRelated_Trail_Logs_NilArn(t *testing.T) {
	res := resource.Resource{
		ID:   "no-logs-trail",
		Name: "no-logs-trail",
		Fields: map[string]string{
			"trail_name": "no-logs-trail",
		},
		RawStruct: cloudtrailtypes.Trail{
			Name:         new("no-logs-trail"),
			S3BucketName: new("some-bucket"),
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nil CloudWatchLogsLogGroupArn)", result.Count())
	}
}

func TestRelated_Trail_SNS_Match(t *testing.T) {
	topicARN := "arn:aws:sns:us-east-1:123456789012:cloudtrail-notifications"
	snsRes := resource.Resource{
		ID:   topicARN,
		Name: "cloudtrail-notifications",
		Fields: map[string]string{
			"topic_arn": topicARN,
		},
		RawStruct: snstypes.Topic{
			TopicArn: new(topicARN),
		},
	}
	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{snsRes}},
	}

	checker := trailCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, trailSrcResource(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

func TestRelated_Trail_SNS_NilArn(t *testing.T) {
	res := resource.Resource{
		ID:   "no-sns-trail",
		Name: "no-sns-trail",
		Fields: map[string]string{
			"trail_name": "no-sns-trail",
		},
		RawStruct: cloudtrailtypes.Trail{
			Name:         new("no-sns-trail"),
			S3BucketName: new("some-bucket"),
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nil SnsTopicARN)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

func TestRelated_Trail_NilClients(t *testing.T) {
	emptyCache := resource.ResourceCache{}
	res := trailSrcResource()

	targets := []string{"s3", "logs", "sns"}
	for _, target := range targets {
		checker := trailCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, emptyCache)
		if result.State() != domain.RelatedUnknown {
			t.Errorf("target=%s: Count = %d, want -1 (empty cache, nil clients)", target, result.Count())
		}
	}
}

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
