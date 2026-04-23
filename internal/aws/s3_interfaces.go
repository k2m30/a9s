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
// Used by EnrichS3PublicAccessBlock to check per-bucket PAB configuration.
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

// S3API is the aggregate interface covering all S3 operations used by a9s fetchers.
// *s3.Client structurally satisfies this interface.
type S3API interface {
	S3ListBucketsAPI
	S3ListObjectsV2API
	S3GetBucketNotificationConfigurationAPI
	S3GetPublicAccessBlockAPI // Wave 2 enrichment
}
