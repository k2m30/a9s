package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/kinesis"
)

// KinesisListStreamsAPI defines the interface for the Kinesis ListStreams operation.
type KinesisListStreamsAPI interface {
	ListStreams(ctx context.Context, params *kinesis.ListStreamsInput, optFns ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error)
}

// KinesisDescribeStreamSummaryAPI defines the interface for DescribeStreamSummary.
type KinesisDescribeStreamSummaryAPI interface {
	DescribeStreamSummary(ctx context.Context, params *kinesis.DescribeStreamSummaryInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamSummaryOutput, error)
}

// KinesisListTagsForStreamAPI defines the interface for ListTagsForStream.
type KinesisListTagsForStreamAPI interface {
	ListTagsForStream(ctx context.Context, params *kinesis.ListTagsForStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.ListTagsForStreamOutput, error)
}

// KinesisAPI is the aggregate interface covering all Kinesis operations used by a9s fetchers.
// *kinesis.Client structurally satisfies this interface.
type KinesisAPI interface {
	KinesisListStreamsAPI
}
