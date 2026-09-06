// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
)

// MSKPublic names the one demo cluster whose brokers are published to the
// internet (msk.public-access), and MSKUnauthenticated the one cluster
// accepting clients with no credentials (msk.unauthenticated). Every other
// cluster keeps public access off and unauthenticated access disabled.
const (
	MSKPublic          = "msk-public"
	MSKUnauthenticated = "msk-unauthenticated"

	mskPublicARN          = "arn:aws:kafka:us-east-1:123456789012:cluster/" + MSKPublic + "/f1a2b3c4"
	mskUnauthenticatedARN = "arn:aws:kafka:us-east-1:123456789012:cluster/" + MSKUnauthenticated + "/f5a6b7c8"
)

// MSKFixtures holds typed fixture data for MSK (Managed Streaming for Kafka).
type MSKFixtures struct {
	Clusters []kafkatypes.Cluster
	// ScramSecretsByCluster maps cluster ARN to its SCRAM secret ARNs — backs
	// kafka:ListScramSecrets for the msk:secrets related-panel pivot.
	ScramSecretsByCluster map[string][]string
}

func mustParseMSKTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewMSKFixtures constructs MSKFixtures from the canonical demo data.
var sharedMSKFixtures = sync.OnceValue(func() *MSKFixtures {
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
						// ClientSubnets — required for msk:subnet and msk:vpc
						// related-panel pivots. Real cached subnets (ec2.go
						// fixtProdPublicSubnetA/fixtProdPrivateSubnetA).
						ClientSubnets: []string{"subnet-0aaa111111111111a", "subnet-0ccc333333333333c"},
						InstanceType:  aws.String("kafka.m5.large"),
						// SecurityGroups — required for the msk:sg related-panel
						// pivot (checkMSKSG). Reuses the acme-web-alb-sg fixture.
						SecurityGroups: []string{"sg-0aaa111111111111a"},
					},
					NumberOfBrokerNodes: aws.Int32(3),
					// EncryptionInfo — required for the msk:kms related-panel
					// pivot (checkMSKKMS). checkMSKKMS passes the raw field
					// value straight through to FetchByIDs (no ARN-to-bare-ID
					// stripping like the sibling kms checkers), so this must
					// be the bare key ID, not the ARN — matches the primary
					// production KMS key (kms.go).
					EncryptionInfo: &kafkatypes.EncryptionInfo{
						EncryptionAtRest: &kafkatypes.EncryptionAtRest{
							DataVolumeKMSKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
						},
					},
					// LoggingInfo — required for the msk:logs and msk:s3
					// related-panel pivots (checkMSKLogs / checkMSKS3).
					LoggingInfo: &kafkatypes.LoggingInfo{
						BrokerLogs: &kafkatypes.BrokerLogs{
							CloudWatchLogs: &kafkatypes.CloudWatchLogs{
								Enabled:  aws.Bool(true),
								LogGroup: aws.String("/aws/msk/acme-events-prod"),
							},
							S3: &kafkatypes.S3{
								Enabled: aws.Bool(true),
								Bucket:  aws.String(LogsBucketName),
								Prefix:  aws.String("msk-broker-logs/acme-events-prod/"),
							},
						},
					},
				},
				Tags: map[string]string{
					"Environment": "production",
					"Team":        "platform",
					// aws:cloudformation:stack-name — required for the msk:cfn
					// related-panel pivot (checkMSKCFN). Points at acme-vpc-stack
					// (cfn.go).
					"aws:cloudformation:stack-name": "acme-vpc-stack",
				},
			},
			// MSKPublic: the only cluster whose brokers carry their own
			// public addresses. Authentication is required, so it trips
			// msk.public-access alone.
			{
				ClusterName:    aws.String(MSKPublic),
				ClusterArn:     aws.String(mskPublicARN),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateActive,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-07-14T09:00:00+00:00")),
				Provisioned: &kafkatypes.Provisioned{
					NumberOfBrokerNodes:       aws.Int32(3),
					CurrentBrokerSoftwareInfo: &kafkatypes.BrokerSoftwareInfo{KafkaVersion: aws.String("3.6.0")},
					BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
						InstanceType: aws.String("kafka.m5.large"),
						ConnectivityInfo: &kafkatypes.ConnectivityInfo{
							PublicAccess: &kafkatypes.PublicAccess{Type: aws.String("SERVICE_PROVIDED_EIPS")},
						},
					},
					ClientAuthentication: &kafkatypes.ClientAuthentication{
						Sasl:            &kafkatypes.Sasl{Iam: &kafkatypes.Iam{Enabled: aws.Bool(true)}},
						Unauthenticated: &kafkatypes.Unauthenticated{Enabled: aws.Bool(false)},
					},
					EncryptionInfo: &kafkatypes.EncryptionInfo{
						EncryptionInTransit: &kafkatypes.EncryptionInTransit{ClientBroker: kafkatypes.ClientBrokerTls},
					},
				},
			},
			// MSKUnauthenticated: the only cluster accepting clients with no
			// credentials. Public access is off, so it trips
			// msk.unauthenticated alone.
			{
				ClusterName:    aws.String(MSKUnauthenticated),
				ClusterArn:     aws.String(mskUnauthenticatedARN),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateActive,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-08-03T13:20:00+00:00")),
				Provisioned: &kafkatypes.Provisioned{
					NumberOfBrokerNodes:       aws.Int32(3),
					CurrentBrokerSoftwareInfo: &kafkatypes.BrokerSoftwareInfo{KafkaVersion: aws.String("3.6.0")},
					BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
						InstanceType: aws.String("kafka.m5.large"),
						ConnectivityInfo: &kafkatypes.ConnectivityInfo{
							PublicAccess: &kafkatypes.PublicAccess{Type: aws.String("DISABLED")},
						},
					},
					ClientAuthentication: &kafkatypes.ClientAuthentication{
						Unauthenticated: &kafkatypes.Unauthenticated{Enabled: aws.Bool(true)},
					},
					EncryptionInfo: &kafkatypes.EncryptionInfo{
						EncryptionInTransit: &kafkatypes.EncryptionInTransit{ClientBroker: kafkatypes.ClientBrokerTls},
					},
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
			// Issue: State=FAILED → Broken (cluster in unrecoverable failure state).
			// EncryptionInTransit.ClientBroker=TLS_PLAINTEXT witnesses
			// msk.encryption-not-tls (EnrichMSKCluster: ClientBroker != TLS).
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
					EncryptionInfo: &kafkatypes.EncryptionInfo{
						EncryptionInTransit: &kafkatypes.EncryptionInTransit{
							ClientBroker: kafkatypes.ClientBrokerTlsPlaintext,
						},
					},
				},
				Tags: map[string]string{
					"Environment": "prod",
				},
			},
			// Issue: State=REBOOTING_BROKER → Warning (broker maintenance in progress).
			// CurrentBrokerSoftwareInfo.KafkaVersion=2.6.2 witnesses
			// msk.broker-outdated (EnrichMSKCluster: version below 2.8 cutoff).
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
					CurrentBrokerSoftwareInfo: &kafkatypes.BrokerSoftwareInfo{
						KafkaVersion: aws.String("2.6.2"),
					},
				},
				Tags: map[string]string{
					"Environment": "prod",
					"Team":        "data",
				},
			},
			// Issue: State=DELETING → Warning (cluster teardown in progress).
			{
				ClusterName:    aws.String("msk-deleting"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/msk-deleting/f1a2b3c4"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateDeleting,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-05-02T09:15:00+00:00")),
				Provisioned: &kafkatypes.Provisioned{
					BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
						ClientSubnets: []string{"subnet-0a1b2c3d4e5f60001"},
						InstanceType:  aws.String("kafka.m5.large"),
					},
					NumberOfBrokerNodes: aws.Int32(3),
				},
				Tags: map[string]string{
					"Environment": "staging",
				},
			},
			// Issue: State=HEALING → Warning (MSK auto-remediating a degraded broker).
			{
				ClusterName:    aws.String("msk-healing"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/msk-healing/a2b3c4d5"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateHealing,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-08-11T13:45:00+00:00")),
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
			// Issue: State=MAINTENANCE → Warning (scheduled maintenance window active).
			{
				ClusterName:    aws.String("msk-maintenance"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/msk-maintenance/b3c4d5e6"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateMaintenance,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-10-01T02:00:00+00:00")),
				Provisioned: &kafkatypes.Provisioned{
					BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
						ClientSubnets: []string{"subnet-0a1b2c3d4e5f60001", "subnet-0a1b2c3d4e5f60002"},
						InstanceType:  aws.String("kafka.m5.large"),
					},
					NumberOfBrokerNodes: aws.Int32(3),
				},
				Tags: map[string]string{
					"Environment": "prod",
				},
			},
			// Issue: State=UPDATING → Warning (cluster config/version update in progress).
			{
				ClusterName:    aws.String("msk-updating"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/msk-updating/c4d5e6f7"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateUpdating,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseMSKTime("2025-11-18T20:30:00+00:00")),
				Provisioned: &kafkatypes.Provisioned{
					BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
						ClientSubnets: []string{"subnet-0a1b2c3d4e5f60001"},
						InstanceType:  aws.String("kafka.m5.large"),
					},
					NumberOfBrokerNodes: aws.Int32(3),
				},
				Tags: map[string]string{
					"Environment": "staging",
					"Team":        "platform",
				},
			},
		},
		// ScramSecretsByCluster — required for the msk:secrets related-panel
		// pivot (checkMSKSecrets → kafka:ListScramSecrets). Points at the
		// prod/database/primary secret (secrets.go).
		ScramSecretsByCluster: map[string][]string{
			"arn:aws:kafka:us-east-1:123456789012:cluster/acme-events-prod/a1b2c3d4": {
				"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf",
			},
		},
	}
})

func NewMSKFixtures() *MSKFixtures {
	return sharedMSKFixtures()
}
