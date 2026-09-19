package unit_test

// The four S3 related-def checkers that
// issue a per-bucket S3 API call (checkS3CFN → GetBucketTagging, checkS3KMS →
// GetBucketEncryption, checkS3Logs → GetBucketLogging, checkS3Role →
// GetBucketPolicy) must NOT bubble PermanentRedirect (301) or
// IllegalLocationConstraintException (400) up as a State: RelatedError result.
// Both codes indicate the configured S3 client's region differs from the target
// bucket's region — a legitimate environmental condition on multi-region
// accounts, not a bug. The checkers soft-truncate to TruncatedResult ("0+");
// a genuine failure (e.g. AccessDenied) stays a RelatedError
// (resource.ErrorRelated).
//
// EnrichS3Posture (s3_issue_enrichment.go) classifies the same error pair as
// operational and marks TruncatedIDs (row "?") instead of logging a failure.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// Each fake embeds s3NoopAPI and overrides the single GetBucket* method its
// checker calls, returning &smithy.GenericAPIError{Code} for any bucket;
// classification is by ErrorCode, not by message string.

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

func TestS3Related_CrossRegion_SoftTruncates(t *testing.T) {
	const xRegionBucket = "ap-south-bucket"
	codes := []string{"PermanentRedirect", "IllegalLocationConstraintException"}

	type kase struct {
		name       string
		clientsFor func(code string) *awsclient.ServiceClients
		checkerFor func(t *testing.T) resource.RelatedChecker
		wantTarget string
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
				if got.TargetType() != c.wantTarget {
					t.Errorf("TargetType = %q, want %q", got.TargetType(), c.wantTarget)
				}
				if got.Count() != 0 {
					t.Errorf("Count = %d, want 0 (soft-truncate)", got.Count())
				}
				if !got.Truncated() {
					t.Error("Truncated = false, want true (rendered as 0+)")
				}
				if got.Err() != nil {
					t.Errorf("Err = %v, want nil (cross-region is operational, not a failure)", got.Err())
				}
			})
		}
	}
}

// A non-cross-region error code (AccessDenied) stays State: RelatedError with
// a non-nil Err.
func TestS3Related_CrossRegion_PreservesUnknownContract(t *testing.T) {
	const xRegionBucket = "no-permission-bucket"
	clients := &awsclient.ServiceClients{S3: &s3TaggingErrFake{code: "AccessDenied"}}

	checker := s3CheckerByTarget(t, "cfn")
	got := checker(context.Background(), clients, emptyBucketResource(xRegionBucket), nil)
	if got.TargetType() != "cfn" {
		t.Errorf("TargetType = %q, want %q", got.TargetType(), "cfn")
	}
	if got.State() != domain.RelatedError {
		t.Errorf("Count = %d, want -1 (real failure must NOT be swallowed)", got.Count())
	}
	if got.Truncated() {
		t.Error("Truncated = true; AccessDenied must remain a hard unknown, not a truncated count")
	}
	if got.Err() == nil {
		t.Error("Err = nil; AccessDenied must surface the underlying error for diagnosis")
	}
}
