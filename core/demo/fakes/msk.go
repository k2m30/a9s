// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/kafka"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// DescribeClusterV2 returns the fixture-registered cluster matching the
// requested ClusterArn, backing EnrichMSKCluster's broker-version and
// encryption-in-transit checks (msk.broker-outdated / msk.encryption-not-tls).
func (f *MSKFake) DescribeClusterV2(_ context.Context, input *kafka.DescribeClusterV2Input, _ ...func(*kafka.Options)) (*kafka.DescribeClusterV2Output, error) {
	if input == nil || input.ClusterArn == nil {
		return &kafka.DescribeClusterV2Output{}, nil
	}
	for _, cluster := range f.fix.Clusters {
		if cluster.ClusterArn != nil && *cluster.ClusterArn == *input.ClusterArn {
			c := cluster
			return &kafka.DescribeClusterV2Output{ClusterInfo: &c}, nil
		}
	}
	return &kafka.DescribeClusterV2Output{}, nil
}

// ListScramSecrets returns SCRAM secret ARNs for the given cluster from
// fixture data. Backs the msk:secrets related-panel pivot (checkMSKSecrets).
func (f *MSKFake) ListScramSecrets(_ context.Context, input *kafka.ListScramSecretsInput, _ ...func(*kafka.Options)) (*kafka.ListScramSecretsOutput, error) {
	var clusterArn string
	if input != nil && input.ClusterArn != nil {
		clusterArn = *input.ClusterArn
	}
	return &kafka.ListScramSecretsOutput{SecretArnList: f.fix.ScramSecretsByCluster[clusterArn]}, nil
}
