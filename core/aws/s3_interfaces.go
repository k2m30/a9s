// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3ListBucketsAPI defines the interface for the S3 ListBuckets operation.
type S3ListBucketsAPI interface {
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
}

// S3ListObjectsV2API defines the interface for the S3 ListObjectsV2 operation.
type S3ListObjectsV2API interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// S3GetBucketLocationAPI defines the interface for the S3 GetBucketLocation operation.
type S3GetBucketLocationAPI interface {
	GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
}

// S3GetBucketNotificationConfigurationAPI defines the interface for
// the S3 GetBucketNotificationConfiguration operation.
type S3GetBucketNotificationConfigurationAPI interface {
	GetBucketNotificationConfiguration(ctx context.Context, params *s3.GetBucketNotificationConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error)
}

// S3GetPublicAccessBlockAPI defines the interface for the S3 GetPublicAccessBlock operation.
// Used by EnrichS3Posture to check per-bucket PAB configuration.
type S3GetPublicAccessBlockAPI interface {
	GetPublicAccessBlock(ctx context.Context, params *s3.GetPublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error)
}

// S3GetBucketEncryptionAPI defines the interface for the S3 GetBucketEncryption operation.
type S3GetBucketEncryptionAPI interface {
	GetBucketEncryption(ctx context.Context, params *s3.GetBucketEncryptionInput, optFns ...func(*s3.Options)) (*s3.GetBucketEncryptionOutput, error)
}

// S3GetBucketLoggingAPI defines the interface for the S3 GetBucketLogging operation.
type S3GetBucketLoggingAPI interface {
	GetBucketLogging(ctx context.Context, params *s3.GetBucketLoggingInput, optFns ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error)
}

// S3GetBucketTaggingAPI defines the interface for the S3 GetBucketTagging operation.
type S3GetBucketTaggingAPI interface {
	GetBucketTagging(ctx context.Context, params *s3.GetBucketTaggingInput, optFns ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error)
}

// S3GetBucketPolicyAPI defines the interface for the S3 GetBucketPolicy
// operation. Used by the s3→role pivot to discover roles named as
// AWS principals in the bucket's resource policy.
type S3GetBucketPolicyAPI interface {
	GetBucketPolicy(ctx context.Context, params *s3.GetBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error)
}

// S3GetBucketCorsAPI defines the interface for the S3 GetBucketCors
// operation. Used by the on-demand s3 detail enricher (enrichS3) — not part
// of the S3API aggregate since no fetcher or related checker needs it.
type S3GetBucketCorsAPI interface {
	GetBucketCors(ctx context.Context, params *s3.GetBucketCorsInput, optFns ...func(*s3.Options)) (*s3.GetBucketCorsOutput, error)
}

// S3GetBucketLifecycleAPI defines the interface for the S3
// GetBucketLifecycleConfiguration operation. Used by the on-demand s3
// detail enricher (enrichS3) — not part of the S3API aggregate since no
// fetcher or related checker needs it.
type S3GetBucketLifecycleAPI interface {
	GetBucketLifecycleConfiguration(ctx context.Context, params *s3.GetBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error)
}

// S3GetBucketPolicyStatusAPI defines the interface for the S3
// GetBucketPolicyStatus operation. Used by EnrichS3Posture: AWS evaluates the
// bucket policy itself and reports whether it makes the bucket public, so a9s
// does not have to re-derive that verdict from the raw document.
type S3GetBucketPolicyStatusAPI interface {
	GetBucketPolicyStatus(ctx context.Context, params *s3.GetBucketPolicyStatusInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error)
}

// S3GetBucketVersioningAPI defines the interface for the S3
// GetBucketVersioning operation. Used by EnrichS3Posture for both the
// versioning-off and MFA-delete-off conditions — one call answers both.
type S3GetBucketVersioningAPI interface {
	GetBucketVersioning(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
}

// S3GetObjectLockConfigurationAPI defines the interface for the S3
// GetObjectLockConfiguration operation. Used by EnrichS3Posture.
type S3GetObjectLockConfigurationAPI interface {
	GetObjectLockConfiguration(ctx context.Context, params *s3.GetObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error)
}

// S3HeadBucketAPI defines the interface for the S3 HeadBucket operation. It
// is the only call that answers whether a bucket exists when the bucket is
// not this account's, so it is reached by type assertion from the cf issue
// enricher rather than joined to the S3API aggregate, which every fetcher's
// client has to satisfy.
type S3HeadBucketAPI interface {
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
}

// S3API is the aggregate interface covering all S3 operations used by a9s fetchers.
// *s3.Client structurally satisfies this interface.
type S3API interface {
	S3ListBucketsAPI
	S3ListObjectsV2API
	S3GetBucketNotificationConfigurationAPI
	// Wave 2 enrichment (EnrichS3Posture).
	S3GetPublicAccessBlockAPI
	S3GetBucketPolicyStatusAPI
	S3GetBucketVersioningAPI
	S3GetBucketLoggingAPI
	S3GetBucketLifecycleAPI
	S3GetObjectLockConfigurationAPI
}
