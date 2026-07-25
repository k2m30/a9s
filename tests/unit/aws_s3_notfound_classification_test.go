package unit_test

// aws_s3_notfound_classification_test.go — issue #456: the four S3
// related-def checkers that issue a per-bucket S3 API call (checkS3CFN →
// GetBucketTagging, checkS3KMS → GetBucketEncryption, checkS3Logs →
// GetBucketLogging, checkS3Role → GetBucketPolicy) must classify a deleted
// bucket (Code "NoSuchBucket", and the empty-body-404 "NotFound" shape) as a
// benign absence — the same honest-zero outcome as the existing
// sub-config-absent cases (NoSuchTagSet, ServerSideEncryptionConfigurationNotFoundError,
// NoSuchBucketPolicy) — never a State: RelatedError related cell.
//
// Also pins that classification is code-based, not message-substring-based:
// the existing benign codes must still resolve as benign when delivered as a
// smithy.GenericAPIError carrying the Code plus an unrelated message.
//
// Reuses the s3NoopAPI / s3*ErrFake fakes and s3CheckerByTarget /
// s3CheckerByDisplayName / emptyBucketResource helpers already defined in
// qa_s3_related_cross_region_test.go and aws_s3_related_test.go (same
// package unit_test) — no new fakes needed, only new `code` values driven
// through the fakes' existing `code` field.

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Deleted-bucket taxonomy — 4 checkers × 2 codes (NoSuchBucket, NotFound) = 8
// sub-tests. Each asserts the benign-absence contract: Count=0,
// State=RelatedResolved (honest zero), Truncated=false, Err=nil — NOT the
// dimmed State: RelatedError related cell a genuine failure would render.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Code-based classification guard — the three checkers with a pre-existing
// benign-absence special case must still resolve benign when the error
// carries the Code plus an unrelated message, proving the match is on
// ErrorCode() and not a strings.Contains(err.Error(), "<code>") scan.
// ---------------------------------------------------------------------------

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
