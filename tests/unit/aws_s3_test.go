package unit

// aws_s3_test.go — Fetcher tests for the s3 resource type.
//
// Covered assertions:
//   - Resource.Issues is always nil/empty (spec has no Wave 1 signals — U7f).
//   - Identity fields are populated from ListBuckets output (spec §1).
//   - notification_lambda / notification_sns / notification_sqs are populated
//     by FetchS3BucketsPageWithNotifications (required by related-panel checkers).
//   - Bucket with no notification config returns empty (not absent) field keys.
//   - Empty bucket list returns zero resources without error.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ---------------------------------------------------------------------------
// U7f — Resource.Issues is always nil/empty (no Wave 1 signals for s3).
// ---------------------------------------------------------------------------

// TestS3_FetcherResourceIssues_AlwaysEmpty verifies that every bucket produced
// by FetchS3Buckets has no Wave 1 issues populated (spec §3.1 = no Wave 1
// signals for s3). This guards against accidentally wiring a classification
// that would produce Issues != nil.
func TestS3_FetcherResourceIssues_AlwaysEmpty(t *testing.T) {
	fake := fakes.NewS3()
	resources, err := awsclient.FetchS3Buckets(context.Background(), fake)
	if err != nil {
		t.Fatalf("FetchS3Buckets: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("expected at least one bucket from demo fake")
	}
	for _, r := range resources {
		if len(r.Issues) != 0 {
			t.Errorf("bucket %q: Issues = %v, want nil/empty (no Wave 1 signals in s3 spec)", r.ID, r.Issues)
		}
	}
}

// ---------------------------------------------------------------------------
// spec §1 — Identity field mapping.
// ---------------------------------------------------------------------------

// TestS3_FetcherIdentityFields_HealthyBucket verifies that the healthy-bucket
// fixture produces the expected identity fields (name, bucket_name, creation_date)
// matching spec §1. This catches mapping regressions.
func TestS3_FetcherIdentityFields_HealthyBucket(t *testing.T) {
	fake := fakes.NewS3()
	resources, err := awsclient.FetchS3Buckets(context.Background(), fake)
	if err != nil {
		t.Fatalf("FetchS3Buckets: %v", err)
	}

	var found bool
	for _, r := range resources {
		if r.ID != fixtures.HealthyBucketName {
			continue
		}
		found = true

		// ID and Name must equal the bucket name (spec §1).
		if r.ID != fixtures.HealthyBucketName {
			t.Errorf("Resource.ID = %q, want %q", r.ID, fixtures.HealthyBucketName)
		}
		if r.Name != fixtures.HealthyBucketName {
			t.Errorf("Resource.Name = %q, want %q", r.Name, fixtures.HealthyBucketName)
		}

		// Fields["name"] and Fields["bucket_name"] must match the bucket name.
		if r.Fields["name"] != fixtures.HealthyBucketName {
			t.Errorf("Fields[name] = %q, want %q", r.Fields["name"], fixtures.HealthyBucketName)
		}
		if r.Fields["bucket_name"] != fixtures.HealthyBucketName {
			t.Errorf("Fields[bucket_name] = %q, want %q", r.Fields["bucket_name"], fixtures.HealthyBucketName)
		}

		// Fields["creation_date"] must be a non-empty formatted date string.
		if r.Fields["creation_date"] == "" {
			t.Errorf("Fields[creation_date] is empty for %q; expected a formatted date", r.ID)
		}

		break
	}
	if !found {
		t.Fatalf("healthy-bucket fixture %q not found in fetcher output", fixtures.HealthyBucketName)
	}
}

// TestS3_FetcherStatus_AlwaysEmpty verifies that no bucket receives a Status
// value from the fetcher. S3 has no Wave 1 signals, so Status must always be
// empty string — enrichment may later set FieldUpdates["status"] but the
// fetcher must not pre-populate it.
func TestS3_FetcherStatus_AlwaysEmpty(t *testing.T) {
	fake := fakes.NewS3()
	resources, err := awsclient.FetchS3Buckets(context.Background(), fake)
	if err != nil {
		t.Fatalf("FetchS3Buckets: %v", err)
	}
	for _, r := range resources {
		if r.Status != "" {
			t.Errorf("bucket %q: Status = %q, want \"\" (fetcher must not set status — Wave 2 only)", r.ID, r.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// Notification fields — required for related-panel checkers.
// ---------------------------------------------------------------------------

// TestS3_FetcherWithNotifications_PopulatesLambdaField verifies that
// FetchS3BucketsPageWithNotifications populates Fields["notification_lambda"]
// with the Lambda function ARN from the healthy bucket's notification config.
// The field is consumed by checkS3Lambda to derive related-panel counts.
func TestS3_FetcherWithNotifications_PopulatesLambdaField(t *testing.T) {
	fake := fakes.NewS3()
	result, err := awsclient.FetchS3BucketsPageWithNotifications(
		context.Background(),
		fake,
		fake,
		"",
	)
	if err != nil {
		t.Fatalf("FetchS3BucketsPageWithNotifications: %v", err)
	}

	var found bool
	for _, r := range result.Resources {
		if r.ID != fixtures.HealthyBucketName {
			continue
		}
		found = true
		lambdaField := r.Fields["notification_lambda"]
		if lambdaField == "" {
			t.Errorf("Fields[notification_lambda] is empty for %q; expected Lambda ARN containing %q",
				r.ID, fixtures.S3NotifierLambdaName)
		} else if !strings.Contains(lambdaField, fixtures.S3NotifierLambdaName) {
			t.Errorf("Fields[notification_lambda] = %q, want value containing %q",
				lambdaField, fixtures.S3NotifierLambdaName)
		}
		break
	}
	if !found {
		t.Fatalf("healthy-bucket %q not found in result", fixtures.HealthyBucketName)
	}
}

// TestS3_FetcherWithNotifications_PopulatesSNSField verifies that
// FetchS3BucketsPageWithNotifications populates Fields["notification_sns"]
// with the SNS topic ARN from the healthy bucket's notification config.
func TestS3_FetcherWithNotifications_PopulatesSNSField(t *testing.T) {
	fake := fakes.NewS3()
	result, err := awsclient.FetchS3BucketsPageWithNotifications(
		context.Background(),
		fake,
		fake,
		"",
	)
	if err != nil {
		t.Fatalf("FetchS3BucketsPageWithNotifications: %v", err)
	}

	var found bool
	for _, r := range result.Resources {
		if r.ID != fixtures.HealthyBucketName {
			continue
		}
		found = true
		snsField := r.Fields["notification_sns"]
		if snsField == "" {
			t.Errorf("Fields[notification_sns] is empty for %q; expected SNS topic ARN containing %q",
				r.ID, fixtures.S3EventsTopicName)
		} else if !strings.Contains(snsField, fixtures.S3EventsTopicName) {
			t.Errorf("Fields[notification_sns] = %q, want value containing %q",
				snsField, fixtures.S3EventsTopicName)
		}
		break
	}
	if !found {
		t.Fatalf("healthy-bucket %q not found in result", fixtures.HealthyBucketName)
	}
}

// TestS3_FetcherWithNotifications_PopulatesSQSField verifies that
// FetchS3BucketsPageWithNotifications populates Fields["notification_sqs"]
// with the SQS queue ARN from the healthy bucket's notification config.
func TestS3_FetcherWithNotifications_PopulatesSQSField(t *testing.T) {
	fake := fakes.NewS3()
	result, err := awsclient.FetchS3BucketsPageWithNotifications(
		context.Background(),
		fake,
		fake,
		"",
	)
	if err != nil {
		t.Fatalf("FetchS3BucketsPageWithNotifications: %v", err)
	}

	var found bool
	for _, r := range result.Resources {
		if r.ID != fixtures.HealthyBucketName {
			continue
		}
		found = true
		sqsField := r.Fields["notification_sqs"]
		if sqsField == "" {
			t.Errorf("Fields[notification_sqs] is empty for %q; expected SQS queue ARN containing %q",
				r.ID, fixtures.S3DLQueueName)
		} else if !strings.Contains(sqsField, fixtures.S3DLQueueName) {
			t.Errorf("Fields[notification_sqs] = %q, want value containing %q",
				sqsField, fixtures.S3DLQueueName)
		}
		break
	}
	if !found {
		t.Fatalf("healthy-bucket %q not found in result", fixtures.HealthyBucketName)
	}
}

// TestS3_FetcherWithNotifications_AbsentBucket_EmptyFields verifies that a
// bucket with no notification configuration has empty (not absent) notification
// field values, so downstream checkers don't crash on missing map lookups.
func TestS3_FetcherWithNotifications_AbsentBucket_EmptyFields(t *testing.T) {
	listMock := &mockS3ListBucketsClient{
		output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{
				{
					Name:         aws.String("bare-bucket"),
					BucketArn:    aws.String("arn:aws:s3:::bare-bucket"),
					BucketRegion: aws.String("us-east-1"),
					CreationDate: aws.Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
		},
	}

	result, err := awsclient.FetchS3BucketsPageWithNotifications(
		context.Background(),
		listMock,
		&s3EmptyNotificationFake{},
		"",
	)
	if err != nil {
		t.Fatalf("FetchS3BucketsPageWithNotifications: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	if r.Fields["notification_lambda"] != "" {
		t.Errorf("Fields[notification_lambda] = %q, want \"\" for bucket with no config", r.Fields["notification_lambda"])
	}
	if r.Fields["notification_sns"] != "" {
		t.Errorf("Fields[notification_sns] = %q, want \"\" for bucket with no config", r.Fields["notification_sns"])
	}
	if r.Fields["notification_sqs"] != "" {
		t.Errorf("Fields[notification_sqs] = %q, want \"\" for bucket with no config", r.Fields["notification_sqs"])
	}
}

// s3EmptyNotificationFake returns an empty notification config for every bucket.
type s3EmptyNotificationFake struct{}

func (f *s3EmptyNotificationFake) GetBucketNotificationConfiguration(
	_ context.Context,
	_ *s3.GetBucketNotificationConfigurationInput,
	_ ...func(*s3.Options),
) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return &s3.GetBucketNotificationConfigurationOutput{}, nil
}

// ---------------------------------------------------------------------------
// Edge-case: empty bucket list.
// ---------------------------------------------------------------------------

// TestS3_FetcherPage_EmptyBucketList verifies that FetchS3BucketsPage handles
// an empty ListBuckets response without error and returns zero resources.
func TestS3_FetcherPage_EmptyBucketList(t *testing.T) {
	mock := &mockS3ListBucketsClient{
		output: &s3.ListBucketsOutput{Buckets: nil},
	}
	result, err := awsclient.FetchS3BucketsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchS3BucketsPage: %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for empty bucket list, got %d", len(result.Resources))
	}
}
