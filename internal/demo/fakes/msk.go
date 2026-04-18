package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/kafka"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// MSKFake implements aws.MSKAPI against fixture data loaded at construction time.
type MSKFake struct {
	fix *fixtures.MSKFixtures
}

// NewMSK constructs an MSKFake backed by fixture data from the fixtures package.
func NewMSK() *MSKFake {
	return &MSKFake{fix: fixtures.NewMSKFixtures()}
}

func (f *MSKFake) ListClustersV2(_ context.Context, _ *kafka.ListClustersV2Input, _ ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error) {
	return &kafka.ListClustersV2Output{ClusterInfoList: f.fix.Clusters}, nil
}

// DescribeClusterV2 is a no-op stub — the demo transport does not exercise Wave 2 enrichment.
func (f *MSKFake) DescribeClusterV2(_ context.Context, _ *kafka.DescribeClusterV2Input, _ ...func(*kafka.Options)) (*kafka.DescribeClusterV2Output, error) {
	return &kafka.DescribeClusterV2Output{}, nil
}
