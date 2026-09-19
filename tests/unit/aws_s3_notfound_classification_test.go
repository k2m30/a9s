package unit_test

// A deleted bucket (Code "NoSuchBucket", or the empty-body-404 "NotFound")
// is a benign absence for the per-bucket S3 checkers, like the
// sub-config-absent codes (NoSuchTagSet,
// ServerSideEncryptionConfigurationNotFoundError, NoSuchBucketPolicy): an
// honest zero, never RelatedError. Classification reads ErrorCode(), not the
// message text.

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestS3Related_DeletedBucket_BenignAbsence(t *testing.T) {
	const deletedBucket = "gone-bucket"
	codes := []string{"NoSuchBucket", "NotFound"}

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
			name: "s3(access-log)/GetBucketLogging",
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
				got := checker(context.Background(), clients, emptyBucketResource(deletedBucket), nil)
				if got.TargetType() != c.wantTarget {
					t.Errorf("TargetType = %q, want %q", got.TargetType(), c.wantTarget)
				}
				if got.Count() != 0 {
					t.Errorf("Count = %d, want 0", got.Count())
				}
				if got.State() != domain.RelatedResolved {
					t.Errorf("State = %v, want RelatedResolved — a deleted bucket is an honest zero, not an error state", got.State())
				}
				if got.Truncated() {
					t.Error("Truncated = true; a deleted bucket is a resolved zero, not a soft-truncated 0+")
				}
				if got.Err() != nil {
					t.Errorf("Err = %v, want nil — a deleted bucket must not surface as a related-panel failure", got.Err())
				}
			})
		}
	}
}

func TestS3Related_ExistingBenignCodes_AreCodeBased(t *testing.T) {
	type kase struct {
		name       string
		code       string
		clientsFor func(code string) *awsclient.ServiceClients
		checkerFor func(t *testing.T) resource.RelatedChecker
		wantTarget string
	}

	cases := []kase{
		{
			name: "cfn/NoSuchTagSet",
			code: "NoSuchTagSet",
			clientsFor: func(code string) *awsclient.ServiceClients {
				return &awsclient.ServiceClients{S3: &s3TaggingErrFake{code: code}}
			},
			checkerFor: func(t *testing.T) resource.RelatedChecker {
				return s3CheckerByTarget(t, "cfn")
			},
			wantTarget: "cfn",
		},
		{
			name: "kms/ServerSideEncryptionConfigurationNotFoundError",
			code: "ServerSideEncryptionConfigurationNotFoundError",
			clientsFor: func(code string) *awsclient.ServiceClients {
				return &awsclient.ServiceClients{S3: &s3EncryptionErrFake{code: code}}
			},
			checkerFor: func(t *testing.T) resource.RelatedChecker {
				return s3CheckerByTarget(t, "kms")
			},
			wantTarget: "kms",
		},
		{
			name: "role/NoSuchBucketPolicy",
			code: "NoSuchBucketPolicy",
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
		t.Run(c.name, func(t *testing.T) {
			checker := c.checkerFor(t)
			// s3*ErrFake always builds &smithy.GenericAPIError{Code: c.code,
			// Message: "cross-region or denied: " + c.code} — an unrelated
			// message that does not restate the AWS-canonical error text.
			clients := c.clientsFor(c.code)
			got := checker(context.Background(), clients, emptyBucketResource("some-bucket"), nil)
			if got.TargetType() != c.wantTarget {
				t.Errorf("TargetType = %q, want %q", got.TargetType(), c.wantTarget)
			}
			if got.Count() != 0 {
				t.Errorf("Count = %d, want 0", got.Count())
			}
			if got.State() != domain.RelatedResolved {
				t.Errorf("State = %v, want RelatedResolved for existing benign code %q", got.State(), c.code)
			}
			if got.Err() != nil {
				t.Errorf("Err = %v, want nil for existing benign code %q", got.Err(), c.code)
			}
		})
	}
}
