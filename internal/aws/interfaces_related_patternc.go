// interfaces_related_patternc.go declares the narrow AWS API interfaces used by
// round-2 related-panel Pattern C checkers (single-call lookups triggered on
// detail-view open). Adding them here keeps additions isolated from ongoing
// edits to interfaces.go.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

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

// GlueGetSecurityConfigurationAPI defines the interface for GetSecurityConfiguration.
type GlueGetSecurityConfigurationAPI interface {
	GetSecurityConfiguration(ctx context.Context, params *glue.GetSecurityConfigurationInput, optFns ...func(*glue.Options)) (*glue.GetSecurityConfigurationOutput, error)
}

// GlueGetTagsAPI defines the interface for the Glue GetTags operation.
type GlueGetTagsAPI interface {
	GetTags(ctx context.Context, params *glue.GetTagsInput, optFns ...func(*glue.Options)) (*glue.GetTagsOutput, error)
}

// KinesisDescribeStreamSummaryAPI defines the interface for DescribeStreamSummary.
type KinesisDescribeStreamSummaryAPI interface {
	DescribeStreamSummary(ctx context.Context, params *kinesis.DescribeStreamSummaryInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamSummaryOutput, error)
}

// KinesisListTagsForStreamAPI defines the interface for ListTagsForStream.
type KinesisListTagsForStreamAPI interface {
	ListTagsForStream(ctx context.Context, params *kinesis.ListTagsForStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.ListTagsForStreamOutput, error)
}

// MSKListScramSecretsAPI defines the interface for the Kafka ListScramSecrets operation.
type MSKListScramSecretsAPI interface {
	ListScramSecrets(ctx context.Context, params *kafka.ListScramSecretsInput, optFns ...func(*kafka.Options)) (*kafka.ListScramSecretsOutput, error)
}

// OpenSearchDescribeDomainConfigAPI defines the interface for DescribeDomainConfig.
type OpenSearchDescribeDomainConfigAPI interface {
	DescribeDomainConfig(ctx context.Context, params *opensearch.DescribeDomainConfigInput, optFns ...func(*opensearch.Options)) (*opensearch.DescribeDomainConfigOutput, error)
}

// OpenSearchListTagsAPI defines the interface for the OpenSearch ListTags operation.
type OpenSearchListTagsAPI interface {
	ListTags(ctx context.Context, params *opensearch.ListTagsInput, optFns ...func(*opensearch.Options)) (*opensearch.ListTagsOutput, error)
}

// CWLogsDescribeSubscriptionFiltersAPI defines the interface for DescribeSubscriptionFilters.
type CWLogsDescribeSubscriptionFiltersAPI interface {
	DescribeSubscriptionFilters(ctx context.Context, params *cloudwatchlogs.DescribeSubscriptionFiltersInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeSubscriptionFiltersOutput, error)
}
