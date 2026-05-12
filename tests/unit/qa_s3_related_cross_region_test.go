package unit_test

// qa_s3_related_cross_region_test.go — AS-489 regression: the four S3 related-def
// checkers that issue a per-bucket S3 API call (checkS3CFN → GetBucketTagging,
// checkS3KMS → GetBucketEncryption, checkS3Logs → GetBucketLogging,
// checkS3Role → GetBucketPolicy) must NOT bubble PermanentRedirect (301) or
// IllegalLocationConstraintException (400) up as Count:-1 errors. Both codes
// indicate the configured S3 client's region differs from the target bucket's
// region — a legitimate environmental condition on multi-region accounts, not a
// bug. The checkers must soft-truncate to ApproximateZero ("0+"), preserving
// the existing -1 contract for genuine failures (e.g. AccessDenied).
//
// Discovery: `TestLiveFullIntegration_AllResourcesBaseline/s3` failed on a live
// account containing buckets in eu-west-2 + ap-south-1; opening any related-pivot
// on the out-of-region bucket emitted Count:-1 and the related panel showed no
// count for the affected pivot.
//
// Pattern A precedent: see s3_issue_enrichment.go EnrichS3PublicAccessBlock,
// which already classifies this exact error pair as operational, not a bug,
// and marks TruncatedIDs (row "?") rather than spamming the failure log.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Fakes — one per affected S3 API. Each embeds s3NoopAPI to inherit the four
// S3API methods needed to satisfy ServiceClients.S3 (ListBuckets,
// ListObjectsV2, GetBucketNotificationConfiguration, GetPublicAccessBlock),
// and overrides the single GetBucket* method the checker calls. The fakes
// return &smithy.GenericAPIError{Code} regardless of the bucket; classification
// is by ErrorCode, not by message string.
// ---------------------------------------------------------------------------

// s3NoopAPI provides empty implementations of the four S3API methods, so test
// fakes can embed it and override only the GetBucket* method under test.
type s3NoopAPI struct{}

func (s3NoopAPI) ListBuckets(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return &s3.ListBucketsOutput{}, nil
}

func (s3NoopAPI) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}

func (s3NoopAPI) GetBucketNotificationConfiguration(_ context.Context, _ *s3.GetBucketNotificationConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return &s3.GetBucketNotificationConfigurationOutput{}, nil
}

func (s3NoopAPI) GetPublicAccessBlock(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	return &s3.GetPublicAccessBlockOutput{}, nil
}

type s3TaggingErrFake struct {
	s3NoopAPI
	code string
}

func (f *s3TaggingErrFake) GetBucketTagging(
	_ context.Context, _ *s3.GetBucketTaggingInput, _ ...func(*s3.Options),
) (*s3.GetBucketTaggingOutput, error) {
	return nil, &smithy.GenericAPIError{Code: f.code, Message: "cross-region or denied: " + f.code}
}

type s3EncryptionErrFake struct {
	s3NoopAPI
	code string
}

func (f *s3EncryptionErrFake) GetBucketEncryption(
	_ context.Context, _ *s3.GetBucketEncryptionInput, _ ...func(*s3.Options),
) (*s3.GetBucketEncryptionOutput, error) {
	return nil, &smithy.GenericAPIError{Code: f.code, Message: "cross-region or denied: " + f.code}
}

type s3LoggingErrFake struct {
	s3NoopAPI
	code string
}

func (f *s3LoggingErrFake) GetBucketLogging(
	_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options),
) (*s3.GetBucketLoggingOutput, error) {
	return nil, &smithy.GenericAPIError{Code: f.code, Message: "cross-region or denied: " + f.code}
}

type s3PolicyErrFake struct {
	s3NoopAPI
	code string
}

func (f *s3PolicyErrFake) GetBucketPolicy(
	_ context.Context, _ *s3.GetBucketPolicyInput, _ ...func(*s3.Options),
) (*s3.GetBucketPolicyOutput, error) {
	return nil, &smithy.GenericAPIError{Code: f.code, Message: "cross-region or denied: " + f.code}
}

// ---------------------------------------------------------------------------
// Cross-region soft-truncate matrix — 4 checkers × 2 error codes = 8 sub-tests.
// Each asserts the soft-truncated contract:
//   Count = 0, Approximate = true, Err = nil, TargetType = <expected>.
// ---------------------------------------------------------------------------

func TestS3Related_CrossRegion_SoftTruncates(t *testing.T) {
	const xRegionBucket = "ap-south-bucket"
	codes := []string{"PermanentRedirect", "IllegalLocationConstraintException"}

	type kase struct {
		name        string
		clientsFor  func(code string) *awsclient.ServiceClients
		checkerFor  func(t *testing.T) resource.RelatedChecker
		wantTarget  string
	}

	cases := []kase{
		{
			name: "cfn/GetBucketTagging",
			clientsFor: func(code string) *awsclient.ServiceClients {
				return &awsclient.ServiceClients{S3: &s3TaggingErrFake{code: code}}
			},
			checkerFor: func(t *testing.T) resource.RelatedChecker {
				return s3CheckerByTarget(t, "cfn")
			},
			wantTarget: "cfn",
		},
		{
			name: "kms/GetBucketEncryption",
			clientsFor: func(code string) *awsclient.ServiceClients {
				return &awsclient.ServiceClients{S3: &s3EncryptionErrFake{code: code}}
			},
			checkerFor: func(t *testing.T) resource.RelatedChecker {
				return s3CheckerByTarget(t, "kms")
			},
			wantTarget: "kms",
		},
		{
			name: "logs/GetBucketLogging",
			clientsFor: func(code string) *awsclient.ServiceClients {
				return &awsclient.ServiceClients{S3: &s3LoggingErrFake{code: code}}
			},
			checkerFor: func(t *testing.T) resource.RelatedChecker {
				// The s3→s3 access-log pivot is disambiguated by display name —
				// two pivots in the s3 registry share TargetType="s3".
				return s3CheckerByDisplayName(t, "Access Log Bucket")
			},
			wantTarget: "s3",
		},
		{
			name: "role/GetBucketPolicy",
			clientsFor: func(code string) *awsclient.ServiceClients {
				return &awsclient.ServiceClients{S3: &s3PolicyErrFake{code: code}}
			},
			checkerFor: func(t *testing.T) resource.RelatedChecker {
				return s3CheckerByTarget(t, "role")
			},
			wantTarget: "role",
		},
	}

	for _, c := range cases {
		for _, code := range codes {
			t.Run(c.name+"/"+code, func(t *testing.T) {
				checker := c.checkerFor(t)
				clients := c.clientsFor(code)
				got := checker(context.Background(), clients, emptyBucketResource(xRegionBucket), nil)
				if got.TargetType != c.wantTarget {
					t.Errorf("TargetType = %q, want %q", got.TargetType, c.wantTarget)
				}
				if got.Count != 0 {
					t.Errorf("Count = %d, want 0 (soft-truncate)", got.Count)
				}
				if !got.Approximate {
					t.Error("Approximate = false, want true (rendered as 0+)")
				}
				if got.Err != nil {
					t.Errorf("Err = %v, want nil (cross-region is operational, not a failure)", got.Err)
				}
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Contract-preservation guard — a NON cross-region error code (AccessDenied)
// must still bubble up as Count=-1 with non-nil Err for one of the four
// affected checkers. Guards against the new branch swallowing real failures.
// ---------------------------------------------------------------------------

func TestS3Related_CrossRegion_PreservesUnknownContract(t *testing.T) {
	const xRegionBucket = "no-permission-bucket"
	clients := &awsclient.ServiceClients{S3: &s3TaggingErrFake{code: "AccessDenied"}}

	checker := s3CheckerByTarget(t, "cfn")
	got := checker(context.Background(), clients, emptyBucketResource(xRegionBucket), nil)
	if got.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", got.TargetType, "cfn")
	}
	if got.Count != -1 {
		t.Errorf("Count = %d, want -1 (real failure must NOT be swallowed)", got.Count)
	}
	if got.Approximate {
		t.Error("Approximate = true; AccessDenied must remain a hard unknown, not an approximation")
	}
	if got.Err == nil {
		t.Error("Err = nil; AccessDenied must surface the underlying error for diagnosis")
	}
}
