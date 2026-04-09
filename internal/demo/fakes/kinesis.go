package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/kinesis"

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
