package unit

// qa_s3_cross_region_test.go — Regression: EnrichS3PublicAccessBlock must handle
// cross-region buckets without spamming the error log.
//
// Reported 2026-04-25 from `./a9s -p <redacted-profile>`:
//   [HH:MM:SS] enrich s3: s3-enrich: GetPublicAccessBlock failed for 1 of 35 IDs:
//     ses-incoming.prod.eu.<redacted-account>.cloud: ... api error PermanentRedirect: The bucket
//     you are attempting to access must be addressed using the specified endpoint.
//
// And:
//   ... api error IllegalLocationConstraintException: The eu-central-2 location
//   constraint is incompatible for the region specific endpoint this request was
//   sent to.
//
// Root cause: ListBuckets returns ALL buckets globally regardless of the configured
// client region, but per-bucket calls (GetPublicAccessBlock) require the bucket's
// own regional endpoint. When the bucket lives in a different region, AWS rejects
// the call with PermanentRedirect (301) or IllegalLocationConstraintException (400).
//
// Contract (post-fix):
//   - Cross-region buckets must NOT appear in the AggregateFailures error.
//   - Cross-region buckets MAY appear in TruncatedIDs (data incomplete) so the
//     row gets a "?" marker, but they must not spam the `!` error log because
//     this is a legitimate environmental condition, not a bug.
//   - The enricher must EITHER (a) re-issue the call against the bucket's region
//     (preferred), or (b) silently drop these specific error classes.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// s3CrossRegionFake returns PermanentRedirect for one bucket and a successful
// (no-PAB-config → not-public) response for another.
type s3CrossRegionFake struct {
	s3PABFake
	crossRegionBuckets map[string]string // bucket → its region
}

func (f *s3CrossRegionFake) GetPublicAccessBlock(
	ctx context.Context,
	input *s3.GetPublicAccessBlockInput,
	optFns ...func(*s3.Options),
) (*s3.GetPublicAccessBlockOutput, error) {
	if input.Bucket == nil {
		return nil, &smithy.GenericAPIError{Code: "InvalidBucketName", Message: "bucket required"}
	}
	bucket := *input.Bucket
	if region, ok := f.crossRegionBuckets[bucket]; ok {
		// If caller asked for the bucket's correct region via optFns, succeed.
		opts := s3.Options{}
		for _, fn := range optFns {
			fn(&opts)
		}
		if opts.Region == region {
			return &s3.GetPublicAccessBlockOutput{
				PublicAccessBlockConfiguration: nil, // NoSuchPublicAccessBlockConfiguration shape
			}, &smithy.GenericAPIError{
				Code:    "NoSuchPublicAccessBlockConfiguration",
				Message: "no PAB config",
			}
		}
		return nil, &smithy.GenericAPIError{
			Code:    "PermanentRedirect",
			Message: "The bucket you are attempting to access must be addressed using the specified endpoint",
		}
	}
	return f.s3PABFake.GetPublicAccessBlock(ctx, input, optFns...)
}

// TestEnrichS3PublicAccessBlock_CrossRegionDoesNotSpamErrorLog verifies that a
// cross-region bucket (PermanentRedirect / IllegalLocationConstraintException)
// does NOT contribute to the AggregateFailures error surfaced to the `!` log.
func TestEnrichS3PublicAccessBlock_CrossRegionDoesNotSpamErrorLog(t *testing.T) {
	const xRegionBucket = "ses-incoming.prod.eu.<redacted-account>.cloud"
	fake := &s3CrossRegionFake{
		s3PABFake: s3PABFake{configs: map[string]*s3.GetPublicAccessBlockOutput{}},
		crossRegionBuckets: map[string]string{
			xRegionBucket: "eu-central-2",
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{
		{ID: xRegionBucket, Name: xRegionBucket},
		{ID: "same-region-bucket", Name: "same-region-bucket"},
	}

	_, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err == nil {
		return
	}
	// If an error is surfaced, it must NOT mention the cross-region bucket.
	if strings.Contains(err.Error(), xRegionBucket) {
		t.Errorf("cross-region bucket %q must not appear in error log; got: %v", xRegionBucket, err)
	}
	if strings.Contains(err.Error(), "PermanentRedirect") {
		t.Errorf("PermanentRedirect must not appear in error log; got: %v", err)
	}
}

// TestEnrichS3PublicAccessBlock_IllegalLocationConstraintNotSpammed verifies the
// other cross-region error class (eu-central-2 endpoint mismatch).
func TestEnrichS3PublicAccessBlock_IllegalLocationConstraintNotSpammed(t *testing.T) {
	const xRegionBucket = "dags-destination-<redacted-account>-euc2-prod"
	fake := &fakeS3IllegalLocation{bucket: xRegionBucket}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{{ID: xRegionBucket, Name: xRegionBucket}}

	_, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil && strings.Contains(err.Error(), "IllegalLocationConstraint") {
		t.Errorf("IllegalLocationConstraintException must not appear in error log; got: %v", err)
	}
}

// fakeS3IllegalLocation always returns IllegalLocationConstraintException.
type fakeS3IllegalLocation struct {
	s3PABFake
	bucket string
}

func (f *fakeS3IllegalLocation) GetPublicAccessBlock(
	_ context.Context,
	input *s3.GetPublicAccessBlockInput,
	_ ...func(*s3.Options),
) (*s3.GetPublicAccessBlockOutput, error) {
	if aws.ToString(input.Bucket) == f.bucket {
		return nil, &smithy.GenericAPIError{
			Code:    "IllegalLocationConstraintException",
			Message: "The eu-central-2 location constraint is incompatible for the region specific endpoint this request was sent to",
		}
	}
	return &s3.GetPublicAccessBlockOutput{}, nil
}
