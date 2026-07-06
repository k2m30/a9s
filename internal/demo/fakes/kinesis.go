package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// KinesisFake implements aws.KinesisAPI against fixture data loaded at construction time.
type KinesisFake struct {
	fix *fixtures.KinesisFixtures
}

// NewKinesis constructs a KinesisFake backed by fixture data from the fixtures package.
func NewKinesis() *KinesisFake {
	return &KinesisFake{fix: fixtures.NewKinesisFixtures()}
}

func (f *KinesisFake) ListStreams(_ context.Context, _ *kinesis.ListStreamsInput, _ ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error) {
	return &kinesis.ListStreamsOutput{StreamSummaries: f.fix.Streams}, nil
}

// ListTagsForStream returns tags for the named stream from fixture data.
// Backs the kinesis:cfn related-panel pivot (checkKinesisCFN).
func (f *KinesisFake) ListTagsForStream(_ context.Context, input *kinesis.ListTagsForStreamInput, _ ...func(*kinesis.Options)) (*kinesis.ListTagsForStreamOutput, error) {
	var name string
	if input != nil && input.StreamName != nil {
		name = *input.StreamName
	}
	return &kinesis.ListTagsForStreamOutput{Tags: f.fix.TagsByStream[name]}, nil
}

// DescribeStreamSummary returns a minimal stream summary carrying the KMS
// KeyId from fixture data. Backs the kinesis:kms related-panel pivot
// (checkKinesisKMS).
func (f *KinesisFake) DescribeStreamSummary(_ context.Context, input *kinesis.DescribeStreamSummaryInput, _ ...func(*kinesis.Options)) (*kinesis.DescribeStreamSummaryOutput, error) {
	var name string
	if input != nil && input.StreamName != nil {
		name = *input.StreamName
	}
	var keyID *string
	if k, ok := f.fix.KeyIDByStream[name]; ok {
		keyID = aws.String(k)
	}
	for _, s := range f.fix.Streams {
		if aws.ToString(s.StreamName) == name {
			return &kinesis.DescribeStreamSummaryOutput{
				StreamDescriptionSummary: &kinesistypes.StreamDescriptionSummary{
					StreamName:            s.StreamName,
					StreamARN:             s.StreamARN,
					StreamStatus:          s.StreamStatus,
					StreamModeDetails:     s.StreamModeDetails,
					KeyId:                 keyID,
					RetentionPeriodHours:  aws.Int32(24),
					OpenShardCount:        aws.Int32(1),
					EnhancedMonitoring:    []kinesistypes.EnhancedMetrics{},
					StreamCreationTimestamp: s.StreamCreationTimestamp,
				},
			}, nil
		}
	}
	return &kinesis.DescribeStreamSummaryOutput{}, nil
}
