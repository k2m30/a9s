package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
)

// MSKFixtures holds typed fixture data for MSK (Managed Streaming for Kafka).
type MSKFixtures struct {
	Clusters []kafkatypes.Cluster
}

func mustParseMSKTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewMSKFixtures constructs MSKFixtures from the canonical demo data.
func NewMSKFixtures() *MSKFixtures {
	return &MSKFixtures{
		Clusters: []kafkatypes.Cluster{
			{
				ClusterName:    aws.String("acme-events-prod"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/acme-events-prod/a1b2c3d4"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateActive,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-04-10T14:00:00+00:00")),
				Provisioned: &kafkatypes.Provisioned{
					BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
						ClientSubnets: []string{"subnet-0a1b2c3d4e5f60001", "subnet-0a1b2c3d4e5f60002"},
						InstanceType:  aws.String("kafka.m5.large"),
					},
					NumberOfBrokerNodes: aws.Int32(3),
				},
				Tags: map[string]string{
					"Environment": "production",
					"Team":        "platform",
				},
			},
			{
				ClusterName:    aws.String("data-pipeline-kafka"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/data-pipeline-kafka/e5f6a7b8"),
				ClusterType:    kafkatypes.ClusterTypeServerless,
				State:          kafkatypes.ClusterStateActive,
				CurrentVersion: aws.String("K7BFGT2P"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-09-20T11:30:00+00:00")),
			},
			{
				ClusterName:    aws.String("staging-events"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/staging-events/c9d0e1f2"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateCreating,
				CurrentVersion: aws.String("K1INITIAL"),
				CreationTime:   aws.Time(mustParseMSKTime("2026-03-20T16:00:00+00:00")),
			},
			// Issue: State=FAILED → Broken (cluster in unrecoverable failure state)
			{
				ClusterName:    aws.String("msk-failed"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/msk-failed/d1e2f3a4"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateFailed,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-12-01T10:00:00+00:00")),
				Provisioned: &kafkatypes.Provisioned{
					BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
						ClientSubnets: []string{"subnet-0a1b2c3d4e5f60001"},
						InstanceType:  aws.String("kafka.m5.large"),
					},
					NumberOfBrokerNodes: aws.Int32(3),
				},
				Tags: map[string]string{
					"Environment": "prod",
				},
			},
			// Issue: State=REBOOTING_BROKER → Warning (broker maintenance in progress)
			{
				ClusterName:    aws.String("msk-rebooting"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/msk-rebooting/b5c6d7e8"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateRebootingBroker,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-07-15T14:30:00+00:00")),
				Provisioned: &kafkatypes.Provisioned{
					BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
						ClientSubnets: []string{"subnet-0a1b2c3d4e5f60001", "subnet-0a1b2c3d4e5f60002"},
						InstanceType:  aws.String("kafka.m5.large"),
					},
					NumberOfBrokerNodes: aws.Int32(3),
				},
				Tags: map[string]string{
					"Environment": "prod",
					"Team":        "data",
				},
			},
		},
	}
}
