package unit_test

// s3_0916_row2_notification_targets_test.go pins the bucket row carrying every
// notification destination rather than the first of each kind, and the three
// notification pivots reporting a failed lookup as an error instead of a false
// zero.
//
// A bucket may fan one event out to several Lambda functions, queues and
// topics; reporting only the first hides the rest of the blast radius. A
// GetBucketNotificationConfiguration that was refused is not "no destinations"
// either — the operator must see that the answer is missing, while a
// cross-region refusal stays operational and soft-truncates like the other S3
// pivots.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	row2Bucket    = "acme-events"
	row2LambdaA   = "arn:aws:lambda:us-east-1:123456789012:function:acme-notify-a"
	row2LambdaB   = "arn:aws:lambda:us-east-1:123456789012:function:acme-notify-b"
	row2QueueQ1   = "arn:aws:sqs:us-east-1:123456789012:acme-events-q1"
	row2QueueQ2   = "arn:aws:sqs:us-east-1:123456789012:acme-events-q2"
	row2TopicT1   = "arn:aws:sns:us-east-1:123456789012:acme-events-t1"
	row2TopicT2   = "arn:aws:sns:us-east-1:123456789012:acme-events-t2"
	row2OtherName = "acme-archive"
)

// row2S3Fake answers ListBuckets with one bucket and
// GetBucketNotificationConfiguration with either the configured destinations
// or the configured error.
type row2S3Fake struct {
	notif *s3.GetBucketNotificationConfigurationOutput
	err   error
}

func (f *row2S3Fake) ListBuckets(
	_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
	return &s3.ListBucketsOutput{
		Buckets: []s3types.Bucket{{
			Name:         aws.String(row2Bucket),
			CreationDate: aws.Time(time.Date(2024, 3, 1, 9, 30, 0, 0, time.UTC)),
		}},
	}, nil
}

func (f *row2S3Fake) GetBucketNotificationConfiguration(
	_ context.Context, _ *s3.GetBucketNotificationConfigurationInput, _ ...func(*s3.Options),
) (*s3.GetBucketNotificationConfigurationOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.notif, nil
}

// row2TwoOfEach is the realistic fan-out answer: two Lambda functions, two
// queues and two topics on a single bucket.
func row2TwoOfEach() *s3.GetBucketNotificationConfigurationOutput {
	return &s3.GetBucketNotificationConfigurationOutput{
		LambdaFunctionConfigurations: []s3types.LambdaFunctionConfiguration{
			{LambdaFunctionArn: aws.String(row2LambdaA), Events: []s3types.Event{s3types.EventS3ObjectCreated}},
			{LambdaFunctionArn: aws.String(row2LambdaB), Events: []s3types.Event{s3types.EventS3ObjectRemoved}},
		},
		QueueConfigurations: []s3types.QueueConfiguration{
			{QueueArn: aws.String(row2QueueQ1), Events: []s3types.Event{s3types.EventS3ObjectCreated}},
			{QueueArn: aws.String(row2QueueQ2), Events: []s3types.Event{s3types.EventS3ObjectRemoved}},
		},
		TopicConfigurations: []s3types.TopicConfiguration{
			{TopicArn: aws.String(row2TopicT1), Events: []s3types.Event{s3types.EventS3ObjectCreated}},
			{TopicArn: aws.String(row2TopicT2), Events: []s3types.Event{s3types.EventS3ObjectRemoved}},
		},
	}
}

func row2FetchOne(t *testing.T, fake *row2S3Fake) (resource.Resource, error) {
	t.Helper()
	res, err := awsclient.FetchS3BucketsPageWithNotifications(context.Background(), fake, fake, "")
	if len(res.Resources) != 1 {
		t.Fatalf("Resources = %d, want 1 bucket", len(res.Resources))
	}
	return res.Resources[0], err
}

func row2WantField(t *testing.T, r resource.Resource, key, want string) {
	t.Helper()
	if got := r.Fields[key]; got != want {
		t.Errorf("Fields[%q] = %q, want %q", key, got, want)
	}
}

func TestS3_0916_Row2_AllNotificationTargetsAreCommaJoined(t *testing.T) {
	r, err := row2FetchOne(t, &row2S3Fake{notif: row2TwoOfEach()})
	if err != nil {
		t.Fatalf("fetch error = %v, want nil", err)
	}
	row2WantField(t, r, "notification_lambda", row2LambdaA+","+row2LambdaB)
	row2WantField(t, r, "notification_sqs", row2QueueQ1+","+row2QueueQ2)
	row2WantField(t, r, "notification_sns", row2TopicT1+","+row2TopicT2)
	row2WantField(t, r, "notification_error", "")
	row2WantField(t, r, "notification_truncated", "")
}

func TestS3_0916_Row2_PivotsListEveryTarget(t *testing.T) {
	r, err := row2FetchOne(t, &row2S3Fake{notif: row2TwoOfEach()})
	if err != nil {
		t.Fatalf("fetch error = %v, want nil", err)
	}

	cases := []struct {
		target string
		want   []string
	}{
		// lambda and sqs drill in by name, sns by full ARN — each target
		// type's canonical Resource.ID.
		{"lambda", []string{"acme-notify-a", "acme-notify-b"}},
		{"sqs", []string{"acme-events-q1", "acme-events-q2"}},
		{"sns", []string{row2TopicT1, row2TopicT2}},
	}
	for _, c := range cases {
		t.Run(c.target, func(t *testing.T) {
			got := checkerByTarget(t, "s3", c.target)(context.Background(), nil, r, resource.ResourceCache{})
			if got.State() != domain.RelatedResolved {
				t.Fatalf("State = %v, want RelatedResolved", got.State())
			}
			if got.Count() != len(c.want) {
				t.Fatalf("Count = %d, want %d", got.Count(), len(c.want))
			}
			ids := got.ResourceIDs()
			for i, want := range c.want {
				if ids[i] != want {
					t.Errorf("ResourceIDs[%d] = %q, want %q", i, ids[i], want)
				}
			}
		})
	}
}

func TestS3_0916_Row2_LookupDeniedIsAnErrorNotAZero(t *testing.T) {
	denied := &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized to perform: s3:GetBucketNotification"}
	r, err := row2FetchOne(t, &row2S3Fake{err: denied})

	if err == nil {
		t.Fatal("fetch error = nil, want an aggregated failure")
	}
	if !strings.Contains(err.Error(), row2Bucket) {
		t.Errorf("fetch error = %q, want it to name bucket %q", err.Error(), row2Bucket)
	}
	if r.Fields["notification_error"] == "" {
		t.Error(`Fields["notification_error"] = "", want the failure text`)
	}
	row2WantField(t, r, "notification_truncated", "")

	for _, target := range []string{"lambda", "sns", "sqs"} {
		t.Run(target, func(t *testing.T) {
			got := checkerByTarget(t, "s3", target)(context.Background(), nil, r, resource.ResourceCache{})
			if got.State() != domain.RelatedError {
				t.Fatalf("State = %v, want RelatedError", got.State())
			}
			if got.Err() == nil {
				t.Fatal("Err = nil, want the lookup failure")
			}
			if got.Count() != 0 {
				t.Errorf("Count = %d, want 0 on an error result", got.Count())
			}
		})
	}
}

func TestS3_0916_Row2_CrossRegionSoftTruncates(t *testing.T) {
	redirect := &smithy.GenericAPIError{Code: "PermanentRedirect", Message: "The bucket is in this region: eu-west-1"}
	r, err := row2FetchOne(t, &row2S3Fake{err: redirect})

	if err != nil {
		t.Fatalf("fetch error = %v, want nil — a cross-region bucket is operational, not a failure", err)
	}
	row2WantField(t, r, "notification_truncated", "true")
	row2WantField(t, r, "notification_error", "")

	for _, target := range []string{"lambda", "sns", "sqs"} {
		t.Run(target, func(t *testing.T) {
			got := checkerByTarget(t, "s3", target)(context.Background(), nil, r, resource.ResourceCache{})
			if got.State() != domain.RelatedResolved {
				t.Fatalf("State = %v, want RelatedResolved", got.State())
			}
			if got.Count() != 0 {
				t.Errorf("Count = %d, want 0", got.Count())
			}
			if !got.Truncated() {
				t.Error("Truncated = false, want true (renders 0+)")
			}
			if got.Err() != nil {
				t.Errorf("Err = %v, want nil", got.Err())
			}
		})
	}
}

// row2LambdaResource is the function resource the lambda→s3 pivot is driven on.
func row2LambdaResource(name, arn string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(name),
			FunctionArn:  aws.String(arn),
		},
	}
}

func TestS3_0916_Row2_LambdaS3MatchesOneElementOfTheList(t *testing.T) {
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: row2Bucket, Name: row2Bucket, Fields: map[string]string{
				"notification_lambda": row2LambdaA + "," + row2LambdaB,
			}},
			{ID: row2OtherName, Name: row2OtherName, Fields: map[string]string{
				"notification_lambda": row2LambdaB,
			}},
		}},
	}
	checker := checkerByTarget(t, "lambda", "s3")

	t.Run("first element of a joined list", func(t *testing.T) {
		got := checker(context.Background(), nil, row2LambdaResource("acme-notify-a", row2LambdaA), cache)
		if got.Count() != 1 || len(got.ResourceIDs()) != 1 || got.ResourceIDs()[0] != row2Bucket {
			t.Fatalf("Count = %d, ResourceIDs = %v, want 1 [%s]", got.Count(), got.ResourceIDs(), row2Bucket)
		}
	})

	t.Run("element shared by both buckets", func(t *testing.T) {
		got := checker(context.Background(), nil, row2LambdaResource("acme-notify-b", row2LambdaB), cache)
		if got.Count() != 2 {
			t.Fatalf("Count = %d, want 2 (both buckets notify this function)", got.Count())
		}
	})

	t.Run("function no bucket notifies", func(t *testing.T) {
		unused := "arn:aws:lambda:us-east-1:123456789012:function:acme-unused"
		got := checker(context.Background(), nil, row2LambdaResource("acme-unused", unused), cache)
		if got.Count() != 0 {
			t.Fatalf("Count = %d, want 0", got.Count())
		}
	})
}

func TestS3_0916_Row2_CatalogDeclaresTheTwoNewFieldKeys(t *testing.T) {
	def := catalog.TopLevelOnly("s3")
	if def == nil {
		t.Fatal(`catalog.TopLevelOnly("s3") = nil`)
	}
	for _, key := range []string{"notification_error", "notification_truncated"} {
		found := false
		for _, k := range def.FieldKeys {
			if k == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("s3 FieldKeys = %v, want it to declare %q", def.FieldKeys, key)
		}
	}
}
