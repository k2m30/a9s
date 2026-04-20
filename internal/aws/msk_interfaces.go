package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/kafka"
)

// MSKListClustersV2API defines the interface for the Kafka ListClustersV2 operation.
type MSKListClustersV2API interface {
	ListClustersV2(ctx context.Context, params *kafka.ListClustersV2Input, optFns ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error)
}

// KafkaDescribeClusterV2API defines the interface for the Kafka DescribeClusterV2 operation.
type KafkaDescribeClusterV2API interface {
	DescribeClusterV2(ctx context.Context, params *kafka.DescribeClusterV2Input, optFns ...func(*kafka.Options)) (*kafka.DescribeClusterV2Output, error)
}

// MSKListScramSecretsAPI defines the interface for the Kafka ListScramSecrets operation.
type MSKListScramSecretsAPI interface {
	ListScramSecrets(ctx context.Context, params *kafka.ListScramSecretsInput, optFns ...func(*kafka.Options)) (*kafka.ListScramSecretsOutput, error)
}

// MSKAPI is the aggregate interface covering all MSK operations used by a9s fetchers.
// *kafka.Client structurally satisfies this interface.
type MSKAPI interface {
	MSKListClustersV2API
	KafkaDescribeClusterV2API
}
