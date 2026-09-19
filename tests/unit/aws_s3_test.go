package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestS3_FetcherResourceIssues_AlwaysEmpty(t *testing.T) {
	fake := fakes.NewS3()
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchS3BucketsPageWithNotifications(context.Background(), fake, nil, token)
	})
	if err != nil {
		t.Fatalf("FetchS3Buckets: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("expected at least one bucket from demo fake")
	}
	for _, r := range resources {
		if len(r.Findings) != 0 {
			t.Errorf("bucket %q: Findings = %v, want nil/empty (no Wave 1 signals in s3 spec)", r.ID, r.Findings)
		}
	}
}

func TestS3_FetcherIdentityFields_HealthyBucket(t *testing.T) {
	fake := fakes.NewS3()
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchS3BucketsPageWithNotifications(context.Background(), fake, nil, token)
	})
	if err != nil {
		t.Fatalf("FetchS3Buckets: %v", err)
	}

	var found bool
	for _, r := range resources {
		if r.ID != fixtures.HealthyBucketName {
			continue
		}
		found = true

		if r.ID != fixtures.HealthyBucketName {
			t.Errorf("Resource.ID = %q, want %q", r.ID, fixtures.HealthyBucketName)
		}
		if r.Name != fixtures.HealthyBucketName {
			t.Errorf("Resource.Name = %q, want %q", r.Name, fixtures.HealthyBucketName)
		}

		if r.Fields["name"] != fixtures.HealthyBucketName {
			t.Errorf("Fields[name] = %q, want %q", r.Fields["name"], fixtures.HealthyBucketName)
		}

		if r.Fields["creation_date"] == "" {
			t.Errorf("Fields[creation_date] is empty for %q; expected a formatted date", r.ID)
		}

		break
	}
	if !found {
		t.Fatalf("healthy-bucket fixture %q not found in fetcher output", fixtures.HealthyBucketName)
	}
}

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

func TestS3_FetcherWithNotifications_AbsentBucket_EmptyFields(t *testing.T) {
	listMock := &fakeS3ListBuckets{
		Output: &s3.ListBucketsOutput{
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
		&S3BucketNotificationFake{Output: &s3.GetBucketNotificationConfigurationOutput{}},
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

// S3BucketNotificationFake implements S3GetBucketNotificationConfigurationAPI
// with a caller-supplied result. It is exported for the unit_test package,
// which cannot share unexported identifiers with this one.
type S3BucketNotificationFake struct {
	Output *s3.GetBucketNotificationConfigurationOutput
	Err    error
}

func (f *S3BucketNotificationFake) GetBucketNotificationConfiguration(
	_ context.Context,
	_ *s3.GetBucketNotificationConfigurationInput,
	_ ...func(*s3.Options),
) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return f.Output, f.Err
}

func TestS3_FetcherPage_EmptyBucketList(t *testing.T) {
	mock := &fakeS3ListBuckets{
		Output: &s3.ListBucketsOutput{Buckets: nil},
	}
	result, err := awsclient.FetchS3BucketsPageWithNotifications(context.Background(), mock, nil, "")
	if err != nil {
		t.Fatalf("FetchS3BucketsPageWithNotifications: %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for empty bucket list, got %d", len(result.Resources))
	}
}
